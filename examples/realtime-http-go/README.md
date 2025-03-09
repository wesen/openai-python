# OpenAI Realtime HTTP Go Server

This is a Go implementation of the OpenAI Realtime HTTP server, which demonstrates real-time audio and text communication with OpenAI's Realtime API. It provides a responsive web interface for users to interact with AI models through both voice and text inputs, with real-time streaming responses.

## Features

- Real-time audio streaming to OpenAI's Realtime API
- Text-based chat with OpenAI's Realtime API
- WebSocket-based communication for low-latency interactions
- Audio visualization
- Responsive web interface using Bootstrap and HTMX

## Requirements

- Go 1.18 or higher
- An OpenAI API key with access to the Realtime API

## Installation

1. Clone the repository:

```bash
git clone https://github.com/openai/openai-python.git
cd openai-python/examples/realtime-http-go
```

2. Install dependencies:

```bash
go mod tidy
```

3. Generate Templ templates:

```bash
go install github.com/a-h/templ/cmd/templ@latest
templ generate
```

## Configuration

Set the following environment variables:

- `OPENAI_API_KEY`: Your OpenAI API key
- `LISTEN_ADDR`: The address and port to listen on (default: `0.0.0.0:8000`)
- `LOG_LEVEL`: The logging level (default: `info`)
- `OPENAI_MODEL`: The OpenAI model to use (default: `gpt-4o-realtime-preview`)

## Running the Server

```bash
go run cmd/server/main.go
```

Then open your browser and navigate to `http://localhost:8000`.

## Usage

1. Click "Start Recording" to begin recording audio
2. Speak into your microphone
3. Click "Stop Recording" to stop recording
4. The AI will respond with both text and audio
5. You can also type a message in the text input and click "Send"

## Architecture

The application follows a client-server architecture:

1. **Backend**: A Go server that handles WebSocket connections, audio processing, and communication with OpenAI's Realtime API.
2. **Frontend**: A web interface built with HTML, CSS, and JavaScript that provides audio recording, visualization, and chat functionality.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 