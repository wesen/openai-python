package realtime

import (
	"context"
	"encoding/base64"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
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
	client.SetEventHandler(EventResponseContentPartAdded, ra.handleContentPart)
	client.SetEventHandler(EventResponseContentPartDone, ra.handleContentDone)
	client.SetEventHandler(EventResponseAudioDelta, ra.handleAudioDelta)
	client.SetEventHandler(EventResponseAudioDone, ra.handleAudioDone)
	client.SetEventHandler(EventResponseDone, ra.handleResponseDone)

	return ra
}

// handleContentPart handles the response.content_part.added event
func (ra *ResponseAssembler) handleContentPart(event Event) error {
	if e, ok := event.(*ContentPartAddedEvent); ok {
		ra.mutex.Lock()
		defer ra.mutex.Unlock()

		ra.responseText += e.Part.Text
	}
	return nil
}

// handleContentDone handles the response.content_part.done event
func (ra *ResponseAssembler) handleContentDone(event Event) error {
	if _, ok := event.(*ContentPartDoneEvent); ok {
		ra.textDone.Store(true)
		ra.checkDone()
	}
	return nil
}

// handleAudioDelta handles the response.audio.delta event
func (ra *ResponseAssembler) handleAudioDelta(event Event) error {
	if e, ok := event.(*AudioDeltaEvent); ok {
		audioBytes, err := base64.StdEncoding.DecodeString(e.Delta)
		if err != nil {
			return err
		}

		ra.mutex.Lock()
		defer ra.mutex.Unlock()
		ra.responseAudio = append(ra.responseAudio, audioBytes...)
	}
	return nil
}

// handleAudioDone handles the response.audio.done event
func (ra *ResponseAssembler) handleAudioDone(event Event) error {
	if _, ok := event.(*AudioDoneEvent); ok {
		ra.audioDone.Store(true)
		ra.checkDone()
	}
	return nil
}

// handleResponseDone handles the response.done event
func (ra *ResponseAssembler) handleResponseDone(event Event) error {
	if _, ok := event.(*ResponseDoneEvent); ok {
		ra.textDone.Store(true)
		ra.audioDone.Store(true)
		ra.done.Store(true)

		// Signal that the response is complete
		select {
		case ra.responseChan <- struct{}{}:
		default:
			// Channel already has a value, no need to send again
		}
	}
	return nil
}

// checkDone checks if both text and audio are done, and if so, signals completion
func (ra *ResponseAssembler) checkDone() {
	if ra.textDone.Load() && ra.audioDone.Load() && !ra.done.Load() {
		ra.done.Store(true)

		// Signal that the response is complete
		select {
		case ra.responseChan <- struct{}{}:
		default:
			// Channel already has a value, no need to send again
		}
	}
}

// WaitForResponse waits for the complete response (text and audio)
func (ra *ResponseAssembler) WaitForResponse(ctx context.Context) (string, []byte, error) {
	// Wait for response or context cancellation
	select {
	case <-ra.responseChan:
		// Response is complete
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}

	// Lock to safely access response data
	ra.mutex.Lock()
	defer ra.mutex.Unlock()

	// Return the assembled response
	return ra.responseText, ra.responseAudio, nil
}

// SendTextAndWaitForResponse sends a text message and waits for a complete response
func (ra *ResponseAssembler) SendTextAndWaitForResponse(ctx context.Context, text string) (string, []byte, error) {
	// Reset state
	ra.Reset()

	// Send the text message
	err := ra.client.SendText(ctx, text)
	if err != nil {
		return "", nil, err
	}

	// Wait for a response
	return ra.WaitForResponse(ctx)
}

// SendAudioAndWaitForResponse sends audio data and waits for a complete response
func (ra *ResponseAssembler) SendAudioAndWaitForResponse(ctx context.Context, audio []byte, commit bool) (string, []byte, error) {
	// Reset state
	ra.Reset()

	// Send the audio data
	err := ra.client.SendAudio(ctx, audio)
	if err != nil {
		return "", nil, err
	}

	// Commit the audio if requested
	if commit {
		err = ra.client.CommitAudio(ctx)
		if err != nil {
			return "", nil, err
		}
	}

	// Wait for a response
	return ra.WaitForResponse(ctx)
}

// StreamAudioAndWaitForResponse streams audio in chunks and waits for a complete response
func (ra *ResponseAssembler) StreamAudioAndWaitForResponse(ctx context.Context, audioChunks <-chan []byte, chunkInterval time.Duration, commit bool) (string, []byte, error) {
	// Reset state
	ra.Reset()

	// Create an errgroup with a derived context
	g, ctx := errgroup.WithContext(ctx)

	// Start a goroutine to stream the audio chunks
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case chunk, ok := <-audioChunks:
				if !ok {
					// Channel is closed, we're done streaming
					return nil
				}

				// Send the audio chunk
				if err := ra.client.SendAudio(ctx, chunk); err != nil {
					return err
				}

				// Wait for the specified interval
				if chunkInterval > 0 {
					select {
					case <-time.After(chunkInterval):
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}
	})

	// Wait for streaming to complete and check for errors
	if err := g.Wait(); err != nil {
		return "", nil, err
	}

	// Commit the audio if requested
	if commit {
		if err := ra.client.CommitAudio(ctx); err != nil {
			return "", nil, err
		}
	}

	// Wait for a response
	return ra.WaitForResponse(ctx)
}

// IsResponseComplete checks if a response has been completed
func (ra *ResponseAssembler) IsResponseComplete() bool {
	return ra.done.Load()
}

// GetPartialResponse gets the current partial response (text and audio)
func (ra *ResponseAssembler) GetPartialResponse() (string, []byte) {
	ra.mutex.Lock()
	defer ra.mutex.Unlock()
	return ra.responseText, ra.responseAudio
}

// Reset resets the assembler state
func (ra *ResponseAssembler) Reset() {
	ra.mutex.Lock()
	defer ra.mutex.Unlock()

	ra.responseText = ""
	ra.responseAudio = nil
	ra.textDone.Store(false)
	ra.audioDone.Store(false)
	ra.done.Store(false)
}
