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
)

// clientImpl implements the Client interface with an improved concurrent architecture
type clientImpl struct {
	// Configuration
	apiKey string
	model  string
	logger zerolog.Logger

	// Components
	connHandler     *connectionHandler
	eventProcessor  *eventProcessor
	responseManager *responseManager

	// Lifecycle management
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Session state
	sessionID atomic.Value // string
}

// eventProcessor handles processing events from the WebSocket
type eventProcessor struct {
	client        *clientImpl
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex
	logger        zerolog.Logger
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
	c.eventProcessor.logger = logger.With().Str("component", "event_processor").Logger()
	c.responseManager.logger = logger.With().Str("component", "response_manager").Logger()
}

// Connect establishes a connection to the API and initializes all components
func (c *clientImpl) Connect(ctx context.Context) error {
	c.logger.Debug().Str("model", c.model).Msg("Connecting to OpenAI realtime API")

	// Create a new context with cancellation
	c.ctx, c.cancelFunc = context.WithCancel(ctx)

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

	// Create the connection handler (replaces connectionManager and messageSender)
	c.connHandler = newConnectionHandler(c)

	// Create the event processor and response manager
	c.eventProcessor = newEventProcessor(c)
	c.responseManager = newResponseManager(c)

	// Set up bidirectional references
	c.connHandler.SetEventProcessor(c.eventProcessor)

	// Initialize atomic values
	c.sessionID.Store("")
	c.responseManager.currentResponseID.Store("")
	c.responseManager.currentResponseText.Store("")
}

// Close gracefully closes the client and all its components
func (c *clientImpl) Close(ctx context.Context) error {
	c.logger.Debug().Msg("Closing client")

	c.logger.Debug().Msg("Closing client")

	// Cancel the context to signal all components to stop
	c.cancelFunc()

	// Close the connection handler
	if err := c.connHandler.Close(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Error closing connection handler")
	}

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
	return c.connHandler.SendMessage(ctx, msg)
}

// SetEventHandler registers a handler for specific event types
func (c *clientImpl) SetEventHandler(eventType string, handler EventHandler) {
	c.logger.Debug().Str("eventType", eventType).Msg("Setting event handler")
	c.eventProcessor.SetEventHandler(eventType, handler)
}

// ListenForEvents starts listening for events from the WebSocket
func (c *clientImpl) ListenForEvents(ctx context.Context) error {
	c.logger.Debug().Msg("Starting to listen for events")

	return c.connHandler.Run(ctx)
}

// UpdateSession updates the session configuration
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
	return c.connHandler.SendMessage(ctx, updateMsg)
}

// GetResponse returns the current response text and audio
func (c *clientImpl) GetResponse() (string, []byte) {
	c.logger.Debug().Msg("Getting current response")
	return c.responseManager.GetResponse()
}

// Helper function to encode audio data in base64
func encodeBase64(data []byte) string {
	return EncodeBase64(data)
}

// Create a proper constructor function for the event processor
func newEventProcessor(client *clientImpl) *eventProcessor {
	return &eventProcessor{
		client:        client,
		eventHandlers: make(map[string][]EventHandler),
		logger:        client.logger.With().Str("component", "event_processor").Logger(),
	}
}

// Create a proper constructor function for the response manager
func newResponseManager(client *clientImpl) *responseManager {
	return &responseManager{
		client: client,
		logger: client.logger.With().Str("component", "response_manager").Logger(),
	}
}

// SetEventHandler registers a handler for a specific event type
func (ep *eventProcessor) SetEventHandler(eventType string, handler EventHandler) {
	ep.handlersMutex.Lock()
	defer ep.handlersMutex.Unlock()

	if ep.eventHandlers == nil {
		ep.eventHandlers = make(map[string][]EventHandler)
	}
	ep.eventHandlers[eventType] = append(ep.eventHandlers[eventType], handler)
}

// ProcessRawEvent processes a raw WebSocket message into an event
func (ep *eventProcessor) ProcessRawEvent(ctx context.Context, data []byte) {
	// First, determine the event type
	var rawEvent struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &rawEvent); err != nil {
		ep.logger.Error().Err(err).Msg("Failed to unmarshal event type")
		return
	}

	eventType := rawEvent.Type
	ep.logger.Debug().Str("event_type", eventType).RawJSON("raw_event", data).Msg("Received event JSON")

	// Create the specific event object based on type
	event, err := ep.createEventObject(eventType, data)
	if err != nil {
		ep.logger.Error().Err(err).Str("event_type", eventType).Msg("Failed to create event object")
		return
	}

	// Send the event to the event channel if running
	if err := ep.processEvent(ctx, event); err != nil {
		ep.logger.Error().Err(err).Str("event_type", eventType).Msg("Error processing event")
	}
}

// createEventObject creates the appropriate event object based on type
func (ep *eventProcessor) createEventObject(eventType string, data []byte) (Event, error) {
	var err error
	var event Event

	// Log the raw JSON data being unmarshaled to help debug issues
	ep.logger.Debug().RawJSON("raw_json", data).Str("event_type", eventType).Msg("Creating event object")

	switch eventType {
	case EventSessionCreated:
		var sessionEvent SessionCreatedEvent
		if err = json.Unmarshal(data, &sessionEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal SessionCreatedEvent")
			return nil, err
		}
		sessionEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &sessionEvent

	case EventSessionUpdated:
		var sessionEvent SessionUpdatedEvent
		if err = json.Unmarshal(data, &sessionEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal SessionUpdatedEvent")
			return nil, err
		}
		sessionEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &sessionEvent

	case EventConversationItemCreated:
		var itemEvent ConversationItemCreatedEvent
		if err = json.Unmarshal(data, &itemEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ConversationItemCreatedEvent")
			return nil, err
		}
		itemEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &itemEvent

	case EventConversationItemInputAudioTranscriptionCompleted:
		var transcriptionEvent TranscriptionCompletedEvent
		if err = json.Unmarshal(data, &transcriptionEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal TranscriptionCompletedEvent")
			return nil, err
		}
		transcriptionEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &transcriptionEvent

		// For logging convenience, log the transcription
		transcript := transcriptionEvent.Transcript
		if transcript != "" {
			ep.logger.Debug().Str("transcript", transcript).Msg("Transcription received")
		}

	case EventResponseCreated:
		var responseEvent ResponseCreatedEvent
		if err = json.Unmarshal(data, &responseEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ResponseCreatedEvent")
			return nil, err
		}
		responseEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &responseEvent

		// Store the current response ID and reset buffers
		ep.client.responseManager.SetResponseID(responseEvent.Response.ID)
		ep.client.responseManager.ResetResponse()

	case EventResponseContentPartAdded:
		var contentEvent ContentPartAddedEvent
		if err = json.Unmarshal(data, &contentEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ContentPartAddedEvent")
			return nil, err
		}
		contentEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &contentEvent

		// For logging convenience, extract the content text
		text := contentEvent.Part.Text
		if text != "" {
			ep.logger.Debug().Str("text", text).Msg("Content received")

			// Append to the response buffer
			ep.client.responseManager.AppendResponseText(contentEvent.ResponseID, text)
		}

	case EventResponseContentPartDone:
		var doneEvent ContentPartDoneEvent
		if err = json.Unmarshal(data, &doneEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ContentPartDoneEvent")
			return nil, err
		}
		doneEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &doneEvent

	case EventResponseAudioDelta:
		var audioEvent AudioDeltaEvent
		if err = json.Unmarshal(data, &audioEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal AudioDeltaEvent")
			return nil, err
		}
		audioEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &audioEvent

		// Decode and store audio
		if audioEvent.Delta != "" {
			audioData, err := base64.StdEncoding.DecodeString(audioEvent.Delta)
			if err != nil {
				ep.logger.Warn().Err(err).Msg("Failed to decode audio data")
			} else {
				ep.client.responseManager.AppendResponseAudio(audioEvent.ResponseID, audioData)
			}
		}

	case EventResponseAudioDone:
		var doneEvent AudioDoneEvent
		if err = json.Unmarshal(data, &doneEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal AudioDoneEvent")
			return nil, err
		}
		doneEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &doneEvent

	case EventResponseDone:
		var doneEvent ResponseDoneEvent
		if err = json.Unmarshal(data, &doneEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ResponseDoneEvent")
			return nil, err
		}
		doneEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &doneEvent

		// Log token usage
		ep.logger.Debug().
			Int("input_tokens", doneEvent.Response.Usage.InputTokens).
			Int("output_tokens", doneEvent.Response.Usage.OutputTokens).
			Int("audio_tokens", doneEvent.Response.Usage.InputTokenDetails.AudioTokens).
			Int("cached_tokens", doneEvent.Response.Usage.InputTokenDetails.CachedTokens).
			Msg("Response completed")

	case EventError:
		var errorEvent ErrorEvent
		if err = json.Unmarshal(data, &errorEvent); err != nil {
			ep.logger.Error().Err(err).RawJSON("raw_json", data).Msg("Failed to unmarshal ErrorEvent")
			return nil, err
		}
		errorEvent.BaseEvent = NewBaseEvent(eventType, data)
		event = &errorEvent

		// Log the error for convenience
		ep.logger.Error().
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
func (ep *eventProcessor) processEvent(ctx context.Context, event Event) error {
	ep.logger.Debug().Str("event_type", event.Type()).Msg("Processing event")
	eventType := event.Type()

	// First check if we have any registered handlers for this event type
	ep.handlersMutex.RLock()
	handlers, exists := ep.eventHandlers[eventType]
	ep.handlersMutex.RUnlock()

	if exists && len(handlers) > 0 {
		ep.logger.Debug().Str("event_type", eventType).Msg("Found handlers for event")
		// Run all registered handlers
		for _, handler := range handlers {
			ep.logger.Debug().Str("event_type", eventType).Msg("Running handler")
			if err := handler(ctx, event); err != nil {
				return fmt.Errorf("handler error for event %s: %w", eventType, err)
			}
		}
		return nil
	}

	// If no handlers registered, use default handling
	ep.logger.Debug().Str("event_type", eventType).Msg("No handlers registered, handle default event")
	return ep.handleDefaultEvent(event)
}

// handleDefaultEvent provides default handling for events
func (ep *eventProcessor) handleDefaultEvent(event Event) error {
	eventType := event.Type()

	// By default, just log that we received the event
	ep.logger.Debug().Str("event_type", eventType).Msg("Handle default event")

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

		ep.client.sessionID.Store(sessionEvent.Session.ID)
		ep.logger.Info().Str("session_id", sessionEvent.Session.ID).Msg("Session created")

	case EventInputAudioBufferSpeechStarted:
		ep.logger.Debug().Msg("Speech started")

	case EventInputAudioBufferSpeechStopped:
		ep.logger.Debug().Msg("Speech stopped")

	case EventResponseDone:
		ep.logger.Debug().Msg("Response complete")
	}

	return nil
}
