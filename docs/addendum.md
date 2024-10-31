# Geppeto Audio System Documentation
## Real-time Audio Chat Client with OpenAI Integration

### 1. Overview
The Geppeto Audio System is a WebSocket-based chat client that enables real-time audio and text communication with OpenAI's APIs. The system supports bi-directional communication, handles streaming audio, and manages concurrent operations using goroutines.

### 2. Project Structure
```
geppetoaudio/
├── audiotypes/
│   └── types.go          # Core type definitions and interfaces
├── main/
│   └── mainaudio.go      # Main application logic and WebSocket handling
├── logs/                 # Generated log files
│   └── Chat:YYYYMMDD_HHMMSS.log
├── audio_output/         # Generated audio files
│   ├── audio_YYYYMMDD_HHMMSS.wav
│   └── audio_YYYYMMDD_HHMMSS.txt
├── go.mod               # Module dependencies
└── go.sum               # Dependency checksums
```

### 3. Type Definitions (audiotypes/types.go)

#### 3.1 Configuration Types
```go
type ClientConfig struct {
    ReadTimeout     time.Duration     // WebSocket read timeout
    WriteTimeout    time.Duration     // WebSocket write timeout
    PingInterval    time.Duration     // WebSocket ping interval
    MaxRetries      int              // Maximum connection retry attempts
    BufferSize      int              // Buffer size for channels
    ShutdownTimeout time.Duration     // Graceful shutdown timeout
    AudioOutputDir  string           // Directory for audio output files
}
```

#### 3.2 Audio Processing Types
```go
type AudioChunk struct {
    ResponseID   string    // Unique response identifier
    ItemID       string    // Unique item identifier
    OutputIndex  int       // Output sequence index
    ContentIndex int       // Content sequence index
    Data         []byte    // Raw audio data
}

type AudioMessage struct {
    Transcript string    // Text transcript of audio
    AudioData  []byte    // Processed audio data
    Complete   bool      // Completion status
}
```

#### 3.3 WebSocket Message Types
```go
type SessionUpdate struct {
    Type    string   `json:"type"`
    Session Session  `json:"session"`
}

type Session struct {
    Modalities              []string  `json:"modalities"`
    Instructions            string    `json:"instructions"`
    Temperature             float64   `json:"temperature"`
    MaxResponseOutputTokens int       `json:"max_response_output_tokens"`
    Voice                   string    `json:"voice"`
    InputAudioFormat        string    `json:"input_audio_format"`
    OutputAudioFormat       string    `json:"output_audio_format"`
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
```

### 4. Core Components

#### 4.1 ChatClient
The primary interface for handling WebSocket communication and audio processing.
```go
type ChatClient struct {
    Conn           *websocket.Conn
    MessageChannel chan string
    DisplayChannel chan ChatMessage
    AudioChannel   chan AudioChunk
    Done           chan struct{}
    Logger         *Logger
    Config         ClientConfig
    Metrics        *Metrics
    AudioBuffer    map[string]*AudioMessage
    WG             sync.WaitGroup
    ShutdownOnce   sync.Once
    AudioMutex     sync.Mutex
}
```

#### 4.2 Logger
Handles structured logging of all communication.
```go
type Logger struct {
    File    *os.File
    Mu      sync.Mutex
    Encoder *json.Encoder
}

type LogEntry struct {
    Timestamp string      `json:"timestamp"`
    Direction string      `json:"direction"`
    Type      string     `json:"type"`
    RawJSON   interface{} `json:"raw_json"`
}
```

#### 4.3 Metrics
Tracks performance and usage statistics.
```go
type Metrics struct {
    MessagesSent     int64
    MessagesReceived int64
    Errors           int64
    Latencies        []time.Duration
    AudioChunks      int64
    Mu               sync.Mutex
}
```

### 5. Key Functionalities

#### 5.1 Audio Processing
- Real-time audio streaming and buffering
- WAV file generation with proper headers
- Transcript generation and storage
- Buffer management for concurrent audio chunks

#### 5.2 WebSocket Communication
- Bi-directional message handling
- Session management
- Error handling and reconnection logic
- Message type routing

#### 5.3 Logging System
- Structured JSON logging
- Timestamp-based log files
- Direction tracking (sent/received)
- Raw message capture

#### 5.4 Concurrency Management
- Goroutine-based audio processing
- Thread-safe operations with mutexes
- Graceful shutdown handling
- Channel-based communication

### 6. Configuration Details

#### 6.1 Default Configuration
```go
DefaultConfig() ClientConfig {
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
```

#### 6.2 Audio Format Specifications
- Sample Rate: 24000 Hz
- Channels: Mono (1)
- Bits per Sample: 16
- Format: PCM
- Container: WAV

### 7. File Formats

#### 7.1 Audio Files
- Format: WAV with PCM encoding
- Naming: audio_YYYYMMDD_HHMMSS.wav
- Location: audio_output directory
- Accompanied by: Matching .txt transcript file

#### 7.2 Log Files
- Format: JSON entries, one per line
- Naming: Chat:YYYYMMDD_HHMMSS.log
- Location: logs directory
- Content: Structured message logs with timestamps

### 8. Error Handling
- Connection failures and retries
- Audio processing errors
- File system errors
- Message parsing errors
- Graceful degradation strategies

### 9. Security Considerations
- API key management
- WebSocket connection security
- File system permissions
- Data validation and sanitization

### 10. Performance Considerations
- Buffer sizes and memory management
- Goroutine lifecycle management
- Channel buffering
- File system operations
- Concurrent audio processing

### 11. Dependencies
- gorilla/websocket: WebSocket client implementation
- Standard library components:
  - context: Timeout and cancellation
  - encoding/binary: Audio data processing
  - encoding/json: Message handling
  - sync: Concurrency control

### 12. Usage
```go
// Initialize client
config := DefaultConfig()
client, err := NewChatClient(conn, config)

// Start session
sessionUpdate := SessionUpdate{...}
err := client.Start(sessionUpdate)

// Clean shutdown
client.shutdown()
```

This documentation provides a comprehensive overview of the system's architecture, components, and functionality. Would you like me to expand on any particular section or add more details about specific components?
