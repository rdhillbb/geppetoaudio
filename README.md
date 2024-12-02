# Note: The updated version can be found in [https://github.com/rdhillbb/ozz-wiz-realtimego]
# Geppetto Audio: High-Level Architecture Overview

**Geppetto Audio** is a Golang application that interfaces with OpenAI's real-time audio API. It allows users to input questions via the command line and receive audio responses from OpenAI. The audio responses are saved as WAV files, and their transcriptions are stored in the `audio_output` directory.

## Key Features

- **Interactive Command Line**: Users can type questions directly into the terminal.
- **Asynchronous Communication**: Utilizes goroutines for non-blocking operations with OpenAI's API.
- **Audio Response Handling**: Receives audio data, processes it, and saves it as WAV files.
- **Transcription Storage**: Saves transcriptions of audio responses alongside the audio files.

## High-Level Architecture

The application is designed using Go's concurrency primitives—goroutines and channels—to manage asynchronous communication and data processing efficiently.

### Main Components and Goroutines

1. **Main Goroutine**

   - **Responsibilities**:
     - Initializes the WebSocket connection to OpenAI.
     - Creates the `ChatClient` instance.
     - Starts other goroutines (`ReceiveRoutine`, `AudioProcessingRoutine`, `KeepAliveRoutine`).
     - Reads user input and sends messages to OpenAI.
     - Handles graceful shutdown on interrupt signals.

2. **ReceiveRoutine Goroutine**

   - **Function**: `receiveRoutine()`
   - **Responsibilities**:
     - Listens for incoming messages from OpenAI via the WebSocket connection.
     - Processes different message types (audio chunks, completion signals).
     - Sends audio chunks to the `AudioProcessingRoutine` via `AudioChannel`.

3. **AudioProcessingRoutine Goroutine**

   - **Function**: `audioProcessingRoutine()`
   - **Responsibilities**:
     - Receives audio chunks from `ReceiveRoutine`.
     - Buffers and assembles audio data into complete audio files.
     - Saves audio files and transcriptions to the `audio_output` directory.

4. **KeepAliveRoutine Goroutine**

   - **Function**: `keepAliveRoutine()`
   - **Responsibilities**:
     - Sends periodic ping messages to OpenAI to keep the WebSocket connection alive.
     - Listens for shutdown signals to exit gracefully.

### Communication Channels

- **`AudioChannel`** (`chan audiotypes.AudioChunk`):
  - Transfers audio chunks from `ReceiveRoutine` to `AudioProcessingRoutine`.
- **`Done`** (`chan struct{}`):
  - Signals all goroutines to shut down gracefully.

## High-Level Interaction Diagram

```plaintext
+------------------+
|                  |                       +----------------------+
|  Main Goroutine  |                       |      OpenAI API       |
|                  |                       |  (WebSocket Server)   |
+---------+--------+                       +----------+-----------+
          |                                            ^
          |                                            |
          |                                            |
          v                                            |
+---------+--------+                                   |
|                  |    Incoming Messages              |
|  ReceiveRoutine  | <---------------------------------+
|    Goroutine     |                                   |
|                  |                                   |
+---------+--------+                                   |
          |                                            |
          | AudioChunks (AudioChannel)                 |
          v                                            |
+---------+--------+                                   |
|                  |                                   |
| AudioProcessing  |                                   |
|    Goroutine     |                                   |
|                  |                                   |
+---------+--------+                                   |
          |                                            |
          | Saves audio files and transcriptions       |
          v                                            |
+------------------+                                   |
|                  |                                   |
|   audio_output   |                                   |
|    Directory     |                                   |
+------------------+                                   |
          ^                                            |
          |                                            |
+---------+--------+                                   |
|                  |                                   |
| KeepAliveRoutine | -- Periodic Ping Messages --------+
|    Goroutine     |                                   |
|                  |                                   |
+------------------+                                   |
          ^                                            |
          |                                            |
+---------+--------+                                   |
|                  |                                   |
|      Done        | (Shutdown Signal)                 |
+------------------+                                   |
```

**Diagram Explanation**:

- **Main Goroutine**:
  - Initializes and orchestrates the application.
  - Starts the other goroutines and handles user input.

- **ReceiveRoutine**:
  - Listens for messages from OpenAI.
  - Sends audio chunks to `AudioProcessingRoutine` via `AudioChannel`.

- **AudioProcessingRoutine**:
  - Receives and processes audio chunks.
  - Assembles complete audio files and saves transcriptions.

- **KeepAliveRoutine**:
  - Sends periodic ping messages to OpenAI to maintain the connection.
  - Monitors `Done` for shutdown signals.

- **Channels**:
  - **`AudioChannel`**: Facilitates communication between `ReceiveRoutine` and `AudioProcessingRoutine`.
  - **`Done`**: Used by all goroutines to listen for shutdown signals.

- **`audio_output` Directory**:
  - Stores the saved audio files and their transcriptions.

## Workflow Overview

1. **Initialization**:
   - The application starts and establishes a WebSocket connection with OpenAI.
   - Initializes `ChatClient` and starts the goroutines.

2. **User Interaction**:
   - The user inputs a question via the command line.
   - The question is sent to OpenAI through the WebSocket connection.

3. **Receiving Responses**:
   - `ReceiveRoutine` listens for responses from OpenAI.
   - Audio data is received in chunks and sent to `AudioProcessingRoutine` via `AudioChannel`.

4. **Processing Audio**:
   - `AudioProcessingRoutine` assembles audio chunks into complete audio files.
   - Saves the audio files as WAV files in the `audio_output` directory.
   - Transcriptions are saved alongside the audio files.

5. **Connection Maintenance**:
   - `KeepAliveRoutine` periodically sends ping messages to keep the connection alive.
   - Ensures uninterrupted communication with OpenAI.

6. **Shutdown**:
   - On interrupt signal (e.g., Ctrl+C), the `Done` channel is closed.
   - All goroutines receive the shutdown signal and exit gracefully.
   - Resources are cleaned up, and the application terminates.

## Concurrency and Communication

- **Goroutines**:
  - Allow concurrent operations without blocking the main execution flow.
  - Each goroutine handles a specific responsibility, improving modularity and maintainability.

- **Channels**:
  - Provide safe communication paths between goroutines.
  - Ensure data consistency and synchronization without explicit locks.

- **Synchronization**:
  - `WaitGroup` is used to wait for all goroutines to finish during shutdown.
  - Mutexes (e.g., `AudioMutex`) are used where necessary to protect shared resources.

## Summary

Geppetto Audio leverages Go's powerful concurrency features to interact with OpenAI's real-time audio API efficiently. By structuring the application with dedicated goroutines and communication channels, it achieves asynchronous communication, real-time audio processing, and a responsive user experience.

- **Asynchronous Communication**: Goroutines handle different tasks concurrently without blocking each other.
- **Efficient Data Processing**: Audio chunks are processed in real-time, ensuring prompt responses.
- **Resource Management**: Graceful shutdown mechanisms ensure that resources are properly released.
- **Scalability**: The architecture allows for easy extension and scaling of features in the future.
