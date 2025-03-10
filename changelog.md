# Changelog

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