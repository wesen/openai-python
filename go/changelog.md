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

## Improved WebSocket Connection Reliability

Enhanced the WebSocket connection handling to fix connection issues and improve reliability:

- Extended connection and session creation timeouts to handle slower API responses
- Added ping/pong mechanisms to keep connections alive
- Improved WebSocket closure handling with graceful disconnection
- Added better error recovery for network issues
- Enhanced debug logging for troubleshooting connection problems

### Changes

- Increased context timeout for WebSocket operations
- Added ping/pong handlers and periodic ping sending
- Implemented proper WebSocket close handler
- Added read timeout handling to prevent indefinite blocking
- Improved logging of WebSocket lifecycle events

## Enhanced Logging and Session Handling

Improved the logging to include full message contents and enhanced the session creation process:

- Added logging for both incoming and outgoing WebSocket messages
- Implemented dedicated handler for initial session.created event
- Improved error detection and reporting during session initialization
- Added handling for common API events without requiring explicit handlers
- Added detailed message content logging for debugging purposes

### Changes

- Added `sendJSONMessage` helper for consistent outgoing message logging
- Added `handleDefaultEvent` method to process common event types
- Improved session creation flow with explicit session.created handling
- Added message body logging for both incoming and outgoing messages
- Added truncation for large message bodies (like audio data) in logs

## Refactored Client Implementation with Improved Concurrency Design

Completely refactored the OpenAI Realtime client implementation to provide a more robust and maintainable concurrency model with clear separation of concerns:

- Implemented a component-based architecture with proper lifecycle management
- Added context-based coordination of goroutines using errgroup
- Created dedicated components for connection management, message sending, event processing, and response handling
- Improved error handling and propagation across components
- Enhanced thread safety with atomic operations and proper mutex usage
- Implemented graceful shutdown sequence with timeouts

### Changes

- Reorganized code into modular components with clear responsibilities
- Added errgroup dependency for coordinated goroutine management
- Improved state management with atomic operations
- Enhanced WebSocket connection lifecycle handling
- Added proper separation between connection and message handling
- Improved event processing with cleaner handler registration
- Enhanced response management with thread-safe operations
- Implemented comprehensive context cancellation propagation 