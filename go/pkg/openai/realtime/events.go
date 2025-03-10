package realtime

import (
	"encoding/base64"
	"encoding/json"
)

// Event type constants
// These are kept for backward compatibility but can be replaced with enum values
const (
	// Server-to-client events
	EventSessionCreated                                   = string(ServerEventTypeSessionCreated)
	EventSessionUpdated                                   = string(ServerEventTypeSessionUpdated)
	EventConversationCreated                              = string(ServerEventTypeConversationCreated)
	EventConversationItemCreated                          = string(ServerEventTypeConversationItemCreated)
	EventConversationItemDeleted                          = string(ServerEventTypeConversationItemDeleted)
	EventConversationItemTruncated                        = string(ServerEventTypeConversationItemTruncated)
	EventResponseCreated                                  = string(ServerEventTypeResponseCreated)
	EventResponseDone                                     = string(ServerEventTypeResponseDone)
	EventRateLimitsUpdated                                = string(ServerEventTypeRateLimitsUpdated)
	EventResponseOutputItemAdded                          = string(ServerEventTypeResponseOutputItemAdded)
	EventResponseOutputItemDone                           = string(ServerEventTypeResponseOutputItemDone)
	EventResponseContentPartAdded                         = string(ServerEventTypeResponseContentPartAdded)
	EventResponseContentPartDone                          = string(ServerEventTypeResponseContentPartDone)
	EventResponseAudioDelta                               = string(ServerEventTypeResponseAudioDelta)
	EventResponseAudioDone                                = string(ServerEventTypeResponseAudioDone)
	EventResponseAudioTranscriptDelta                     = string(ServerEventTypeResponseAudioTranscriptDelta)
	EventResponseAudioTranscriptDone                      = string(ServerEventTypeResponseAudioTranscriptDone)
	EventResponseTextDelta                                = string(ServerEventTypeResponseTextDelta)
	EventResponseTextDone                                 = string(ServerEventTypeResponseTextDone)
	EventResponseFunctionCallArgumentsDelta               = string(ServerEventTypeResponseFunctionCallArgumentsDelta)
	EventResponseFunctionCallArgumentsDone                = string(ServerEventTypeResponseFunctionCallArgumentsDone)
	EventInputAudioBufferSpeechStarted                    = string(ServerEventTypeInputAudioBufferSpeechStarted)
	EventInputAudioBufferSpeechStopped                    = string(ServerEventTypeInputAudioBufferSpeechStopped)
	EventConversationItemInputAudioTranscriptionCompleted = string(ServerEventTypeConversationItemInputAudioTranscriptionCompleted)
	EventConversationItemInputAudioTranscriptionFailed    = string(ServerEventTypeConversationItemInputAudioTranscriptionFailed)
	EventInputAudioBufferCommitted                        = string(ServerEventTypeInputAudioBufferCommitted)
	EventInputAudioBufferCleared                          = string(ServerEventTypeInputAudioBufferCleared)
	EventError                                            = string(ServerEventTypeError)

	// Client-to-server events
	EventSessionUpdate            = string(ClientEventTypeSessionUpdate)
	EventAudioBufferAppend        = string(ClientEventTypeInputAudioBufferAppend)
	EventAudioBufferCommit        = string(ClientEventTypeInputAudioBufferCommit)
	EventAudioBufferClear         = string(ClientEventTypeInputAudioBufferClear)
	EventConversationItemCreate   = string(ClientEventTypeConversationItemCreate)
	EventConversationItemDelete   = string(ClientEventTypeConversationItemDelete)
	EventConversationItemTruncate = string(ClientEventTypeConversationItemTruncate)
	EventResponseCreate           = string(ClientEventTypeResponseCreate)
	EventResponseCancel           = string(ClientEventTypeResponseCancel)
	EventInputText                = string(ContentPartTypeInputText)
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

// TurnDetectionConfig represents turn detection configuration
type TurnDetectionConfig struct {
	Type              TurnDetectionType `json:"type,omitempty"` // server_vad or disabled
	Threshold         float64           `json:"threshold,omitempty"`
	PrefixPaddingMs   int               `json:"prefix_padding_ms,omitempty"`
	SilenceDurationMs int               `json:"silence_duration_ms,omitempty"`
	CreateResponse    bool              `json:"create_response,omitempty"`
	InterruptResponse bool              `json:"interrupt_response,omitempty"`
}

// InputAudioTranscriptionConfig represents input audio transcription configuration
type InputAudioTranscriptionConfig struct {
	Language          string                       `json:"language,omitempty"`
	Type              string                       `json:"type,omitempty"` // "server" or "client"
	Model             AudioInputTranscriptionModel `json:"model,omitempty"`
	Interim           bool                         `json:"interim,omitempty"`
	PhraseHints       []string                     `json:"phrase_hints,omitempty"`
	ProfanityFilter   bool                         `json:"profanity_filter,omitempty"`
	Redact            []string                     `json:"redact,omitempty"`
	Diarize           bool                         `json:"diarize,omitempty"`
	EndpointingConfig map[string]interface{}       `json:"endpointing_config,omitempty"`
}

// SpeechSettings represents speech configuration
type SpeechSettings struct {
	Voice        Voice   `json:"voice,omitempty"`
	Speed        float64 `json:"speed,omitempty"`
	Stability    float64 `json:"stability,omitempty"`
	Similarity   float64 `json:"similarity,omitempty"`
	Style        float64 `json:"style,omitempty"`
	PresenceText bool    `json:"presence_text,omitempty"`
}

// UsageDetails represents token usage statistics
type UsageDetails struct {
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
}

// OutputItem represents an output item in a response
type OutputItem struct {
	ID     string   `json:"id"`
	Object string   `json:"object"`
	Type   ItemType `json:"type"`
}

// StatusDetails represents status details in a response
type StatusDetails struct {
	Type ResponseStatus `json:"type"`
}

// ContentPart represents a content part in a response
type ContentPart struct {
	Type ContentPartType `json:"type"` // text, audio, etc.
	Text string          `json:"text,omitempty"`
}

// FunctionCallInfo represents information about a function call
type FunctionCallInfo struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"` // JSON string of arguments
}

// FunctionResultInfo represents information about a function result
type FunctionResultInfo struct {
	Name   string          `json:"name"`
	Result json.RawMessage `json:"result"` // JSON result data
}

// ItemContent represents the content of a conversation item
type ItemContent struct {
	Text           string              `json:"text,omitempty"`
	FunctionCall   *FunctionCallInfo   `json:"function_call,omitempty"`
	FunctionResult *FunctionResultInfo `json:"function_result,omitempty"`
}

// ConversationItem represents a conversation item
type ConversationItem struct {
	ID       string                 `json:"id,omitempty"`
	Role     MessageRole            `json:"role"` // user, assistant, or system
	Type     ItemType               `json:"type"` // message, function_call, etc.
	Content  []ContentPart          `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SessionErrorInfo represents error information
type SessionErrorInfo struct {
	Type    string      `json:"type"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Param   string      `json:"param,omitempty"`
	EventID interface{} `json:"event_id"`
}

// SessionInfo represents the realtime session information
type SessionInfo struct {
	ID                      string                        `json:"id"`
	Object                  string                        `json:"object,omitempty"` // "realtime.session"
	Model                   string                        `json:"model,omitempty"`
	Modalities              []string                      `json:"modalities,omitempty"`
	Instructions            string                        `json:"instructions,omitempty"`
	Voice                   Voice                         `json:"voice,omitempty"`
	TurnDetection           TurnDetectionConfig           `json:"turn_detection"`
	InputAudioFormat        AudioFormat                   `json:"input_audio_format,omitempty"`
	OutputAudioFormat       AudioFormat                   `json:"output_audio_format,omitempty"`
	InputAudioTranscription InputAudioTranscriptionConfig `json:"input_audio_transcription,omitempty"`
	ToolChoice              ToolChoiceLiteral             `json:"tool_choice,omitempty"`
	Temperature             float64                       `json:"temperature,omitempty"`
	TopP                    float64                       `json:"top_p,omitempty"`
	PresencePenalty         float64                       `json:"presence_penalty,omitempty"`
	FrequencyPenalty        float64                       `json:"frequency_penalty,omitempty"`
	MaxResponseOutputTokens interface{}                   `json:"max_response_output_tokens,omitempty"`
	ClientSecret            interface{}                   `json:"client_secret,omitempty"`
	Tools                   []interface{}                 `json:"tools,omitempty"`
	SpeechSettings          SpeechSettings                `json:"speech_settings,omitempty"`
}

// ResponseInfo represents response information
type ResponseInfo struct {
	Object        string         `json:"object"`
	ID            string         `json:"id"`
	Status        ResponseStatus `json:"status"`
	StatusDetails *StatusDetails `json:"status_details,omitempty"`
	Output        []OutputItem   `json:"output"`
	Usage         UsageDetails   `json:"usage"`
}

// ConversationInfo represents conversation information
type ConversationInfo struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

// TranscriptionErrorInfo represents error information for input audio transcription
type TranscriptionErrorInfo struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SessionCreatedEvent represents the session.created event
type SessionCreatedEvent struct {
	*BaseEvent
	EventID string      `json:"event_id"`
	Session SessionInfo `json:"session"`
}

// SessionUpdatedEvent represents the session.updated event
type SessionUpdatedEvent struct {
	*BaseEvent
	Session SessionInfo `json:"session"`
}

// ConversationItemCreatedEvent represents the conversation.item.created event
type ConversationItemCreatedEvent struct {
	*BaseEvent
	PreviousItemID string           `json:"previous_item_id,omitempty"`
	Item           ConversationItem `json:"item"`
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
	Response ResponseInfo `json:"response"`
}

// ContentPartAddedEvent represents the response.content_part.added event
type ContentPartAddedEvent struct {
	*BaseEvent
	ResponseID   string      `json:"response_id"`
	ItemID       string      `json:"item_id"`
	OutputIndex  int         `json:"output_index"`
	ContentIndex int         `json:"content_index"`
	Part         ContentPart `json:"part"`
}

// ContentPartDoneEvent represents the response.content_part.done event
type ContentPartDoneEvent struct {
	*BaseEvent
	ResponseID   string      `json:"response_id"`
	ItemID       string      `json:"item_id"`
	OutputIndex  int         `json:"output_index"`
	ContentIndex int         `json:"content_index"`
	Part         ContentPart `json:"part"`
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
	Response ResponseInfo `json:"response"`
}

// ErrorEvent represents the error event
type ErrorEvent struct {
	*BaseEvent
	EventID string           `json:"event_id"`
	Error   SessionErrorInfo `json:"error"`
}

// ConversationCreatedEvent represents the conversation.created event
type ConversationCreatedEvent struct {
	*BaseEvent
	Conversation ConversationInfo `json:"conversation"`
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

// RateLimit represents a rate limit entry
type RateLimit struct {
	Name         string `json:"name"`
	Limit        int    `json:"limit"`
	Remaining    int    `json:"remaining"`
	ResetSeconds int    `json:"reset_seconds"`
}

// RateLimitsUpdatedEvent represents the rate_limits.updated event
type RateLimitsUpdatedEvent struct {
	*BaseEvent
	RateLimits []RateLimit `json:"rate_limits"`
}

// ResponseOutputItemAddedEvent represents the response.output_item.added event
type ResponseOutputItemAddedEvent struct {
	*BaseEvent
	ResponseID  string     `json:"response_id"`
	OutputIndex int        `json:"output_index"`
	Item        OutputItem `json:"item"`
}

// ResponseOutputItemDoneEvent represents the response.output_item.done event
type ResponseOutputItemDoneEvent struct {
	*BaseEvent
	ResponseID  string     `json:"response_id"`
	OutputIndex int        `json:"output_index"`
	Item        OutputItem `json:"item"`
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
	ItemID       string                 `json:"item_id"`
	ContentIndex int                    `json:"content_index"`
	Error        TranscriptionErrorInfo `json:"error"`
}

// Client-to-server event types
// These are used for constructing messages to send to the OpenAI API

// SessionConfig holds configuration for session update
type SessionConfig struct {
	Voice                   Voice                          `json:"voice,omitempty"`
	Modalities              []string                       `json:"modalities,omitempty"`
	InputAudioFormat        AudioFormat                    `json:"input_audio_format,omitempty"`
	OutputAudioFormat       AudioFormat                    `json:"output_audio_format,omitempty"`
	Instructions            string                         `json:"instructions,omitempty"`
	Temperature             *float64                       `json:"temperature,omitempty"`
	TurnDetection           *TurnDetectionConfig           `json:"turn_detection,omitempty"`
	TopP                    *float64                       `json:"top_p,omitempty"`
	PresencePenalty         *float64                       `json:"presence_penalty,omitempty"`
	FrequencyPenalty        *float64                       `json:"frequency_penalty,omitempty"`
	MaxResponseOutputTokens interface{}                    `json:"max_response_output_tokens,omitempty"` // can be number or "inf"
	InputAudioTranscription *InputAudioTranscriptionConfig `json:"input_audio_transcription,omitempty"`
	SpeechSettings          *SpeechSettings                `json:"speech_settings,omitempty"`
	Tools                   []interface{}                  `json:"tools,omitempty"`
	ToolChoice              ToolChoiceLiteral              `json:"tool_choice,omitempty"`
}

// SessionUpdateRequest represents the session.update request
type SessionUpdateRequest struct {
	Type    ClientEventType `json:"type"` // "session.update"
	Session SessionConfig   `json:"session"`
}

// AudioBufferAppendRequest represents the input_audio_buffer.append request
type AudioBufferAppendRequest struct {
	Type  ClientEventType `json:"type"`  // "input_audio_buffer.append"
	Audio string          `json:"audio"` // Base64-encoded audio chunk
}

// AudioBufferCommitRequest represents the input_audio_buffer.commit request
type AudioBufferCommitRequest struct {
	Type ClientEventType `json:"type"` // "input_audio_buffer.commit"
}

// ConversationItemCreateRequest represents the conversation.item.create request
type ConversationItemCreateRequest struct {
	Type           ClientEventType  `json:"type"` // "conversation.item.create"
	PreviousItemID string           `json:"previous_item_id,omitempty"`
	Item           ConversationItem `json:"item"`
}

// AudioBufferClearRequest represents the input_audio_buffer.clear message
type AudioBufferClearRequest struct {
	Type ClientEventType `json:"type"` // "input_audio_buffer.clear"
}

// ConversationItemTruncateRequest represents the conversation.item.truncate message
type ConversationItemTruncateRequest struct {
	Type         ClientEventType `json:"type"`    // "conversation.item.truncate"
	ItemID       string          `json:"item_id"` // ID of the item to truncate from
	ContentIndex int             `json:"content_index"`
	AudioEndMs   int             `json:"audio_end_ms"`
}

// ConversationItemDeleteRequest represents the conversation.item.delete message
type ConversationItemDeleteRequest struct {
	Type   ClientEventType `json:"type"`    // "conversation.item.delete"
	ItemID string          `json:"item_id"` // ID of the item to delete
}

// ResponseConfig holds configuration for response creation
type ResponseConfig struct {
	Modalities              []string          `json:"modalities,omitempty"`
	Instructions            string            `json:"instructions,omitempty"`
	Voice                   Voice             `json:"voice,omitempty"`
	OutputAudioFormat       AudioFormat       `json:"output_audio_format,omitempty"`
	Tools                   []interface{}     `json:"tools,omitempty"`
	ToolChoice              ToolChoiceLiteral `json:"tool_choice,omitempty"`
	Temperature             *float64          `json:"temperature,omitempty"`
	MaxResponseOutputTokens interface{}       `json:"max_response_output_tokens,omitempty"` // can be number or "inf"
	Conversation            string            `json:"conversation,omitempty"`               // "auto" or "none"
	Metadata                map[string]string `json:"metadata,omitempty"`
	Input                   []interface{}     `json:"input,omitempty"`
}

// ResponseCreateRequest represents the response.create message
type ResponseCreateRequest struct {
	Type     ClientEventType `json:"type"` // "response.create"
	Response *ResponseConfig `json:"response,omitempty"`
}

// ResponseCancelRequest represents the response.cancel message
type ResponseCancelRequest struct {
	Type ClientEventType `json:"type"` // "response.cancel"
}

// Helper function to encode audio data in base64
func EncodeBase64(data []byte) string {
	return "base64:" + base64.StdEncoding.EncodeToString(data)
}
