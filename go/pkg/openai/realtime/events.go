package realtime

import (
	"encoding/json"
)

// Event type constants
const (
	// Server-to-client events
	EventSessionCreated          = "session.created"
	EventSessionUpdated          = "session.updated"
	EventConversationItemCreated = "conversation.item.created"
	EventTranscriptionCompleted  = "conversation.item.input_audio_transcription.completed"
	EventResponseCreated         = "response.created"
	EventContentPartAdded        = "response.content_part.added"
	EventContentPartDone         = "response.content_part.done"
	EventAudioDelta              = "response.audio.delta"
	EventAudioDone               = "response.audio.done"
	EventResponseDone            = "response.done"
	EventError                   = "error"

	// Client-to-server events
	EventSessionUpdate          = "session.update"
	EventAudioBufferAppend      = "input_audio_buffer.append"
	EventAudioBufferCommit      = "input_audio_buffer.commit"
	EventConversationItemCreate = "conversation.item.create"
	EventInputText              = "input_text"
)

// BaseEvent implements the common properties for all events
type BaseEvent struct {
	eventType string
	rawData   []byte
}

// Type returns the event type
func (e *BaseEvent) Type() string {
	return e.eventType
}

// RawData returns the raw event data
func (e *BaseEvent) RawData() []byte {
	return e.rawData
}

// NewBaseEvent creates a new base event
func NewBaseEvent(eventType string, rawData []byte) *BaseEvent {
	return &BaseEvent{
		eventType: eventType,
		rawData:   rawData,
	}
}

// SessionCreatedEvent represents the session.created event
type SessionCreatedEvent struct {
	*BaseEvent
	EventID string `json:"event_id"`
	Session struct {
		ID            string   `json:"id"`
		Object        string   `json:"object"` // "realtime.session"
		Model         string   `json:"model"`
		ExpiresAt     int64    `json:"expires_at"`
		Modalities    []string `json:"modalities"`
		Instructions  string   `json:"instructions"`
		Voice         string   `json:"voice"`
		TurnDetection struct {
			Type              string  `json:"type"` // "server_vad" or "disabled"
			Threshold         float64 `json:"threshold,omitempty"`
			PrefixPaddingMs   int     `json:"prefix_padding_ms,omitempty"`
			SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
			CreateResponse    bool    `json:"create_response,omitempty"`
			InterruptResponse bool    `json:"interrupt_response,omitempty"`
		} `json:"turn_detection"`
		InputAudioFormat        string        `json:"input_audio_format"`
		OutputAudioFormat       string        `json:"output_audio_format"`
		InputAudioTranscription interface{}   `json:"input_audio_transcription"`
		ToolChoice              string        `json:"tool_choice"`
		Temperature             float64       `json:"temperature"`
		MaxResponseOutputTokens string        `json:"max_response_output_tokens"`
		ClientSecret            interface{}   `json:"client_secret"`
		Tools                   []interface{} `json:"tools"`
	} `json:"session"`
}

// SessionUpdatedEvent represents the session.updated event
type SessionUpdatedEvent struct {
	*BaseEvent
	Session struct {
		// Same fields as SessionCreatedEvent.Session
		SessionID         string   `json:"session_id"`
		Model             string   `json:"model"`
		Voice             string   `json:"voice,omitempty"`
		InputAudioFormat  string   `json:"input_audio_format,omitempty"`
		OutputAudioFormat string   `json:"output_audio_format,omitempty"`
		Modalities        []string `json:"modalities,omitempty"`
		TurnDetection     struct {
			Type string `json:"type"` // "server_vad" or "disabled"
		} `json:"turn_detection,omitempty"`
		Instructions string   `json:"instructions,omitempty"`
		Temperature  *float64 `json:"temperature,omitempty"`
	} `json:"session"`
}

// ConversationItemCreatedEvent represents the conversation.item.created event
type ConversationItemCreatedEvent struct {
	*BaseEvent
	Item struct {
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
	} `json:"item"`
}

// TranscriptionCompletedEvent represents the conversation.item.input_audio_transcription.completed event
type TranscriptionCompletedEvent struct {
	*BaseEvent
	Item struct {
		ID      string `json:"id"`
		Role    string `json:"role"` // "user"
		Content struct {
			Text string `json:"text"` // The transcribed text
		} `json:"content"`
	} `json:"item"`
}

// ResponseCreatedEvent represents the response.created event
type ResponseCreatedEvent struct {
	*BaseEvent
	ResponseID string `json:"response_id"`
}

// ContentPartAddedEvent represents the response.content_part.added event
type ContentPartAddedEvent struct {
	*BaseEvent
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Content     struct {
		Text string `json:"text"` // Partial text content
	} `json:"content"`
}

// ContentPartDoneEvent represents the response.content_part.done event
type ContentPartDoneEvent struct {
	*BaseEvent
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
}

// AudioDeltaEvent represents the response.audio.delta event
type AudioDeltaEvent struct {
	*BaseEvent
	ResponseID string `json:"response_id"`
	ItemID     string `json:"item_id"`
	Audio      string `json:"audio"` // Base64-encoded audio chunk
}

// AudioDoneEvent represents the response.audio.done event
type AudioDoneEvent struct {
	*BaseEvent
	ResponseID string `json:"response_id"`
	ItemID     string `json:"item_id"`
}

// ResponseDoneEvent represents the response.done event
type ResponseDoneEvent struct {
	*BaseEvent
	ResponseID string `json:"response_id"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		AudioTokens  int `json:"audio_tokens"`
		CachedTokens int `json:"cached_tokens,omitempty"`
	} `json:"usage"`
}

// ErrorEvent represents the error event
type ErrorEvent struct {
	*BaseEvent
	EventID string `json:"event_id"`
	Error   struct {
		Type    string      `json:"type"`
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Param   string      `json:"param,omitempty"`
		EventID interface{} `json:"event_id"`
	} `json:"error"`
}

// Client-to-server event types
// These are used for constructing messages to send to the OpenAI API

// SessionUpdateRequest represents the session.update request
type SessionUpdateRequest struct {
	Type    string `json:"type"` // "session.update"
	Session struct {
		Voice             string   `json:"voice,omitempty"`
		Modalities        []string `json:"modalities,omitempty"`
		InputAudioFormat  string   `json:"input_audio_format,omitempty"`
		OutputAudioFormat string   `json:"output_audio_format,omitempty"`
		Instructions      string   `json:"instructions,omitempty"`
		Temperature       *float64 `json:"temperature,omitempty"`
		TurnDetection     *struct {
			Type string `json:"type"` // "server_vad" or "disabled"
		} `json:"turn_detection,omitempty"`
	} `json:"session"`
}

// AudioBufferAppendRequest represents the input_audio_buffer.append request
type AudioBufferAppendRequest struct {
	Type  string `json:"type"`  // "input_audio_buffer.append"
	Audio string `json:"audio"` // Base64-encoded audio chunk
}

// AudioBufferCommitRequest represents the input_audio_buffer.commit request
type AudioBufferCommitRequest struct {
	Type string `json:"type"` // "input_audio_buffer.commit"
}

// ConversationItemCreateRequest represents the conversation.item.create request
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

// AudioBufferClearRequest represents the input_audio_buffer.clear message
type AudioBufferClearRequest struct {
	Type string `json:"type"` // "input_audio_buffer.clear"
}

// ConversationItemTruncateRequest represents the conversation.item.truncate message
type ConversationItemTruncateRequest struct {
	Type   string `json:"type"`    // "conversation.item.truncate"
	ItemID string `json:"item_id"` // ID of the item to truncate from
}

// ConversationItemDeleteRequest represents the conversation.item.delete message
type ConversationItemDeleteRequest struct {
	Type   string `json:"type"`    // "conversation.item.delete"
	ItemID string `json:"item_id"` // ID of the item to delete
}

// ResponseCreateRequest represents the response.create message
type ResponseCreateRequest struct {
	Type string `json:"type"` // "response.create"
}

// ResponseCancelRequest represents the response.cancel message
type ResponseCancelRequest struct {
	Type string `json:"type"` // "response.cancel"
}
