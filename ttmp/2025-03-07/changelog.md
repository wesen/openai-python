# Update ResponseAssembler to match updated event structures

Updated the ResponseAssembler to work with the current event structures in events.go. This includes:

- Using the correct event type constants from events.go
- Properly accessing event fields based on the current event structures
- Fixing audio data decoding in the AudioDelta event handler
- Improving response completion detection and signaling
- Adding a Reset method for reusing the assembler
- Using errgroup for improved goroutine management and error handling in audio streaming 