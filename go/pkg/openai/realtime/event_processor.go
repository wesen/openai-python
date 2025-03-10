package realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// eventProcessor handles processing events from the WebSocket
type eventProcessor struct {
	client        *clientImpl
	conn          *websocket.Conn
	connMutex     *sync.RWMutex // Points to the same mutex as connectionManager
	eventChan     chan Event
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex
	logger        zerolog.Logger
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

// ProcessRawEvent processes a raw event from the WebSocket
func (ep *eventProcessor) ProcessRawEvent(data []byte) {
	// Log the raw event data
	ep.logger.Debug().RawJSON("raw_event", data).Msg("Processing raw event")

	// First, just try to extract the type field
	var eventTypeExtract struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &eventTypeExtract); err != nil {
		ep.logger.Error().Err(err).Msg("Failed to extract event type")
		return
	}

	eventType := eventTypeExtract.Type

	// Create appropriate event object based on the event type
	event, err := ep.createEventObject(eventType, data)
	if err != nil {
		ep.logger.Error().Err(err).
			Str("event_type", eventType).
			RawJSON("raw_event", data).
			Msg("Failed to create event object")
		return
	}

	// Queue event for processing
	select {
	case ep.eventChan <- event:
		// Event queued successfully
	default:
		ep.logger.Warn().Str("event_type", eventType).Msg("Event channel full, dropping event")
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

// Start begins the event processor operation
func (ep *eventProcessor) Start(ctx context.Context) error {
	// Start the event processing goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				ep.logger.Debug().Msg("Event processor stopping")
				return
			case event := <-ep.eventChan:
				if err := ep.processEvent(event); err != nil {
					ep.logger.Error().Err(err).Str("event_type", event.Type()).Msg("Error processing event")
				}
			}
		}
	}()

	return nil
}

// Stop ends the event processor operation
func (ep *eventProcessor) Stop(ctx context.Context) error {
	// Just rely on context cancellation to stop the goroutine
	return nil
}

// listenLoop listens for messages from the WebSocket
func (ep *eventProcessor) listenLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Ensure we have a connection
			ep.connMutex.RLock()
			conn := ep.conn
			ep.connMutex.RUnlock()

			if conn == nil {
				ep.logger.Debug().Msg("No connection for listening")
				return fmt.Errorf("no active connection")
			}

			// Set a deadline for reading
			err := conn.SetReadDeadline(time.Now().Add(PongWait))
			if err != nil {
				ep.logger.Warn().Err(err).Msg("Error setting read deadline")
			}

			// Read message
			_, message, err := conn.ReadMessage()
			if err != nil {
				// Check if it's a normal close
				if websocket.IsCloseError(err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure) {
					ep.logger.Info().Msg("WebSocket closed normally")
				} else {
					ep.logger.Error().Err(err).Msg("Error reading from WebSocket")
				}
				return err
			}

			// Process the received message
			ep.ProcessRawEvent(message)
		}
	}
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
