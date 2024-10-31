# Technical Documentation: mainaudio.go
## Real-time Chat Client with Audio Support

### 1. Overview
`mainaudio.go` is a WebSocket-based chat client that communicates with OpenAI's real-time API, supporting both text and audio interactions. The application handles bidirectional communication, processes audio responses, and manages concurrent operations using goroutines.

### 2. Core Components

#### 2.1 Main Structures
```go
type ChatClient struct {
    conn           *websocket.Conn     // WebSocket connection
    messageChannel chan string         // User message queue
    displayChannel chan ChatMessage    // Display message queue
    audioChannel   chan AudioChunk     // Audio processing queue
    done          chan struct{}        // Shutdown signal
    shutdownOnce  sync.Once           // Ensure single shutdown
    wg            sync.WaitGroup      // Goroutine synchronization
    logger        *Logger             // Message logging
    config        ClientConfig        // Configuration
    metrics       *Metrics            // Performance tracking
    audioBuffer   map[string]*AudioMessage  // Audio storage
    audioMutex    sync.Mutex          // Audio buffer protection
}
```

#### 2.2 Concurrent Operations
The application runs four main goroutines:
1. `inputRoutine`: Handles user input
2. `receiveRoutine`: Processes WebSocket messages
3. `audioProcessingRoutine`: Manages audio chunk assembly
4. `displayRoutine`: Handles console output

### 3. Data Flow

#### 3.1 Input Processing
```plaintext
User Input → inputRoutine → messageChannel → sendRoutine → WebSocket
```

#### 3.2 Message Processing
```plaintext
WebSocket → receiveRoutine → 
    ├─ Text → displayChannel → Console
    └─ Audio → audioChannel → audioBuffer → WAV File
```

### 4. Key Features

#### 4.1 Audio Processing
- Handles streaming audio in chunks
- Assembles PCM16 format audio
- Creates WAV files with proper headers
- Saves transcripts alongside audio

#### 4.2 Resource Management
```go
func (c *ChatClient) shutdown() {
    c.shutdownOnce.Do(func() {
        close(c.done)         // Signal shutdown
        c.conn.Close()        // Close WebSocket
        c.logger.Close()      // Close logger
        c.wg.Wait()           // Wait for goroutines
        // Close channels
    })
}
```

### 5. Configuration
```go
type ClientConfig struct {
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    PingInterval    time.Duration
    MaxRetries      int
    BufferSize      int
    ShutdownTimeout time.Duration
    AudioOutputDir  string
}
```

### 6. Error Handling

#### 6.1 Network Errors
- Timeout handling
- Connection loss detection
- Retry mechanism for messages

#### 6.2 Resource Cleanup
- Graceful shutdown
- Channel closing sequence
- File handle management

### 7. Audio Processing Details

#### 7.1 Audio Format
```go
// WAV Header Structure
header := []interface{}{
    [4]byte{'R', 'I', 'F', 'F'},
    uint32(dataSize + 36),
    [4]byte{'W', 'A', 'V', 'E'},
    // ... format specifications
}
```

#### 7.2 Audio Buffer Management
- Uses map for multiple concurrent streams
- Thread-safe operations with mutex
- Automatic cleanup after processing

### 8. Logging System

#### 8.1 Log Entry Structure
```go
type LogEntry struct {
    Timestamp string
    Direction string
    Type      string
    RawJSON   interface{}
}
```

#### 8.2 Log Management
- Thread-safe logging
- JSON formatting
- Automatic file rotation

### 9. Implementation Guidelines

#### 9.1 Adding New Features
- Use channel-based communication
- Implement thread-safe operations
- Follow error handling patterns
- Update metrics appropriately

#### 9.2 Best Practices
- Use mutexes for shared resources
- Implement graceful shutdowns
- Handle context cancellation
- Maintain proper error chains

### 10. Dependencies
- gorilla/websocket: WebSocket client
- Standard library components:
  - context: Timeout management
  - sync: Concurrency control
  - encoding/binary: Audio processing
  - encoding/json: Message handling

### 11. Metrics and Monitoring
```go
type Metrics struct {
    messagesSent     int64
    messagesReceived int64
    errors          int64
    latencies       []time.Duration
    audioChunks     int64
}
