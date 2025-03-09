# OpenAI Realtime HTTP Go Implementation

## Project Overview
This is a Go implementation of the OpenAI Realtime HTTP server, enabling real-time voice and text conversations with OpenAI's models through a browser interface.

## Development Setup

### Prerequisites
- Go 1.21+ installed
- FFmpeg installed (for audio processing)
- OpenAI API key with access to Realtime API

### Build/Run Commands
- **Build**: `go build -o realtime-server`
- **Run**: `./realtime-server` or `go run main.go`
- **Test single package**: `go test ./pkg/websocket`
- **Test all**: `go test ./...`
- **Format code**: `go fmt ./...`
- **Lint**: `golangci-lint run`
- **Generate SSL cert** (for HTTPS): `./scripts/generate-cert.sh`

### Environment Variables
- `OPENAI_API_KEY`: Your OpenAI API key
- `PORT`: Server port (default: 8000)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `TLS_CERT`: Path to TLS certificate (optional)
- `TLS_KEY`: Path to TLS key (optional)

## Code Organization
- `cmd/server/`: Main application entry point
- `pkg/`: Shared packages
  - `api/`: OpenAI API client
  - `audio/`: Audio processing utilities
  - `config/`: Configuration management
  - `websocket/`: WebSocket server implementation
- `internal/`: Application-specific code
  - `handler/`: HTTP and WebSocket handlers
  - `middleware/`: HTTP middleware
  - `service/`: Business logic
- `web/`: Frontend assets (HTML, CSS, JS)
  - `static/`: Static assets
  - `templates/`: HTML templates

## Style Guidelines
- Use meaningful variable/function names
- Add comments for complex logic
- Implement proper error handling (don't use `panic`)
- Structure WebSocket message handling with clear type definitions
- Implement proper graceful shutdown
- Use context for cancellation/timeouts
- Prefer dependency injection for testability

## Key Components Implementation
- WebSocket connections manager
- OpenAI Realtime client with event processing
- Audio format conversion with FFmpeg
- JSON message protocol matching Python implementation
- State management with concurrent access protection