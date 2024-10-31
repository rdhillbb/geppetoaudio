package main

import (
	"bufio"
	"container/ring"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Configuration types
type ClientConfig struct {
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PingInterval    time.Duration
	MaxRetries      int
	BufferSize      int
	ShutdownTimeout time.Duration
}

func DefaultConfig() ClientConfig {
	return ClientConfig{
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    10 * time.Second,
		PingInterval:    30 * time.Second,
		MaxRetries:      3,
		BufferSize:      100,
		ShutdownTimeout: 5 * time.Second,
	}
}

// Message types for WebSocket communication
type SessionUpdate struct {
	Type    string  `json:"type"`
	Session Session `json:"session"`
}

type Session struct {
	Modalities              []string `json:"modalities"`
	Instructions            string   `json:"instructions"`
	Temperature             float64  `json:"temperature"`
	MaxResponseOutputTokens int      `json:"max_response_output_tokens"`
}

type ConversationItem struct {
	Type string `json:"type"`
	Item struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"item"`
}

type ResponseCreate struct {
	Type string `json:"type"`
}

type ResponseMessage struct {
	Type     string   `json:"type"`
	Response Response `json:"response"`
}

type Response struct {
	Output []OutputItem `json:"output"`
	Status string       `json:"status"`
}

type OutputItem struct {
	Content []ContentItem `json:"content"`
	Role    string        `json:"role"`
	Status  string        `json:"status"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ChatMessage struct {
	Role    string
	Content string
}

// Metrics tracking
type Metrics struct {
	messagesSent     int64
	messagesReceived int64
	errors           int64
	latencies        []time.Duration
	mu               sync.Mutex
}

func (m *Metrics) recordLatency(start time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencies = append(m.latencies, time.Since(start))
}

func (m *Metrics) recordError() {
	atomic.AddInt64(&m.errors, 1)
}

// Logger implementation
type Logger struct {
	file    *os.File
	mu      sync.Mutex
	encoder *json.Encoder
}

type LogEntry struct {
	Timestamp string      `json:"timestamp"`
	Direction string      `json:"direction"`
	Type      string      `json:"type"`
	RawJSON   interface{} `json:"raw_json"`
}

// ChatClient structure
type ChatClient struct {
	conn           *websocket.Conn
	messageChannel chan string
	displayChannel chan ChatMessage
	done           chan struct{}
	shutdownOnce   sync.Once
	wg             sync.WaitGroup
	logger         *Logger
	config         ClientConfig
	metrics        *Metrics
	messageBuffer  *ring.Ring
	bufferMutex    sync.Mutex
}

func NewLogger() (*Logger, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	logDir := filepath.Join(exeDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(logDir, fmt.Sprintf("Chat:%s.log", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	log.Printf("Logging to: %s", filename)
	return &Logger{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

func (l *Logger) Log(direction, msgType string, content interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Direction: direction,
		Type:      msgType,
		RawJSON:   content,
	}

	if err := l.encoder.Encode(entry); err != nil {
		log.Printf("Error writing to log: %v", err)
	}
	l.file.Sync()
}

func (l *Logger) Close() error {
	return l.file.Close()
}

func NewChatClient(conn *websocket.Conn, config ClientConfig) (*ChatClient, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	// Setup ping handler
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	client := &ChatClient{
		conn:           conn,
		messageChannel: make(chan string, 1),
		displayChannel: make(chan ChatMessage),
		done:           make(chan struct{}),
		logger:         logger,
		config:         config,
		metrics:        &Metrics{},
		messageBuffer:  ring.New(config.BufferSize),
	}

	// Start ping routine
	go client.pingRoutine()

	return client, nil
}
func (c *ChatClient) pingRoutine() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(c.config.WriteTimeout)); err != nil {
				log.Printf("Ping error: %v", err)
				c.metrics.recordError()
			}
		}
	}
}

func (c *ChatClient) bufferMessage(msg string) {
	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()

	c.messageBuffer = c.messageBuffer.Next()
	c.messageBuffer.Value = msg
}

func isExitCommand(text string) bool {
	exitCommands := []string{
		".exit", ".quit",
		"exit", "quit",
		":exit", ":quit",
		"/exit", "/quit",
	}

	lowercaseText := strings.ToLower(strings.TrimSpace(text))
	for _, cmd := range exitCommands {
		if lowercaseText == cmd {
			return true
		}
	}
	return false
}

func (c *ChatClient) shutdown() {
	c.shutdownOnce.Do(func() {
		log.Println("Starting graceful shutdown...")

		// Signal shutdown
		close(c.done)

		// Setup shutdown deadline
		shutdownCtx, cancel := context.WithTimeout(context.Background(), c.config.ShutdownTimeout)
		defer cancel()

		// Channel to signal completion
		complete := make(chan struct{})

		go func() {
			// Close websocket
			c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second),
			)
			c.conn.Close()

			// Close logger
			if err := c.logger.Close(); err != nil {
				log.Printf("Error closing logger: %v", err)
			}

			// Wait for goroutines
			c.wg.Wait()

			// Close channels
			close(c.messageChannel)
			close(c.displayChannel)

			close(complete)
		}()

		// Wait for completion or timeout
		select {
		case <-complete:
			log.Println("Shutdown completed successfully")
		case <-shutdownCtx.Done():
			log.Println("Shutdown timed out")
		}
	})
}

func (c *ChatClient) displayRoutine() {
	defer c.wg.Done()

	for {
		select {
		case <-c.done:
			return
		case msg, ok := <-c.displayChannel:
			if !ok {
				return
			}
			if msg.Role == "assistant" {
				fmt.Printf("Assistant: %s\n\n", msg.Content)
				fmt.Print("You: ")
			}
		}
	}
}

func (c *ChatClient) inputRoutine() {
	defer c.wg.Done()
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		select {
		case <-c.done:
			return
		default:
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}

			if isExitCommand(text) {
				fmt.Println("\nGoodbye! Thanks for chatting.")
				c.shutdown()
				return
			}

			c.bufferMessage(text) // Buffer the message

			select {
			case c.messageChannel <- text:
			case <-c.done:
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
		c.shutdown()
	}
}

func (c *ChatClient) sendWithRetry(msg interface{}) error {
	var lastErr error
	for i := 0; i < c.config.MaxRetries; i++ {
		c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
		if err := c.conn.WriteJSON(msg); err != nil {
			lastErr = err
			c.metrics.recordError()
			time.Sleep(time.Second * time.Duration(i+1)) // Exponential backoff
			continue
		}
		atomic.AddInt64(&c.metrics.messagesSent, 1)
		return nil
	}
	return fmt.Errorf("failed after %d retries: %v", c.config.MaxRetries, lastErr)
}

func (c *ChatClient) sendRoutine() {
	defer c.wg.Done()

	for {
		select {
		case <-c.done:
			return
		case text, ok := <-c.messageChannel:
			if !ok {
				return
			}

			start := time.Now()
			if err := c.sendUserMessage(text); err != nil {
				log.Printf("Error sending message: %v", err)
				c.metrics.recordError()
				c.shutdown()
				return
			}
			c.metrics.recordLatency(start)
		}
	}
}

func (c *ChatClient) receiveRoutine() {
	defer c.wg.Done()

	for {
		select {
		case <-c.done:
			return
		default:
			c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return
				}

				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				log.Printf("Read error: %v", err)
				c.metrics.recordError()
				c.shutdown()
				return
			}

			atomic.AddInt64(&c.metrics.messagesReceived, 1)

			var rawJSON interface{}
			if err := json.Unmarshal(message, &rawJSON); err == nil {
				c.logger.Log("received", "raw", rawJSON)
			}

			var resp ResponseMessage
			if err := json.Unmarshal(message, &resp); err != nil {
				continue
			}

			if resp.Type == "response.done" && resp.Response.Status == "completed" {
				for _, output := range resp.Response.Output {
					for _, content := range output.Content {
						if content.Type == "text" {
							select {
							case c.displayChannel <- ChatMessage{Role: "assistant", Content: content.Text}:
							case <-c.done:
								return
							}
						}
					}
				}
			}
		}
	}
}

func (c *ChatClient) sendUserMessage(text string) error {
	conversationItem := ConversationItem{
		Type: "conversation.item.create",
		Item: struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}{
			Type: "message",
			Role: "user",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{
					Type: "input_text",
					Text: text,
				},
			},
		},
	}

	c.logger.Log("sent", "conversation.item.create", conversationItem)

	if err := c.sendWithRetry(conversationItem); err != nil {
		return fmt.Errorf("write conversation item: %w", err)
	}
	time.Sleep(time.Second)

	responseCreate := ResponseCreate{
		Type: "response.create",
	}

	c.logger.Log("sent", "response.create", responseCreate)

	if err := c.sendWithRetry(responseCreate); err != nil {
		return fmt.Errorf("write response create: %w", err)
	}

	return nil
}

func (c *ChatClient) Start() {
	defer c.shutdown()

	sessionUpdate := SessionUpdate{
		Type: "session.update",
		Session: Session{
			Modalities:              []string{"text"},
			Temperature:             0.8,
			MaxResponseOutputTokens: 4096,
			Instructions:            "System settings:\nInstructions:\n- You are an artificial intelligence agent\n- Be kind, helpful, and curteous\n- It is okay to ask the user questions\n- Be open to exploration and conversation\n- Remember: this is just for fun and testing!\n\nPersonality:\n- Be upbeat and genuine\n",
		},
	}

	c.logger.Log("sent", "session.update", sessionUpdate)

	if err := c.sendWithRetry(sessionUpdate); err != nil {
		log.Fatal("write session update:", err)
	}

	c.wg.Add(4)
	fmt.Print("You: ")
	go c.displayRoutine()
	go c.inputRoutine()
	go c.sendRoutine()
	go c.receiveRoutine()

	c.wg.Wait()
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set")
	}

	// Get configuration
	config := DefaultConfig()

	header := make(map[string][]string)
	header["Authorization"] = []string{"Bearer " + apiKey}
	header["OpenAI-Beta"] = []string{"realtime=v1"}

	// Create dialer with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Set up context with timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := "wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01"
	conn, _, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		log.Fatal("dial:", err)
	}

	client, err := NewChatClient(conn, config)
	if err != nil {
		log.Fatal("create chat client:", err)
	}

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal. Shutting down...")
		client.shutdown()
	}()

	client.Start()
}
