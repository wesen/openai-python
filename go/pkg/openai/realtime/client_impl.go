package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Constants for default values
const (
	DefaultModel      = "gpt-4o-realtime-preview-2024-10-01"
	BaseURL           = "wss://api.openai.com/v1/realtime"
	ConnectionTimeout = 10 * time.Second
	PingInterval      = 20 * time.Second
	PongWait          = 30 * time.Second
)

// Component interface for all components of the client
type component interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// clientImpl implements the Client interface with an improved concurrent architecture
type clientImpl struct {
	// Configuration
	apiKey string
	model  string
	logger zerolog.Logger

	// Components
	connManager     *connectionManager
	messageSender   *messageSender
	eventProcessor  *eventProcessor
	responseManager *responseManager

	// Lifecycle management
	ctx        context.Context
	cancelFunc context.CancelFunc
	eg         *errgroup.Group
	running    atomic.Bool

	// Session state
	sessionID atomic.Value // string
}

// connectionManager handles WebSocket connection and lifecycle
type connectionManager struct {
	client     *clientImpl
	conn       *websocket.Conn
	connMutex  sync.RWMutex
	pingTicker *time.Ticker
	logger     zerolog.Logger
}

// messageSender handles sending messages to the WebSocket
type messageSender struct {
	client    *clientImpl
	conn      *websocket.Conn
	connMutex *sync.RWMutex // Points to the same mutex as connectionManager
	msgQueue  chan interface{}
	logger    zerolog.Logger
}

// eventProcessor handles incoming events from the WebSocket
type eventProcessor struct {
	client        *clientImpl
	conn          *websocket.Conn
	connMutex     *sync.RWMutex // Points to the same mutex as connectionManager
	eventChan     chan Event
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex
	logger        zerolog.Logger
}

// responseManager manages response assembly
type responseManager struct {
	client               *clientImpl
	currentResponseID    atomic.Value // string
	currentResponseText  atomic.Value // string
	responseMutex        sync.Mutex
	currentResponseAudio []byte
	logger               zerolog.Logger
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

// NewClient creates a new client with the provided API key and model
func NewClient(apiKey string, model string) Client {
	if model == "" {
		model = DefaultModel
	}

	// Set up a default logger that logs to stderr
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	// Create the client instance
	c := &clientImpl{
		apiKey: apiKey,
		model:  model,
		logger: logger,
	}

	// Create shared connection mutex
	connMutex := &sync.RWMutex{}

	// Initialize components
	c.connManager = &connectionManager{
		client:    c,
		connMutex: *connMutex,
		logger:    logger.With().Str("component", "connection_manager").Logger(),
	}

	c.messageSender = &messageSender{
		client:    c,
		connMutex: connMutex,
		msgQueue:  make(chan interface{}, 100),
		logger:    logger.With().Str("component", "message_sender").Logger(),
	}

	c.eventProcessor = &eventProcessor{
		client:        c,
		connMutex:     connMutex,
		eventChan:     make(chan Event, 100),
		eventHandlers: make(map[string][]EventHandler),
		logger:        logger.With().Str("component", "event_processor").Logger(),
	}

	c.responseManager = &responseManager{
		client: c,
		logger: logger.With().Str("component", "response_manager").Logger(),
	}

	// Initialize atomic values
	c.sessionID.Store("")
	c.responseManager.currentResponseID.Store("")
	c.responseManager.currentResponseText.Store("")

	return c
}

// SetLogger sets the logger for the client
func (c *clientImpl) SetLogger(logger zerolog.Logger) {
	c.logger = logger

	// Update logger for all components
	c.connManager.logger = logger.With().Str("component", "connection_manager").Logger()
	c.messageSender.logger = logger.With().Str("component", "message_sender").Logger()
	c.eventProcessor.logger = logger.With().Str("component", "event_processor").Logger()
	c.responseManager.logger = logger.With().Str("component", "response_manager").Logger()
}

// Connect establishes a WebSocket connection with the OpenAI Realtime API
func (c *clientImpl) Connect(ctx context.Context) error {
	// Check if already connected and running
	if c.running.Load() {
		c.logger.Debug().Msg("Client is already connected and running")
		return errors.New("client is already connected and running")
	}

	// Create a context with cancellation for the client lifecycle
	c.ctx, c.cancelFunc = context.WithCancel(context.Background())

	// Create an error group for coordinating goroutines
	c.eg, c.ctx = errgroup.WithContext(c.ctx)

	// Establish WebSocket connection
	if err := c.connManager.connect(ctx); err != nil {
		c.cancelFunc()
		return err
	}

	// Start all components
	if err := c.startComponents(); err != nil {
		c.Close(ctx)
		return err
	}

	c.running.Store(true)
	return nil
}

// startComponents starts all client components
func (c *clientImpl) startComponents() error {
	// Get the connection reference
	c.connManager.connMutex.RLock()
	conn := c.connManager.conn
	c.connManager.connMutex.RUnlock()

	// Set connection reference in components
	c.messageSender.conn = conn
	c.eventProcessor.conn = conn

	// Start components with errgroup
	c.eg.Go(func() error {
		return c.messageSender.Start(c.ctx)
	})

	c.eg.Go(func() error {
		return c.eventProcessor.Start(c.ctx)
	})

	c.eg.Go(func() error {
		return c.connManager.Start(c.ctx)
	})

	return nil
}

// Close terminates the WebSocket connection
func (c *clientImpl) Close(ctx context.Context) error {
	if !c.running.Load() {
		return nil
	}

	c.logger.Debug().Msg("Closing client connection")

	// Cancel the client context to signal all components to stop
	c.cancelFunc()

	// Create a timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Wait for all goroutines to complete or timeout
	shutdownDone := make(chan struct{})
	go func() {
		_ = c.eg.Wait()
		close(shutdownDone)
	}()

	// Wait for shutdown or timeout
	select {
	case <-shutdownDone:
		c.logger.Debug().Msg("All components shut down gracefully")
	case <-shutdownCtx.Done():
		c.logger.Warn().Msg("Shutdown timed out, some components may not have shut down cleanly")
	}

	// Close the connection explicitly
	c.connManager.connMutex.Lock()
	if c.connManager.conn != nil {
		err := c.connManager.conn.Close()
		c.connManager.conn = nil
		c.connManager.connMutex.Unlock()
		if err != nil {
			c.logger.Warn().Err(err).Msg("Error closing WebSocket connection")
		}
	} else {
		c.connManager.connMutex.Unlock()
	}

	c.running.Store(false)
	c.logger.Info().Msg("Client closed")
	return nil
}

// SendAudio sends audio data to the API
func (c *clientImpl) SendAudio(ctx context.Context, audio []byte) error {
	if !c.running.Load() {
		return errors.New("client is not connected")
	}

	// Create the message
	msg := map[string]interface{}{
		"type":  "input_audio_buffer.append",
		"audio": base64.StdEncoding.EncodeToString(audio),
	}

	// Send through the message sender
	return c.messageSender.SendMessage(ctx, msg)
}

// CommitAudio signals that the user has finished speaking
func (c *clientImpl) CommitAudio(ctx context.Context) error {
	if !c.running.Load() {
		return errors.New("client is not connected")
	}

	// Create the message
	msg := map[string]string{
		"type": "input_audio_buffer.commit",
	}

	// Send through the message sender
	return c.messageSender.SendMessage(ctx, msg)
}

// SendText sends a text message to the API
func (c *clientImpl) SendText(ctx context.Context, text string) error {
	if !c.running.Load() {
		return errors.New("client is not connected")
	}

	// Create the message
	msg := map[string]interface{}{
		"type": "input_text",
		"text": text,
	}

	// Send through the message sender
	return c.messageSender.SendMessage(ctx, msg)
}

// SetEventHandler registers a handler for a specific event type
func (c *clientImpl) SetEventHandler(eventType string, handler EventHandler) {
	c.eventProcessor.SetEventHandler(eventType, handler)
}

// ListenForEvents starts processing events from the API
func (c *clientImpl) ListenForEvents(ctx context.Context) error {
	// This method is kept for API compatibility
	// With the new architecture, event listening starts automatically on Connect()
	if !c.running.Load() {
		return errors.New("client is not connected")
	}
	return nil
}

// UpdateSession updates the session configuration
func (c *clientImpl) UpdateSession(ctx context.Context, config *Config) error {
	if !c.running.Load() {
		return errors.New("client is not connected")
	}

	// Build session update
	sessionUpdate := map[string]interface{}{
		"type":    "session.update",
		"session": map[string]interface{}{},
	}

	session := sessionUpdate["session"].(map[string]interface{})

	// Only add fields that are set
	if config.Voice != "" {
		session["voice"] = config.Voice
	}
	if len(config.Modalities) > 0 {
		session["modalities"] = config.Modalities
	}
	if config.InputFormat != "" {
		session["input_audio_format"] = config.InputFormat
	}
	if config.OutputFormat != "" {
		session["output_audio_format"] = config.OutputFormat
	}
	if config.Instructions != "" {
		session["instructions"] = config.Instructions
	}
	if config.TurnDetection != "" {
		turnDetection := map[string]string{
			"type": config.TurnDetection,
		}
		session["turn_detection"] = turnDetection
	}
	if config.Temperature != nil {
		session["temperature"] = *config.Temperature
	}
	if config.Logger != nil {
		// Don't actually send this to the API
		c.SetLogger(*config.Logger)
	}

	// Send through the message sender
	return c.messageSender.SendMessage(ctx, sessionUpdate)
}

// GetResponse returns the current response text and audio
func (c *clientImpl) GetResponse() (string, []byte) {
	return c.responseManager.GetResponse()
}

// ================== Connection Manager Implementation ==================

// connect establishes a WebSocket connection
func (cm *connectionManager) connect(ctx context.Context) error {
	cm.logger.Info().
		Str("model", cm.client.model).
		Str("url", BaseURL).
		Msg("Connecting to OpenAI Realtime API")

	// Create a context with timeout
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Prepare dialer and headers
	dialer := websocket.DefaultDialer
	url := fmt.Sprintf("%s?model=%s", BaseURL, cm.client.model)

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+cm.client.apiKey)
	headers.Add("OpenAI-Beta", "realtime=v1")

	cm.logger.Debug().Str("url", url).Msg("Dialing WebSocket")

	// Establish connection
	conn, resp, err := dialer.DialContext(connectCtx, url, headers)
	if err != nil {
		if resp != nil {
			body, readErr := io.ReadAll(resp.Body)
			if readErr == nil {
				cm.logger.Error().
					Err(err).
					Int("status_code", resp.StatusCode).
					Str("response_body", string(body)).
					Msg("Failed to connect to OpenAI Realtime API")
			} else {
				cm.logger.Error().
					Err(err).
					Int("status_code", resp.StatusCode).
					Msg("Failed to connect to OpenAI Realtime API")
			}
		} else {
			cm.logger.Error().Err(err).Msg("Failed to connect to OpenAI Realtime API")
		}
		return fmt.Errorf("failed to connect to OpenAI Realtime API: %w", err)
	}

	// Store the connection
	cm.connMutex.Lock()
	cm.conn = conn
	cm.connMutex.Unlock()

	// Configure WebSocket handlers
	conn.SetPingHandler(func(appData string) error {
		cm.logger.Debug().Str("data", appData).Msg("Received ping, sending pong")
		err := conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
		if err != nil {
			cm.logger.Error().Err(err).Msg("Failed to send pong")
		}
		return nil
	})

	// Set initial read deadline for session establishment
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))

	// Create channel for session establishment signal
	sessionCreated := make(chan struct{})

	// Handle incoming messages during connection phase
	go func() {
		defer conn.SetReadDeadline(time.Time{}) // Reset deadline

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				cm.logger.Error().Err(err).Msg("Error reading initial messages")
				return
			}

			// Check for session.created event
			var eventType struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(message, &eventType); err != nil {
				cm.logger.Error().Err(err).Msg("Failed to parse event type")
				continue
			}

			// Process session.created event
			if eventType.Type == EventSessionCreated {
				var sessionEvent SessionCreatedEvent
				if err := json.Unmarshal(message, &sessionEvent); err != nil {
					cm.logger.Error().Err(err).Msg("Failed to parse session.created event")
					continue
				}

				// Store session ID
				cm.client.sessionID.Store(sessionEvent.Session.SessionID)
				cm.logger.Info().
					Str("session_id", sessionEvent.Session.SessionID).
					Str("model", sessionEvent.Session.Model).
					Str("voice", sessionEvent.Session.Voice).
					Msg("Session created")

				// Signal session creation
				close(sessionCreated)

				// Add to event processing channel
				cm.client.eventProcessor.ProcessRawEvent(message)
			} else {
				// Process other events
				cm.client.eventProcessor.ProcessRawEvent(message)
			}
		}
	}()

	// Wait for session creation or timeout
	select {
	case <-sessionCreated:
		cm.logger.Debug().Msg("Session created successfully")
	case <-connectCtx.Done():
		// Close connection on timeout
		conn.Close()
		return errors.New("timed out waiting for session.created event")
	}

	cm.logger.Debug().Msg("Connection established successfully")
	return nil
}

// Start begins the connection manager operation
func (cm *connectionManager) Start(ctx context.Context) error {
	cm.logger.Debug().Msg("Starting connection manager")

	// Start ping ticker
	cm.pingTicker = time.NewTicker(PingInterval)
	defer cm.pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			cm.logger.Debug().Msg("Connection manager stopping due to context cancellation")
			return nil

		case <-cm.pingTicker.C:
			// Send ping
			cm.connMutex.RLock()
			if cm.conn != nil {
				cm.logger.Debug().Msg("Sending ping to keep connection alive")
				err := cm.conn.WriteControl(
					websocket.PingMessage,
					[]byte{},
					time.Now().Add(5*time.Second),
				)
				cm.connMutex.RUnlock()

				if err != nil {
					cm.logger.Error().Err(err).Msg("Failed to send ping")
					// Try to reconnect on ping failure
					// Note: Implement reconnection logic here if needed
				}
			} else {
				cm.connMutex.RUnlock()
				cm.logger.Warn().Msg("Connection is nil, cannot send ping")
			}
		}
	}
}

// Stop halts the connection manager operation
func (cm *connectionManager) Stop(ctx context.Context) error {
	cm.logger.Debug().Msg("Stopping connection manager")

	// Stop ping ticker if running
	if cm.pingTicker != nil {
		cm.pingTicker.Stop()
	}

	// Connection closing is handled by the main Close method
	return nil
}

// ================== Message Sender Implementation ==================

// SendMessage queues a message to be sent
func (ms *messageSender) SendMessage(ctx context.Context, msg interface{}) error {
	select {
	case ms.msgQueue <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full, this is usually a sign of a bigger problem
		return errors.New("message queue is full, cannot send message")
	}
}

// Start begins the message sender operation
func (ms *messageSender) Start(ctx context.Context) error {
	ms.logger.Debug().Msg("Starting message sender")

	for {
		select {
		case <-ctx.Done():
			ms.logger.Debug().Msg("Message sender stopping due to context cancellation")
			return nil

		case msg, ok := <-ms.msgQueue:
			if !ok {
				ms.logger.Debug().Msg("Message queue closed, stopping sender")
				return nil
			}

			// Send the message
			if err := ms.sendJSONMessage(msg); err != nil {
				ms.logger.Error().Err(err).Interface("message", msg).Msg("Failed to send message")
				// Continue processing other messages
			}
		}
	}
}

// Stop halts the message sender operation
func (ms *messageSender) Stop(ctx context.Context) error {
	ms.logger.Debug().Msg("Stopping message sender")
	// No specific cleanup needed as the main loop will terminate due to context cancellation
	return nil
}

// sendJSONMessage sends a JSON message to the WebSocket with logging
func (ms *messageSender) sendJSONMessage(msg interface{}) error {
	// Marshal to JSON for logging
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		ms.logger.Error().Err(err).Msg("Failed to marshal message to JSON")
		return err
	}

	// Log the outgoing message
	ms.logger.Debug().
		Str("type", fmt.Sprintf("%T", msg)).
		RawJSON("body", jsonBytes).
		Int("size", len(jsonBytes)).
		Msg("Sending WebSocket message")

	// Acquire connection lock for sending
	ms.connMutex.RLock()
	defer ms.connMutex.RUnlock()

	// Check if connection is available
	if ms.conn == nil {
		return errors.New("not connected")
	}

	// Send the message
	return ms.conn.WriteJSON(msg)
}

// ================== Event Processor Implementation ==================

// SetEventHandler registers a handler for an event type
func (ep *eventProcessor) SetEventHandler(eventType string, handler EventHandler) {
	ep.handlersMutex.Lock()
	defer ep.handlersMutex.Unlock()

	ep.eventHandlers[eventType] = append(ep.eventHandlers[eventType], handler)
}

// ProcessRawEvent processes a raw WebSocket message
func (ep *eventProcessor) ProcessRawEvent(data []byte) {
	// Parse the event type
	var eventMap map[string]interface{}
	if err := json.Unmarshal(data, &eventMap); err != nil {
		ep.logger.Error().Err(err).Str("data", string(data)).Msg("Failed to parse message JSON")
		return
	}

	// Extract the event type
	eventType, ok := eventMap["type"].(string)
	if !ok {
		ep.logger.Error().Str("data", string(data)).Msg("Message missing 'type' field")
		return
	}

	// Create the appropriate event object
	event, err := ep.createEventObject(eventType, data)
	if err != nil {
		ep.logger.Error().Err(err).Str("event_type", eventType).Msg("Error creating event object")
		return
	}

	// Send event to channel for processing
	select {
	case ep.eventChan <- event:
		// Event queued successfully
	default:
		ep.logger.Warn().Str("event_type", eventType).Msg("Event channel full, dropping event")
	}
}

// createEventObject creates the appropriate event object based on type
func (ep *eventProcessor) createEventObject(eventType string, data []byte) (Event, error) {
	var event Event
	var err error

	switch eventType {
	case EventSessionCreated:
		var e SessionCreatedEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	case EventSessionUpdated:
		var e SessionUpdatedEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	case EventConversationItemCreated:
		var e ConversationItemCreatedEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	case EventTranscriptionCompleted:
		var e TranscriptionCompletedEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	case EventResponseCreated:
		var e ResponseCreatedEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

		// Store the response ID for tracking
		ep.client.responseManager.SetResponseID(e.ResponseID)

	case EventContentPartAdded:
		var e ContentPartAddedEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

		// Update the response text
		ep.client.responseManager.AppendResponseText(e.ResponseID, e.Content.Text)

	case EventContentPartDone:
		var e ContentPartDoneEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	case EventAudioDelta:
		var e AudioDeltaEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

		// Update the response audio
		if e.Audio != "" {
			audioBytes, decodeErr := base64.StdEncoding.DecodeString(e.Audio)
			if decodeErr == nil {
				ep.client.responseManager.AppendResponseAudio(e.ResponseID, audioBytes)
			}
		}

	case EventAudioDone:
		var e AudioDoneEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	case EventResponseDone:
		var e ResponseDoneEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	case EventError:
		var e ErrorEvent
		err = json.Unmarshal(data, &e)
		e.BaseEvent = NewBaseEvent(eventType, data)
		event = &e

	default:
		ep.logger.Warn().Str("event_type", eventType).Msg("Unknown event type")
		// For unknown event types, create a generic event
		event = &genericEvent{
			eventType: eventType,
			data:      data,
		}
	}

	if err != nil {
		return nil, err
	}

	return event, nil
}

// Start begins the event processor operation
func (ep *eventProcessor) Start(ctx context.Context) error {
	ep.logger.Debug().Msg("Starting event processor")

	// Start the WebSocket listener
	ep.client.eg.Go(func() error {
		return ep.listenLoop(ctx)
	})

	// Process events from the channel
	for {
		select {
		case <-ctx.Done():
			ep.logger.Debug().Msg("Event processor stopping due to context cancellation")
			return nil

		case event, ok := <-ep.eventChan:
			if !ok {
				ep.logger.Debug().Msg("Event channel closed, stopping processor")
				return nil
			}

			// Process the event
			if err := ep.processEvent(event); err != nil {
				ep.logger.Error().Err(err).Str("event_type", event.Type()).Msg("Error processing event")
			}
		}
	}
}

// Stop halts the event processor operation
func (ep *eventProcessor) Stop(ctx context.Context) error {
	ep.logger.Debug().Msg("Stopping event processor")
	// No specific cleanup needed as the main loop will terminate due to context cancellation
	return nil
}

// listenLoop reads messages from the WebSocket connection
func (ep *eventProcessor) listenLoop(ctx context.Context) error {
	ep.logger.Debug().Msg("Starting WebSocket listen loop")

	defer ep.logger.Debug().Msg("WebSocket listen loop ended")

	for {
		// Check if context is done
		select {
		case <-ctx.Done():
			ep.logger.Debug().Msg("Context cancelled, stopping WebSocket listen loop")
			return nil
		default:
			// Continue
		}

		// Get current connection
		ep.connMutex.RLock()
		conn := ep.conn
		ep.connMutex.RUnlock()

		// Check if connection is closed
		if conn == nil {
			ep.logger.Error().Msg("WebSocket connection is nil, stopping listen loop")
			return errors.New("connection is nil")
		}

		// Read message from WebSocket with a timeout
		conn.SetReadDeadline(time.Now().Add(PongWait))
		messageType, message, err := conn.ReadMessage()

		// Reset the read deadline
		conn.SetReadDeadline(time.Time{})

		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				ep.logger.Info().Err(err).Msg("WebSocket closed normally")
			} else if websocket.IsUnexpectedCloseError(err) {
				ep.logger.Error().Err(err).Msg("WebSocket closed unexpectedly")
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				ep.logger.Warn().Msg("WebSocket read timeout, continuing")
				continue
			} else {
				ep.logger.Error().Err(err).Msg("Error reading from WebSocket")
			}
			return err
		}

		// Log received message details
		if len(message) > 1000 {
			// For larger messages that might contain audio data, truncate the logging
			ep.logger.Debug().
				Int("type", messageType).
				Int("size", len(message)).
				Str("preview", string(message[:100])+"...").
				Msg("Received WebSocket message")
		} else {
			// For smaller messages, log the full content for debug purposes
			ep.logger.Debug().
				Int("type", messageType).
				Int("size", len(message)).
				RawJSON("body", message).
				Msg("Received WebSocket message")
		}

		// Process the raw event
		ep.ProcessRawEvent(message)
	}
}

// processEvent handles an event and dispatches it to registered handlers
func (ep *eventProcessor) processEvent(event Event) error {
	eventType := event.Type()

	ep.logger.Debug().
		Str("event_type", eventType).
		Msg("Processing event")

	// Find handlers for this event type
	ep.handlersMutex.RLock()
	handlers, exists := ep.eventHandlers[eventType]
	ep.handlersMutex.RUnlock()

	// Call registered handlers
	if exists && len(handlers) > 0 {
		for _, handler := range handlers {
			if err := handler(event); err != nil {
				ep.logger.Error().
					Err(err).
					Str("event_type", eventType).
					Msg("Error in event handler")
			}
		}
	} else {
		// No handlers found, use default handler for common events
		if err := ep.handleDefaultEvent(event); err != nil {
			ep.logger.Error().
				Err(err).
				Str("event_type", eventType).
				Msg("Error in default event handler")
		}
	}

	return nil
}

// handleDefaultEvent provides default handling for common events
func (ep *eventProcessor) handleDefaultEvent(event Event) error {
	switch event.Type() {
	case EventSessionCreated:
		if e, ok := event.(*SessionCreatedEvent); ok {
			ep.client.sessionID.Store(e.Session.SessionID)
		}

	case EventResponseDone:
		if e, ok := event.(*ResponseDoneEvent); ok {
			ep.logger.Info().
				Str("response_id", e.ResponseID).
				Int("input_tokens", e.Usage.InputTokens).
				Int("output_tokens", e.Usage.OutputTokens).
				Int("audio_tokens", e.Usage.AudioTokens).
				Msg("Response completed")

			// Reset for next response
			ep.client.responseManager.ResetResponse()
		}

	case EventError:
		if e, ok := event.(*ErrorEvent); ok {
			ep.logger.Error().
				Str("code", e.Code).
				Str("message", e.Message).
				Msg("Received error event from API")
		}
	}

	return nil
}

// ================== Response Manager Implementation ==================

// SetResponseID sets the current response ID
func (rm *responseManager) SetResponseID(responseID string) {
	rm.currentResponseID.Store(responseID)
	rm.responseMutex.Lock()
	rm.currentResponseText.Store("")
	rm.currentResponseAudio = nil
	rm.responseMutex.Unlock()
}

// AppendResponseText appends text to the current response
func (rm *responseManager) AppendResponseText(responseID string, text string) {
	currentID := rm.currentResponseID.Load().(string)
	if currentID == responseID {
		currentText := rm.currentResponseText.Load().(string)
		rm.currentResponseText.Store(currentText + text)
	}
}

// AppendResponseAudio appends audio to the current response
func (rm *responseManager) AppendResponseAudio(responseID string, audio []byte) {
	currentID := rm.currentResponseID.Load().(string)
	if currentID == responseID {
		rm.responseMutex.Lock()
		rm.currentResponseAudio = append(rm.currentResponseAudio, audio...)
		rm.responseMutex.Unlock()
	}
}

// ResetResponse resets the response buffer
func (rm *responseManager) ResetResponse() {
	rm.currentResponseID.Store("")
	rm.currentResponseText.Store("")
	rm.responseMutex.Lock()
	rm.currentResponseAudio = nil
	rm.responseMutex.Unlock()
}

// GetResponse returns the current response text and audio
func (rm *responseManager) GetResponse() (string, []byte) {
	text := rm.currentResponseText.Load().(string)

	rm.responseMutex.Lock()
	// Create a copy of the audio to avoid race conditions
	audioBytes := make([]byte, len(rm.currentResponseAudio))
	copy(audioBytes, rm.currentResponseAudio)
	rm.responseMutex.Unlock()

	return text, audioBytes
}
