package realtime

import (
	"encoding/json"
)

// Event type constants
const (
	// Server-to-client events
	EventSessionCreated                                   = "session.created"
	EventSessionUpdated                                   = "session.updated"
	EventConversationCreated                              = "conversation.created"
	EventConversationItemCreated                          = "conversation.item.created"
	EventConversationItemDeleted                          = "conversation.item.deleted"
	EventConversationItemTruncated                        = "conversation.item.truncated"
	EventResponseCreated                                  = "response.created"
	EventResponseDone                                     = "response.done"
	EventRateLimitsUpdated                                = "rate_limits.updated"
	EventResponseOutputItemAdded                          = "response.output_item.added"
	EventResponseOutputItemDone                           = "response.output_item.done"
	EventResponseContentPartAdded                         = "response.content_part.added"
	EventResponseContentPartDone                          = "response.content_part.done"
	EventResponseAudioDelta                               = "response.audio.delta"
	EventResponseAudioDone                                = "response.audio.done"
	EventResponseAudioTranscriptDelta                     = "response.audio_transcript.delta"
	EventResponseAudioTranscriptDone                      = "response.audio_transcript.done"
	EventResponseTextDelta                                = "response.text.delta"
	EventResponseTextDone                                 = "response.text.done"
	EventResponseFunctionCallArgumentsDelta               = "response.function_call_arguments.delta"
	EventResponseFunctionCallArgumentsDone                = "response.function_call_arguments.done"
	EventInputAudioBufferSpeechStarted                    = "input_audio_buffer.speech_started"
	EventInputAudioBufferSpeechStopped                    = "input_audio_buffer.speech_stopped"
	EventConversationItemInputAudioTranscriptionCompleted = "conversation.item.input_audio_transcription.completed"
	EventConversationItemInputAudioTranscriptionFailed    = "conversation.item.input_audio_transcription.failed"
	EventInputAudioBufferCommitted                        = "input_audio_buffer.committed"
	EventInputAudioBufferCleared                          = "input_audio_buffer.cleared"
	EventError                                            = "error"

	// Client-to-server events
	EventSessionUpdate            = "session.update"
	EventAudioBufferAppend        = "input_audio_buffer.append"
	EventAudioBufferCommit        = "input_audio_buffer.commit"
	EventAudioBufferClear         = "input_audio_buffer.clear"
	EventConversationItemCreate   = "conversation.item.create"
	EventConversationItemDelete   = "conversation.item.delete"
	EventConversationItemTruncate = "conversation.item.truncate"
	EventResponseCreate           = "response.create"
	EventResponseCancel           = "response.cancel"
	EventInputText                = "input_text"
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
		InputAudioFormat        string `json:"input_audio_format"`
		OutputAudioFormat       string `json:"output_audio_format"`
		InputAudioTranscription struct {
			Language          string                 `json:"language,omitempty"`
			Type              string                 `json:"type,omitempty"` // "server" or "client"
			Interim           bool                   `json:"interim,omitempty"`
			PhraseHints       []string               `json:"phrase_hints,omitempty"`
			ProfanityFilter   bool                   `json:"profanity_filter,omitempty"`
			Redact            []string               `json:"redact,omitempty"`
			Diarize           bool                   `json:"diarize,omitempty"`
			EndpointingConfig map[string]interface{} `json:"endpointing_config,omitempty"`
		} `json:"input_audio_transcription"`
		ToolChoice              string        `json:"tool_choice"`
		Temperature             float64       `json:"temperature"`
		TopP                    float64       `json:"top_p,omitempty"`
		PresencePenalty         float64       `json:"presence_penalty,omitempty"`
		FrequencyPenalty        float64       `json:"frequency_penalty,omitempty"`
		MaxResponseOutputTokens int           `json:"max_response_output_tokens"`
		ClientSecret            interface{}   `json:"client_secret"`
		Tools                   []interface{} `json:"tools"`
		SpeechSettings          struct {
			Voice        string  `json:"voice"`
			Speed        float64 `json:"speed,omitempty"`
			Stability    float64 `json:"stability,omitempty"`
			Similarity   float64 `json:"similarity,omitempty"`
			Style        float64 `json:"style,omitempty"`
			PresenceText bool    `json:"presence_text,omitempty"`
		} `json:"speech_settings,omitempty"`
	} `json:"session"`
}

// SessionUpdatedEvent represents the session.updated event
type SessionUpdatedEvent struct {
	*BaseEvent
	Session struct {
		ID                string   `json:"id"`
		Object            string   `json:"object,omitempty"` // "realtime.session"
		Model             string   `json:"model,omitempty"`
		Voice             string   `json:"voice,omitempty"`
		InputAudioFormat  string   `json:"input_audio_format,omitempty"`
		OutputAudioFormat string   `json:"output_audio_format,omitempty"`
		Modalities        []string `json:"modalities,omitempty"`
		TurnDetection     struct {
			Type              string  `json:"type,omitempty"` // "server_vad" or "disabled"
			Threshold         float64 `json:"threshold,omitempty"`
			PrefixPaddingMs   int     `json:"prefix_padding_ms,omitempty"`
			SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
			CreateResponse    bool    `json:"create_response,omitempty"`
			InterruptResponse bool    `json:"interrupt_response,omitempty"`
		} `json:"turn_detection,omitempty"`
		Instructions            string   `json:"instructions,omitempty"`
		Temperature             *float64 `json:"temperature,omitempty"`
		TopP                    *float64 `json:"top_p,omitempty"`
		PresencePenalty         *float64 `json:"presence_penalty,omitempty"`
		FrequencyPenalty        *float64 `json:"frequency_penalty,omitempty"`
		MaxResponseOutputTokens *int     `json:"max_response_output_tokens,omitempty"`
		InputAudioTranscription struct {
			Language        string   `json:"language,omitempty"`
			Type            string   `json:"type,omitempty"`
			Interim         bool     `json:"interim,omitempty"`
			PhraseHints     []string `json:"phrase_hints,omitempty"`
			ProfanityFilter bool     `json:"profanity_filter,omitempty"`
			Redact          []string `json:"redact,omitempty"`
			Diarize         bool     `json:"diarize,omitempty"`
		} `json:"input_audio_transcription,omitempty"`
		SpeechSettings struct {
			Voice        string  `json:"voice,omitempty"`
			Speed        float64 `json:"speed,omitempty"`
			Stability    float64 `json:"stability,omitempty"`
			Similarity   float64 `json:"similarity,omitempty"`
			Style        float64 `json:"style,omitempty"`
			PresenceText bool    `json:"presence_text,omitempty"`
		} `json:"speech_settings,omitempty"`
		Tools      []interface{} `json:"tools,omitempty"`
		ToolChoice string        `json:"tool_choice,omitempty"`
	} `json:"session"`
}

// ConversationItemCreatedEvent represents the conversation.item.created event
type ConversationItemCreatedEvent struct {
	*BaseEvent
	PreviousItemID string `json:"previous_item_id,omitempty"`
	Item struct {
		ID      string `json:"id"`
		Role    string `json:"role"` // "user", "assistant", or "function"
		Type    string `json:"type"` // "text" for text messages
		Content struct {
			Text string `json:"text,omitempty"`
			// Can include function_call for assistant role
			FunctionCall *struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"` // JSON string of arguments
			} `json:"function_call,omitempty"`
			// For function results when role is "function"
			FunctionResult *struct {
				Name   string          `json:"name"`
				Result json.RawMessage `json:"result"` // JSON result data
			} `json:"function_result,omitempty"`
		} `json:"content"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	} `json:"item"`
}

// TranscriptionCompletedEvent represents the conversation.item.input_audio_transcription.completed event
type TranscriptionCompletedEvent struct {
	*BaseEvent
	ItemID       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	Transcript   string `json:"transcript"`
}

// ResponseCreatedEvent represents the response.created event
type ResponseCreatedEvent struct {
	*BaseEvent
	Response struct {
		Object        string `json:"object"`
		ID            string `json:"id"`
		Status        string `json:"status"`
		StatusDetails *struct {
			Type string `json:"type"`
		} `json:"status_details,omitempty"`
		Output []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
			Type   string `json:"type"`
		} `json:"output"`
		Usage struct {
			TotalTokens       int `json:"total_tokens"`
			InputTokens       int `json:"input_tokens"`
			OutputTokens      int `json:"output_tokens"`
			InputTokenDetails struct {
				CachedTokens int `json:"cached_tokens"`
				TextTokens   int `json:"text_tokens"`
				AudioTokens  int `json:"audio_tokens"`
			} `json:"input_token_details"`
			OutputTokenDetails struct {
				TextTokens  int `json:"text_tokens"`
				AudioTokens int `json:"audio_tokens"`
			} `json:"output_token_details"`
		} `json:"usage"`
	} `json:"response"`
}

// ContentPartAddedEvent represents the response.content_part.added event
type ContentPartAddedEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Part         struct {
		Type string `json:"type"` // "text", "audio", etc.
		Text string `json:"text,omitempty"`
	} `json:"part"`
}

// ContentPartDoneEvent represents the response.content_part.done event
type ContentPartDoneEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Part         struct {
		Type string `json:"type"` // "text", "audio", etc.
		Text string `json:"text,omitempty"`
	} `json:"part"`
}

// AudioDeltaEvent represents the response.audio.delta event
type AudioDeltaEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"` // Base64-encoded audio chunk
}

// AudioDoneEvent represents the response.audio.done event
type AudioDoneEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
}

// ResponseDoneEvent represents the response.done event
type ResponseDoneEvent struct {
	*BaseEvent
	Response struct {
		Object        string `json:"object"`
		ID            string `json:"id"`
		Status        string `json:"status"`
		StatusDetails *struct {
			Type string `json:"type"`
		} `json:"status_details,omitempty"`
		Output []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
			Type   string `json:"type"`
		} `json:"output"`
		Usage struct {
			TotalTokens       int `json:"total_tokens"`
			InputTokens       int `json:"input_tokens"`
			OutputTokens      int `json:"output_tokens"`
			InputTokenDetails struct {
				CachedTokens int `json:"cached_tokens"`
				TextTokens   int `json:"text_tokens"`
				AudioTokens  int `json:"audio_tokens"`
			} `json:"input_token_details"`
			OutputTokenDetails struct {
				TextTokens  int `json:"text_tokens"`
				AudioTokens int `json:"audio_tokens"`
			} `json:"output_token_details"`
		} `json:"usage"`
	} `json:"response"`
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

// ConversationCreatedEvent represents the conversation.created event
type ConversationCreatedEvent struct {
	*BaseEvent
	Conversation struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	} `json:"conversation"`
}

// ConversationItemDeletedEvent represents the conversation.item.deleted event
type ConversationItemDeletedEvent struct {
	*BaseEvent
	ItemID string `json:"item_id"`
}

// ConversationItemTruncatedEvent represents the conversation.item.truncated event
type ConversationItemTruncatedEvent struct {
	*BaseEvent
	ItemID       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	AudioEndMs   int    `json:"audio_end_ms"`
}

// RateLimitsUpdatedEvent represents the rate_limits.updated event
type RateLimitsUpdatedEvent struct {
	*BaseEvent
	RateLimits []struct {
		Name         string `json:"name"`
		Limit        int    `json:"limit"`
		Remaining    int    `json:"remaining"`
		ResetSeconds int    `json:"reset_seconds"`
	} `json:"rate_limits"`
}

// ResponseOutputItemAddedEvent represents the response.output_item.added event
type ResponseOutputItemAddedEvent struct {
	*BaseEvent
	ResponseID  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	Item        struct {
		ID     string `json:"id"`
		Object string `json:"object"`
		Type   string `json:"type"`
	} `json:"item"`
}

// ResponseOutputItemDoneEvent represents the response.output_item.done event
type ResponseOutputItemDoneEvent struct {
	*BaseEvent
	ResponseID  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	Item        struct {
		ID     string `json:"id"`
		Object string `json:"object"`
		Type   string `json:"type"`
	} `json:"item"`
}

// ResponseAudioTranscriptDeltaEvent represents the response.audio_transcript.delta event
type ResponseAudioTranscriptDeltaEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

// ResponseAudioTranscriptDoneEvent represents the response.audio_transcript.done event
type ResponseAudioTranscriptDoneEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Transcript   string `json:"transcript"`
}

// ResponseTextDeltaEvent represents the response.text.delta event
type ResponseTextDeltaEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

// ResponseTextDoneEvent represents the response.text.done event
type ResponseTextDoneEvent struct {
	*BaseEvent
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Text         string `json:"text"`
}

// ResponseFunctionCallArgumentsDeltaEvent represents the response.function_call_arguments.delta event
type ResponseFunctionCallArgumentsDeltaEvent struct {
	*BaseEvent
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	CallID      string `json:"call_id"`
	Delta       string `json:"delta"`
}

// ResponseFunctionCallArgumentsDoneEvent represents the response.function_call_arguments.done event
type ResponseFunctionCallArgumentsDoneEvent struct {
	*BaseEvent
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	CallID      string `json:"call_id"`
	Arguments   string `json:"arguments"`
}

// InputAudioBufferSpeechStartedEvent represents the input_audio_buffer.speech_started event
type InputAudioBufferSpeechStartedEvent struct {
	*BaseEvent
	AudioStartMs int    `json:"audio_start_ms"`
	ItemID       string `json:"item_id"`
}

// InputAudioBufferSpeechStoppedEvent represents the input_audio_buffer.speech_stopped event
type InputAudioBufferSpeechStoppedEvent struct {
	*BaseEvent
	AudioEndMs int    `json:"audio_end_ms"`
	ItemID     string `json:"item_id"`
}

// InputAudioBufferCommittedEvent represents the input_audio_buffer.committed event
type InputAudioBufferCommittedEvent struct {
	*BaseEvent
	PreviousItemID string `json:"previous_item_id,omitempty"`
	ItemID         string `json:"item_id"`
}

// InputAudioBufferClearedEvent represents the input_audio_buffer.cleared event
type InputAudioBufferClearedEvent struct {
	*BaseEvent
}

// ConversationItemInputAudioTranscriptionFailedEvent represents the conversation.item.input_audio_transcription.failed event
type ConversationItemInputAudioTranscriptionFailedEvent struct {
	*BaseEvent
	ItemID       string `json:"item_id"`
	ContentIndex int    `json:"content_index"`
	Error        struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
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
			Type              string  `json:"type,omitempty"` // "server_vad" or "disabled"
			Threshold         float64 `json:"threshold,omitempty"`
			PrefixPaddingMs   int     `json:"prefix_padding_ms,omitempty"`
			SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
			CreateResponse    bool    `json:"create_response,omitempty"`
			InterruptResponse bool    `json:"interrupt_response,omitempty"`
		} `json:"turn_detection,omitempty"`
		TopP                    *float64 `json:"top_p,omitempty"`
		PresencePenalty         *float64 `json:"presence_penalty,omitempty"`
		FrequencyPenalty        *float64 `json:"frequency_penalty,omitempty"`
		MaxResponseOutputTokens *int     `json:"max_response_output_tokens,omitempty"`
		InputAudioTranscription *struct {
			Language        string   `json:"language,omitempty"`
			Type            string   `json:"type,omitempty"`
			Interim         bool     `json:"interim,omitempty"`
			PhraseHints     []string `json:"phrase_hints,omitempty"`
			ProfanityFilter bool     `json:"profanity_filter,omitempty"`
			Redact          []string `json:"redact,omitempty"`
			Diarize         bool     `json:"diarize,omitempty"`
		} `json:"input_audio_transcription,omitempty"`
		SpeechSettings *struct {
			Voice        string  `json:"voice,omitempty"`
			Speed        float64 `json:"speed,omitempty"`
			Stability    float64 `json:"stability,omitempty"`
			Similarity   float64 `json:"similarity,omitempty"`
			Style        float64 `json:"style,omitempty"`
			PresenceText bool    `json:"presence_text,omitempty"`
		} `json:"speech_settings,omitempty"`
		Tools      []interface{} `json:"tools,omitempty"`
		ToolChoice string        `json:"tool_choice,omitempty"`
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
	Type           string `json:"type"` // "conversation.item.create"
	PreviousItemID string `json:"previous_item_id,omitempty"`
	Item           struct {
		Role    string `json:"role"` // "user" or "function"
		Type    string `json:"type"` // "text"
		Content struct {
			Text string `json:"text,omitempty"` // For user messages
			// For function results when role is "function"
			FunctionResult *struct {
				Name   string          `json:"name"`
				Result json.RawMessage `json:"result"` // JSON result data
			} `json:"function_result,omitempty"`
		} `json:"content"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	} `json:"item"`
}

// AudioBufferClearRequest represents the input_audio_buffer.clear message
type AudioBufferClearRequest struct {
	Type string `json:"type"` // "input_audio_buffer.clear"
}

// ConversationItemTruncateRequest represents the conversation.item.truncate message
type ConversationItemTruncateRequest struct {
	Type         string `json:"type"`    // "conversation.item.truncate"
	ItemID       string `json:"item_id"` // ID of the item to truncate from
	ContentIndex int    `json:"content_index"`
	AudioEndMs   int    `json:"audio_end_ms"`
}

// ConversationItemDeleteRequest represents the conversation.item.delete message
type ConversationItemDeleteRequest struct {
	Type   string `json:"type"`    // "conversation.item.delete"
	ItemID string `json:"item_id"` // ID of the item to delete
}

// ResponseCreateRequest represents the response.create message
type ResponseCreateRequest struct {
	Type     string `json:"type"` // "response.create"
	Response *struct {
		Modalities              []string          `json:"modalities,omitempty"`
		Instructions            string            `json:"instructions,omitempty"`
		Voice                   string            `json:"voice,omitempty"`
		OutputAudioFormat       string            `json:"output_audio_format,omitempty"`
		Tools                   []interface{}     `json:"tools,omitempty"`
		ToolChoice              string            `json:"tool_choice,omitempty"`
		Temperature             *float64          `json:"temperature,omitempty"`
		MaxResponseOutputTokens interface{}       `json:"max_response_output_tokens,omitempty"` // can be number or "inf"
		Conversation            string            `json:"conversation,omitempty"`               // "auto" or "none"
		Metadata                map[string]string `json:"metadata,omitempty"`
		Input                   []interface{}     `json:"input,omitempty"`
	} `json:"response,omitempty"`
}

// ResponseCancelRequest represents the response.cancel message
type ResponseCancelRequest struct {
	Type string `json:"type"` // "response.cancel"
}
