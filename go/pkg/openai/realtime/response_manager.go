package realtime

import (
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// responseManager handles storing and retrieving response data
type responseManager struct {
	client               *clientImpl
	currentResponseID    atomic.Value // string
	currentResponseText  atomic.Value // string
	responseMutex        sync.Mutex
	currentResponseAudio []byte
	logger               zerolog.Logger
}

// SetResponseID sets the current response ID
func (rm *responseManager) SetResponseID(responseID string) {
	rm.currentResponseID.Store(responseID)
}

// AppendResponseText appends text to the current response
func (rm *responseManager) AppendResponseText(responseID string, text string) {
	currentID := rm.currentResponseID.Load().(string)

	// Only append if the response ID matches the current one
	if responseID == currentID {
		currentText := rm.currentResponseText.Load().(string)
		rm.currentResponseText.Store(currentText + text)
	}
}

// AppendResponseAudio appends audio to the current response
func (rm *responseManager) AppendResponseAudio(responseID string, audio []byte) {
	currentID := rm.currentResponseID.Load().(string)

	// Only append if the response ID matches the current one
	if responseID == currentID {
		rm.responseMutex.Lock()
		rm.currentResponseAudio = append(rm.currentResponseAudio, audio...)
		rm.responseMutex.Unlock()
	}
}

// ResetResponse resets the response buffer
func (rm *responseManager) ResetResponse() {
	rm.currentResponseID.Store("")
	rm.currentResponseText.Store("")
	rm.responseMutex.Lock()
	rm.currentResponseAudio = nil
	rm.responseMutex.Unlock()
}

// GetResponse returns the current response text and audio
func (rm *responseManager) GetResponse() (string, []byte) {
	text := rm.currentResponseText.Load().(string)

	rm.responseMutex.Lock()
	// Create a copy of the audio to avoid race conditions
	audioBytes := make([]byte, len(rm.currentResponseAudio))
	copy(audioBytes, rm.currentResponseAudio)
	rm.responseMutex.Unlock()

	return text, audioBytes
}
