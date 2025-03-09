# Changelog

## OpenAI Realtime API Go Client Implementation

Initial implementation of the OpenAI Realtime API Go client. This implementation provides a Go package for interacting with the OpenAI Realtime API for voice-based conversations.

### Added
- Core client interface and implementation for WebSocket communication
- Comprehensive event types for all API events
- Response assembler utility for streamlining response handling
- Voice assistant example application
- Context-aware methods for connection, audio/text input, and response processing
- Robust error handling with context cancellation support
- Documentation with examples and audio format guidance

## Added Structured Logging with Zerolog

Added structured logging to the OpenAI Realtime API client to help debug WebSocket connection issues. The logging implementation uses zerolog and includes the following:

- Added a logger field to the client implementation
- Added a `SetLogger` method to the Client interface
- Added a log level flag to the voice-assistant example command
- Added detailed logging throughout the WebSocket lifecycle
- Enhanced error handling in WebSocket connection process
- Updated README with documentation on logging features

### Changes

- Added zerolog and cobra dependencies
- Rewritten voice-assistant to use cobra CLI framework
- Improved WebSocket error handling with detailed error logs
- Added logging for all major client operations
- Added debug-level logging for detailed WebSocket diagnostics 