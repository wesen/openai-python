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

## Updated Client Implementation to Match Latest OpenAI Realtime API Specification

Updated the client implementation to align with the latest OpenAI Realtime API specification, fixing compatibility issues:

- Fixed the text input method to use `conversation.item.create` instead of the unsupported `input_text` event type
- Updated error event structure to match the actual API response format with nested error object
- Updated session created event structure to match the actual API response format with detailed fields
- Added support for additional event types: `input_audio_buffer.clear`, `conversation.item.truncate`, `conversation.item.delete`, `response.create`, and `response.cancel`
- Updated session ID handling to use the correct field path in the API response

### Changes

- Updated `SendText` method to use the correct event type
- Revised event structs to match actual API response formats
- Added missing client-to-server event types
- Fixed error handling to correctly process error messages from the API
- Updated session creation handling to use the proper session ID field

## Fixed Conversation Item Create Message Format

Fixed the `conversation.item.create` message format to include the required `item.type` field:

- Added the missing `item.type` field to the `ConversationItemCreateRequest` struct
- Updated the `SendText` method to include the `type: "text"` field in the item object
- Fixed compatibility issue with the OpenAI Realtime API which requires this field

### Changes

- Updated the `ConversationItemCreateRequest` struct with the new field
- Modified the `SendText` method implementation to include the field
- Fixed the "Missing required parameter: 'item.type'" error

## Updated Realtime API Message Types to Match Azure Documentation

Updated the message types in the OpenAI Realtime API client to match the Azure OpenAI Realtime API reference. This includes:

- Added missing fields to the `SessionCreatedEvent` structure (InputAudioTranscription, TopP, PresencePenalty, FrequencyPenalty, SpeechSettings)
- Updated `SessionUpdatedEvent` structure with all configurable fields and proper field types
- Added `Type` field to conversation items and metadata support
- Enhanced `Config` struct with additional options for speech settings, audio transcription, and model parameters
- Updated `UpdateSession` method to handle all the new configuration options
- Improved error handling and logging in the event processor
- Properly structured request and response types to match the API specification

These changes ensure compatibility with the latest API specification and provide access to all available features. 

## Fixed Nil Pointer Dereference in NewClient

Fixed a nil pointer dereference in the NewClient function that was causing the client to crash when initializing:

- Added a nil check for the context error before calling Error() on it
- Added proper error handling for the errgroup context in the initialization process
- Improved debug logging to handle the case where the context is not yet canceled

This fixes the panic that occurred when running the voice assistant with text input. 

## Fixed WebSocket Protocol Handling for Frame Fragmentation

Enhanced the WebSocket protocol handling to address "continuation after FIN" errors that were causing connection failures:

- Disabled WebSocket compression to avoid fragmentation issues
- Added improved logging for WebSocket protocol errors to help diagnose connection issues
- Enhanced error handling specifically for fragmentation-related protocol errors
- Added more detailed connection logging to provide better diagnostics for WebSocket connectivity
- Fixed potential handling of fragmented frames by ensuring proper WebSocket configuration

These changes help prevent and diagnose WebSocket protocol errors that were causing the client to disconnect prematurely.

### Changes

- Disabled compression in the WebSocket dialer configuration
- Added specific error detection for "continuation after FIN" and other protocol errors
- Enhanced logging to include WebSocket message type and size information
- Improved WebSocket connection establishment with better configuration
- Added more detailed error logging to help diagnose connection issues

## Refactored WebSocket Connection Handling Architecture

Redesigned the WebSocket connection handling to improve reliability and reduce complexity:

- Merged connectionManager and messageSender into a single unified connectionHandler component
- Removed unnecessary mutex synchronization by adopting a clear ownership model for the WebSocket connection
- Improved message routing between components with a cleaner event flow design
- Fixed WebSocket protocol handling issues that caused "continuation after FIN" errors
- Enhanced WebSocket connection stability with improved frame handling
- Reduced goroutine count while maintaining concurrency benefits

These changes simplify the codebase, make it more maintainable, and fix issues with WebSocket fragmentation.

### Changes

- Created new connectionHandler component that combines the functionality of connectionManager and messageSender
- Modified event processor to work with the connectionHandler instead of accessing the WebSocket directly
- Updated client implementation to use the unified architecture
- Added better WebSocket protocol error detection with specific error messages
- Improved WebSocket connection settings to prevent fragmentation issues
- Enhanced logging to provide more context for WebSocket errors

# Changelog 