# OpenAI Realtime API - Go Client

This is a Go client library for the OpenAI Realtime API, which allows developers to create low-latency, multi-modal conversational experiences with voice. With this library, you can:

- Create real-time voice conversations with GPT-4o
- Stream audio input from a microphone
- Receive both text and audio responses
- Configure voices, system instructions, and more
- Handle streaming events for real-time interaction

## Features

- **Simple interface**: Easy-to-use client for the OpenAI Realtime API
- **Event-based architecture**: Handle events like transcription, text, and audio
- **Response assembly**: Utilities to simplify handling of streaming responses
- **Context support**: All methods support context for cancellation and timeouts
- **Configurable voices**: Choose from multiple preset voices
- **Audio format support**: Handle PCM audio data
- **Error handling**: Robust error handling for WebSocket connections
- **Function calling**: Support for OpenAI function calling (tools)
- **Structured logging**: Zerolog-based structured logging with configurable levels

## Installation

```bash
go get github.com/go-go-golems/openai-realtime
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-go-golems/openai-realtime/pkg/openai/realtime"
	"github.com/rs/zerolog"
)

func main() {
	// Set up a logger
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	logger := zerolog.New(output).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	// Create a client with your API key
	client := realtime.NewClient(os.Getenv("OPENAI_API_KEY"), "")
	
	// Set the logger for the client
	client.SetLogger(logger)

	// Connect to the API
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		logger.Error().Err(err).Msg("Failed to connect")
		return
	}
	defer client.Close(ctx)

	// Configure the session
	config := &realtime.Config{
		Voice:        "echo",
		Modalities:   []string{"text", "audio"},
		Instructions: "You are a helpful assistant.",
	}
	if err := client.UpdateSession(ctx, config); err != nil {
		logger.Error().Err(err).Msg("Failed to update session")
		return
	}

	// Create a response assembler
	assembler := realtime.NewResponseAssembler(client)

	// Start listening for events
	if err := client.ListenForEvents(ctx); err != nil {
		logger.Error().Err(err).Msg("Failed to start event listener")
		return
	}

	// Send a text message and wait for response
	responseText, responseAudio, err := assembler.SendTextAndWaitForResponse(ctx, "Hello, how are you?")
	if err != nil {
		logger.Error().Err(err).Msg("Error getting response")
		return
	}

	// Print the response text
	fmt.Printf("Assistant: %s\n", responseText)

	// Save the audio to a file
	if err := os.WriteFile("response.pcm", responseAudio, 0644); err != nil {
		logger.Error().Err(err).Msg("Failed to write audio")
		return
	}
	logger.Info().Str("file", "response.pcm").Msg("Audio saved to file")
}
```

## Example Application

This repository includes a simple voice assistant example in `cmd/voice-assistant`. You can run it like this:

```bash
# Using text input
go run cmd/voice-assistant/main.go --api-key YOUR_API_KEY --text "Hello, how are you?"

# Using audio input
go run cmd/voice-assistant/main.go --api-key YOUR_API_KEY --input input.pcm

# With debug logging
go run cmd/voice-assistant/main.go --api-key YOUR_API_KEY --text "Hello" --log-level debug
```

Available flags:
- `--api-key`: Your OpenAI API key (can also use OPENAI_API_KEY env var)
- `--voice`: Voice to use (alloy, echo, fable, onyx, nova, shimmer)
- `--input`: Input audio file (PCM format)
- `--output`: Output audio file (default: output.pcm)
- `--instructions`: System instructions for the assistant
- `--text`: Text input (instead of audio file)
- `--log-level`: Logging level (debug, info, warn, error)

## Logging

The client includes structured logging with zerolog. You can configure the log level to help debug connection issues:

- `debug`: Verbose logging including WebSocket message details, useful for troubleshooting
- `info`: Standard operational information (default)
- `warn`: Only warnings and errors
- `error`: Only errors

Example of enabling debug logging:

```go
// Create a console writer with pretty output
output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
logger := zerolog.New(output).With().Timestamp().Logger().Level(zerolog.DebugLevel)

// Set the logger for the client
client.SetLogger(logger)
```

## Audio Format

The default audio format is 16 kHz 16-bit PCM. The API allows configuring:
- `input_audio_format`: Format of audio sent to the API
- `output_audio_format`: Format of audio received from the API

Common formats include `pcm_s16le` (16-bit PCM) and `g711_ulaw` (used in telephony).

## Working with PCM Audio

To convert an existing audio file to PCM format using FFmpeg:

```bash
ffmpeg -i input.mp3 -ar 16000 -ac 1 -f s16le output.pcm
```

To play a PCM audio file:

```bash
# Using SoX
play -t raw -r 16k -e signed -b 16 -c 1 output.pcm

# Using FFmpeg
ffplay -f s16le -ar 16000 -ac 1 output.pcm
```

To record PCM audio from a microphone:

```bash
# Using SoX
rec -t raw -r 16k -e signed -b 16 -c 1 input.pcm
```

## Advanced Usage

### Registering Event Handlers

You can register custom handlers for various event types:

```go
client.SetEventHandler(realtime.EventTranscriptionCompleted, func(event realtime.Event) error {
	if e, ok := event.(*realtime.TranscriptionCompletedEvent); ok {
		fmt.Printf("Transcription: %s\n", e.Item.Content.Text)
	}
	return nil
})
```

### Streaming Audio in Chunks

For real-time applications, you can stream audio in chunks:

```go
// Create a channel to send audio chunks
audioChunks := make(chan []byte)

// In a separate goroutine, capture audio from microphone and send to channel
go captureAudio(audioChunks)

// Stream audio and wait for response
responseText, responseAudio, err := assembler.StreamAudioAndWaitForResponse(
	ctx, audioChunks, 100*time.Millisecond, false)
```

### Debugging WebSocket Issues

If you're experiencing WebSocket connection issues, enable debug logging to see detailed information about the connection process:

```bash
go run cmd/voice-assistant/main.go --api-key YOUR_API_KEY --text "Hello" --log-level debug
```

Common issues and solutions:

1. **"use of closed network connection"**: This typically happens when the WebSocket connection is closed unexpectedly. Enable debug logging to see the exact reason for the closure.

2. **"context deadline exceeded"**: This occurs when the connection takes too long to establish. This could be due to network issues or the API being temporarily unavailable.

3. **"Failed to connect: 401 Unauthorized"**: Check that your API key is valid and has access to the Realtime API beta.

### Function Calling Support

The library supports OpenAI function calling, though detailed documentation for this feature is planned for a future release.

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Requirements

- Go 1.18 or higher
- An OpenAI API key with access to the Realtime API beta 