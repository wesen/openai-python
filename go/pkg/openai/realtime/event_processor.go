package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// eventProcessor handles processing events from the WebSocket
type eventProcessor struct {
	client        *clientImpl
	eventChan     chan Event
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex
	logger        zerolog.Logger
	running       atomic.Bool
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
func (ep *eventProcessor) ProcessRawEvent(data []byte) {
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

	// Handle default events that all clients should process
	if err := ep.handleDefaultEvent(event); err != nil {
		ep.logger.Error().Err(err).Str("event_type", eventType).Msg("Error handling default event")
	}

	// Send the event to the event channel if running
	if ep.running.Load() {
		select {
		case ep.eventChan <- event:
			// Event sent successfully
		default:
			ep.logger.Warn().Str("event_type", eventType).Msg("Event channel full, dropping event")
		}
	}

	// Process the event with registered handlers
	if err := ep.processEvent(event); err != nil {
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

// Start begins processing events
func (ep *eventProcessor) Start(ctx context.Context) error {
	if ep.running.Load() {
		return nil // Already running
	}

	// Create a new event channel if needed
	if ep.eventChan == nil {
		ep.eventChan = make(chan Event, 100)
	}

	// Start a goroutine to process events from the channel
	ep.client.eg.Go(func() error {
		defer ep.logger.Debug().Msg("Event processor stopping")
		return ep.processEvents(ctx)
	})

	ep.running.Store(true)
	return nil
}

// processEvents is the main event processing loop
func (ep *eventProcessor) processEvents(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-ep.eventChan:
			if !ok {
				ep.logger.Debug().Msg("Event channel closed")
				return nil
			}

			if err := ep.processEvent(event); err != nil {
				ep.logger.Error().Err(err).Str("event_type", event.Type()).Msg("Error processing event")
			}
		}
	}
}

// Stop terminates event processing
func (ep *eventProcessor) Stop(ctx context.Context) error {
	if !ep.running.Load() {
		return nil
	}

	ep.running.Store(false)
	ep.logger.Debug().Msg("Event processor stopping")
	return nil
}

// processEvent processes a parsed event
func (ep *eventProcessor) processEvent(event Event) error {
	eventType := event.Type()

	// First check if we have any registered handlers for this event type
	ep.handlersMutex.RLock()
	handlers, exists := ep.eventHandlers[eventType]
	ep.handlersMutex.RUnlock()

	if exists && len(handlers) > 0 {
		// Run all registered handlers
		for _, handler := range handlers {
			if err := handler(event); err != nil {
				return fmt.Errorf("handler error for event %s: %w", eventType, err)
			}
		}
		return nil
	}

	// If no handlers registered, use default handling
	return ep.handleDefaultEvent(event)
}

// handleDefaultEvent provides default handling for events
func (ep *eventProcessor) handleDefaultEvent(event Event) error {
	eventType := event.Type()

	// By default, just log that we received the event
	ep.logger.Debug().Str("event_type", eventType).Msg("Received event")

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
