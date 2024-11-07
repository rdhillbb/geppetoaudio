#GeppetoAudio Client
##Technical Reference and Implementation Guide
# Comprehensive Go Chat Client Documentation

## 1. Introduction

This documentation provides a complete guide to the Go chat client that interacts with OpenAI's WebSocket server to facilitate real-time text and audio communication. The application handles bidirectional communication, processes audio in PCM16 format, and manages concurrent operations through goroutines.

## 2. System Architecture

### 2.1 Core Components Overview
The system is built around several key components working together to handle real-time communication:
- WebSocket connection management
- Concurrent message processing
- Audio streaming and processing
- Resource and error management
- Logging and metrics collection

### 2.2 Key Structures

#### Basic Message Types
```go
type MessageType int

const (
    TextMessage MessageType = iota
    AudioMessage
)

type UserMessage struct {
    Type    MessageType
    Content string // Text content or file path for audio
}
```

#### Communication Structures
```go
type ConversationItem struct {
    Type string `json:"type"`
    Item struct {
        Type    string        `json:"type"`
        Role    string        `json:"role"`
        Content []ContentItem `json:"content"`
    } `json:"item"`
}

type ContentItem struct {
    Type     string `json:"type"`
    Text     string `json:"text,omitempty"`      // Used for text messages
    AudioRef string `json:"audio_ref,omitempty"` // Used for audio messages
}
```

#### Client Structure
```go
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
```

### 2.3 Configuration Management
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

### 2.4 Channel Communication
- MessageChannel: User text messages
- AudioChannel: Audio chunk processing
- DisplayChannel: Console output messages
- Done: Shutdown signaling

### 2.5 Resource Management
- WaitGroup: Goroutine synchronization
- Mutex: Thread-safe operations
- ShutdownOnce: Single shutdown execution
- AudioBuffer: Audio data management
## 3. Communication Protocol and Message Handling

### 3.1 Message Flow Patterns

#### Text Message Flow
```plaintext
User Input -> parseUserInput() -> sendUserMessage()
    |
    v
1. conversation.item.create
2. response.create
```

#### Audio Message Flow
```plaintext
Audio File -> parseUserInput() -> sendAudioMessage()
    |
    v
1. conversation.item.create
2. input_audio_buffer.append (multiple chunks)
3. input_audio_buffer.commit
4. response.create
```

### 3.2 Message Types and Formats

#### Session Update
```go
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
```

#### Response Handling
```go
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

### 3.3 Message Processing Functions

#### Text Message Processing
```go
func (c *ChatClient) sendUserMessage(text string) error {
    // Create conversation item
    // Send message
    // Request response
}
```

#### Audio Message Processing
```go
func (c *ChatClient) sendAudioMessage(audioFilePath string) error {
    // Validate audio file
    // Send in chunks
    // Signal completion
    // Request response
}
```

### 3.4 Protocol Sequence Examples

#### Text Conversation
```json
// Send message
{
    "type": "conversation.item.create",
    "item": {
        "type": "message",
        "role": "user",
        "content": [{
            "type": "input_text",
            "text": "Hello"
        }]
    }
}

// Request response
{
    "type": "response.create"
}
```

#### Audio Conversation
```json
// Initial message
{
    "type": "conversation.item.create",
    "item": {
        "type": "message",
        "role": "user",
        "content": [{
            "type": "input_audio",
            "audio_ref": "audio_0"
        }]
    }
}

// Audio chunks
{
    "type": "input_audio_buffer.append",
    "event_id": "evt_audio_N",
    "audio": "<base64-encoded-data>"
}

// Completion
{
    "type": "input_audio_buffer.commit",
    "event_id": "evt_commit_N"
}
```
## 4. Audio Processing and Handling

### 4.1 Audio Format Requirements

#### WAV File Specifications
```go
type WAVHeader struct {
    ChunkID       [4]byte // "RIFF"
    ChunkSize     uint32
    Format        [4]byte // "WAVE"
    Subchunk1ID   [4]byte // "fmt "
    Subchunk1Size uint32  // 16 for PCM
    AudioFormat   uint16  // 1 for PCM
    NumChannels   uint16  // 1 for mono
    SampleRate    uint32  // 24000 Hz
    ByteRate      uint32  // SampleRate * NumChannels * BitsPerSample/8
    BlockAlign    uint16  // NumChannels * BitsPerSample/8
    BitsPerSample uint16  // 16 for PCM16
}
```

#### Required Format
- Format: WAV (RIFF)
- Encoding: PCM (Linear)
- Bit Depth: 16-bit
- Channels: Mono (1 channel)
- Sample Rate: 24000 Hz

### 4.2 Audio Processing Components

#### Audio Chunk Management
```go
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

type AudioChunkConfig struct {
    ChunkSize       int  // 16KB default
    ChunkDurationMs int  // Calculated duration
}
```

#### Audio Processing Pipeline
```plaintext
1. File Validation
   └── validateWAVFormat()
       └── Check header and format requirements

2. Chunking Process
   └── DefaultAudioChunkConfig()
       └── 16KB chunks
       └── Progress tracking

3. Processing
   └── handleAudioChunk()
       └── Buffer management
       └── Assembly tracking

4. Storage
   └── saveAudioOnly()
       └── WAV file creation
       └── Header writing
```

### 4.3 Audio Processing Functions

#### Validation Function
```go
func (c *ChatClient) validateWAVFormat(file *os.File) error {
    // Read WAV header
    // Verify format parameters
    // Check specifications
    // Return detailed errors if invalid
}
```

#### Chunk Processing
```go
func (c *ChatClient) handleAudioChunk(chunk audiotypes.AudioChunk) {
    // Generate unique key
    // Initialize/update buffer
    // Accumulate data
    // Track metrics
}
```

#### Audio Saving
```go
func (c *ChatClient) saveAudioOnly(responseID, itemID string, filepath string) error {
    // Retrieve from buffer
    // Create WAV file
    // Write header
    // Save audio data
    // Clean up buffer
}
```

#### WAV Header Writing
```go
func (c *ChatClient) writeWAVHeader(file io.Writer, dataSize uint32) error {
    // Write RIFF header
    // Set format parameters
    // Write data size
    // Handle endianness
}
```

### 4.4 Audio Buffer Management

#### Buffer Operations
```go
// Key format for audio buffer
audioKey := fmt.Sprintf("%s_%s", responseID, itemID)

// Thread-safe buffer access
c.AudioMutex.Lock()
defer c.AudioMutex.Unlock()

// Buffer initialization
c.AudioBuffer[audioKey] = &AudioMessage{
    AudioData: make([]byte, 0, 1024*1024), // 1MB initial capacity
}
```

#### Progress Tracking
```go
// During audio sending
progress := float64(bytesSent) / float64(audioDataSize) * 100
log.Printf("Sent chunk %d (%.1f%% complete)", chunkCount, progress)
```

### 4.5 Common Audio Issues and Solutions

```plaintext
1. Format Issues
   - Issue: "invalid WAV format"
   - Solution: Ensure RIFF header is present and valid

2. Channel Configuration
   - Issue: "audio must be mono"
   - Solution: Convert stereo to mono using audio software

3. Sample Rate
   - Issue: "sample rate must be 24000Hz"
   - Solution: Resample audio to correct rate

4. Bit Depth
   - Issue: "audio must be 16-bit"
   - Solution: Convert to 16-bit PCM format
```
## 4. Audio Processing and Handling

### 4.1 Audio Format Requirements

#### WAV File Specifications
```go
type WAVHeader struct {
    ChunkID       [4]byte // "RIFF"
    ChunkSize     uint32
    Format        [4]byte // "WAVE"
    Subchunk1ID   [4]byte // "fmt "
    Subchunk1Size uint32  // 16 for PCM
    AudioFormat   uint16  // 1 for PCM
    NumChannels   uint16  // 1 for mono
    SampleRate    uint32  // 24000 Hz
    ByteRate      uint32  // SampleRate * NumChannels * BitsPerSample/8
    BlockAlign    uint16  // NumChannels * BitsPerSample/8
    BitsPerSample uint16  // 16 for PCM16
}
```

#### Required Format
- Format: WAV (RIFF)
- Encoding: PCM (Linear)
- Bit Depth: 16-bit
- Channels: Mono (1 channel)
- Sample Rate: 24000 Hz

### 4.2 Audio Processing Components

#### Audio Chunk Management
```go
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

type AudioChunkConfig struct {
    ChunkSize       int  // 16KB default
    ChunkDurationMs int  // Calculated duration
}
```

#### Audio Processing Pipeline
```plaintext
1. File Validation
   └── validateWAVFormat()
       └── Check header and format requirements

2. Chunking Process
   └── DefaultAudioChunkConfig()
       └── 16KB chunks
       └── Progress tracking

3. Processing
   └── handleAudioChunk()
       └── Buffer management
       └── Assembly tracking

4. Storage
   └── saveAudioOnly()
       └── WAV file creation
       └── Header writing
```

### 4.3 Audio Processing Functions

#### Validation Function
```go
func (c *ChatClient) validateWAVFormat(file *os.File) error {
    // Read WAV header
    // Verify format parameters
    // Check specifications
    // Return detailed errors if invalid
}
```

#### Chunk Processing
```go
func (c *ChatClient) handleAudioChunk(chunk audiotypes.AudioChunk) {
    // Generate unique key
    // Initialize/update buffer
    // Accumulate data
    // Track metrics
}
```

#### Audio Saving
```go
func (c *ChatClient) saveAudioOnly(responseID, itemID string, filepath string) error {
    // Retrieve from buffer
    // Create WAV file
    // Write header
    // Save audio data
    // Clean up buffer
}
```

#### WAV Header Writing
```go
func (c *ChatClient) writeWAVHeader(file io.Writer, dataSize uint32) error {
    // Write RIFF header
    // Set format parameters
    // Write data size
    // Handle endianness
}
```

### 4.4 Audio Buffer Management

#### Buffer Operations
```go
// Key format for audio buffer
audioKey := fmt.Sprintf("%s_%s", responseID, itemID)

// Thread-safe buffer access
c.AudioMutex.Lock()
defer c.AudioMutex.Unlock()

// Buffer initialization
c.AudioBuffer[audioKey] = &AudioMessage{
    AudioData: make([]byte, 0, 1024*1024), // 1MB initial capacity
}
```

#### Progress Tracking
```go
// During audio sending
progress := float64(bytesSent) / float64(audioDataSize) * 100
log.Printf("Sent chunk %d (%.1f%% complete)", chunkCount, progress)
```

### 4.5 Common Audio Issues and Solutions

```plaintext
1. Format Issues
   - Issue: "invalid WAV format"
   - Solution: Ensure RIFF header is present and valid

2. Channel Configuration
   - Issue: "audio must be mono"
   - Solution: Convert stereo to mono using audio software

3. Sample Rate
   - Issue: "sample rate must be 24000Hz"
   - Solution: Resample audio to correct rate

4. Bit Depth
   - Issue: "audio must be 16-bit"
   - Solution: Convert to 16-bit PCM format
```
## 6. Best Practices and Advanced Usage

### 6.1 Configuration Best Practices

#### Default Configuration
```go
func DefaultConfig() audiotypes.ClientConfig {
    return audiotypes.ClientConfig{
        ReadTimeout:     90 * time.Second,
        WriteTimeout:    30 * time.Second,
        PingInterval:    20 * time.Second,
        MaxRetries:      3,
        BufferSize:      100,
        ShutdownTimeout: 5 * time.Second,
        AudioOutputDir:  "audio_output",
    }
}
```

#### Configuration Optimization Examples
```go
// High-throughput configuration
config := DefaultConfig()
config.BufferSize = 200
config.ReadTimeout = 120 * time.Second
config.WriteTimeout = 45 * time.Second

// Low-latency configuration
config := DefaultConfig()
config.PingInterval = 10 * time.Second
config.WriteTimeout = 15 * time.Second
config.ReadTimeout = 45 * time.Second
```

### 6.2 Resource Management

#### Goroutine Management
```go
// Proper goroutine startup
c.WG.Add(1)
go func() {
    defer c.WG.Done()
    // Goroutine work
}()

// Graceful shutdown
func (c *ChatClient) shutdown() {
    c.ShutdownOnce.Do(func() {
        close(c.Done)
        c.WG.Wait()
        // Resource cleanup
    })
}
```

#### Memory Management
```go
// Buffer cleanup
defer func() {
    c.AudioMutex.Lock()
    delete(c.AudioBuffer, audioKey)
    c.AudioMutex.Unlock()
}()

// Resource limiting
buffer := make([]byte, chunkConfig.ChunkSize)
defer runtime.GC()
```

### 6.3 Thread Safety Patterns

#### Mutex Usage
```go
// Audio buffer access
c.AudioMutex.Lock()
defer c.AudioMutex.Unlock()

// Metrics update
func (m *Metrics) RecordLatency(start time.Time) {
    m.Mu.Lock()
    defer m.Mu.Unlock()
    m.Latencies = append(m.Latencies, time.Since(start))
}
```

#### Channel Communication
```go
// Safe channel sending
select {
case c.AudioChannel <- chunk:
    log.Printf("Sent audio chunk")
case <-c.Done:
    return fmt.Errorf("shutdown during send")
default:
    return fmt.Errorf("channel full")
}
```

### 6.4 Advanced Usage Patterns

#### Custom Session Configuration
```go
sessionUpdate := audiotypes.SessionUpdate{
    Type: "session.update",
    Session: audiotypes.Session{
        Modalities: []string{"text", "audio"},
        Temperature: 0.8,
        MaxResponseOutputTokens: 4096,
        Voice: "alloy",
        InputAudioFormat: "pcm16",
        OutputAudioFormat: "pcm16",
        Instructions: customInstructions,
    },
}
```

#### Extended Audio Processing
```go
// Custom audio chunk size
config := DefaultAudioChunkConfig()
config.ChunkSize = 32 * 1024  // 32KB chunks

// Progress monitoring
type ProgressMonitor struct {
    TotalBytes   int64
    BytesSent    int64
    ChunkCount   int
    StartTime    time.Time
}
```

Let me continue with Part 6, as there are more important aspects to cover.


## 7. Implementation Examples and Troubleshooting

### 7.1 Common Implementation Patterns

#### Message Processing Pipeline
```go
func processMessage(msg *UserMessage) error {
    // 1. Validate message
    if err := validateMessage(msg); err != nil {
        return fmt.Errorf("validation error: %w", err)
    }

    // 2. Process based on type
    switch msg.Type {
    case TextMessage:
        return processTextMessage(msg)
    case AudioMessage:
        return processAudioMessage(msg)
    default:
        return fmt.Errorf("unknown message type")
    }
}
```

#### Audio Processing Pipeline
```go
func processAudioFile(filepath string) error {
    // 1. Open and validate file
    file, err := os.Open(filepath)
    if err != nil {
        return fmt.Errorf("open file: %w", err)
    }
    defer file.Close()

    // 2. Validate format
    if err := validateWAVFormat(file); err != nil {
        return fmt.Errorf("invalid format: %w", err)
    }

    // 3. Process chunks
    return processAudioChunks(file)
}
```

### 7.2 Troubleshooting Guide

#### Connection Issues
```plaintext
Problem: WebSocket connection fails
Solutions:
1. Check API key validity
2. Verify network connectivity
3. Check URL format
4. Validate SSL certificates
5. Review proxy settings
```

#### Audio Processing Issues
```plaintext
Problem: Audio validation fails
Solutions:
1. Verify WAV format using audio tool
2. Check sample rate (must be 24000Hz)
3. Ensure mono channel
4. Confirm 16-bit depth
5. Validate header structure
```

#### Performance Issues
```plaintext
Problem: High latency
Solutions:
1. Reduce chunk size
2. Increase buffer size
3. Optimize network settings
4. Monitor memory usage
5. Check goroutine leaks
```

### 7.3 Debugging Techniques

#### Logging Analysis
```go
// Enable verbose logging
log.Printf("Audio chunk details: size=%d, position=%d, remaining=%d",
    chunkSize, position, remaining)

// Track timing
start := time.Now()
defer func() {
    log.Printf("Operation took: %v", time.Since(start))
}()
```

#### Metrics Analysis
```go
// Performance monitoring
type PerformanceMetrics struct {
    ProcessingTime    time.Duration
    ChunkCount       int
    BufferUtilization float64
    ErrorRate        float64
}

func calculateMetrics() *PerformanceMetrics {
    // Calculate and return metrics
}
```

Let me proceed with Part 7 to complete the documentation.


## 8. Advanced Topics and Reference

### 8.1 Advanced Customization

#### Custom Audio Processing
```go
type AudioProcessor interface {
    Process([]byte) ([]byte, error)
    ValidateFormat(header WAVHeader) error
    GetChunkSize() int
}

// Implementation example
type CustomAudioProcessor struct {
    chunkSize int
    format    string
    options   map[string]interface{}
}
```

#### Extended Configuration
```go
type AdvancedConfig struct {
    ClientConfig
    AudioProcessing struct {
        ChunkSize          int
        BufferSize         int
        ConcurrentStreams  int
        ProcessingOptions  map[string]interface{}
    }
    Networking struct {
        MaxConcurrentConns int
        KeepAlive         time.Duration
        RetryStrategy     RetryStrategy
    }
}
```

### 8.2 API Reference

#### Main Interfaces
```go
// Client interface
type ChatClientInterface interface {
    Start(SessionUpdate) error
    SendMessage(*UserMessage) error
    ProcessAudio(string) error
    Shutdown() error
}

// Audio processing interface
type AudioHandler interface {
    ProcessChunk(AudioChunk) error
    SaveAudio(string) error
    ValidateFormat(io.Reader) error
}
```

#### Key Constants and Defaults
```go
const (
    DefaultChunkSize = 16 * 1024
    DefaultSampleRate = 24000
    DefaultBitDepth = 16
    DefaultChannels = 1
)

var (
    DefaultRetryStrategy = ExponentialBackoff
    DefaultBufferSize = 100
    DefaultTimeouts = TimeoutConfig{
        Read:  90 * time.Second,
        Write: 30 * time.Second,
    }
)
```

### 8.3 Future Considerations

#### Planned Enhancements
```plaintext
1. Dynamic audio format support
2. Improved error recovery
3. Enhanced metrics collection
4. Automated testing suite
5. Performance optimizations
```

#### Integration Points
```plaintext
1. Custom audio processors
2. Alternative transport layers
3. Extended logging systems
4. Metrics exporters
5. Custom authentication
```
