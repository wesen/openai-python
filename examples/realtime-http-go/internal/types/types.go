package types

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data,omitempty"`
	Text    string      `json:"text,omitempty"`
	ItemID  string      `json:"item_id,omitempty"`
	Status  string      `json:"status,omitempty"`
	IsFinal bool        `json:"is_final,omitempty"`
	Message string      `json:"message,omitempty"`
}

// AudioFormat represents the audio format information
type AudioFormat struct {
	MimeType   string `json:"mimeType,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

// OpenAIEvent represents an event from OpenAI's Realtime API
type OpenAIEvent struct {
	Type    string      `json:"type"`
	Session string      `json:"session,omitempty"`
	ItemID  string      `json:"item_id,omitempty"`
	Text    string      `json:"text,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	IsFinal bool        `json:"is_final,omitempty"`
	Message string      `json:"message,omitempty"`
}
