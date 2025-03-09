package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openai/realtime-http-go/pkg/config"
	"github.com/rs/zerolog/log"
)

// OpenAIRealtimeClient is the interface for communicating with the OpenAI Realtime API
type OpenAIRealtimeClient interface {
	// Connect connects to the OpenAI Realtime API
	Connect(ctx context.Context) error
	// Disconnect disconnects from the OpenAI Realtime API
	Disconnect() error
	// SendAudio sends audio data to the OpenAI Realtime API
	SendAudio(audioData string) error
	// CommitAudio commits the audio buffer
	CommitAudio() error
	// SendText sends a text message to the OpenAI Realtime API
	SendText(text string) error
	// IsConnected returns true if the client is connected
	IsConnected() bool
}

// Event types for OpenAI Realtime API
const (
	EventTypeSessionCreated                     = "session.created"
	EventTypeSessionUpdated                     = "session.updated"
	EventTypeConversationItemCreated            = "conversation.item.created"
	EventTypeConversationItemInputAudioTransComplete = "conversation.item.input_audio_transcription.completed"
	EventTypeConversationItemInputAudioTransFailed  = "conversation.item.input_audio_transcription.failed"
	EventTypeResponseTextDelta                  = "response.text.delta"
	EventTypeResponseTextDone                   = "response.text.done"
	EventTypeResponseAudioDelta                 = "response.audio.delta"
	EventTypeResponseAudioDone                  = "response.audio.done"
	EventTypeResponseAudioTranscriptDelta       = "response.audio_transcript.delta"
	EventTypeResponseAudioTranscriptDone        = "response.audio_transcript.done"
	EventTypeResponseDone                       = "response.done"
	EventTypeError                              = "error"
)

// EventHandler is a function that handles OpenAI Realtime API events
type EventHandler func(event map[string]interface{}) error

// DefaultOpenAIRealtimeClient is the default implementation of OpenAIRealtimeClient
type DefaultOpenAIRealtimeClient struct {
	// Configuration
	config *config.Config

	// WebSocket connection
	conn *websocket.Conn

	// Session information
	sessionID string

	// Event handlers
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex

	// Connection status
	connected bool
	connMutex sync.RWMutex

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewOpenAIRealtimeClient creates a new OpenAI Realtime client
func NewOpenAIRealtimeClient(cfg *config.Config) OpenAIRealtimeClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &DefaultOpenAIRealtimeClient{
		config:        cfg,
		eventHandlers: make(map[string][]EventHandler),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// RegisterEventHandler registers an event handler for a specific event type
func (c *DefaultOpenAIRealtimeClient) RegisterEventHandler(eventType string, handler EventHandler) {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()

	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
}

// Connect connects to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) Connect(ctx context.Context) error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.connected {
		return nil
	}

	// Determine API base URL
	apiBase := "wss://api.openai.com/v1/beta/realtime"
	if c.config.OpenAIAPIBase != "" {
		apiBase = c.config.OpenAIAPIBase
	}

	// Add model as query parameter
	u, err := url.Parse(apiBase)
	if err != nil {
		return fmt.Errorf("failed to parse API base URL: %w", err)
	}

	q := u.Query()
	q.Set("model", c.config.OpenAIModelName)
	u.RawQuery = q.Encode()

	// Set up headers
	headers := http.Header{}
	headers.Add("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	headers.Add("Content-Type", "application/json")

	log.Info().Str("url", u.String()).Msg("Connecting to OpenAI Realtime API")

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to connect to OpenAI Realtime API: %w", err)
	}

	c.conn = conn
	c.connected = true

	// Start event reader
	go c.readEvents()

	// Set up server-side Voice Activity Detection
	err = c.setupServerVAD()
	if err != nil {
		c.Disconnect()
		return fmt.Errorf("failed to set up server VAD: %w", err)
	}

	return nil
}

// setupServerVAD sets up server-side Voice Activity Detection
func (c *DefaultOpenAIRealtimeClient) setupServerVAD() error {
	message := map[string]interface{}{
		"type": "session.update",
		"session": map[string]interface{}{
			"turn_detection": map[string]interface{}{
				"type": "server_vad",
			},
			"input_audio_transcription": map[string]interface{}{
				"model": "whisper-1",
			},
		},
	}

	return c.sendJSON(message)
}

// Disconnect disconnects from the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) Disconnect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	// Close WebSocket connection
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Error().Err(err).Msg("Error sending close message")
	}

	// Wait a bit for the close to be sent
	time.Sleep(100 * time.Millisecond)

	// Close the connection
	err = c.conn.Close()
	c.conn = nil
	c.connected = false

	// Signal cancellation
	c.cancel()

	return err
}

// SendAudio sends audio data to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) SendAudio(audioData string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Send audio buffer append event
	message := map[string]interface{}{
		"type": "input_audio_buffer.append",
		"audio": audioData,
	}

	return c.sendJSON(message)
}

// CommitAudio commits the audio buffer and triggers a response
func (c *DefaultOpenAIRealtimeClient) CommitAudio() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Send audio buffer commit event
	commitMsg := map[string]interface{}{
		"type": "input_audio_buffer.commit",
	}

	if err := c.sendJSON(commitMsg); err != nil {
		return err
	}

	// Send response create event
	responseMsg := map[string]interface{}{
		"type": "response.create",
	}

	return c.sendJSON(responseMsg)
}

// SendText sends a text message to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) SendText(text string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Send conversation item create event
	itemMsg := map[string]interface{}{
		"type": "conversation.item.create",
		"item": map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []map[string]interface{}{
				{
					"type": "input_text",
					"text": text,
				},
			},
		},
	}

	if err := c.sendJSON(itemMsg); err != nil {
		return err
	}

	// Send response create event
	responseMsg := map[string]interface{}{
		"type": "response.create",
	}

	return c.sendJSON(responseMsg)
}

// IsConnected returns true if the client is connected
func (c *DefaultOpenAIRealtimeClient) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()

	return c.connected && c.conn != nil
}

// sendJSON sends a JSON message to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) sendJSON(message interface{}) error {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	return c.conn.WriteJSON(message)
}

// readEvents reads events from the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) readEvents() {
	defer func() {
		c.connMutex.Lock()
		c.connected = false
		c.connMutex.Unlock()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Continue reading
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("WebSocket closed unexpectedly")
			}
			return
		}

		// Parse the message
		var event map[string]interface{}
		if err := json.Unmarshal(message, &event); err != nil {
			log.Error().Err(err).Str("message", string(message)).Msg("Failed to unmarshal event")
			continue
		}

		// Extract event type
		eventType, ok := event["type"].(string)
		if !ok {
			log.Error().Interface("event", event).Msg("Event has no type")
			continue
		}

		// Extract session ID from session.created event
		if eventType == EventTypeSessionCreated {
			if session, ok := event["session"].(map[string]interface{}); ok {
				if id, ok := session["id"].(string); ok {
					c.sessionID = id
					log.Info().Str("session_id", id).Msg("Session created")
				}
			}
		}

		// Handle the event
		c.handleEvent(eventType, event)
	}
}

// handleEvent handles an event from the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) handleEvent(eventType string, event map[string]interface{}) {
	c.handlersMutex.RLock()
	handlers := c.eventHandlers[eventType]
	// Also get generic handlers (those that handle all events)
	genericHandlers := c.eventHandlers["*"]
	c.handlersMutex.RUnlock()

	// Log event type but not entire event (which may be large)
	log.Debug().
		Str("event_type", eventType).
		Int("handlers", len(handlers)).
		Int("generic_handlers", len(genericHandlers)).
		Msg("Received event")

	// Call handlers for this event type
	for _, handler := range handlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling event")
		}
	}

	// Call generic handlers
	for _, handler := range genericHandlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling event (generic handler)")
		}
	}

	// Special handling for error events
	if eventType == EventTypeError {
		errorMsg := "Unknown error"
		if errObj, ok := event["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok {
				errorMsg = msg
			}
		}
		log.Error().Str("error", errorMsg).Msg("OpenAI API error")
	}
}

// MockOpenAIRealtimeClient is a mock implementation of OpenAIRealtimeClient for testing
type MockOpenAIRealtimeClient struct {
	connected bool
	handlers  map[string][]EventHandler
	mutex     sync.RWMutex
}

// NewMockOpenAIRealtimeClient creates a new mock OpenAI Realtime client
func NewMockOpenAIRealtimeClient() OpenAIRealtimeClient {
	return &MockOpenAIRealtimeClient{
		handlers: make(map[string][]EventHandler),
	}
}

// RegisterEventHandler registers an event handler for a specific event type
func (m *MockOpenAIRealtimeClient) RegisterEventHandler(eventType string, handler EventHandler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.handlers[eventType] = append(m.handlers[eventType], handler)
}

// Connect connects to the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) Connect(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.connected = true
	return nil
}

// Disconnect disconnects from the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) Disconnect() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.connected = false
	return nil
}

// SendAudio sends audio data to the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) SendAudio(audioData string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}
	return nil
}

// CommitAudio commits the audio buffer
func (m *MockOpenAIRealtimeClient) CommitAudio() error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Simulate a response for testing
	go func() {
		// Wait a bit to simulate processing
		time.Sleep(500 * time.Millisecond)

		// Simulate a transcript
		m.mockEvent(EventTypeConversationItemInputAudioTransComplete, map[string]interface{}{
			"transcript": "Hello, world!",
			"item_id":    "transcript_1",
		})

		// Simulate a text response
		m.mockEvent(EventTypeResponseTextDelta, map[string]interface{}{
			"delta":   "Hello, ",
			"item_id": "response_1",
		})

		time.Sleep(100 * time.Millisecond)

		m.mockEvent(EventTypeResponseTextDelta, map[string]interface{}{
			"delta":   "how can I help you?",
			"item_id": "response_1",
		})

		time.Sleep(100 * time.Millisecond)

		// Simulate done event
		m.mockEvent(EventTypeResponseDone, map[string]interface{}{})
	}()

	return nil
}

// SendText sends a text message to the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) SendText(text string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Simulate a response for testing
	go func() {
		// Wait a bit to simulate processing
		time.Sleep(300 * time.Millisecond)

		response := fmt.Sprintf("You said: %s. I'm a mock OpenAI client.", text)
		words := strings.Split(response, " ")

		for i, word := range words {
			// Simulate text delta events
			m.mockEvent(EventTypeResponseTextDelta, map[string]interface{}{
				"delta":   word + " ",
				"item_id": "response_1",
			})
			time.Sleep(50 * time.Millisecond)

			// Every few words, simulate an audio delta event
			if i%3 == 0 {
				m.mockEvent(EventTypeResponseAudioDelta, map[string]interface{}{
					"delta":   "base64-encoded-audio-data",
					"item_id": "audio_1",
				})
			}
		}

		// Simulate done event
		m.mockEvent(EventTypeResponseDone, map[string]interface{}{})
	}()

	return nil
}

// IsConnected returns true if the client is connected
func (m *MockOpenAIRealtimeClient) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.connected
}

// mockEvent simulates an event from the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) mockEvent(eventType string, data map[string]interface{}) {
	m.mutex.RLock()
	handlers := m.handlers[eventType]
	genericHandlers := m.handlers["*"]
	m.mutex.RUnlock()

	event := data
	event["type"] = eventType

	// Call handlers for this event type
	for _, handler := range handlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling mock event")
		}
	}

	// Call generic handlers
	for _, handler := range genericHandlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling mock event (generic handler)")
		}
	}
}