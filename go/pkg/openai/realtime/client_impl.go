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
	connHandler     *connectionHandler
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
	client.ctx = egCtx

	// Create and initialize components
	client.initializeComponents()

	return client
}

// SetLogger sets the logger for the client and its components
func (c *clientImpl) SetLogger(logger zerolog.Logger) {
	c.logger = logger

	// Update logger in all components
	c.connHandler.logger = logger.With().Str("component", "connection_handler").Logger()
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
	if err := c.connHandler.Connect(ctx); err != nil {
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
	c.logger.Debug().Msg("Initializing components")

	// Create the connection handler (replaces connectionManager and messageSender)
	c.connHandler = newConnectionHandler(c)

	// Create the event processor and response manager
	c.eventProcessor = newEventProcessor(c)
	c.responseManager = newResponseManager(c)

	// Set up bidirectional references
	c.connHandler.SetEventProcessor(c.eventProcessor)

	// Initialize atomic values
	c.sessionID.Store("")
	c.responseManager.currentResponseID.Store("")
	c.responseManager.currentResponseText.Store("")
}

// startComponents starts all client components
func (c *clientImpl) startComponents() error {
	c.logger.Debug().Msg("Starting components")

	// Connect using the connection handler
	if err := c.connHandler.Connect(c.ctx); err != nil {
		c.logger.Error().Err(err).Msg("Failed to connect")
		return err
	}

	// Start the event processor
	if err := c.eventProcessor.Start(c.ctx); err != nil {
		c.logger.Error().Err(err).Msg("Failed to start event processor")
		return err
	}

	c.running.Store(true)
	return nil
}

// Close terminates the WebSocket connection
func (c *clientImpl) Close(ctx context.Context) error {
	c.logger.Debug().Msg("Closing client")

	if !c.running.Load() {
		c.logger.Debug().Msg("Client already closed")
		return nil
	}

	// Cancel the context to signal all components to stop
	c.cancelFunc()

	// Close the connection handler
	if err := c.connHandler.Close(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Error closing connection handler")
	}

	// Stop the event processor
	if err := c.eventProcessor.Stop(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Error stopping event processor")
	}

	// Wait for the error group to complete with a timeout
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		err := c.eg.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			c.logger.Error().Err(err).Msg("Error in goroutine group")
		}
		close(done)
	}()

	select {
	case <-done:
		c.logger.Debug().Msg("All goroutines exited cleanly")
	case <-waitCtx.Done():
		c.logger.Warn().Msg("Timeout waiting for goroutines to exit")
	}

	c.running.Store(false)
	return nil
}

// SendAudio sends audio data to the API
func (c *clientImpl) SendAudio(ctx context.Context, audio []byte) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

	// Base64 encode the audio data
	audioBase64 := encodeBase64(audio)

	// Create the audio buffer append message
	msg := AudioBufferAppendRequest{
		Type:  ClientEventTypeInputAudioBufferAppend,
		Audio: audioBase64,
	}

	// Send the message through the connection handler
	return c.connHandler.SendMessage(ctx, msg)
}

// CommitAudio signals that the user has finished speaking
func (c *clientImpl) CommitAudio(ctx context.Context) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

	// Create the commit message
	msg := AudioBufferCommitRequest{
		Type: ClientEventTypeInputAudioBufferCommit,
	}

	// Send the message through the connection handler
	return c.connHandler.SendMessage(ctx, msg)
}

// SendText sends a text message to the API
func (c *clientImpl) SendText(ctx context.Context, text string) error {
	if !c.running.Load() {
		return errors.New("client is not running")
	}

	// Create the message
	msg := ConversationItemCreateRequest{
		Type: ClientEventTypeConversationItemCreate,
		Item: ConversationItem{
			Role: MessageRoleUser,
			Type: ItemTypeMessage,
			Content: ItemContent{
				Text: text,
			},
		},
	}

	// Send the message through the connection handler
	return c.connHandler.SendMessage(ctx, msg)
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
	return c.connHandler.SendMessage(ctx, updateMsg)
}

// GetResponse returns the current response text and audio
func (c *clientImpl) GetResponse() (string, []byte) {
	return c.responseManager.GetResponse()
}

// Helper function to encode audio data in base64
func encodeBase64(data []byte) string {
	return EncodeBase64(data)
}

// Create a proper constructor function for the event processor
func newEventProcessor(client *clientImpl) *eventProcessor {
	return &eventProcessor{
		client:        client,
		eventChan:     make(chan Event, 100),
		eventHandlers: make(map[string][]EventHandler),
		logger:        client.logger.With().Str("component", "event_processor").Logger(),
	}
}

// Create a proper constructor function for the response manager
func newResponseManager(client *clientImpl) *responseManager {
	return &responseManager{
		client: client,
		logger: client.logger.With().Str("component", "response_manager").Logger(),
	}
}
