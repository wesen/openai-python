package realtime

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ResponseAssembler is a utility to assemble a complete response from streaming events
type ResponseAssembler struct {
	client        Client
	responseText  string
	responseAudio []byte
	textDone      atomic.Bool
	audioDone     atomic.Bool
	done          atomic.Bool
	mutex         sync.Mutex
	responseChan  chan struct{}
}

// NewResponseAssembler creates a new response assembler for the given client
func NewResponseAssembler(client Client) *ResponseAssembler {
	ra := &ResponseAssembler{
		client:       client,
		responseChan: make(chan struct{}),
	}

	// Register handlers for the various response events
	client.SetEventHandler(EventContentPartAdded, ra.handleContentPart)
	client.SetEventHandler(EventContentPartDone, ra.handleContentDone)
	client.SetEventHandler(EventAudioDelta, ra.handleAudioDelta)
	client.SetEventHandler(EventAudioDone, ra.handleAudioDone)
	client.SetEventHandler(EventResponseDone, ra.handleResponseDone)

	return ra
}

// handleContentPart handles the response.content_part.added event
func (ra *ResponseAssembler) handleContentPart(event Event) error {
	if e, ok := event.(*ContentPartAddedEvent); ok {
		ra.mutex.Lock()
		defer ra.mutex.Unlock()

		ra.responseText += e.Content.Text
	}
	return nil
}

// handleContentDone handles the response.content_part.done event
func (ra *ResponseAssembler) handleContentDone(event Event) error {
	ra.textDone.Store(true)
	ra.checkDone()
	return nil
}

// handleAudioDelta handles the response.audio.delta event
func (ra *ResponseAssembler) handleAudioDelta(event Event) error {
	if e, ok := event.(*AudioDeltaEvent); ok {
		// Audio decoding is handled in the clientImpl directly
		// This event is just for notification purposes
		_ = e
	}
	return nil
}

// handleAudioDone handles the response.audio.done event
func (ra *ResponseAssembler) handleAudioDone(event Event) error {
	ra.audioDone.Store(true)
	ra.checkDone()
	return nil
}

// handleResponseDone handles the response.done event
func (ra *ResponseAssembler) handleResponseDone(event Event) error {
	ra.mutex.Lock()
	defer ra.mutex.Unlock()

	// Get the final response from the client
	responseText, responseAudio := ra.client.(interface{ GetResponse() (string, []byte) }).GetResponse()
	ra.responseText = responseText
	ra.responseAudio = make([]byte, len(responseAudio))
	copy(ra.responseAudio, responseAudio)

	ra.done.Store(true)
	close(ra.responseChan)

	return nil
}

// checkDone checks if both text and audio are done, and if so, signals completion
func (ra *ResponseAssembler) checkDone() {
	if ra.textDone.Load() && ra.audioDone.Load() {
		ra.mutex.Lock()
		defer ra.mutex.Unlock()

		if !ra.done.Load() {
			// Both text and audio are done, but we didn't get a response.done event yet
			// We'll signal completion here just in case

			// Get the final response from the client
			responseText, responseAudio := ra.client.(interface{ GetResponse() (string, []byte) }).GetResponse()
			ra.responseText = responseText
			ra.responseAudio = make([]byte, len(responseAudio))
			copy(ra.responseAudio, responseAudio)

			ra.done.Store(true)
			close(ra.responseChan)
		}
	}
}

// WaitForResponse waits for the complete response (text and audio)
func (ra *ResponseAssembler) WaitForResponse(ctx context.Context) (string, []byte, error) {
	// Reset the state for a new response
	ra.mutex.Lock()
	ra.responseText = ""
	ra.responseAudio = nil
	ra.textDone.Store(false)
	ra.audioDone.Store(false)
	ra.done.Store(false)
	ra.responseChan = make(chan struct{})
	ra.mutex.Unlock()

	// Wait for the response to complete or context to cancel
	select {
	case <-ra.responseChan:
		// Response is complete
		ra.mutex.Lock()
		defer ra.mutex.Unlock()

		// Create a copy of the audio bytes to avoid race conditions
		audioCopy := make([]byte, len(ra.responseAudio))
		copy(audioCopy, ra.responseAudio)

		return ra.responseText, audioCopy, nil

	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

// SendTextAndWaitForResponse sends a text message and waits for a complete response
func (ra *ResponseAssembler) SendTextAndWaitForResponse(ctx context.Context, text string) (string, []byte, error) {
	// Send the text message
	err := ra.client.SendText(ctx, text)
	if err != nil {
		return "", nil, err
	}

	// Wait for the response
	return ra.WaitForResponse(ctx)
}

// SendAudioAndWaitForResponse sends audio data and waits for a complete response
func (ra *ResponseAssembler) SendAudioAndWaitForResponse(ctx context.Context, audio []byte, commit bool) (string, []byte, error) {
	// Send the audio data
	err := ra.client.SendAudio(ctx, audio)
	if err != nil {
		return "", nil, err
	}

	// If manual commit is required, send the commit signal
	if commit {
		err = ra.client.CommitAudio(ctx)
		if err != nil {
			return "", nil, err
		}
	}

	// Wait for the response
	return ra.WaitForResponse(ctx)
}

// StreamAudioAndWaitForResponse streams audio in chunks and waits for a complete response
func (ra *ResponseAssembler) StreamAudioAndWaitForResponse(ctx context.Context, audioChunks <-chan []byte, chunkInterval time.Duration, commit bool) (string, []byte, error) {
	// Create a new context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start a goroutine to send audio chunks
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)

		for {
			select {
			case <-ctx.Done():
				return

			case chunk, ok := <-audioChunks:
				if !ok {
					// Channel closed, all chunks sent
					if commit {
						if err := ra.client.CommitAudio(ctx); err != nil {
							errChan <- err
						}
					}
					return
				}

				// Send the audio chunk
				if err := ra.client.SendAudio(ctx, chunk); err != nil {
					errChan <- err
					return
				}

				// Wait for the chunk interval
				if chunkInterval > 0 {
					time.Sleep(chunkInterval)
				}
			}
		}
	}()

	// Wait for the audio to be sent or an error to occur
	var err error
	select {
	case err = <-errChan:
		if err != nil {
			return "", nil, err
		}
	case <-ctx.Done():
		return "", nil, ctx.Err()
	default:
		// Continue
	}

	// Wait for the response
	respText, respAudio, err := ra.WaitForResponse(ctx)
	if err != nil {
		return "", nil, err
	}

	return respText, respAudio, nil
}

// IsResponseComplete checks if a response has been completed
func (ra *ResponseAssembler) IsResponseComplete() bool {
	return ra.done.Load()
}

// GetPartialResponse gets the current partial response (text and audio)
func (ra *ResponseAssembler) GetPartialResponse() (string, []byte) {
	ra.mutex.Lock()
	defer ra.mutex.Unlock()

	// Create a copy of the audio bytes to avoid race conditions
	audioCopy := make([]byte, len(ra.responseAudio))
	copy(audioCopy, ra.responseAudio)

	return ra.responseText, audioCopy
}
