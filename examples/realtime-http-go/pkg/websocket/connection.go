package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024
)

var (
	newline = []byte{'\n'}
)

// MessageHandler is a function type for handling incoming messages
type MessageHandler func(message []byte)

// Connection represents a WebSocket connection
type Connection struct {
	// The WebSocket connection
	Conn *websocket.Conn

	// Uniquely identifies this connection
	ID string

	// Indicates if the connection is active
	IsActive bool

	// Protects the connection for concurrent writing
	writeMutex sync.Mutex

	// Channel for outbound messages
	Send chan []byte

	// User defined attributes
	attributes sync.Map

	// Signal for closing the connection
	ctx    context.Context
	cancel context.CancelFunc

	// Message handler
	messageHandler MessageHandler
	handlerMutex   sync.RWMutex
}

// NewConnection creates a new WebSocket connection
func NewConnection(conn *websocket.Conn, id string) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &Connection{
		Conn:     conn,
		ID:       id,
		Send:     make(chan []byte, 256),
		IsActive: true,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Context returns the connection context
func (c *Connection) Context() context.Context {
	return c.ctx
}

// SetAttribute sets a connection attribute
func (c *Connection) SetAttribute(key, value interface{}) {
	c.attributes.Store(key, value)
}

// GetAttribute gets a connection attribute
func (c *Connection) GetAttribute(key interface{}) (interface{}, bool) {
	return c.attributes.Load(key)
}

// DeleteAttribute deletes a connection attribute
func (c *Connection) DeleteAttribute(key interface{}) {
	c.attributes.Delete(key)
}

// SetMessageHandler sets the message handler
func (c *Connection) SetMessageHandler(handler MessageHandler) {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	c.messageHandler = handler
}

// Close closes the WebSocket connection
func (c *Connection) Close() {
	c.cancel()
	c.Conn.Close()
}

// WritePump pumps messages from the hub to the websocket connection
func (c *Connection) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump pumps messages from the websocket connection to the hub
func (c *Connection) ReadPump(manager *Manager) {
	defer func() {
		manager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Error().Err(err).Str("connection_id", c.ID).Msg("Unexpected close error")
				}
				return
			}

			// Process the received message
			log.Debug().Str("connection_id", c.ID).Str("message_size", byteSizeToString(len(message))).Msg("Received message")

			// Log message type for debugging
			var data map[string]interface{}
			if err := json.Unmarshal(message, &data); err == nil {
				if msgType, ok := data["type"].(string); ok {
					log.Debug().Str("connection_id", c.ID).Str("type", msgType).Msg("Message type")
				}
			}

			// Call message handler if set
			c.handlerMutex.RLock()
			handler := c.messageHandler
			c.handlerMutex.RUnlock()

			if handler != nil {
				handler(message)
			} else {
				// Forward the message to all clients if no handler is set
				manager.Broadcast <- message
			}
		}
	}
}

// writeMessage writes a message to the WebSocket connection
func (c *Connection) writeMessage(messageType int, data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	return c.Conn.WriteMessage(messageType, data)
}

// SendText sends a JSON message to the WebSocket connection
func (c *Connection) SendText(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	select {
	case c.Send <- jsonData:
		return nil
	case <-c.ctx.Done():
		return context.Canceled
	default:
		// Channel is full, try again after a short delay
		time.Sleep(50 * time.Millisecond)
		select {
		case c.Send <- jsonData:
			return nil
		case <-c.ctx.Done():
			return context.Canceled
		default:
			log.Warn().Str("connection_id", c.ID).Msg("Send channel is full, dropping message")
			return nil
		}
	}
}

// SendRaw sends raw data to the client
func (c *Connection) SendRaw(data []byte) error {
	select {
	case c.Send <- data:
		return nil
	case <-c.ctx.Done():
		return context.Canceled
	default:
		// Channel is full, try again after a short delay
		time.Sleep(50 * time.Millisecond)
		select {
		case c.Send <- data:
			return nil
		case <-c.ctx.Done():
			return context.Canceled
		default:
			log.Warn().Str("connection_id", c.ID).Msg("Send channel is full, dropping message")
			return nil
		}
	}
}

// byteSizeToString converts a byte size to a human readable string
func byteSizeToString(size int) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}