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