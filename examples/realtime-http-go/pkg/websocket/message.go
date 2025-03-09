package websocket

// MessageType represents the type of a WebSocket message
type MessageType string

// MessageTypes
const (
	// Client to server message types
	MessageTypeClientAudioFormat MessageType = "audio_format"
	MessageTypeClientAudioData   MessageType = "audio_data"
	MessageTypeClientAudioEnd    MessageType = "audio_end"
	MessageTypeClientTextMessage MessageType = "text_message"

	// Server to client message types
	MessageTypeServerConnectionEstablished  MessageType = "connection_established"
	MessageTypeServerAudioFormatReceived    MessageType = "audio_format_received"
	MessageTypeServerAudioDataReceived      MessageType = "audio_data_received"
	MessageTypeServerAudioEndReceived       MessageType = "audio_end_received"
	MessageTypeServerTextMessageReceived    MessageType = "text_message_received"
	MessageTypeServerAudioStreamStart       MessageType = "audio_stream_start"
	MessageTypeServerAudioData              MessageType = "audio_data"
	MessageTypeServerTextDelta              MessageType = "text_delta"
	MessageTypeServerTranscript             MessageType = "transcript"
	MessageTypeServerUserTranscript         MessageType = "user_transcript"
	MessageTypeServerError                  MessageType = "error"
	MessageTypeServerResponseDone           MessageType = "response.done"
	MessageTypeServerSessionCreated         MessageType = "session.created"
	MessageTypeServerSessionUpdated         MessageType = "session.updated"
)

// Message represents a WebSocket message
type Message struct {
	Type    MessageType `json:"type"`
	Status  string      `json:"status,omitempty"`
	Message string      `json:"message,omitempty"`
	// Optional fields for different message types
	Data     string       `json:"data,omitempty"`
	Text     string       `json:"text,omitempty"`
	Format   *AudioFormat `json:"format,omitempty"`
	ItemID   string       `json:"item_id,omitempty"`
	Delta    string       `json:"delta,omitempty"`
	IsFinal  bool         `json:"is_final,omitempty"`
	Session  interface{}  `json:"session,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
}

// AudioFormat represents the audio format information
type AudioFormat struct {
	MimeType      string `json:"mimeType,omitempty"`
	SampleRate    int    `json:"sampleRate,omitempty"`
	Channels      int    `json:"channels,omitempty"`
	BitsPerSample int    `json:"bitsPerSample,omitempty"`
}

// NewAudioFormatMessage creates a new audio format message
func NewAudioFormatMessage(format *AudioFormat) Message {
	return Message{
		Type:   MessageTypeClientAudioFormat,
		Format: format,
	}
}

// NewAudioDataMessage creates a new audio data message
func NewAudioDataMessage(data string) Message {
	return Message{
		Type: MessageTypeClientAudioData,
		Data: data,
	}
}

// NewAudioEndMessage creates a new audio end message
func NewAudioEndMessage() Message {
	return Message{
		Type: MessageTypeClientAudioEnd,
	}
}

// NewTextMessage creates a new text message
func NewTextMessage(text string) Message {
	return Message{
		Type: MessageTypeClientTextMessage,
		Text: text,
	}
}

// NewErrorMessage creates a new error message
func NewErrorMessage(errorMessage string) Message {
	return Message{
		Type:    MessageTypeServerError,
		Message: errorMessage,
	}
}

// NewAcknowledgmentMessage creates a new acknowledgment message
func NewAcknowledgmentMessage(messageType MessageType, status string) Message {
	return Message{
		Type:   messageType,
		Status: status,
	}
}

// NewAudioStreamStartMessage creates a new audio stream start message
func NewAudioStreamStartMessage(itemID string) Message {
	return Message{
		Type:   MessageTypeServerAudioStreamStart,
		ItemID: itemID,
	}
}

// NewServerAudioDataMessage creates a new audio data message from server to client
func NewServerAudioDataMessage(itemID, data string) Message {
	return Message{
		Type:   MessageTypeServerAudioData,
		ItemID: itemID,
		Data:   data,
	}
}

// NewTextDeltaMessage creates a new text delta message
func NewTextDeltaMessage(itemID, delta, text string) Message {
	return Message{
		Type:   MessageTypeServerTextDelta,
		ItemID: itemID,
		Delta:  delta,
		Text:   text,
	}
}

// NewTranscriptMessage creates a new transcript message
func NewTranscriptMessage(itemID, text string, isFinal bool) Message {
	return Message{
		Type:    MessageTypeServerTranscript,
		ItemID:  itemID,
		Text:    text,
		IsFinal: isFinal,
	}
}

// NewResponseDoneMessage creates a new response done message
func NewResponseDoneMessage() Message {
	return Message{
		Type: MessageTypeServerResponseDone,
	}
}

// NewSessionCreatedMessage creates a new session created message
func NewSessionCreatedMessage(sessionID string) Message {
	return Message{
		Type:      MessageTypeServerSessionCreated,
		SessionID: sessionID,
	}
}

// NewSessionUpdatedMessage creates a new session updated message
func NewSessionUpdatedMessage(session interface{}) Message {
	return Message{
		Type:    MessageTypeServerSessionUpdated,
		Session: session,
	}
}