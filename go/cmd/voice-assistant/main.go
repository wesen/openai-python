package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-go-golems/openai-realtime/pkg/openai/realtime"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	apiKey       string
	voice        string
	inputFile    string
	outputFile   string
	instructions string
	textInput    string
	logLevel     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "voice-assistant",
		Short: "A simple voice assistant using OpenAI Realtime API",
		Long: `A simple voice assistant example that shows how to use the
OpenAI Realtime API to create a voice-enabled assistant. It can take
either text or audio input and outputs both text and audio responses.`,
		Run: runVoiceAssistant,
	}

	// Define flags for the command
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "OpenAI API key (or set OPENAI_API_KEY env var)")
	rootCmd.Flags().StringVar(&voice, "voice", "echo", "Voice to use (alloy, echo, fable, onyx, nova, shimmer)")
	rootCmd.Flags().StringVar(&inputFile, "input", "", "Input audio file (WAV/PCM format)")
	rootCmd.Flags().StringVar(&outputFile, "output", "output.pcm", "Output audio file (PCM format)")
	rootCmd.Flags().StringVar(&instructions, "instructions", "You are a helpful assistant.", "System instructions for the assistant")
	rootCmd.Flags().StringVar(&textInput, "text", "", "Text input (instead of audio file)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runVoiceAssistant(cmd *cobra.Command, args []string) {
	// Set up logger
	logger := setupLogger(logLevel)

	// Get API key from environment if not provided via flag
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			logger.Fatal().Msg("OpenAI API key is required. Provide it with --api-key flag or OPENAI_API_KEY environment variable.")
		}
	}

	// Check that we have either text or audio input
	if textInput == "" && inputFile == "" {
		logger.Fatal().Msg("Either --text or --input flag must be provided.")
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create a new client
	client := realtime.NewClient(apiKey, "")

	// Set the logger
	client.SetLogger(logger)

	// Configure session
	config := &realtime.Config{
		Voice:        realtime.Voice(voice),
		Modalities:   []string{"text", "audio"},
		Instructions: instructions,
	}

	// Connect to the API
	logger.Info().Msg("Connecting to OpenAI Realtime API...")
	if err := client.Connect(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect")
	}
	defer client.Close(ctx)

	// Start the event listener
	go func() {
		if err := client.ListenForEvents(ctx); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start event listener")
		}
	}()

	err := client.WaitForSessionCreated(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to wait for session to be created")
	}

	// Update session with our configuration
	if err := client.UpdateSession(ctx, config); err != nil {
		logger.Fatal().Err(err).Msg("Failed to update session")
	}

	logger.Info().Str("voice", voice).Msg("Connected to OpenAI Realtime API")

	// Create a response assembler to help with handling responses
	assembler := realtime.NewResponseAssembler(client)

	// Process either text or audio input
	var responseText string
	var responseAudio []byte

	if textInput != "" {
		// Send text input
		logger.Info().Str("text", textInput).Msg("Sending text input")
		responseText, responseAudio, err = assembler.SendTextAndWaitForResponse(ctx, textInput)
	} else {
		// Read the input audio file
		logger.Info().Str("file", inputFile).Msg("Reading audio file")
		audioData, err := os.ReadFile(inputFile)
		if err != nil {
			logger.Fatal().Err(err).Str("file", inputFile).Msg("Failed to read audio file")
		}

		// Send audio input
		logger.Info().Int("bytes", len(audioData)).Msg("Sending audio input")

		// In a real application, you would stream audio in chunks
		// Here, we simulate by sending the whole file at once
		responseText, responseAudio, err = assembler.SendAudioAndWaitForResponse(ctx, audioData, false)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to send audio input")
		}
	}

	if err != nil {
		logger.Fatal().Err(err).Msg("Error processing input")
	}

	// Output the response
	logger.Info().Str("text", responseText).Msg("Received response")
	logger.Info().Int("bytes", len(responseAudio)).Msg("Received audio response")

	// Save the audio to a file
	if err := os.WriteFile(outputFile, responseAudio, 0644); err != nil {
		logger.Fatal().Err(err).Str("file", outputFile).Msg("Failed to write output file")
	}
	logger.Info().Str("file", outputFile).Msg("Audio saved to file")

	// Also print the response to stdout for user convenience
	fmt.Printf("\nAssistant response:\n%s\n", responseText)
	fmt.Printf("Audio saved to %s (%d bytes)\n", outputFile, len(responseAudio))
}

// setupLogger configures the zerolog logger with the specified log level
func setupLogger(level string) zerolog.Logger {
	// Set default level to info
	var logLvl zerolog.Level = zerolog.InfoLevel

	// Parse log level from flag
	switch level {
	case "debug":
		logLvl = zerolog.DebugLevel
	case "info":
		logLvl = zerolog.InfoLevel
	case "warn":
		logLvl = zerolog.WarnLevel
	case "error":
		logLvl = zerolog.ErrorLevel
	default:
		fmt.Printf("Unknown log level: %s, using 'info'\n", level)
	}

	// Create pretty console writer
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}

	// Create logger
	return zerolog.New(output).With().Timestamp().Caller().Logger().Level(logLvl)
}
