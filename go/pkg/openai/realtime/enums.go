package realtime

// AudioFormat represents the format of audio data
type AudioFormat string

const (
	// AudioFormatPCM16 is the PCM 16-bit audio format
	AudioFormatPCM16 AudioFormat = "pcm16"
	// AudioFormatG711ULAW is the G.711 μ-law audio format
	AudioFormatG711ULAW AudioFormat = "g711_ulaw"
	// AudioFormatG711ALAW is the G.711 A-law audio format
	AudioFormatG711ALAW AudioFormat = "g711_alaw"
)

// AudioInputTranscriptionModel represents the model used for audio transcription
type AudioInputTranscriptionModel string

const (
	// AudioInputTranscriptionModelWhisper1 is the Whisper-1 transcription model
	AudioInputTranscriptionModelWhisper1 AudioInputTranscriptionModel = "whisper-1"
)

// ClientEventType represents the type of client events that can be sent to the server
type ClientEventType string

const (
	// ClientEventTypeSessionUpdate is the event type for updating session configuration
	ClientEventTypeSessionUpdate ClientEventType = "session.update"
	// ClientEventTypeInputAudioBufferAppend is the event type for appending audio to the input buffer
	ClientEventTypeInputAudioBufferAppend ClientEventType = "input_audio_buffer.append"
	// ClientEventTypeInputAudioBufferCommit is the event type for committing the input audio buffer
	ClientEventTypeInputAudioBufferCommit ClientEventType = "input_audio_buffer.commit"
	// ClientEventTypeInputAudioBufferClear is the event type for clearing the input audio buffer
	ClientEventTypeInputAudioBufferClear ClientEventType = "input_audio_buffer.clear"
	// ClientEventTypeConversationItemCreate is the event type for creating a conversation item
	ClientEventTypeConversationItemCreate ClientEventType = "conversation.item.create"
	// ClientEventTypeConversationItemDelete is the event type for deleting a conversation item
	ClientEventTypeConversationItemDelete ClientEventType = "conversation.item.delete"
	// ClientEventTypeConversationItemTruncate is the event type for truncating a conversation item
	ClientEventTypeConversationItemTruncate ClientEventType = "conversation.item.truncate"
	// ClientEventTypeResponseCreate is the event type for creating a response
	ClientEventTypeResponseCreate ClientEventType = "response.create"
	// ClientEventTypeResponseCancel is the event type for canceling a response
	ClientEventTypeResponseCancel ClientEventType = "response.cancel"
)

// ContentPartType represents the type of content parts within messages
type ContentPartType string

const (
	// ContentPartTypeInputText is the type for input text content
	ContentPartTypeInputText ContentPartType = "input_text"
	// ContentPartTypeInputAudio is the type for input audio content
	ContentPartTypeInputAudio ContentPartType = "input_audio"
	// ContentPartTypeText is the type for text content
	ContentPartTypeText ContentPartType = "text"
	// ContentPartTypeAudio is the type for audio content
	ContentPartTypeAudio ContentPartType = "audio"
	// ContentPartTypeItemReference is the type for item reference content
	ContentPartTypeItemReference ContentPartType = "item_reference"
)

// ItemStatus represents the status of an item in the conversation
type ItemStatus string

const (
	// ItemStatusInProgress indicates that the item is in progress
	ItemStatusInProgress ItemStatus = "in_progress"
	// ItemStatusCompleted indicates that the item is completed
	ItemStatusCompleted ItemStatus = "completed"
	// ItemStatusIncomplete indicates that the item is incomplete
	ItemStatusIncomplete ItemStatus = "incomplete"
)

// ItemType represents the type of items in the conversation
type ItemType string

const (
	// ItemTypeMessage is the type for message items
	ItemTypeMessage ItemType = "message"
	// ItemTypeFunctionCall is the type for function call items
	ItemTypeFunctionCall ItemType = "function_call"
	// ItemTypeFunctionCallOutput is the type for function call output items
	ItemTypeFunctionCallOutput ItemType = "function_call_output"
)

// MessageRole represents the role of a message in the conversation
type MessageRole string

const (
	// MessageRoleSystem is the role for system messages
	MessageRoleSystem MessageRole = "system"
	// MessageRoleUser is the role for user messages
	MessageRoleUser MessageRole = "user"
	// MessageRoleAssistant is the role for assistant messages
	MessageRoleAssistant MessageRole = "assistant"
)

// ResponseStatus represents the status of a response
type ResponseStatus string

const (
	// ResponseStatusInProgress indicates that the response is in progress
	ResponseStatusInProgress ResponseStatus = "in_progress"
	// ResponseStatusCompleted indicates that the response is completed
	ResponseStatusCompleted ResponseStatus = "completed"
	// ResponseStatusCancelled indicates that the response was cancelled
	ResponseStatusCancelled ResponseStatus = "cancelled"
	// ResponseStatusIncomplete indicates that the response is incomplete
	ResponseStatusIncomplete ResponseStatus = "incomplete"
	// ResponseStatusFailed indicates that the response failed
	ResponseStatusFailed ResponseStatus = "failed"
)

// ServerEventType represents the type of server events that can be received from the server
type ServerEventType string

const (
	// ServerEventTypeSessionCreated is the event type for session creation
	ServerEventTypeSessionCreated ServerEventType = "session.created"
	// ServerEventTypeSessionUpdated is the event type for session updates
	ServerEventTypeSessionUpdated ServerEventType = "session.updated"
	// ServerEventTypeConversationCreated is the event type for conversation creation
	ServerEventTypeConversationCreated ServerEventType = "conversation.created"
	// ServerEventTypeConversationItemCreated is the event type for conversation item creation
	ServerEventTypeConversationItemCreated ServerEventType = "conversation.item.created"
	// ServerEventTypeConversationItemDeleted is the event type for conversation item deletion
	ServerEventTypeConversationItemDeleted ServerEventType = "conversation.item.deleted"
	// ServerEventTypeConversationItemTruncated is the event type for conversation item truncation
	ServerEventTypeConversationItemTruncated ServerEventType = "conversation.item.truncated"
	// ServerEventTypeResponseCreated is the event type for response creation
	ServerEventTypeResponseCreated ServerEventType = "response.created"
	// ServerEventTypeResponseDone is the event type for response completion
	ServerEventTypeResponseDone ServerEventType = "response.done"
	// ServerEventTypeRateLimitsUpdated is the event type for rate limit updates
	ServerEventTypeRateLimitsUpdated ServerEventType = "rate_limits.updated"
	// ServerEventTypeResponseOutputItemAdded is the event type for response output item addition
	ServerEventTypeResponseOutputItemAdded ServerEventType = "response.output_item.added"
	// ServerEventTypeResponseOutputItemDone is the event type for response output item completion
	ServerEventTypeResponseOutputItemDone ServerEventType = "response.output_item.done"
	// ServerEventTypeResponseContentPartAdded is the event type for response content part addition
	ServerEventTypeResponseContentPartAdded ServerEventType = "response.content_part.added"
	// ServerEventTypeResponseContentPartDone is the event type for response content part completion
	ServerEventTypeResponseContentPartDone ServerEventType = "response.content_part.done"
	// ServerEventTypeResponseAudioDelta is the event type for response audio delta updates
	ServerEventTypeResponseAudioDelta ServerEventType = "response.audio.delta"
	// ServerEventTypeResponseAudioDone is the event type for response audio completion
	ServerEventTypeResponseAudioDone ServerEventType = "response.audio.done"
	// ServerEventTypeResponseAudioTranscriptDelta is the event type for response audio transcript delta updates
	ServerEventTypeResponseAudioTranscriptDelta ServerEventType = "response.audio_transcript.delta"
	// ServerEventTypeResponseAudioTranscriptDone is the event type for response audio transcript completion
	ServerEventTypeResponseAudioTranscriptDone ServerEventType = "response.audio_transcript.done"
	// ServerEventTypeResponseTextDelta is the event type for response text delta updates
	ServerEventTypeResponseTextDelta ServerEventType = "response.text.delta"
	// ServerEventTypeResponseTextDone is the event type for response text completion
	ServerEventTypeResponseTextDone ServerEventType = "response.text.done"
	// ServerEventTypeResponseFunctionCallArgumentsDelta is the event type for response function call arguments delta updates
	ServerEventTypeResponseFunctionCallArgumentsDelta ServerEventType = "response.function_call_arguments.delta"
	// ServerEventTypeResponseFunctionCallArgumentsDone is the event type for response function call arguments completion
	ServerEventTypeResponseFunctionCallArgumentsDone ServerEventType = "response.function_call_arguments.done"
	// ServerEventTypeInputAudioBufferSpeechStarted is the event type for input audio buffer speech start detection
	ServerEventTypeInputAudioBufferSpeechStarted ServerEventType = "input_audio_buffer.speech_started"
	// ServerEventTypeInputAudioBufferSpeechStopped is the event type for input audio buffer speech stop detection
	ServerEventTypeInputAudioBufferSpeechStopped ServerEventType = "input_audio_buffer.speech_stopped"
	// ServerEventTypeConversationItemInputAudioTranscriptionCompleted is the event type for conversation item input audio transcription completion
	ServerEventTypeConversationItemInputAudioTranscriptionCompleted ServerEventType = "conversation.item.input_audio_transcription.completed"
	// ServerEventTypeConversationItemInputAudioTranscriptionFailed is the event type for conversation item input audio transcription failure
	ServerEventTypeConversationItemInputAudioTranscriptionFailed ServerEventType = "conversation.item.input_audio_transcription.failed"
	// ServerEventTypeInputAudioBufferCommitted is the event type for input audio buffer commitment
	ServerEventTypeInputAudioBufferCommitted ServerEventType = "input_audio_buffer.committed"
	// ServerEventTypeInputAudioBufferCleared is the event type for input audio buffer clearing
	ServerEventTypeInputAudioBufferCleared ServerEventType = "input_audio_buffer.cleared"
	// ServerEventTypeError is the event type for errors
	ServerEventTypeError ServerEventType = "error"
)

// ToolType represents the type of tool
type ToolType string

const (
	// ToolTypeFunction is the type for function tools
	ToolTypeFunction ToolType = "function"
)

// ToolChoiceLiteral represents the available tool choice options
type ToolChoiceLiteral string

const (
	// ToolChoiceLiteralAuto automatically selects tools
	ToolChoiceLiteralAuto ToolChoiceLiteral = "auto"
	// ToolChoiceLiteralNone disables tool usage
	ToolChoiceLiteralNone ToolChoiceLiteral = "none"
	// ToolChoiceLiteralRequired requires tool usage
	ToolChoiceLiteralRequired ToolChoiceLiteral = "required"
)

// TurnDetectionType represents the type of turn detection
type TurnDetectionType string

const (
	// TurnDetectionTypeServerVAD uses server-side Voice Activity Detection
	TurnDetectionTypeServerVAD TurnDetectionType = "server_vad"
)

// Voice represents the voice options for audio responses
type Voice string

const (
	// VoiceAlloy is the alloy voice option
	VoiceAlloy Voice = "alloy"
	// VoiceAsh is the ash voice option
	VoiceAsh Voice = "ash"
	// VoiceBallad is the ballad voice option
	VoiceBallad Voice = "ballad"
	// VoiceCoral is the coral voice option
	VoiceCoral Voice = "coral"
	// VoiceEcho is the echo voice option
	VoiceEcho Voice = "echo"
	// VoiceSage is the sage voice option
	VoiceSage Voice = "sage"
	// VoiceShimmer is the shimmer voice option
	VoiceShimmer Voice = "shimmer"
	// VoiceVerse is the verse voice option
	VoiceVerse Voice = "verse"
)
