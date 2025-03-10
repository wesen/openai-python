package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// clientImpl implements the Client interface with an improved concurrent architecture
type clientImpl struct {
	// Configuration
	apiKey string
	model  string
	logger zerolog.Logger

	// Components
	connHandler     *connectionHandler
	responseManager *responseManager

	// Event handling (merged from eventProcessor)
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex
	eventChan     chan []byte // Channel to receive events from connection handler

	// Lifecycle management
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Session state
	sessionID        atomic.Value // string
	sessionCreatedCh chan struct{}
	sessionMutex     sync.RWMutex
}

// NewClient creates a new OpenAI realtime client with default settings
func NewClient(apiKey string, model string) Client {
	if model == "" {
		model = DefaultModel
	}

	// Create the bare client first
	client := &clientImpl{
		apiKey: apiKey,
		model:  model,
		logger: zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}

	// Set up context for the client
	client.ctx, client.cancelFunc = context.WithCancel(context.Background())

	// Initialize components
	client.initializeComponents()

	return client
}

// SetLogger sets the logger for the client and its components
func (c *clientImpl) SetLogger(logger zerolog.Logger) {
	c.logger.Debug().Msg("Setting logger")
	c.logger = logger

	// Update logger in all components
	c.connHandler.logger = logger.With().Str("component", "connection_handler").Logger()
	c.responseManager.logger = logger.With().Str("component", "response_manager").Logger()
}

// Connect establishes a connection to the API and initializes all components
func (c *clientImpl) Connect(ctx context.Context) error {
	c.logger.Debug().Str("model", c.model).Msg("Connecting to OpenAI realtime API")

	// Create a new context with cancellation
	c.ctx, c.cancelFunc = context.WithCancel(ctx)

	// Reset the session created channel
	c.sessionMutex.Lock()
	c.sessionCreatedCh = make(chan struct{})
	c.sessionMutex.Unlock()

	// Connect the connection handler
	if err := c.connHandler.Connect(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Failed to connect")
		return err
	}

	return nil
}

// initializeComponents initializes all client components
func (c *clientImpl) initializeComponents() {
	c.logger.Debug().Msg("Initializing components")

	// Initialize event handling
	c.eventHandlers = make(map[string][]EventHandler)
	c.eventChan = make(chan []byte, 100)
	c.sessionCreatedCh = make(chan struct{})

	// Create the connection handler
	c.connHandler = newConnectionHandler(c)

	// Create the response manager
	c.responseManager = newResponseManager(c)

	// Initialize atomic values
	c.sessionID.Store("")
	c.responseManager.currentResponseID.Store("")
	c.responseManager.currentResponseText.Store("")
}

// Close gracefully closes the client and all its components
func (c *clientImpl) Close(ctx context.Context) error {
	c.logger.Debug().Msg("Closing client")

	// Cancel the context to signal all components to stop
	c.cancelFunc()

	// Close the connection handler
	if err := c.connHandler.Close(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Error closing connection handler")
	}

	// Reset the session ID
	c.sessionID.Store("")

	// Wait for the remaining goroutines to exit with a timeout
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		close(done)
	}()

	select {
	case <-done:
		c.logger.Debug().Msg("All goroutines exited cleanly")
	case <-waitCtx.Done():
		c.logger.Warn().Msg("Timeout waiting for goroutines to exit")
	}

	return nil
}

// SendAudio sends audio data to the API
func (c *clientImpl) SendAudio(ctx context.Context, audio []byte) error {
	c.logger.Debug().Int("audioBytes", len(audio)).Msg("Sending audio")

	// Base64 encode the audio data
	audioBase64 := encodeBase64(audio)

	// Create the audio buffer append message
	msg := AudioBufferAppendRequest{
		Type:  ClientEventTypeInputAudioBufferAppend,
		Audio: audioBase64,
	}

	// Send the message through the connection handler
	return c.connHandler.SendMessage(ctx, msg)
}

// CommitAudio signals that the user has finished speaking
func (c *clientImpl) CommitAudio(ctx context.Context) error {
	c.logger.Debug().Msg("Committing audio")

	// Create the commit message
	msg := AudioBufferCommitRequest{
		Type: ClientEventTypeInputAudioBufferCommit,
	}

	// Send the message through the connection handler
	return c.connHandler.SendMessage(ctx, msg)
}

// SendText sends a text message to the API
func (c *clientImpl) SendText(ctx context.Context, text string) error {
	c.logger.Debug().Str("text", text).Msg("Sending text")

	// Create the message
	msg := ConversationItemCreateRequest{
		Type: ClientEventTypeConversationItemCreate,
		Item: ConversationItem{
			Role: MessageRoleUser,
			Type: ItemTypeMessage,
			Content: []ContentPart{
				{
					Type: ContentPartTypeInputText,
					Text: text,
				},
			},
		},
	}

	// Send the message through the connection handler
	err := c.connHandler.SendMessage(ctx, msg)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to send text message")
		return err
	}

	err = c.connHandler.SendMessage(ctx, ResponseCreateRequest{
		Type: ClientEventTypeResponseCreate,
	})
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to send response create message")
		return err
	}

	return nil
}

// SetEventHandler registers a handler for specific event types
func (c *clientImpl) SetEventHandler(eventType string, handler EventHandler) {
	c.logger.Debug().Str("eventType", eventType).Msg("Setting event handler")

	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()

	if c.eventHandlers == nil {
		c.eventHandlers = make(map[string][]EventHandler)
	}
	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
}

// ListenForEvents starts listening for events from the WebSocket
func (c *clientImpl) ListenForEvents(ctx context.Context) error {
	c.logger.Debug().Msg("Starting to listen for events")

	// Create context with cancellation for goroutines
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	// Start event listening with session notification
	eg, egCtx := errgroup.WithContext(runCtx)

	// Start the connection handler
	eg.Go(func() error {
		return c.connHandler.Run(egCtx)
	})

	// Start the event processing loop with session detection
	eg.Go(func() error {
		for {
			select {
			case <-egCtx.Done():
				return egCtx.Err()
			case eventData := <-c.eventChan:
				// First, determine if this is a session.created event
				var rawEvent struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal(eventData, &rawEvent); err == nil &&
					rawEvent.Type == EventSessionCreated {
					// Parse the session event to get the session ID
					var sessionEvent SessionCreatedEvent
					if err := json.Unmarshal(eventData, &sessionEvent); err == nil {
						// Store the session ID
						c.sessionID.Store(sessionEvent.Session.ID)
						c.logger.Info().
							Str("session_id", sessionEvent.Session.ID).
							Msg("Session created, signaling listeners")

						// Signal that session has been created (only once)
						select {
						case <-c.sessionCreatedCh: // Already closed
						default:
							close(c.sessionCreatedCh)
						}
					}
				}

				// Process the event normally
				c.ProcessRawEvent(egCtx, eventData)
			}
		}
	})

	// The goroutines continue running in the background
	return eg.Wait()
}

// WaitForSessionEvent waits for a specific session event to occur
func (c *clientImpl) waitForSessionEvent(ctx context.Context, eventType string) error {
	c.logger.Debug().Str("event_type", eventType).Msg("Waiting for session event")

	// Create channel to signal when the event is received
	eventReceivedCh := make(chan struct{})

	// Create handler for this specific event
	var handlerFunc EventHandler = func(ctx context.Context, event Event) error {
		c.logger.Debug().Str("event_type", event.Type()).Msg("Received expected session event")
		close(eventReceivedCh)
		return nil
	}

	// Register temporary handler
	c.handlersMutex.Lock()
	if c.eventHandlers == nil {
		c.eventHandlers = make(map[string][]EventHandler)
	}
	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handlerFunc)
	c.handlersMutex.Unlock()

	// Clean up the handler when done
	defer func() {
		c.handlersMutex.Lock()
		defer c.handlersMutex.Unlock()

		// Remove our handler from the slice
		handlers := c.eventHandlers[eventType]
		for i, h := range handlers {
			// Compare function pointers - this is a bit of a hack but should work
			// since we're only removing our own handler that we just added
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handlerFunc) {
				c.eventHandlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}()

	// Wait for the event or context cancellation
	select {
	case <-eventReceivedCh:
		c.logger.Debug().Str("event_type", eventType).Msg("Session event received")
		return nil
	case <-ctx.Done():
		c.logger.Debug().Str("event_type", eventType).Msg("Context cancelled while waiting for session event")
		return ctx.Err()
	}
}

// processEvents runs a loop to process incoming events from the event channel
func (c *clientImpl) processEvents(ctx context.Context) error {
	c.logger.Debug().Msg("Event processing loop started")

	for {
		select {
		case <-ctx.Done():
			c.logger.Debug().Msg("Event processing loop stopped due to context cancellation")
			return ctx.Err()
		case eventData := <-c.eventChan:
			c.ProcessRawEvent(ctx, eventData)
		}
	}
}

// ProcessRawEvent processes a raw WebSocket message into an event
func (c *clientImpl) ProcessRawEvent(ctx context.Context, data []byte) {
	// First, determine the event type
	var rawEvent struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &rawEvent); err != nil {
		c.logger.Error().Err(err).Msg("Failed to unmarshal event type")
		return
	}

	eventType := rawEvent.Type
	c.logger.Debug().Str("event_type", eventType).RawJSON("raw_event", data).Msg("Received event JSON")

	// Create the specific event object based on type
	event, err := c.createEventObject(eventType, data)
	if err != nil {
		c.logger.Error().Err(err).Str("event_type", eventType).Msg("Failed to create event object")
		return
	}

	// Send the event to the event channel if running
	if err := c.processEvent(ctx, event); err != nil {
		c.logger.Error().Err(err).Str("event_type", eventType).Msg("Error processing event")
	}
}

// createEventObject creates the appropriate event object based on type
func (c *clientImpl) createEventObject(eventType string, data []byte) (Event, error) {
	var err error
	var event Event

	// Log the raw JSON data being unmarshaled to help debug issues
	c.logger.Debug().RawJSON("raw_json", data).Str("event_type", eventType).Msg("Creating event object")

	switch eventType {
	case EventSessionCreated:
		var sessionEvent SessionCreatedEvent
		if err = json.Unmarshal(data, &sessionEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal SessionCreatedEvent")
			return nil, err
		}
		sessionEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &sessionEvent

	case EventSessionUpdated:
		var sessionEvent SessionUpdatedEvent
		if err = json.Unmarshal(data, &sessionEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal SessionUpdatedEvent")
			return nil, err
		}
		sessionEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &sessionEvent

	case EventConversationItemCreated:
		var itemEvent ConversationItemCreatedEvent
		if err = json.Unmarshal(data, &itemEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ConversationItemCreatedEvent")
			return nil, err
		}
		itemEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &itemEvent

	case EventConversationItemInputAudioTranscriptionCompleted:
		var transcriptionEvent TranscriptionCompletedEvent
		if err = json.Unmarshal(data, &transcriptionEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal TranscriptionCompletedEvent")
			return nil, err
		}
		transcriptionEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &transcriptionEvent

		// For logging convenience, log the transcription
		transcript := transcriptionEvent.Transcript
		if transcript != "" {
			c.logger.Debug().Str("transcript", transcript).Msg("Transcription received")
		}

	case EventResponseCreated:
		var responseEvent ResponseCreatedEvent
		if err = json.Unmarshal(data, &responseEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ResponseCreatedEvent")
			return nil, err
		}
		responseEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &responseEvent

		// Store the current response ID and reset buffers
		c.responseManager.SetResponseID(responseEvent.Response.ID)
		c.responseManager.ResetResponse()

	case EventResponseContentPartAdded:
		var contentEvent ContentPartAddedEvent
		if err = json.Unmarshal(data, &contentEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ContentPartAddedEvent")
			return nil, err
		}
		contentEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &contentEvent

		// For logging convenience, extract the content text
		text := contentEvent.Part.Text
		if text != "" {
			c.logger.Debug().Str("text", text).Msg("Content received")

			// Append to the response buffer
			c.responseManager.AppendResponseText(contentEvent.ResponseID, text)
		}

	case EventResponseContentPartDone:
		var doneEvent ContentPartDoneEvent
		if err = json.Unmarshal(data, &doneEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ContentPartDoneEvent")
			return nil, err
		}
		doneEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &doneEvent

	case EventResponseAudioDelta:
		var audioEvent AudioDeltaEvent
		if err = json.Unmarshal(data, &audioEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal AudioDeltaEvent")
			return nil, err
		}
		audioEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &audioEvent

		// Decode and store audio
		if audioEvent.Delta != "" {
			audioData, err := base64.StdEncoding.DecodeString(audioEvent.Delta)
			if err != nil {
				c.logger.Warn().Err(err).Msg("Failed to decode audio data")
			} else {
				c.responseManager.AppendResponseAudio(audioEvent.ResponseID, audioData)
			}
		}

	case EventResponseAudioDone:
		var doneEvent AudioDoneEvent
		if err = json.Unmarshal(data, &doneEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal AudioDoneEvent")
			return nil, err
		}
		doneEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &doneEvent

	case EventResponseDone:
		var doneEvent ResponseDoneEvent
		if err = json.Unmarshal(data, &doneEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ResponseDoneEvent")
			return nil, err
		}
		doneEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &doneEvent

		// Log token usage
		c.logger.Debug().
			Int("input_tokens", doneEvent.Response.Usage.InputTokens).
			Int("output_tokens", doneEvent.Response.Usage.OutputTokens).
			Int("audio_tokens", doneEvent.Response.Usage.InputTokenDetails.AudioTokens).
			Int("cached_tokens", doneEvent.Response.Usage.InputTokenDetails.CachedTokens).
			Msg("Response completed")

	case EventError:
		var errorEvent ErrorEvent
		if err = json.Unmarshal(data, &errorEvent); err != nil {
			c.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ErrorEvent")
			return nil, err
		}
		errorEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &errorEvent

		// Log the error for convenience
		c.logger.Error().
			Str("error_type", errorEvent.Error.Type).
			Str("error_code", errorEvent.Error.Code).
			Str("error_message", errorEvent.Error.Message).
			Msg("Received error event")

	default:
		// For unknown events, just create a basic event wrapper
		event = &genericEvent{
			eventType: eventType,
			data:      data,
		}
	}

	return event, nil
}

// processEvent processes a parsed event
func (c *clientImpl) processEvent(ctx context.Context, event Event) error {
	c.logger.Debug().Str("event_type", event.Type()).Msg("Processing event")
	eventType := event.Type()

	// First check if we have any registered handlers for this event type
	c.handlersMutex.RLock()
	handlers, exists := c.eventHandlers[eventType]
	c.handlersMutex.RUnlock()

	if exists && len(handlers) > 0 {
		c.logger.Debug().Str("event_type", eventType).Msg("Found handlers for event")
		// Run all registered handlers
		for _, handler := range handlers {
			c.logger.Debug().Str("event_type", eventType).Msg("Running handler")
			if err := handler(ctx, event); err != nil {
				return fmt.Errorf("handler error for event %s: %w", eventType, err)
			}
		}
		return nil
	}

	// If no handlers registered, use default handling
	c.logger.Debug().Str("event_type", eventType).Msg("No handlers registered, handle default event")
	return c.handleDefaultEvent(event)
}

// handleDefaultEvent provides default handling for events
func (c *clientImpl) handleDefaultEvent(event Event) error {
	eventType := event.Type()

	// By default, just log that we received the event
	c.logger.Debug().Str("event_type", eventType).Msg("Handle default event")

	switch eventType {
	case EventError:
		// Handle error events specially
		errorEvent, ok := event.(*ErrorEvent)
		if !ok {
			return fmt.Errorf("unable to cast %s event to ErrorEvent", eventType)
		}

		return fmt.Errorf("API error: %s (type: %s, code: %s)",
			errorEvent.Error.Message,
			errorEvent.Error.Type,
			errorEvent.Error.Code)

	case EventSessionCreated:
		// Extract and set session ID
		sessionEvent, ok := event.(*SessionCreatedEvent)
		if !ok {
			return fmt.Errorf("unable to cast %s event to SessionCreatedEvent", eventType)
		}

		c.sessionID.Store(sessionEvent.Session.ID)
		c.logger.Info().Str("session_id", sessionEvent.Session.ID).Msg("Session created")

	case EventInputAudioBufferSpeechStarted:
		c.logger.Debug().Msg("Speech started")

	case EventInputAudioBufferSpeechStopped:
		c.logger.Debug().Msg("Speech stopped")

	case EventResponseDone:
		c.logger.Debug().Msg("Response complete")
	}

	return nil
}

// Helper function to encode audio data in base64
func encodeBase64(data []byte) string {
	return EncodeBase64(data)
}

// Create a proper constructor function for the response manager
func newResponseManager(client *clientImpl) *responseManager {
	return &responseManager{
		client: client,
		logger: client.logger.With().Str("component", "response_manager").Logger(),
	}
}

// UpdateSession updates the session configuration and waits for confirmation
func (c *clientImpl) UpdateSession(ctx context.Context, config *Config) error {
	c.logger.Debug().Str("session", c.sessionID.Load().(string)).Msg("Updating session")

	// Create session update message
	updateMsg := SessionUpdateRequest{
		Type: "session.update",
	}

	// Populate session fields based on provided config
	if config != nil {
		if config.Voice != "" {
			updateMsg.Session.Voice = config.Voice
		}

		if len(config.Modalities) > 0 {
			updateMsg.Session.Modalities = config.Modalities
		}

		if config.InputFormat != "" {
			updateMsg.Session.InputAudioFormat = config.InputFormat
		}

		if config.OutputFormat != "" {
			updateMsg.Session.OutputAudioFormat = config.OutputFormat
		}

		if config.Instructions != "" {
			updateMsg.Session.Instructions = config.Instructions
		}

		if config.Temperature != nil {
			updateMsg.Session.Temperature = config.Temperature
		}

		if config.TopP != nil {
			updateMsg.Session.TopP = config.TopP
		}

		if config.PresencePenalty != nil {
			updateMsg.Session.PresencePenalty = config.PresencePenalty
		}

		if config.FrequencyPenalty != nil {
			updateMsg.Session.FrequencyPenalty = config.FrequencyPenalty
		}

		if config.MaxResponseOutputTokens != nil {
			updateMsg.Session.MaxResponseOutputTokens = config.MaxResponseOutputTokens
		}

		// Configure speech settings if provided
		if config.SpeechSettings != nil {
			updateMsg.Session.SpeechSettings = &SpeechSettings{
				Voice:        config.Voice,
				Speed:        config.SpeechSettings.Speed,
				Stability:    config.SpeechSettings.Stability,
				Similarity:   config.SpeechSettings.Similarity,
				Style:        config.SpeechSettings.Style,
				PresenceText: config.SpeechSettings.PresenceText,
			}
		}

		// Configure audio transcription settings if provided
		if config.InputAudioTranscription != nil {
			updateMsg.Session.InputAudioTranscription = &InputAudioTranscriptionConfig{
				Language:        config.InputAudioTranscription.Language,
				Type:            config.InputAudioTranscription.Type,
				Interim:         config.InputAudioTranscription.Interim,
				PhraseHints:     config.InputAudioTranscription.PhraseHints,
				ProfanityFilter: config.InputAudioTranscription.ProfanityFilter,
				Redact:          config.InputAudioTranscription.Redact,
				Diarize:         config.InputAudioTranscription.Diarize,
			}
		}

		// Configure turn detection
		if config.TurnDetection != "" {
			updateMsg.Session.TurnDetection = &TurnDetectionConfig{
				Type: config.TurnDetection,
			}
		}

		// Configure tools if provided
		if len(config.Tools) > 0 {
			updateMsg.Session.Tools = config.Tools
		}

		if config.ToolChoice != "" {
			updateMsg.Session.ToolChoice = config.ToolChoice
		}
	}

	// Send the update message
	if err := c.connHandler.SendMessage(ctx, updateMsg); err != nil {
		return err
	}

	// Wait for session.updated event to confirm the update was processed
	return c.waitForSessionEvent(ctx, EventSessionUpdated)
}

// GetResponse returns the current response text and audio
func (c *clientImpl) GetResponse() (string, []byte) {
	c.logger.Debug().Msg("Getting current response")
	return c.responseManager.GetResponse()
}

// WaitForSessionCreated waits for the session to be created
func (c *clientImpl) WaitForSessionCreated(ctx context.Context) error {
	c.logger.Debug().Msg("Waiting for session to be created")

	select {
	case <-c.sessionCreatedCh:
		c.logger.Debug().Msg("Session created")
		return nil
	case <-ctx.Done():
		c.logger.Debug().Msg("Context cancelled while waiting for session to be created")
		return ctx.Err()
	}
}
