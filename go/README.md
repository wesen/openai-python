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
)

func main() {
	// Create a client with your API key
	client := realtime.NewClient(os.Getenv("OPENAI_API_KEY"), "")

	// Connect to the API
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
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
		fmt.Printf("Failed to update session: %v\n", err)
		return
	}

	// Create a response assembler
	assembler := realtime.NewResponseAssembler(client)

	// Start listening for events
	if err := client.ListenForEvents(ctx); err != nil {
		fmt.Printf("Failed to start event listener: %v\n", err)
		return
	}

	// Send a text message and wait for response
	responseText, responseAudio, err := assembler.SendTextAndWaitForResponse(ctx, "Hello, how are you?")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print the response text
	fmt.Printf("Assistant: %s\n", responseText)

	// Save the audio to a file
	if err := os.WriteFile("response.pcm", responseAudio, 0644); err != nil {
		fmt.Printf("Failed to write audio: %v\n", err)
		return
	}
	fmt.Println("Audio saved to response.pcm")
}
```

## Example Application

This repository includes a simple voice assistant example in `cmd/voice-assistant`. You can run it like this:

```bash
# Using text input
go run cmd/voice-assistant/main.go --api-key YOUR_API_KEY --text "Hello, how are you?"

# Using audio input
go run cmd/voice-assistant/main.go --api-key YOUR_API_KEY --input input.pcm
```

Available flags:
- `--api-key`: Your OpenAI API key (can also use OPENAI_API_KEY env var)
- `--voice`: Voice to use (alloy, echo, fable, onyx, nova, shimmer)
- `--input`: Input audio file (PCM format)
- `--output`: Output audio file (default: output.pcm)
- `--instructions`: System instructions for the assistant
- `--text`: Text input (instead of audio file)

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

### Function Calling Support

The library supports OpenAI function calling, though detailed documentation for this feature is planned for a future release.

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Requirements

- Go 1.18 or higher
- An OpenAI API key with access to the Realtime API beta 