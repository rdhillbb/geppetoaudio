package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"geppetoaudio/audiotypes"
	"github.com/gorilla/websocket"
)

// Local wrapper types
type ChatClient struct {
	*audiotypes.ChatClient
}

type Logger struct {
	*audiotypes.Logger
}

//NEW

func DefaultConfig() audiotypes.ClientConfig {
	return audiotypes.ClientConfig{
		ReadTimeout:     90 * time.Second, // Increased from 30
		WriteTimeout:    30 * time.Second, // Increased from 10
		PingInterval:    20 * time.Second, // Decreased from 30
		MaxRetries:      3,
		BufferSize:      100,
		ShutdownTimeout: 5 * time.Second,
		AudioOutputDir:  "audio_output",
	}
}

//OLD

func (c *ChatClient) saveTranscript(filepath string, transcript string) error {
	if transcript == "" {
		log.Printf("Warning: Empty transcript received")
		transcript = "No transcript available"
	}

	textPath := strings.TrimSuffix(filepath, ".wav") + ".txt"

	// Format the transcript with timestamp and more information
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedTranscript := fmt.Sprintf("Generated: %s\nAudio File: %s\nTranscript:\n%s\n",
		timestamp,
		filepath,
		transcript)

	log.Printf("Writing transcript to file: %s\nContent length: %d bytes",
		textPath,
		len(formattedTranscript))

	// Write transcript to file
	if err := os.WriteFile(textPath, []byte(formattedTranscript), 0644); err != nil {
		return fmt.Errorf("write transcript file: %w", err)
	}

	// Verify file was written
	if info, err := os.Stat(textPath); err != nil {
		log.Printf("Error verifying transcript file: %v", err)
	} else {
		log.Printf("Transcript file written successfully, size: %d bytes", info.Size())
	}

	return nil
}

// New Logger
func NewLogger() (*audiotypes.Logger, error) {
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
	return &audiotypes.Logger{
		File:    file,
		Encoder: json.NewEncoder(file),
	}, nil
}

func (l *Logger) Log(direction, msgType string, content interface{}) {
	l.Mu.Lock()
	defer l.Mu.Unlock()

	entry := audiotypes.LogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Direction: direction,
		Type:      msgType,
		RawJSON:   content,
	}

	if err := l.Encoder.Encode(entry); err != nil {
		log.Printf("Error writing to log: %v", err)
	}
	l.File.Sync()
}

func (l *Logger) Close() error {
	return l.File.Close()
}

// NewChatClient

func NewChatClient(conn *websocket.Conn, config audiotypes.ClientConfig) (*ChatClient, error) {
	logger, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	// Ensure audio directory exists
	audioDir := filepath.Join(config.AudioOutputDir)
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		return nil, fmt.Errorf("create audio directory: %w", err)
	}
	log.Printf("Audio directory initialized: %s", audioDir)

	// Setup ping handler
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	baseClient := &audiotypes.ChatClient{
		Conn:           conn,
		MessageChannel: make(chan string, 1),
		DisplayChannel: make(chan audiotypes.ChatMessage),
		AudioChannel:   make(chan audiotypes.AudioChunk, 100),
		Done:           make(chan struct{}),
		Logger:         logger, // Use logger directly, not logger.Logger
		Config:         config,
		Metrics:        &audiotypes.Metrics{},
		AudioBuffer:    make(map[string]*audiotypes.AudioMessage),
		WG:             sync.WaitGroup{},
		ShutdownOnce:   sync.Once{},
		AudioMutex:     sync.Mutex{},
	}

	client := &ChatClient{
		ChatClient: baseClient,
	}

	// Start audio processing routine
	client.WG.Add(1)
	go client.audioProcessingRoutine()

	log.Printf("Chat client initialized with audio processing")
	return client, nil
}

func (c *ChatClient) audioProcessingRoutine() {
	defer c.WG.Done()
	log.Printf("Starting audio processing routine")

	for {
		select {
		case <-c.Done:
			log.Printf("Audio processing routine shutting down")
			return
		case chunk, ok := <-c.AudioChannel:
			if !ok {
				log.Printf("Audio channel closed")
				return
			}
			c.handleAudioChunk(chunk)
		}
	}
}

func (c *ChatClient) handleAudioChunk(chunk audiotypes.AudioChunk) {
	audioKey := fmt.Sprintf("%s_%s", chunk.ResponseID, chunk.ItemID)
	log.Printf("Processing audio chunk for key: %s", audioKey)

	c.AudioMutex.Lock()
	defer c.AudioMutex.Unlock()

	if c.AudioBuffer[audioKey] == nil {
		c.AudioBuffer[audioKey] = &audiotypes.AudioMessage{
			AudioData: make([]byte, 0, 1024*1024), // 1MB initial capacity
		}
		log.Printf("Created new audio buffer for key: %s", audioKey)
	}

	c.AudioBuffer[audioKey].AudioData = append(c.AudioBuffer[audioKey].AudioData, chunk.Data...)
	c.Metrics.RecordAudioChunk()
	log.Printf("Audio chunk processed, buffer size: %d bytes", len(c.AudioBuffer[audioKey].AudioData))
}

func (c *ChatClient) handleAudioResponse(message []byte) error {
	var audioMsg struct {
		Type         string `json:"type"`
		ResponseID   string `json:"response_id"`
		ItemID       string `json:"item_id"`
		OutputIndex  int    `json:"output_index"`
		ContentIndex int    `json:"content_index"`
		Delta        string `json:"delta"`
	}

	if err := json.Unmarshal(message, &audioMsg); err != nil {
		return fmt.Errorf("unmarshal audio message: %w", err)
	}

	data := audioMsg.Delta
	var processedData []byte

	if strings.HasPrefix(data, "[trimmed: ") && strings.HasSuffix(data, " bytes]") {
		data = strings.TrimPrefix(data, "[trimmed: ")
		data = strings.TrimSuffix(data, " bytes]")
		size, err := strconv.Atoi(data)
		if err != nil {
			return fmt.Errorf("parse audio size: %w", err)
		}
		processedData = make([]byte, size)
	} else {
		var err error
		processedData, err = base64.StdEncoding.DecodeString(audioMsg.Delta)
		if err != nil {
			return fmt.Errorf("decode audio data: %w", err)
		}
	}

	chunk := audiotypes.AudioChunk{
		ResponseID:   audioMsg.ResponseID,
		ItemID:       audioMsg.ItemID,
		OutputIndex:  audioMsg.OutputIndex,
		ContentIndex: audioMsg.ContentIndex,
		Data:         processedData,
	}

	select {
	case c.AudioChannel <- chunk:
		log.Printf("Sent audio chunk to processing channel")
	case <-c.Done:
		return fmt.Errorf("client shutdown while processing audio")
	}

	return nil
}

func (c *ChatClient) saveAudioOnly(responseID, itemID string, filepath string) error {
	audioKey := fmt.Sprintf("%s_%s", responseID, itemID)

	c.AudioMutex.Lock()
	audio, exists := c.AudioBuffer[audioKey]
	if !exists || audio == nil {
		c.AudioMutex.Unlock()
		return fmt.Errorf("no audio data found for key: %s", audioKey)
	}
	audioData := audio.AudioData
	c.AudioMutex.Unlock()

	if len(audioData) == 0 {
		return fmt.Errorf("empty audio data for key: %s", audioKey)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("create audio file: %w", err)
	}
	defer file.Close()

	if err := c.writeWAVHeader(file, uint32(len(audioData))); err != nil {
		return fmt.Errorf("write WAV header: %w", err)
	}

	if _, err := file.Write(audioData); err != nil {
		return fmt.Errorf("write audio data: %w", err)
	}

	// Clean up the buffer
	c.AudioMutex.Lock()
	delete(c.AudioBuffer, audioKey)
	c.AudioMutex.Unlock()

	return nil
}

func (c *ChatClient) writeWAVHeader(file io.Writer, dataSize uint32) error {
	header := []interface{}{
		[4]byte{'R', 'I', 'F', 'F'},
		uint32(dataSize + 36),
		[4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '},
		uint32(16),    // Size of fmt chunk
		uint16(1),     // Audio format (PCM)
		uint16(1),     // Number of channels (mono)
		uint32(24000), // Sample rate
		uint32(48000), // Byte rate
		uint16(2),     // Block align
		uint16(16),    // Bits per sample
		[4]byte{'d', 'a', 't', 'a'},
		dataSize,
	}

	for _, v := range header {
		if err := binary.Write(file, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	return nil
}

// NEW KEEP ALIVE
func (c *ChatClient) keepAliveRoutine() {
	ticker := time.NewTicker(c.Config.PingInterval)
	defer ticker.Stop()
	defer c.WG.Done()

	for {
		select {
		case <-ticker.C:
			if err := c.Conn.WriteControl(
				websocket.PingMessage,
				[]byte{},
				time.Now().Add(c.Config.WriteTimeout),
			); err != nil {
				log.Printf("Error sending ping: %v", err)
				return
			}
		case <-c.Done:
			return
		}
	}
}

// NEW
func (c *ChatClient) receiveRoutine() {
	defer c.WG.Done()
	var audioFiles = make(map[string]string) // Map to store audio file paths by responseID_itemID

	for {
		select {
		case <-c.Done:
			return
		default:
			c.Conn.SetReadDeadline(time.Now().Add(c.Config.ReadTimeout))
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Read error: %v", err)
				c.Metrics.RecordError()
				if websocket.IsUnexpectedCloseError(err) {
					c.shutdown()
					return
				}
				continue
			}

			// Reset read deadline after successful read
			c.Conn.SetReadDeadline(time.Time{})

			var baseMessage struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(message, &baseMessage); err != nil {
				continue
			}

			// Log raw message
			var rawJSON interface{}
			if err := json.Unmarshal(message, &rawJSON); err == nil {
				c.Logger.Log("received", baseMessage.Type, rawJSON)
			}

			switch baseMessage.Type {
			case "response.audio.delta":
				if err := c.handleAudioResponse(message); err != nil {
					log.Printf("Error handling audio response: %v", err)
				}

			case "response.audio.done":
				var doneMsg struct {
					ResponseID string `json:"response_id"`
					ItemID     string `json:"item_id"`
				}
				if err := json.Unmarshal(message, &doneMsg); err != nil {
					log.Printf("Error unmarshaling audio done message: %v", err)
					continue
				}

				// Save audio file without transcript
				timestamp := time.Now().Format("20060102_150405")
				filename := fmt.Sprintf("audio_%s.wav", timestamp)
				filepath := filepath.Join(c.Config.AudioOutputDir, filename)

				if err := c.saveAudioOnly(doneMsg.ResponseID, doneMsg.ItemID, filepath); err != nil {
					log.Printf("Error saving audio: %v", err)
				}

				// Store the filepath for later transcript writing
				audioKey := fmt.Sprintf("%s_%s", doneMsg.ResponseID, doneMsg.ItemID)
				audioFiles[audioKey] = filepath

			case "response.done":
				var respDone audiotypes.CompleteResponse
				if err := json.Unmarshal(message, &respDone); err != nil {
					log.Printf("Error unmarshaling response done message: %v", err)
					continue
				}

				// Process the response
				for _, output := range respDone.Response.Output {
					for _, content := range output.Content {
						if content.Type == "audio" && content.Transcript != "" {
							// Get the audio file path using response ID and item ID
							audioKey := fmt.Sprintf("%s_%s", respDone.Response.ID, output.ID)
							if audioPath, exists := audioFiles[audioKey]; exists {
								// Write the transcript
								if err := c.saveTranscript(audioPath, content.Transcript); err != nil {
									log.Printf("Error saving transcript: %v", err)
								}
								delete(audioFiles, audioKey) // Cleanup
							}
							fmt.Printf("\nAssistant: %s\n", content.Transcript)
							fmt.Print("You: ")
						}
					}
				}
			}
		}
	}
}

func (c *ChatClient) shutdown() {
	c.ShutdownOnce.Do(func() {
		log.Println("Starting graceful shutdown...")
		close(c.Done)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), c.Config.ShutdownTimeout)
		defer cancel()

		complete := make(chan struct{})

		go func() {
			c.Conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second),
			)
			c.Conn.Close()

			if err := c.Logger.Close(); err != nil {
				log.Printf("Error closing logger: %v", err)
			}

			c.WG.Wait()

			close(c.MessageChannel)
			close(c.DisplayChannel)
			close(c.AudioChannel)

			close(complete)
		}()

		select {
		case <-complete:
			log.Println("Shutdown completed successfully")
		case <-shutdownCtx.Done():
			log.Println("Shutdown timed out")
		}
	})
}

//Start

func (c *ChatClient) Start(sessionUpdate audiotypes.SessionUpdate) error {
	defer c.shutdown()

	c.Logger.Log("sent", "session.update", sessionUpdate)
	if err := c.Conn.WriteJSON(sessionUpdate); err != nil {
		return fmt.Errorf("write session update: %w", err)
	}

	c.WG.Add(1)
	go c.receiveRoutine()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nYou: ")

	for {
		// Read entire line including spaces
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v", err)
			break
		}

		// Trim spaces and newline characters
		input = strings.TrimSpace(input)

		if input == ".quit" || input == ".exit" {
			break
		}

		if input != "" {
			if err := c.sendUserMessage(input); err != nil {
				log.Printf("Error sending message: %v", err)
				break
			}
			fmt.Print("You: ")
		}
	}

	return nil
}

//OLD

func (c *ChatClient) sendUserMessage(text string) error {
	fmt.Println("SENDING", text)
	msg := audiotypes.ConversationItem{
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

	c.Logger.Log("sent", "conversation.item.create", msg)
	if err := c.Conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	responseCreate := audiotypes.ResponseCreate{Type: "response.create"}
	c.Logger.Log("sent", "response.create", responseCreate)
	if err := c.Conn.WriteJSON(responseCreate); err != nil {
		return fmt.Errorf("write response create: %w", err)
	}

	return nil
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set")
	}

	config := DefaultConfig()

	header := make(map[string][]string)
	header["Authorization"] = []string{"Bearer " + apiKey}
	header["OpenAI-Beta"] = []string{"realtime=v1"}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal. Shutting down...")
		client.shutdown()
	}()

	sessionUpdate := audiotypes.SessionUpdate{
		Type: "session.update",
		Session: audiotypes.Session{
			Modalities:              []string{"text", "audio"},
			Temperature:             0.8,
			MaxResponseOutputTokens: 4096,
			Voice:                   "alloy",
			InputAudioFormat:        "pcm16",
			OutputAudioFormat:       "pcm16",
			Instructions: "System settings:\nInstructions:\n" +
				"- You are an artificial intelligence agent\n" +
				"- Be kind, helpful, and curteous\n" +
				"- It is okay to ask the user questions\n" +
				"- Be open to exploration and conversation\n" +
				"- Remember: this is just for fun and testing!\n\n" +
				"Personality:\n" +
				"- Be upbeat and genuine\n" +
				"- Try to be informative and engaging\n" +
				"- Use a natural, conversational tone\n",
		},
	}

	if err := client.Start(sessionUpdate); err != nil {
		log.Fatal("client start:", err)
	}
}
