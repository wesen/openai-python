package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/openai/realtime-http-go/internal/types"
)

// truncateForLogging truncates long strings for logging purposes
func truncateForLogging(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen/2] + "..." + s[len(s)-maxLen/2:]
}

// Message represents a message sent over WebSocket
type Message struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data,omitempty"`
	Text    string      `json:"text,omitempty"`
	ItemID  string      `json:"item_id,omitempty"`
	Status  string      `json:"status,omitempty"`
	IsFinal bool        `json:"is_final,omitempty"`
	Message string      `json:"message,omitempty"`
}

// AudioFormat represents the audio format information
type AudioFormat struct {
	MimeType   string `json:"mimeType,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

// Connection represents a WebSocket connection
type Connection struct {
	Conn        *websocket.Conn
	AudioFormat *types.AudioFormat
	Mutex       sync.Mutex
}

// ConnectionManager manages WebSocket connections
type ConnectionManager struct {
	connections  map[*websocket.Conn]*Connection
	openaiClient OpenAIClientInterface
	mutex        sync.RWMutex
}

// OpenAIClientInterface defines the interface for OpenAI client
type OpenAIClientInterface interface {
	Connect(conn *Connection) (OpenAIConnectionInterface, error)
}

// OpenAIConnectionInterface defines the interface for OpenAI connection
type OpenAIConnectionInterface interface {
	SendAudio(audioData string) error
	CommitAudio() error
	SendText(text string) error
	Events() <-chan types.OpenAIEvent
	Close()
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(openaiClient OpenAIClientInterface) *ConnectionManager {
	return &ConnectionManager{
		connections:  make(map[*websocket.Conn]*Connection),
		openaiClient: openaiClient,
	}
}

// Connect adds a new WebSocket connection to the manager
func (cm *ConnectionManager) Connect(conn *websocket.Conn) {
	log.Printf("[WS Manager] Adding new WebSocket connection from %s", conn.RemoteAddr().String())

	// Create a new connection
	connection := &Connection{
		Conn: conn,
	}

	// Add to connections map
	cm.mutex.Lock()
	cm.connections[conn] = connection
	cm.mutex.Unlock()

	log.Printf("[WS Manager] Connection added, total connections: %d", len(cm.connections))

	// Send connection established message
	connection.SendMessage(types.WebSocketMessage{
		Type: "connection_established",
	})

	log.Printf("[WS Manager] Sent 'connection_established' message to client")

	// Start listening for messages
	go cm.handleConnection(connection)
}

// handleConnection handles messages from a WebSocket connection
func (cm *ConnectionManager) handleConnection(conn *Connection) {
	log.Printf("[WS Manager] Starting message handler for connection from %s", conn.Conn.RemoteAddr().String())

	// Initialize the OpenAI Realtime connection
	log.Printf("[WS Manager] Initializing OpenAI connection")
	openaiConn, err := cm.openaiClient.Connect(conn)
	if err != nil {
		log.Printf("[WS Manager] Error connecting to OpenAI: %v", err)
		conn.SendMessage(types.WebSocketMessage{
			Type:    "error",
			Message: "Failed to connect to OpenAI",
		})
		cm.Disconnect(conn.Conn)
		return
	}

	log.Printf("[WS Manager] OpenAI connection established successfully")

	// Set up an event handler for OpenAI events
	log.Printf("[WS Manager] Starting OpenAI event handler")
	go cm.handleOpenAIEvents(conn, openaiConn)

	// Read messages from the WebSocket
	log.Printf("[WS Manager] Starting WebSocket message reading loop")
	for {
		messageType, raw, err := conn.Conn.ReadMessage()
		if err != nil {
			log.Printf("[WS Manager] Error reading from WebSocket: %v", err)
			cm.Disconnect(conn.Conn)
			return
		}

		// Log received message with truncation for audio data
		logMsg := string(raw)
		if len(logMsg) > 200 && (messageType == websocket.BinaryMessage ||
			(messageType == websocket.TextMessage && len(logMsg) > 1000)) {
			log.Printf("[WS Manager] Received message type %d, length: %d bytes, data: %s",
				messageType, len(raw), truncateForLogging(logMsg, 50))
		} else {
			log.Printf("[WS Manager] Received message type %d, raw data length: %d bytes", messageType, len(raw))
		}

		// Parse the message
		var message types.WebSocketMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			log.Printf("[WS Manager] Error parsing WebSocket message: %v, raw message: %s", err, string(raw))
			conn.SendMessage(types.WebSocketMessage{
				Type:    "error",
				Message: "Invalid message format",
			})
			continue
		}

		log.Printf("[WS Manager] Parsed message type: %s", message.Type)

		// Handle the message based on its type
		switch message.Type {
		case "audio_format":
			// Parse audio format
			var format map[string]interface{}
			formatJson, err := json.Marshal(message.Data)
			if err != nil {
				log.Printf("Error marshaling audio format: %v", err)
				continue
			}

			err = json.Unmarshal(formatJson, &format)
			if err != nil {
				log.Printf("Error unmarshaling audio format: %v", err)
				continue
			}

			// Extract format information
			mimeType, _ := format["mimeType"].(string)
			sampleRate, _ := format["sampleRate"].(float64)
			channels, _ := format["channels"].(float64)

			conn.AudioFormat = &types.AudioFormat{
				MimeType:   mimeType,
				SampleRate: int(sampleRate),
				Channels:   int(channels),
			}

			// Send acknowledgment
			conn.SendMessage(types.WebSocketMessage{
				Type:   "audio_format_received",
				Status: "ok",
			})

		case "audio_data":
			// Extract audio data
			audioData, ok := message.Data.(string)
			if !ok {
				log.Printf("[WS Manager] Invalid audio data")
				continue
			}

			// Log truncated audio data
			log.Printf("[WS Manager] Received audio data, length: %d bytes, data: %s",
				len(audioData), truncateForLogging(audioData, 50))

			// Process the audio data
			if err := cm.processAudioData(conn, openaiConn, audioData); err != nil {
				log.Printf("[WS Manager] Error processing audio data: %v", err)
				conn.SendMessage(types.WebSocketMessage{
					Type:    "error",
					Message: "Error processing audio data",
				})
			}

		case "audio_end":
			// Commit audio buffer
			if openaiConn != nil {
				if err := openaiConn.CommitAudio(); err != nil {
					log.Printf("Error committing audio: %v", err)
				}
			}

		case "text_message":
			// Send text message to OpenAI
			if openaiConn != nil {
				if err := openaiConn.SendText(message.Text); err != nil {
					log.Printf("Error sending text to OpenAI: %v", err)
					conn.SendMessage(types.WebSocketMessage{
						Type:    "error",
						Message: "Error sending text to OpenAI",
					})
				}
			}
		}
	}
}

// handleOpenAIEvents handles events from the OpenAI connection
func (cm *ConnectionManager) handleOpenAIEvents(conn *Connection, openaiConn OpenAIConnectionInterface) {
	log.Printf("[WS Manager] Starting OpenAI event handler")

	// Get the events channel
	events := openaiConn.Events()

	// Listen for events
	for event := range events {
		log.Printf("[WS Manager] Received OpenAI event: %s", event.Type)

		// Handle the event based on its type
		switch event.Type {
		case "session.created":
			log.Printf("[WS Manager] Session created with ID: %s", event.Session)
			conn.SendMessage(types.WebSocketMessage{
				Type:   "session_created",
				Status: "ready",
			})

		case "session.updated":
			conn.SendMessage(types.WebSocketMessage{
				Type: "session.updated",
				Data: event.Data,
			})

		case "transcript":
			conn.SendMessage(types.WebSocketMessage{
				Type:    "transcript",
				ItemID:  event.ItemID,
				Text:    event.Text,
				IsFinal: event.IsFinal,
			})

		case "user_transcript":
			conn.SendMessage(types.WebSocketMessage{
				Type:   "user_transcript",
				ItemID: event.ItemID,
				Text:   event.Text,
			})

		case "audio_data":
			conn.SendMessage(types.WebSocketMessage{
				Type:   "audio_data",
				ItemID: event.ItemID,
				Data:   event.Data,
			})

		case "audio_stream_start":
			conn.SendMessage(types.WebSocketMessage{
				Type:   "audio_stream_start",
				ItemID: event.ItemID,
			})

		case "error":
			conn.SendMessage(types.WebSocketMessage{
				Type:    "error",
				Message: event.Message,
			})

		default:
			log.Printf("[WS Manager] Unhandled OpenAI event type: %s", event.Type)
		}
	}

	log.Printf("[WS Manager] OpenAI event handler terminated")
}

// processAudioData processes audio data from a WebSocket connection
func (cm *ConnectionManager) processAudioData(conn *Connection, openaiConn OpenAIConnectionInterface, audioData string) error {
	// Log the audio data with truncation
	log.Printf("[WS Manager] Processing audio data, length: %d bytes, sample: %s",
		len(audioData), truncateForLogging(audioData, 50))

	// Send the audio data to OpenAI if not empty
	if len(audioData) > 0 {
		return openaiConn.SendAudio(audioData)
	}

	log.Printf("[WS Manager] Warning: Received empty audio data")
	return nil
}

// Disconnect removes a WebSocket connection from the manager
func (cm *ConnectionManager) Disconnect(conn *websocket.Conn) {
	// Lock the mutex
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get the connection
	_, exists := cm.connections[conn]
	if !exists {
		return
	}

	// Close the WebSocket connection
	conn.Close()

	// Remove from connections map
	delete(cm.connections, conn)
}

// BroadcastTextMessage broadcasts a text message to all connections
func (cm *ConnectionManager) BroadcastTextMessage(text string) {
	// Lock the mutex for reading
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// Send the message to all connections
	for _, conn := range cm.connections {
		conn.SendMessage(types.WebSocketMessage{
			Type: "text_message",
			Text: text,
		})
	}
}

// ConnectionCount returns the number of active connections
func (cm *ConnectionManager) ConnectionCount() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return len(cm.connections)
}

// SendMessage sends a JSON message to the WebSocket connection
func (c *Connection) SendMessage(message types.WebSocketMessage) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// Marshal the message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[WS Connection] Error marshaling message: %v", err)
		return
	}

	log.Printf("[WS Connection] Sending message type: %s, data: %s", message.Type, string(data))

	// Send the message
	if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("[WS Connection] Error writing to WebSocket: %v", err)
	} else {
		log.Printf("[WS Connection] Message sent successfully")
	}
}
