package audiotypes

import (
    "encoding/json"
    "os"
    "sync"
    "sync/atomic"
    "time"
    "log"
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
    AudioOutputDir  string
}

// Audio handling types
type AudioChunk struct {
    ResponseID   string
    ItemID       string
    OutputIndex  int
    ContentIndex int
    Data         []byte
}

type AudioMessage struct {
    Transcript string
    AudioData  []byte
    Complete   bool
}

type CompleteResponse struct {
    Type     string `json:"type"`
    Response struct {
        ID     string `json:"id"`
        Object string `json:"object"`
        Output []struct {
            Content []struct {
                Type       string `json:"type"`
                Transcript string `json:"transcript"`
            } `json:"content"`
            ID     string `json:"id"`
            Object string `json:"object"`
            Role   string `json:"role"`
            Status string `json:"status"`
            Type   string `json:"type"`
        } `json:"output"`
        Status        string      `json:"status"`
        StatusDetails interface{} `json:"status_details"`
        Usage         struct {
            InputTokens  int `json:"input_tokens"`
            OutputTokens int `json:"output_tokens"`
            TotalTokens  int `json:"total_tokens"`
        } `json:"usage"`
    } `json:"response"`
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
    Voice                   string   `json:"voice"`
    InputAudioFormat        string   `json:"input_audio_format"`
    OutputAudioFormat       string   `json:"output_audio_format"`
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
    Type       string `json:"type"`
    Text       string `json:"text"`
    Transcript string `json:"transcript,omitempty"`
}

type ChatMessage struct {
    Role    string
    Content string
}

// Metrics tracking
type Metrics struct {
    MessagesSent     int64
    MessagesReceived int64
    Errors           int64
    Latencies        []time.Duration
    AudioChunks      int64
    Mu               sync.Mutex
}

// Logger implementation
type Logger struct {
    File    *os.File
    Mu      sync.Mutex
    Encoder *json.Encoder
}

type LogEntry struct {
    Timestamp string      `json:"timestamp"`
    Direction string      `json:"direction"`
    Type      string      `json:"type"`
    RawJSON   interface{} `json:"raw_json"`
}

// ChatClient structure
type ChatClient struct {
    Conn           *websocket.Conn
    MessageChannel chan string
    DisplayChannel chan ChatMessage
    AudioChannel   chan AudioChunk
    Done           chan struct{}
    ShutdownOnce   sync.Once
    WG             sync.WaitGroup
    Logger         *Logger
    Config         ClientConfig
    Metrics        *Metrics
    AudioBuffer    map[string]*AudioMessage
    AudioMutex     sync.Mutex
}

// Default configuration
func DefaultConfig() ClientConfig {
    return ClientConfig{
        ReadTimeout:     30 * time.Second,
        WriteTimeout:    10 * time.Second,
        PingInterval:    30 * time.Second,
        MaxRetries:      3,
        BufferSize:      100,
        ShutdownTimeout: 5 * time.Second,
        AudioOutputDir:  "audio_output",
    }
}

// Metrics methods
func (m *Metrics) RecordLatency(start time.Time) {
    m.Mu.Lock()
    defer m.Mu.Unlock()
    m.Latencies = append(m.Latencies, time.Since(start))
}

func (m *Metrics) RecordError() {
    atomic.AddInt64(&m.Errors, 1)
}

func (m *Metrics) RecordAudioChunk() {
    atomic.AddInt64(&m.AudioChunks, 1)
}

func (l *Logger) Log(direction, msgType string, content interface{}) {
    l.Mu.Lock()
    defer l.Mu.Unlock()

    entry := LogEntry{
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
