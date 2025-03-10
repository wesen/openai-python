package realtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Constants for default values
const (
	DefaultModel      = "gpt-4o-realtime-preview-2024-10-01"
	BaseURL           = "wss://api.openai.com/v1/realtime"
	ConnectionTimeout = 10 * time.Second
	PingInterval      = 20 * time.Second
	PongWait          = 30 * time.Second
)

// clientImpl implements the Client interface with an improved concurrent architecture
type clientImpl struct {
	// Configuration
	apiKey string
	model  string
	logger zerolog.Logger

	// Components
	connManager     *connectionManager
	messageSender   *messageSender
	eventProcessor  *eventProcessor
	responseManager *responseManager

	// Lifecycle management
	ctx        context.Context
	cancelFunc context.CancelFunc
	eg         *errgroup.Group
	running    atomic.Bool

	// Session state
	sessionID atomic.Value // string
}

// NewClient creates a new Client with the given API key and model
func NewClient(apiKey string, model string) Client {
	// Use default model if none provided
	if model == "" {
		model = DefaultModel
	}

	// Create client with default logger
	client := &clientImpl{
		apiKey: apiKey,
		model:  model,
		logger: zerolog.New(os.Stderr).With().Timestamp().Logger(),
	}

	// Initialize the context for this client
	client.ctx, client.cancelFunc = context.WithCancel(context.Background())

	// Create the errgroup
	eg, egCtx := errgroup.WithContext(client.ctx)
	client.eg = eg

	// Use egCtx in place of client.ctx where cancellation needs to be propagated
	client.logger.Debug().Msg("Initialized errgroup with context: " + egCtx.Err().Error())

	// Create and initialize components
	client.initializeComponents()

	return client
}

// SetLogger sets the logger for the client and its components
func (c *clientImpl) SetLogger(logger zerolog.Logger) {
	c.logger = logger

	// Update logger in all components
	c.connManager.logger = logger.With().Str("component", "connection_manager").Logger()
	c.messageSender.logger = logger.With().Str("component", "message_sender").Logger()
	c.eventProcessor.logger = logger.With().Str("component", "event_processor").Logger()
	c.responseManager.logger = logger.With().Str("component", "response_manager").Logger()
}

// Connect establishes a WebSocket connection with the API
func (c *clientImpl) Connect(ctx context.Context) error {
	// Don't allow connecting if already running
	if c.running.Load() {
		return errors.New("client is already running")
	}

	// First initialize the connection
	if err := c.connManager.connect(ctx); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	// Start internal components after successful connection
	if err := c.startComponents(); err != nil {
		c.logger.Error().Err(err).Msg("Failed to start components")
		return fmt.Errorf("failed to start components: %w", err)
	}

	// Mark client as running
	c.running.Store(true)

	// Start listening for events
	err := c.ListenForEvents(c.ctx)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to start event listening")
		c.Close(ctx)
		return fmt.Errorf("failed to start event listening: %w", err)
	}

	return nil
}

// initializeComponents initializes all client components
func (c *clientImpl) initializeComponents() {
	// Create connection manager
	c.connManager = &connectionManager{
		client: c,
		logger: c.logger.With().Str("component", "connection_manager").Logger(),
	}

	// Create message sender (shares connection mutex with connection manager)
	c.messageSender = &messageSender{
		client:    c,
		connMutex: &c.connManager.connMutex,
		msgQueue:  make(chan interface{}, 100),
		logger:    c.logger.With().Str("component", "message_sender").Logger(),
	}

	// Create event processor (shares connection mutex with connection manager)
	c.eventProcessor = &eventProcessor{
		client:    c,
		connMutex: &c.connManager.connMutex,
		eventChan: make(chan Event, 100),
		logger:    c.logger.With().Str("component", "event_processor").Logger(),
	}

	// Create response manager
	c.responseManager = &responseManager{
		client: c,
		logger: c.logger.With().Str("component", "response_manager").Logger(),
	}

	// Initialize atomic values
	c.sessionID.Store("")
	c.responseManager.currentResponseID.Store("")
	c.responseManager.currentResponseText.Store("")
}

// startComponents starts all client components
func (c *clientImpl) startComponents() error {
	// Update connection references in components after connection is established
	c.connManager.connMutex.RLock()
	conn := c.connManager.conn
	c.connManager.connMutex.RUnlock()

	if conn == nil {
		return errors.New("no active connection")
	}

	// Update connection reference in components
	c.messageSender.conn = conn
	c.eventProcessor.conn = conn

	// Start each component
	if err := c.connManager.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start connection manager: %w", err)
	}

	if err := c.messageSender.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start message sender: %w", err)
	}

	if err := c.eventProcessor.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start event processor: %w", err)
	}

	return nil
}

// Close terminates the WebSocket connection
func (c *clientImpl) Close(ctx context.Context) error {
	if !c.running.Load() {
		return nil // Already closed
	}

	// Set running to false first to prevent new operations
	c.running.Store(false)

	// Cancel the client context to stop all operations
	c.cancelFunc()

	// Stop components in reverse order
	var firstErr error

	// Stop event processor
	if err := c.eventProcessor.Stop(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("failed to stop event processor: %w", err)
	}

	// Stop message sender
	if err := c.messageSender.Stop(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("failed to stop message sender: %w", err)
	}

	// Stop connection manager (closes the connection)
	if err := c.connManager.Stop(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("failed to stop connection manager: %w", err)
	}

	// Wait for all goroutines managed by errgroup to complete
	if err := c.eg.Wait(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("error while waiting for goroutines to complete: %w", err)
	}

	return firstErr
}

// SendAudio sends audio data to the API
func (c *clientImpl) SendAudio(ctx context.Context, audio []byte) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

	// Encode audio to base64
	encAudio := encodeBase64(audio)

	// Create audio buffer append message
	appendMsg := AudioBufferAppendRequest{
		Type:  "input_audio_buffer.append",
		Audio: encAudio,
	}

	// Send the message
	return c.messageSender.SendMessage(ctx, appendMsg)
}

// CommitAudio signals that the user has finished speaking
func (c *clientImpl) CommitAudio(ctx context.Context) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

	// Create commit message
	commitMsg := AudioBufferCommitRequest{
		Type: "input_audio_buffer.commit",
	}

	// Send the message
	return c.messageSender.SendMessage(ctx, commitMsg)
}

// SendText sends a text message to the API
func (c *clientImpl) SendText(ctx context.Context, text string) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

	// Create text message
	textMsg := ConversationItemCreateRequest{
		Type: "conversation.item.create",
		Item: ConversationItem{
			Role: "user",
			Type: "text",
			Content: ItemContent{
				Text: text,
			},
		},
	}

	// Send the message
	return c.messageSender.SendMessage(ctx, textMsg)
}

// SetEventHandler registers a handler for specific event types
func (c *clientImpl) SetEventHandler(eventType string, handler EventHandler) {
	c.eventProcessor.SetEventHandler(eventType, handler)
}

// ListenForEvents starts listening for events from the WebSocket
func (c *clientImpl) ListenForEvents(ctx context.Context) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

	// Add to the errgroup to track this goroutine
	c.eg.Go(func() error {
		return c.eventProcessor.listenLoop(ctx)
	})

	return nil
}

// UpdateSession updates the session configuration
func (c *clientImpl) UpdateSession(ctx context.Context, config *Config) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

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
		if config.Tools != nil && len(config.Tools) > 0 {
			updateMsg.Session.Tools = config.Tools
		}

		if config.ToolChoice != "" {
			updateMsg.Session.ToolChoice = config.ToolChoice
		}
	}

	// Send the update message
	return c.messageSender.SendMessage(ctx, updateMsg)
}

// GetResponse returns the current response text and audio
func (c *clientImpl) GetResponse() (string, []byte) {
	return c.responseManager.GetResponse()
}

// Helper function to encode audio data in base64
func encodeBase64(data []byte) string {
	return EncodeBase64(data)
}
