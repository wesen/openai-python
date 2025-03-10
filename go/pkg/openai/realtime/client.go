// Package realtime provides a client for the OpenAI Realtime API.
package realtime

import (
	"context"

	"github.com/rs/zerolog"
)

// Client provides methods to interact with the OpenAI Realtime API
type Client interface {
	// Connect establishes a WebSocket connection with the OpenAI Realtime API
	Connect(ctx context.Context) error

	// Close terminates the WebSocket connection
	Close(ctx context.Context) error

	// SendAudio sends audio data to the API
	SendAudio(ctx context.Context, audio []byte) error

	// CommitAudio signals that the user has finished speaking
	CommitAudio(ctx context.Context) error

	// SendText sends a text message to the API
	SendText(ctx context.Context, text string) error

	// SetEventHandler registers handlers for different event types
	SetEventHandler(eventType string, handler EventHandler)

	// ListenForEvents starts processing events from the API
	ListenForEvents(ctx context.Context) error

	// UpdateSession updates the session configuration
	UpdateSession(ctx context.Context, config *Config) error

	// SetLogger sets the logger for the client
	SetLogger(logger zerolog.Logger)
}

// Config stores configuration options for the Realtime client
type Config struct {
	APIKey        string
	Model         string
	Voice         Voice
	Modalities    []string
	InputFormat   AudioFormat
	OutputFormat  AudioFormat
	Instructions  string
	TurnDetection TurnDetectionType
	Temperature   *float64
	Logger        *zerolog.Logger // Optional logger for the client
	LogLevel      zerolog.Level   // Optional log level (debug, info, warn, error)

	// Additional model parameters
	TopP                    *float64
	PresencePenalty         *float64
	FrequencyPenalty        *float64
	MaxResponseOutputTokens interface{} // can be number or "inf"

	// Input audio transcription configuration
	InputAudioTranscription *struct {
		Language        string
		Type            string // "server" or "client"
		Model           AudioInputTranscriptionModel
		Interim         bool
		PhraseHints     []string
		ProfanityFilter bool
		Redact          []string
		Diarize         bool
	}

	// Speech settings
	SpeechSettings *struct {
		Speed        float64
		Stability    float64
		Similarity   float64
		Style        float64
		PresenceText bool
	}

	// Function calling options
	Tools      []interface{}
	ToolChoice ToolChoiceLiteral
}

// Event represents a message from the API
type Event interface {
	Type() string
	RawData() []byte
}

// EventHandler processes an event from the API
type EventHandler func(Event) error
