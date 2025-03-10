package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// messageSender handles sending messages to the WebSocket
type messageSender struct {
	client    *clientImpl
	conn      *websocket.Conn
	connMutex *sync.RWMutex // Points to the same mutex as connectionManager
	msgQueue  chan interface{}
	logger    zerolog.Logger
}

// SendMessage sends a message to the WebSocket
func (ms *messageSender) SendMessage(ctx context.Context, msg interface{}) error {
	select {
	case ms.msgQueue <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second): // Timeout for sending
		return fmt.Errorf("timeout sending message")
	}
}

// Start begins the message sender operation
func (ms *messageSender) Start(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				ms.logger.Debug().Msg("Message sender stopping")
				return
			case msg := <-ms.msgQueue:
				ms.connMutex.Lock()
				// Check that we have a connection
				if ms.conn == nil {
					ms.logger.Error().Msg("No active connection for sending message")
					ms.connMutex.Unlock()
					continue
				}

				// Log the message being sent (for debugging)
				jsonBytes, _ := json.Marshal(msg)
				ms.logger.Debug().RawJSON("message", jsonBytes).Msg("Sending message")

				// Send the message
				if err := ms.sendJSONMessage(msg); err != nil {
					ms.logger.Error().Err(err).Msg("Failed to send message")
				}
				ms.connMutex.Unlock()
			}
		}
	}()

	return nil
}

// Stop ends the message sender operation
func (ms *messageSender) Stop(ctx context.Context) error {
	// Just rely on context cancellation to stop the goroutine
	return nil
}

// sendJSONMessage sends a JSON message over the WebSocket
func (ms *messageSender) sendJSONMessage(msg interface{}) error {
	// Need to special case the audio buffer append message to avoid json marshaling the binary data
	if audioMsg, ok := msg.(AudioBufferAppendRequest); ok {
		// For binary audio data, manually construct a more efficient representation
		// that doesn't encode the base64 data to JSON string and escape it
		type jsonAudioBufferAppend struct {
			Type  ClientEventType `json:"type"`
			Audio string          `json:"audio"`
		}

		jsonMsg := jsonAudioBufferAppend{
			Type:  audioMsg.Type,
			Audio: audioMsg.Audio,
		}

		// Log the outgoing JSON message
		jsonBytes, _ := json.Marshal(jsonMsg)
		ms.logger.Debug().RawJSON("outgoing_json", jsonBytes).Msg("Sending JSON message")

		// Wrap in WriteJSON to handle locking and other error states
		return ms.conn.WriteJSON(jsonMsg)
	}

	// For all other message types, log and use standard JSON serialization
	jsonBytes, _ := json.Marshal(msg)
	ms.logger.Debug().RawJSON("outgoing_json", jsonBytes).Msg("Sending JSON message")

	// For all other message types, use standard JSON serialization
	return ms.conn.WriteJSON(msg)
}
