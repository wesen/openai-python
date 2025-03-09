# OpenAI Realtime API - Go Implementation Plan

## 0. API Overview

### 0.1 What is the OpenAI Realtime API?

The OpenAI Realtime API (also called the Realtime API) allows developers to create low-latency, multi-modal conversational experiences with voice. It supports both text and audio as inputs and outputs, and offers function calling capabilities. Using this API, you can build natural speech-to-speech conversations with an AI model over a persistent WebSocket connection, similar to ChatGPT's voice mode.

Key benefits of the API include:
- Native speech-to-speech communication without text intermediary, reducing latency
- Expressive and steerable voices
- Simultaneous text + audio output for each response
- Support for interruptions (barge-in)
- Function calling capabilities to enable AI assistants to take actions

The API uses a WebSocket interface to stream audio in and out, allowing you to maintain a persistent session with the model (currently GPT-4o) for continuous interaction.

### 0.2 Authentication and Connection

To use the Realtime API, you need:

1. An OpenAI API key with access to the beta
2. A WebSocket client implementation

**Connection details:**
- WebSocket URL: `wss://api.openai.com/v1/realtime`
- Query parameters: `?model=gpt-4o-realtime-preview-2024-10-01` (model ID may change as new versions are released)
- HTTP Headers:
  - `Authorization: Bearer YOUR_API_KEY`
  - `OpenAI-Beta: realtime=v1` (required for beta access)

When the client successfully opens the WebSocket connection, the server responds by creating a new session. The first message from the server is a JSON event of type `session.created`, which contains a session ID and default session configuration.

### 0.3 Request-Response Protocol

Communication with the Realtime API is event-driven. Both the client and server exchange messages over the WebSocket in a JSON-based event format. Each message is a JSON object with a `"type"` field and additional fields depending on the event type.

#### 0.3.1 Basic Message Flow

A typical conversation follows this flow:

1. **Connection & Session Initialization:**
   - Client connects to WebSocket
   - Server sends `session.created` event
   - Client can optionally send `session.update` to configure the session (voice, modalities, etc.)

2. **User Input:**
   - **For audio input:** Client streams audio chunks via `input_audio_buffer.append` events, followed by an optional `input_audio_buffer.commit` (if server Voice Activity Detection is disabled)
   - **For text input:** Client sends a `conversation.item.create` event with text content or uses the simpler `input_text` event

3. **Transcription (for audio input):**
   - Server transcribes audio and sends `conversation.item.input_audio_transcription.completed` with the recognized text

4. **Assistant Response Generation:**
   - Server sends `response.created` to indicate the model is formulating a response
   - Server creates a conversation item for the assistant's answer (`conversation.item.created`)
   - Server streams partial text via `response.content_part.added` events
   - When text is complete, server sends `response.content_part.done`
   - Concurrently, server streams audio chunks via `response.audio.delta` events
   - When audio is complete, server sends `response.audio.done`
   - Finally, server sends `response.done` with usage statistics

5. **Repeat:** The session remains open for the next user input, starting again from step 2

### 0.4 Key Concepts and Features

#### 0.4.1 Session Management

A "session" represents a conversation with the model. It includes:
- Session ID
- Model identifier
- Voice configuration
- Audio format settings
- Turn detection settings
- System instructions

Once created, some session parameters cannot be changed (like voice), while others can be updated during the session.

#### 0.4.2 Voice Options

The API supports multiple preset voices (the same ones available in ChatGPT's voice mode):
- "alloy", "ash", "coral", "echo", "fable", "onyx", "nova", "sage", "shimmer"

Each voice has a distinct tone and style. Once a voice is chosen for a session, it cannot be changed.

#### 0.4.3 Audio Formats

The default audio format is 16 kHz 16-bit PCM. The API allows configuring:
- `input_audio_format`: Format of audio sent to the API
- `output_audio_format`: Format of audio received from the API

Common formats include `pcm_s16le` (16-bit PCM) and `g711_ulaw` (used in telephony).

#### 0.4.4 Voice Activity Detection (VAD)

The API supports two modes for determining when a user has finished speaking:
- `server_vad` (default): Server automatically detects when the user has stopped speaking
- `disabled`: Client must explicitly signal the end of user input via `input_audio_buffer.commit`

#### 0.4.5 Modalities

The API can provide responses in multiple modalities:
- Text: Textual content of the assistant's response
- Audio: Spoken audio of the assistant's response

By default, both modalities are returned. This can be configured in the session settings.

#### 0.4.6 Function Calling

The API supports function calling, allowing the assistant to invoke developer-defined functions. When the model decides a function call is needed, it will output a function call event instead of normal content. The client can then execute the function and send the result back to continue the conversation.

#### 0.4.7 Interruptions (Barge-in)

The API can handle interruptions gracefully. If a user starts speaking while the assistant is speaking, the client can send new audio input, and the server will stop the assistant's speech.

### 0.5 Example Usage Flow

Here's a simplified flow of a single conversation turn:

1. User speaks "Hello, how are you?" into the microphone
2. Client sends audio chunks via `input_audio_buffer.append` events
3. Server (with VAD) detects the end of speech and transcribes
4. Server sends `conversation.item.input_audio_transcription.completed` with the text "Hello, how are you?"
5. Server begins formulating a response (`response.created`)
6. Server streams text content via `response.content_part.added` events
7. Server streams audio chunks via `response.audio.delta` events
8. Server sends completion events (`response.content_part.done`, `response.audio.done`, `response.done`)
9. Client displays the text and plays the audio
10. Session remains open for the next user input

This event-driven protocol allows for real-time, interactive voice conversations with the AI model.

## 1. Project Structure and Setup

- [x] Create the directory structure for the project
```
/home/manuel/code/others/llms/openai-python/go/
├── cmd/                    # Example applications
│   └── voice-assistant/    # Simple voice assistant example
├── pkg/                    # Package code
│   └── openai/             # OpenAI specific code
│       └── realtime/       # Realtime API implementation
├── go.mod                  # Go module file
├── go.sum                  # Go dependencies checksum
├── LICENSE                 # License file
└── README.md               # Documentation
```

- [x] Initialize the Go module with `github.com/go-go-golems/openai-realtime`
- [x] Add necessary dependencies (gorilla/websocket, etc.)

## 2. Core Types and Interfaces

- [x] Define the main client interface and implementation
```go
// Client provides methods to interact with the OpenAI Realtime API
type Client interface {
    // Connect establishes a WebSocket connection with the OpenAI Realtime API
    Connect(ctx context.Context) error
    
    // Close terminates the WebSocket connection
    Close(ctx context.Context) error
    
    // SendAudio sends audio data to the API
    SendAudio(ctx context.Context, audio []byte) error
    
    // CommitAudio signals that the user has finished speaking
    CommitAudio(ctx context.Context) error
    
    // SendText sends a text message to the API
    SendText(ctx context.Context,  text string) error
    
    // SetEventHandler registers handlers for different event types
    SetEventHandler(eventType string, handler EventHandler)
    
    // ListenForEvents starts processing events from the API
    ListenForEvents(ctx context.Context) error
}
```

- [x] Define configuration struct
```go
// Config stores configuration options for the Realtime client
type Config struct {
    APIKey         string
    Model          string
    Voice          string
    Modalities     []string
    InputFormat    string
    OutputFormat   string
    Instructions   string
    TurnDetection  string
    Temperature    *float64
}
```

- [x] Define event types and handler interfaces
```go
// Event represents a message from the API
type Event interface {
    Type() string
    RawData() []byte
}

// EventHandler processes an event from the API
type EventHandler func(Event) error
```

## 3. Implementation of Client

- [x] Implement the WebSocket connection handler
```go
// Implementation pseudocode
func (c *client) Connect(ctx context.Context) error {
    // Create WebSocket dialer
    // Add headers (Authorization, OpenAI-Beta)
    // Connect to wss://api.openai.com/v1/realtime?model=...
    // Start goroutine to read messages
    // Wait for session.created event
    // Update session if needed with config values
}
```

- [x] Implement message parsing and event dispatch (potentially needs context argument)
```go
// Implementation pseudocode
func (c *client) handleMessage(data []byte) error {
    // Parse JSON message
    // Determine event type
    // Create appropriate event object
    // Dispatch to registered handler or default handler
}
```

- [x] Implement audio streaming methods
```go
// Implementation pseudocode
func (c *client) SendAudio(ctx context.Context, audio []byte) error {
    // Encode audio as base64
    // Create input_audio_buffer.append message
    // Send over WebSocket
}

func (c *client) CommitAudio(ctx context.Context) error {
    // Create input_audio_buffer.commit message
    // Send over WebSocket
}
```

- [x] Implement text sending method
```go
// Implementation pseudocode
func (c *client) SendText(ctx context.Context, text string) error {
    // Create conversation.item.create message with text content
    // Send over WebSocket
}
```

- [x] Implement session management methods
```go
// Implementation pseudocode
func (c *client) UpdateSession(ctx context.Context, config *Config) error {
    // Create session.update message with config values
    // Send over WebSocket
}
```

## 4. Specialized Event Types and Payloads

This section defines all the event types exchanged in the OpenAI Realtime API WebSocket communication, both from server to client and client to server.

### 4.1 Server-to-Client Events

These events are sent from the OpenAI server to the client:

- [x] `session.created` - Initial event when a session is established
```go
type SessionCreatedEvent struct {
    Type    string `json:"type"` // "session.created"
    EventID string `json:"event_id"`
    Session struct {
        ID                  string   `json:"id"`
        Object              string   `json:"object"` // "realtime.session"
        Model               string   `json:"model"`
        ExpiresAt           int64    `json:"expires_at"`
        Modalities          []string `json:"modalities"`
        Instructions        string   `json:"instructions"`
        Voice               string   `json:"voice"`
        TurnDetection       struct {
            Type               string  `json:"type"` // "server_vad" or "disabled"
            Threshold          float64 `json:"threshold,omitempty"`
            PrefixPaddingMs    int     `json:"prefix_padding_ms,omitempty"`
            SilenceDurationMs  int     `json:"silence_duration_ms,omitempty"`
            CreateResponse     bool    `json:"create_response,omitempty"`
            InterruptResponse  bool    `json:"interrupt_response,omitempty"`
        } `json:"turn_detection"`
        InputAudioFormat       string  `json:"input_audio_format"`
        OutputAudioFormat      string  `json:"output_audio_format"`
        InputAudioTranscription interface{} `json:"input_audio_transcription"`
        ToolChoice              string  `json:"tool_choice"`
        Temperature            float64 `json:"temperature"`
        MaxResponseOutputTokens string  `json:"max_response_output_tokens"`
        ClientSecret           interface{} `json:"client_secret"`
        Tools                  []interface{} `json:"tools"`
    } `json:"session"`
}
```

- [x] `session.updated` - Confirms session configuration update
```go
type SessionUpdatedEvent struct {
    Type    string `json:"type"` // "session.updated"
    Session struct {
        // Same fields as SessionCreatedEvent.Session
    } `json:"session"`
}
```

- [x] `conversation.item.created` - New message added to conversation
```go
type ConversationItemCreatedEvent struct {
    Type    string `json:"type"` // "conversation.item.created"
    Item    struct {
        ID      string `json:"id"`
        Role    string `json:"role"` // "user", "assistant", or "function"
        Content struct {
            Text string `json:"text,omitempty"`
            // Can include function_call for assistant role
            FunctionCall *struct {
                Name      string          `json:"name"`
                Arguments json.RawMessage `json:"arguments"` // JSON string of arguments
            } `json:"function_call,omitempty"`
        } `json:"content"`
        // Additional metadata fields
    } `json:"item"`
}
```

- [x] `conversation.item.input_audio_transcription.completed` - Transcribed user audio input
```go
type TranscriptionCompletedEvent struct {
    Type    string `json:"type"` // "conversation.item.input_audio_transcription.completed"
    Item    struct {
        ID      string `json:"id"`
        Role    string `json:"role"` // "user"
        Content struct {
            Text string `json:"text"` // The transcribed text
        } `json:"content"`
    } `json:"item"`
}
```

- [x] `response.created` - Model has begun formulating a response
```go
type ResponseCreatedEvent struct {
    Type       string `json:"type"` // "response.created"
    ResponseID string `json:"response_id"`
    // May include additional metadata
}
```

- [x] `response.content_part.added` - Partial text content from the assistant
```go
type ContentPartAddedEvent struct {
    Type        string `json:"type"` // "response.content_part.added"
    ResponseID  string `json:"response_id"`
    ItemID      string `json:"item_id"`
    OutputIndex int    `json:"output_index"`
    Content     struct {
        Text string `json:"text"` // Partial text content
    } `json:"content"`
}
```

- [x] `response.content_part.done` - Text content stream for this part is complete
```go
type ContentPartDoneEvent struct {
    Type        string `json:"type"` // "response.content_part.done"
    ResponseID  string `json:"response_id"`
    ItemID      string `json:"item_id"`
    OutputIndex int    `json:"output_index"`
}
```

- [x] `response.audio.delta` - Chunk of audio for the assistant's speech
```go
type AudioDeltaEvent struct {
    Type       string `json:"type"` // "response.audio.delta"
    ResponseID string `json:"response_id"`
    ItemID     string `json:"item_id"`
    Audio      string `json:"audio"` // Base64-encoded audio chunk
}
```

- [x] `response.audio.done` - All audio for the assistant's response has been sent
```go
type AudioDoneEvent struct {
    Type       string `json:"type"` // "response.audio.done"
    ResponseID string `json:"response_id"`
    ItemID     string `json:"item_id"`
}
```

- [x] `response.done` - Assistant's complete response is finished
```go
type ResponseDoneEvent struct {
    Type       string `json:"type"` // "response.done"
    ResponseID string `json:"response_id"`
    Usage      struct {
        InputTokens  int `json:"input_tokens"`
        OutputTokens int `json:"output_tokens"`
        AudioTokens  int `json:"audio_tokens"`
        CachedTokens int `json:"cached_tokens,omitempty"`
    } `json:"usage"`
}
```

- [x] `error` - Error event from the server
```go
type ErrorEvent struct {
    Type    string `json:"type"` // "error"
    Message string `json:"message"`
    Code    string `json:"code,omitempty"`
}
```

### 4.2 Client-to-Server Events

These events are sent from the client to the OpenAI server:

- [x] `session.update` - Update session configuration
```go
type SessionUpdateRequest struct {
    Type    string `json:"type"` // "session.update"
    Session struct {
        Voice            string   `json:"voice,omitempty"`
        Modalities       []string `json:"modalities,omitempty"`
        InputAudioFormat string   `json:"input_audio_format,omitempty"`
        OutputAudioFormat string  `json:"output_audio_format,omitempty"`
        Instructions     string   `json:"instructions,omitempty"`
        Temperature      *float64 `json:"temperature,omitempty"`
        TurnDetection    *struct {
            Type string `json:"type"` // "server_vad" or "disabled"
        } `json:"turn_detection,omitempty"`
        // Additional configuration fields
    } `json:"session"`
}
```

- [x] `input_audio_buffer.append` - Send chunk of user audio input
```go
type AudioBufferAppendRequest struct {
    Type  string `json:"type"` // "input_audio_buffer.append"
    Audio string `json:"audio"` // Base64-encoded audio chunk
}
```

- [x] `input_audio_buffer.commit` - Signal end of user speech
```go
type AudioBufferCommitRequest struct {
    Type string `json:"type"` // "input_audio_buffer.commit"
}
```

- [x] `input_audio_buffer.clear` - Clear any buffered audio
```go
type AudioBufferClearRequest struct {
    Type string `json:"type"` // "input_audio_buffer.clear"
}
```

- [x] `conversation.item.create` - Send a text message from the user
```go
type ConversationItemCreateRequest struct {
    Type string `json:"type"` // "conversation.item.create"
    Item struct {
        Role    string `json:"role"` // "user" or "function"
        Content struct {
            Text string `json:"text,omitempty"` // For user messages
            // For function results when role is "function"
            FunctionResult *struct {
                Name   string          `json:"name"`
                Result json.RawMessage `json:"result"` // JSON result data
            } `json:"function_result,omitempty"`
        } `json:"content"`
    } `json:"item"`
}
```

- [x] `conversation.item.truncate` - Truncate conversation history
```go
type ConversationItemTruncateRequest struct {
    Type string `json:"type"` // "conversation.item.truncate"
    ItemID string `json:"item_id"` // ID of the item to truncate from
}
```

- [x] `conversation.item.delete` - Delete a specific conversation item
```go
type ConversationItemDeleteRequest struct {
    Type string `json:"type"` // "conversation.item.delete"
    ItemID string `json:"item_id"` // ID of the item to delete
}
```

- [x] `response.create` - Explicitly request a response from the model
```go
type ResponseCreateRequest struct {
    Type string `json:"type"` // "response.create"
}
```

- [x] `response.cancel` - Cancel an ongoing response generation
```go
type ResponseCancelRequest struct {
    Type string `json:"type"` // "response.cancel"
}
```

### 4.3 Event Handler Implementation

- [x] Implement the event handler registry system
```go
// Implementation pseudocode
type client struct {
    // Other fields...
    eventHandlers map[string][]EventHandler
}

func (c *client) SetEventHandler(eventType string, handler EventHandler) {
    if c.eventHandlers == nil {
        c.eventHandlers = make(map[string][]EventHandler)
    }
    c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
}

func (c *client) handleEvent(event Event) {
    // Get handlers for this event type
    handlers, exists := c.eventHandlers[event.Type()]
    if exists {
        for _, handler := range handlers {
            handler(event)
        }
    }
}
```

### 4.4 High-Level Event Categories and Processing

- [x] Session Events - For managing session lifecycle and configuration
- [x] Conversation Events - For tracking conversation items/messages
- [x] Response Events - For streaming model responses (text and audio)
- [x] Input Events - For sending user input (text or audio)
- [x] Error Events - For handling error conditions

All event types will implement the `Event` interface defined in section 2.

## 5. Helper Utilities

- [x] Implement response assemblers
```go
// Implementation pseudocode
type ResponseAssembler struct {
    // Track partial text and audio for a response
    text  strings.Builder
    audio []byte
}

func (r *ResponseAssembler) AddText(text string) {
    r.text.WriteString(text)
}

func (r *ResponseAssembler) AddAudio(audio []byte) {
    r.audio = append(r.audio, audio...)
}
```

## 7. Example Applications

- [x] Implement a simple voice assistant example that takes text input and writes wav file out
```go
// Implementation pseudocode
func main() {
    // Create client with API key
    // Connect to API
    // Set up event handlers for different event types
    // Send text or audio
    // Process and play response
    // Close connection
}
```

## 8. Documentation and Tests

- [x] Write comprehensive GoDoc comments for all exported types and functions
- [x] Create README.md with usage examples and API documentation

## 9. Error Handling and Reliability

- [x] Add reconnection logic for network issues
- [x] Handle WebSocket close events appropriately
- [x] Implement context cancellation for all blocking operations


## 6. High-Level Convenience Methods

- [x] Implement blocking response methods
```go
// Implementation pseudocode
func (c *client) SendTextAndWaitForResponse(ctx context.Context, text string) (string, []byte, error) {
    // Send text
    // Wait for complete response (text and audio)
    // Return text and audio
}

func (c *client) SendAudioAndWaitForResponse(ctx context.Context, audio []byte) (string, []byte, error) {
    // Send audio
    // Wait for complete response (text and audio)
    // Return text and audio
}
```

- [ ] Implement audio format utilities
```go
// Implementation pseudocode
func ConvertAudioFormat(audio []byte, fromFormat, toFormat string) ([]byte, error) {
    // Convert between audio formats if needed
}
```
