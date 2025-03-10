# Changelog

## Added Enum Types for Better Type Safety

Created Go enum types for all TypeScript enums to improve type safety and code readability:

- Added enum types like `AudioFormat`, `ContentPartType`, `MessageRole`, etc. that correspond to TypeScript enums
- Updated all struct fields to use these typed enums instead of string values
- Updated the client `Config` struct to use the new enum types
- Refactored event type constants to reference the enum values
- Ensures consistent usage of enum values throughout the codebase
- Makes the Go code more aligned with the TypeScript definitions
- Fixed issue with `MaxResponseOutputTokens` in `Config` to handle both int and string values

## Fixed MaxResponseOutputTokens Type to Handle String Values

Updated the SessionConfig struct to properly handle both numeric values and string literals for max_response_output_tokens.

- Changed MaxResponseOutputTokens field in SessionConfig from *int to interface{} to support both number and "inf" values
- This fixes JSON unmarshaling errors when receiving string values like "inf" for this field
- Ensures consistent type handling across all structs (SessionInfo, SessionConfig, ResponseConfig)

## Improved Type Safety in Events Module

Refactored the events.go file to use named types for all nested structs, improving type safety and preventing type mismatch errors when using event structures elsewhere in the codebase.

- Created named types for all previously anonymous nested structs:
  - TurnDetectionConfig, InputAudioTranscriptionConfig, and SpeechSettings for configuration
  - ItemContent and ConversationItem for messaging
  - SessionInfo, ResponseInfo, and ConversationInfo for state representation
  - UsageDetails, OutputItem, and StatusDetails for response data
  - Various error info structures (SessionErrorInfo, TranscriptionErrorInfo)
- Updated all event structures to use the new named types
- Fixed a bug in the encodeBase64 function that was incorrectly trying to use String() on json.RawMessage
- Replaced it with a proper base64 encoding implementation
- Fixed client_impl.go to use the new named types, resolving type mismatch errors

## Refactor Realtime Client Implementation

Split the monolithic client_impl.go file into separate component files for better maintainability and improved the logging to diagnose JSON unmarshaling issues.

- Split client_impl.go into separate files for each component:
  - component_interface.go: Defines the common component interface
  - connection_manager.go: WebSocket connection management 
  - message_sender.go: WebSocket message sending
  - event_processor.go: Event processing and handling
  - response_manager.go: Response data management
  - generic_event.go: Generic event implementation
- Added detailed logging of raw JSON data before unmarshaling to diagnose type mismatches
- Improved error reporting when parsing events to show the actual JSON data causing problems
- Fixed an issue where max_response_output_tokens could be received as a string but was expected as an int 

## Improved Error Handling for Backend Connection Issues

### Why
Enhanced the frontend application to better handle backend connection errors, particularly for AsyncRealtimeConnectionManager issues.

- Added specific error detection and handling for AsyncRealtimeConnectionManager errors
- Implemented improved user-friendly error messages with automatic cleanup
- Added automatic reconnection attempts when backend errors occur
- Styled error messages to be more visible and then fade out automatically
- Made the application more resilient to server-side configuration issues

## Audio API Compatibility Improvements

### Why
Fixed issues with the realtime audio example JavaScript files to improve compatibility across different browsers and environments.

- Fixed import/export syntax by adding type="module" to script tags
- Added robust error handling for environments where navigator.mediaDevices is undefined
- Implemented a text-only fallback mode when audio APIs are unavailable
- Made the application more resilient by continuing initialization even when audio fails
- Fixed issues with module imports in main.js and audio-processor.js 