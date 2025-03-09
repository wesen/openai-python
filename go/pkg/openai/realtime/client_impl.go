package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
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
	logger        zerolog.Logger

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

	// Set up a default logger that logs to stderr
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	return &clientImpl{
		apiKey:        apiKey,
		model:         model,
		eventHandlers: make(map[string][]EventHandler),
		eventChan:     make(chan Event, 100), // Buffered channel for events
		logger:        logger,
	}
}

// SetLogger sets the logger for the client
func (c *clientImpl) SetLogger(logger zerolog.Logger) {
	c.logger = logger
}

// Connect establishes a WebSocket connection with the OpenAI Realtime API
func (c *clientImpl) Connect(ctx context.Context) error {
	// Check if already connected
	if c.conn != nil {
		c.logger.Debug().Msg("Client is already connected")
		return errors.New("client is already connected")
	}

	c.logger.Info().
		Str("model", c.model).
		Str("url", BaseURL).
		Msg("Connecting to OpenAI Realtime API")

	// Create a context with timeout if the provided context doesn't have one
	ctx, cancel := context.WithTimeout(ctx, ConnectionTimeout)
	defer cancel()

	// Prepare dialer and headers
	dialer := websocket.DefaultDialer
	url := fmt.Sprintf("%s?model=%s", BaseURL, c.model)

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+c.apiKey)
	headers.Add("OpenAI-Beta", "realtime=v1")

	c.logger.Debug().Str("url", url).Msg("Dialing WebSocket")

	// Establish connection
	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		if resp != nil {
			body, readErr := io.ReadAll(resp.Body)
			if readErr == nil {
				c.logger.Error().
					Err(err).
					Int("status_code", resp.StatusCode).
					Str("response_body", string(body)).
					Msg("Failed to connect to OpenAI Realtime API")
			} else {
				c.logger.Error().
					Err(err).
					Int("status_code", resp.StatusCode).
					Msg("Failed to connect to OpenAI Realtime API")
			}
		} else {
			c.logger.Error().Err(err).Msg("Failed to connect to OpenAI Realtime API")
		}
		return fmt.Errorf("failed to connect to OpenAI Realtime API: %w", err)
	}
	c.conn = conn
	c.logger.Debug().Msg("WebSocket connection established")

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
			c.logger.Info().
				Str("session_id", c.sessionID).
				Str("model", sessionEvent.Session.Model).
				Str("voice", sessionEvent.Session.Voice).
				Msg("Session created")
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
	c.logger.Debug().Msg("Closing WebSocket connection")

	if c.conn == nil {
		c.logger.Debug().Msg("WebSocket connection already closed")
		return nil
	}

	// Send close message
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		c.logger.Error().Err(err).Msg("Error sending close message")
	}

	// Close the connection
	err = c.conn.Close()
	if err != nil {
		c.logger.Error().Err(err).Msg("Error closing WebSocket connection")
		return fmt.Errorf("error closing WebSocket connection: %w", err)
	}

	c.conn = nil
	c.logger.Info().Msg("WebSocket connection closed")
	return nil
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

// listenLoop reads messages from the WebSocket connection
func (c *clientImpl) listenLoop(ctx context.Context) {
	c.logger.Debug().Msg("Starting WebSocket listen loop")

	for {
		// Check if context is done
		select {
		case <-ctx.Done():
			c.logger.Debug().Msg("Context cancelled, stopping WebSocket listen loop")
			return
		default:
			// Continue
		}

		// Check if connection is closed
		if c.conn == nil {
			c.logger.Error().Msg("WebSocket connection is nil, stopping listen loop")
			return
		}

		// Read message from WebSocket
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				c.logger.Info().Msg("WebSocket closed normally")
			} else if websocket.IsUnexpectedCloseError(err) {
				c.logger.Error().Err(err).Msg("WebSocket closed unexpectedly")
			} else {
				c.logger.Error().Err(err).Msg("Error reading from WebSocket")
			}
			return
		}

		// Handle the message
		if err := c.handleMessage(message); err != nil {
			c.logger.Error().Err(err).Msg("Error handling message")
		}
	}
}

// genericEvent represents an event with an unknown type
type genericEvent struct {
	eventType string
	data      []byte
}

// Type returns the type of the event
func (e *genericEvent) Type() string {
	return e.eventType
}

// RawData returns the raw data of the event
func (e *genericEvent) RawData() []byte {
	return e.data
}

// handleMessage processes a message received from the WebSocket
func (c *clientImpl) handleMessage(data []byte) error {
	// Parse the event type
	var eventMap map[string]interface{}
	if err := json.Unmarshal(data, &eventMap); err != nil {
		c.logger.Error().Err(err).Str("data", string(data)).Msg("Failed to parse message JSON")
		return fmt.Errorf("failed to parse message JSON: %w", err)
	}

	// Extract the event type
	eventType, ok := eventMap["type"].(string)
	if !ok {
		c.logger.Error().Str("data", string(data)).Msg("Message missing 'type' field")
		return errors.New("message missing 'type' field")
	}

	c.logger.Debug().Str("event_type", eventType).Msg("Received event")

	// Create the appropriate event object based on the type
	var event Event
	var err error

	switch eventType {
	case EventSessionCreated:
		var e SessionCreatedEvent
		err = json.Unmarshal(data, &e)
		event = &e

	case EventSessionUpdated:
		var e SessionUpdatedEvent
		err = json.Unmarshal(data, &e)
		event = &e

	case EventConversationItemCreated:
		var e ConversationItemCreatedEvent
		err = json.Unmarshal(data, &e)
		event = &e

	case EventTranscriptionCompleted:
		var e TranscriptionCompletedEvent
		err = json.Unmarshal(data, &e)
		event = &e

	case EventResponseCreated:
		var e ResponseCreatedEvent
		err = json.Unmarshal(data, &e)
		event = &e

		// Store the response ID for tracking
		c.responseMutex.Lock()
		c.currentResponseID = e.ResponseID
		c.currentResponseText = ""
		c.currentResponseAudio = nil
		c.responseMutex.Unlock()

	case EventContentPartAdded:
		var e ContentPartAddedEvent
		err = json.Unmarshal(data, &e)
		event = &e

		// Append the text part to our buffer
		c.responseMutex.Lock()
		if e.ResponseID == c.currentResponseID {
			c.currentResponseText += e.Content.Text
		}
		c.responseMutex.Unlock()

	case EventContentPartDone:
		var e ContentPartDoneEvent
		err = json.Unmarshal(data, &e)
		event = &e

	case EventAudioDelta:
		var e AudioDeltaEvent
		err = json.Unmarshal(data, &e)
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
		err = json.Unmarshal(data, &e)
		event = &e

	case EventResponseDone:
		var e ResponseDoneEvent
		err = json.Unmarshal(data, &e)
		event = &e

	case EventError:
		var e ErrorEvent
		err = json.Unmarshal(data, &e)
		event = &e

	default:
		c.logger.Warn().Str("event_type", eventType).Msg("Unknown event type")
		// For unknown event types, create a generic event
		event = &genericEvent{
			eventType: eventType,
			data:      data,
		}
	}

	if err != nil {
		c.logger.Error().Err(err).Str("event_type", eventType).Msg("Error creating event object")
		return err
	}

	// Add the event to the channel for processing
	c.eventChan <- event
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
