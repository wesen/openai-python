package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Constants for default values
const (
	DefaultModel = "gpt-4o-realtime-preview-2024-10-01"
	BaseURL      = "wss://api.openai.com/v1/realtime"

	// Default timeouts
	ConnectionTimeout = 10 * time.Second
)

// clientImpl implements the Client interface
type clientImpl struct {
	apiKey        string
	model         string
	conn          *websocket.Conn
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex
	sessionID     string
	isRunning     bool
	runningMutex  sync.Mutex
	eventChan     chan Event

	// Buffer for assembling responses
	responseMutex        sync.Mutex
	currentResponseID    string
	currentResponseText  string
	currentResponseAudio []byte
}

// NewClient creates a new client with the provided API key and model
func NewClient(apiKey string, model string) Client {
	if model == "" {
		model = DefaultModel
	}

	return &clientImpl{
		apiKey:        apiKey,
		model:         model,
		eventHandlers: make(map[string][]EventHandler),
		eventChan:     make(chan Event, 100), // Buffered channel for events
	}
}

// Connect establishes a WebSocket connection with the OpenAI Realtime API
func (c *clientImpl) Connect(ctx context.Context) error {
	// Check if already connected
	if c.conn != nil {
		return errors.New("client is already connected")
	}

	// Create a context with timeout if the provided context doesn't have one
	ctx, cancel := context.WithTimeout(ctx, ConnectionTimeout)
	defer cancel()

	// Prepare dialer and headers
	dialer := websocket.DefaultDialer
	url := fmt.Sprintf("%s?model=%s", BaseURL, c.model)

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+c.apiKey)
	headers.Add("OpenAI-Beta", "realtime=v1")

	// Establish connection
	conn, _, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to OpenAI Realtime API: %w", err)
	}
	c.conn = conn

	// Start background listener for incoming messages
	go c.listenLoop(context.Background())

	// Wait for a session.created event to confirm successful connection
	sessionCreatedChan := make(chan struct{})
	var sessionErr error

	// Temporarily register a handler for session.created
	c.handlersMutex.Lock()
	originalHandlers := c.eventHandlers[EventSessionCreated]
	c.eventHandlers[EventSessionCreated] = append(c.eventHandlers[EventSessionCreated], func(e Event) error {
		if sessionEvent, ok := e.(*SessionCreatedEvent); ok {
			c.sessionID = sessionEvent.Session.SessionID
			close(sessionCreatedChan)
		}
		return nil
	})
	c.handlersMutex.Unlock()

	// Wait for session.created or context timeout
	select {
	case <-sessionCreatedChan:
		// Session successfully created
	case <-ctx.Done():
		sessionErr = fmt.Errorf("timed out waiting for session creation: %w", ctx.Err())
	}

	// Restore original handlers
	c.handlersMutex.Lock()
	c.eventHandlers[EventSessionCreated] = originalHandlers
	c.handlersMutex.Unlock()

	if sessionErr != nil {
		c.conn.Close()
		c.conn = nil
		return sessionErr
	}

	return nil
}

// Close terminates the WebSocket connection
func (c *clientImpl) Close(ctx context.Context) error {
	if c.conn == nil {
		return errors.New("not connected")
	}

	// Stop the event listener loop
	c.runningMutex.Lock()
	c.isRunning = false
	c.runningMutex.Unlock()

	// Close the connection
	err := c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(time.Second),
	)
	if err != nil {
		c.conn.Close()
	}

	c.conn = nil
	return err
}

// SendAudio sends audio data to the API
func (c *clientImpl) SendAudio(ctx context.Context, audio []byte) error {
	if c.conn == nil {
		return errors.New("not connected")
	}

	// Encode the audio data as base64
	encoded := base64.StdEncoding.EncodeToString(audio)

	// Create the message
	msg := AudioBufferAppendRequest{
		Type:  EventAudioBufferAppend,
		Audio: encoded,
	}

	// Send the message
	return c.conn.WriteJSON(msg)
}

// CommitAudio signals that the user has finished speaking
func (c *clientImpl) CommitAudio(ctx context.Context) error {
	if c.conn == nil {
		return errors.New("not connected")
	}

	// Create the message
	msg := AudioBufferCommitRequest{
		Type: EventAudioBufferCommit,
	}

	// Send the message
	return c.conn.WriteJSON(msg)
}

// SendText sends a text message to the API
func (c *clientImpl) SendText(ctx context.Context, text string) error {
	if c.conn == nil {
		return errors.New("not connected")
	}

	// Create the message using the simplified input_text approach
	msg := InputTextRequest{
		Type: EventInputText,
		Text: text,
	}

	// Send the message
	return c.conn.WriteJSON(msg)
}

// SetEventHandler registers a handler for a specific event type
func (c *clientImpl) SetEventHandler(eventType string, handler EventHandler) {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()

	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
}

// ListenForEvents starts processing events from the API
func (c *clientImpl) ListenForEvents(ctx context.Context) error {
	// Check if we're connected
	if c.conn == nil {
		return errors.New("not connected")
	}

	// Check if already running
	c.runningMutex.Lock()
	if c.isRunning {
		c.runningMutex.Unlock()
		return errors.New("event listener is already running")
	}
	c.isRunning = true
	c.runningMutex.Unlock()

	// Start a goroutine to receive messages
	go c.listenLoop(ctx)

	// Start another goroutine to process events from the channel
	go c.processEvents(ctx)

	return nil
}

// listenLoop is an internal method that continuously reads from the WebSocket
// and dispatches messages to the appropriate handlers
func (c *clientImpl) listenLoop(ctx context.Context) {
	defer func() {
		c.runningMutex.Lock()
		c.isRunning = false
		c.runningMutex.Unlock()
	}()

	for {
		// Check if we should stop
		select {
		case <-ctx.Done():
			return
		default:
			// Continue
		}

		// Check if connection is still alive
		if c.conn == nil {
			return
		}

		// Read next message
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// Handle connection closed or error
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				// Normal closure
				return
			}

			// Log error and exit loop
			fmt.Printf("Error reading from WebSocket: %v\n", err)
			return
		}

		// Process the message
		if err := c.handleMessage(message); err != nil {
			fmt.Printf("Error handling message: %v\n", err)
		}
	}
}

// handleMessage parses a raw message and creates the appropriate event
func (c *clientImpl) handleMessage(data []byte) error {
	// Parse the message to determine its type
	var rawMsg struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &rawMsg); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	// Create an event based on the type
	var event Event

	switch rawMsg.Type {
	case EventSessionCreated:
		var e SessionCreatedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	case EventSessionUpdated:
		var e SessionUpdatedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	case EventConversationItemCreated:
		var e ConversationItemCreatedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	case EventTranscriptionCompleted:
		var e TranscriptionCompletedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	case EventResponseCreated:
		var e ResponseCreatedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

		// Store the response ID for tracking
		c.responseMutex.Lock()
		c.currentResponseID = e.ResponseID
		c.currentResponseText = ""
		c.currentResponseAudio = nil
		c.responseMutex.Unlock()

	case EventContentPartAdded:
		var e ContentPartAddedEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

		// Append the text part to our buffer
		c.responseMutex.Lock()
		if e.ResponseID == c.currentResponseID {
			c.currentResponseText += e.Content.Text
		}
		c.responseMutex.Unlock()

	case EventContentPartDone:
		var e ContentPartDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	case EventAudioDelta:
		var e AudioDeltaEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

		// Decode and append the audio part to our buffer
		if e.Audio != "" {
			audioBytes, err := base64.StdEncoding.DecodeString(e.Audio)
			if err == nil {
				c.responseMutex.Lock()
				if e.ResponseID == c.currentResponseID {
					c.currentResponseAudio = append(c.currentResponseAudio, audioBytes...)
				}
				c.responseMutex.Unlock()
			}
		}

	case EventAudioDone:
		var e AudioDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	case EventResponseDone:
		var e ResponseDoneEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	case EventError:
		var e ErrorEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		e.BaseEvent = NewBaseEvent(rawMsg.Type, data)
		event = &e

	default:
		// Create a generic base event for unknown types
		event = NewBaseEvent(rawMsg.Type, data)
	}

	// Send the event to the channel for processing
	select {
	case c.eventChan <- event:
		// Successfully sent event to channel
	default:
		// Channel is full, log warning
		fmt.Printf("Event channel is full, dropping event of type: %s\n", rawMsg.Type)
	}

	return nil
}

// processEvents reads events from the channel and dispatches them to handlers
func (c *clientImpl) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-c.eventChan:
			if !ok {
				// Channel closed
				return
			}

			// Find and call handlers for this event type
			c.handlersMutex.RLock()
			handlers, exists := c.eventHandlers[event.Type()]
			c.handlersMutex.RUnlock()

			if exists {
				for _, handler := range handlers {
					if err := handler(event); err != nil {
						fmt.Printf("Error in event handler for %s: %v\n", event.Type(), err)
					}
				}
			}
		}
	}
}

// UpdateSession updates the session configuration
func (c *clientImpl) UpdateSession(ctx context.Context, config *Config) error {
	if c.conn == nil {
		return errors.New("not connected")
	}

	// Create the session update request
	updateReq := SessionUpdateRequest{
		Type: EventSessionUpdate,
	}

	// Add configured fields
	if config.Voice != "" {
		updateReq.Session.Voice = config.Voice
	}

	if len(config.Modalities) > 0 {
		updateReq.Session.Modalities = config.Modalities
	}

	if config.InputFormat != "" {
		updateReq.Session.InputAudioFormat = config.InputFormat
	}

	if config.OutputFormat != "" {
		updateReq.Session.OutputAudioFormat = config.OutputFormat
	}

	if config.Instructions != "" {
		updateReq.Session.Instructions = config.Instructions
	}

	if config.Temperature != nil {
		updateReq.Session.Temperature = config.Temperature
	}

	if config.TurnDetection != "" {
		updateReq.Session.TurnDetection = &struct {
			Type string `json:"type"`
		}{
			Type: config.TurnDetection,
		}
	}

	// Send the update request
	return c.conn.WriteJSON(updateReq)
}

// GetResponse returns the current response text and audio
func (c *clientImpl) GetResponse() (string, []byte) {
	c.responseMutex.Lock()
	defer c.responseMutex.Unlock()

	// Return copies to avoid race conditions
	audioBytes := make([]byte, len(c.currentResponseAudio))
	copy(audioBytes, c.currentResponseAudio)

	return c.currentResponseText, audioBytes
}
