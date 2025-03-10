package realtime

// genericEvent is a simple wrapper for events with no specific structure
type genericEvent struct {
	eventType string
	data      []byte
}

// Type returns the event type
func (e *genericEvent) Type() string {
	return e.eventType
}

// RawData returns the raw event data
func (e *genericEvent) RawData() []byte {
	return e.data
}
