# Update ResponseAssembler to match updated event structures

Updated the ResponseAssembler to work with the current event structures in events.go. This includes:

- Using the correct event type constants from events.go
- Properly accessing event fields based on the current event structures
- Fixing audio data decoding in the AudioDelta event handler
- Improving response completion detection and signaling
- Adding a Reset method for reusing the assembler
- Using errgroup for improved goroutine management and error handling in audio streaming 

# Update client_impl.go to match event structures

Updated the event handling in client_impl.go to match the correct event structures in events.go. This includes:

- Using the proper event type constants (e.g., EventResponseContentPartAdded instead of EventContentPartAdded)
- Correctly accessing fields in event structures (e.g., ResponseCreatedEvent.Response.ID instead of ResponseCreatedEvent.ResponseID)
- Fixed audio data access in AudioDeltaEvent (using Delta field instead of Audio)
- Updated token usage access in ResponseDoneEvent to match the correct structure
- Ensure all event processing code is consistent with the defined event structures 

# Fixed OpenAI-Beta header format in WebSocket connection

Fixed an issue where the WebSocket connection to the OpenAI Realtime API was failing due to an invalid beta header format:

- Updated the OpenAI-Beta header from "realtime" to "realtime=v1" to match the required format
- Fixed the "Invalid beta header provided" error which was preventing successful connection
- Ensured compatibility with the latest OpenAI Realtime API requirements 

# Added consistent logging to client implementation methods

Added debug logging statements to all methods in client_impl.go for improved traceability and debugging:

- Added consistent entry logging to all public methods
- Included relevant parameters in log entries (e.g., audio size, model, eventType)
- Ensured all method executions can be tracked in logs
- Maintained the existing log format and level (Debug) for consistency 

# Added consistent logging to connection handler methods

Added debug logging statements to all methods in connection_handler.go to match the logging in client_impl.go:

- Added consistent entry logging to all methods including constructor
- Included message type information in sendMessage logs
- Added logs before method execution to improve traceability
- Maintained consistent log format across both client and connection handler components 

# Merged event processor into client implementation

Consolidated the codebase by merging event_processor.go into client_impl.go:

- Moved the eventProcessor struct and all its methods into client_impl.go
- Maintained all existing event processing functionality
- Removed the now-redundant event_processor.go file
- Improved code organization by keeping related components together
- Simplified the codebase structure while preserving the functionality 