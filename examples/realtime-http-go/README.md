# OpenAI Realtime HTTP Go Implementation

A Go implementation of the OpenAI Realtime HTTP server, enabling real-time voice and text conversations with OpenAI's models through a browser interface. This implementation uses templ for templating and htmx for interactive web interfaces without complex JavaScript.

## Features

- Real-time chat using WebSockets
- Text-based messaging
- Audio input/output (with proper configuration)
- Simple, responsive UI
- Built with Go, templ and htmx

## Prerequisites

- Go 1.21+ installed
- FFmpeg installed (for audio processing)
- OpenAI API key with access to Realtime API
- templ CLI for template generation (optional for development)

## Quick Start

1. Clone the repository
2. Copy `.env.example` to `.env` and add your OpenAI API key
3. Build and run the server:

```bash
# Download dependencies
go mod tidy

# Generate templ templates
go generate ./...

# Build the server
go build -o realtime-server ./cmd/server

# Run the server
./realtime-server
```

4. Open your browser at http://localhost:8000

## Development

### Project Structure

- `cmd/server/`: Main application entry point
- `pkg/`: Shared packages
  - `api/`: OpenAI API client
  - `audio/`: Audio processing utilities
  - `config/`: Configuration management
  - `websocket/`: WebSocket server implementation
- `internal/`: Application-specific code
  - `handler/`: HTTP and WebSocket handlers
- `web/`: Frontend assets
  - `templates/`: templ HTML templates
  - `static/`: Static assets (CSS, JS)

### Key Commands

- **Build**: `go build -o realtime-server ./cmd/server`
- **Run**: `./realtime-server` or `go run ./cmd/server/main.go`
- **Generate templates**: `templ generate`
- **Format code**: `go fmt ./...`
- **Test**: `go test ./...`

### Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key
- `PORT`: Server port (default: 8000)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `TLS_CERT`: Path to TLS certificate (optional)
- `TLS_KEY`: Path to TLS key (optional)

## How It Works

1. The server establishes a WebSocket connection with the browser
2. Text messages are sent to the OpenAI API via the WebSocket
3. The API responds with text that is displayed in the chat
4. (When configured) Audio recording can be sent to the API for transcription and response
5. htmx handles updating the DOM based on WebSocket events

## Extending the Application

### Adding New API Features

To add new OpenAI API features, extend the `api/client.go` file with new methods for the desired endpoints.

### Customizing the UI

UI templates are in `web/templates/` and use the templ syntax. The CSS is in `web/static/css/styles.css`.

### Adding Authentication

For production use, consider adding authentication middleware in the `internal/middleware/` directory.

## License

[MIT License](LICENSE)