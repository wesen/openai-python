package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openai/realtime-http-go/internal/types"
	"github.com/openai/realtime-http-go/internal/ws"
)

// OpenAI API endpoints
const (
	OpenAIRealtimeEndpoint = "wss://realtime.openai.com/v1/realtime"
)

// Client represents an OpenAI client
type Client struct {
	APIKey string
}

// RealtimeConnection represents a connection to OpenAI's Realtime API
type RealtimeConnection struct {
	conn       *websocket.Conn
	sessionID  string
	closed     bool
	closeMutex sync.Mutex
	eventChan  chan types.OpenAIEvent
}

// OpenAIMessage represents a message sent to OpenAI's Realtime API
type OpenAIMessage struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content,omitempty"`
}

// OpenAIResponse represents a response from OpenAI's Realtime API
type OpenAIResponse struct {
	Type       string          `json:"type"`
	SessionID  string          `json:"session_id,omitempty"`
	ItemID     string          `json:"item_id,omitempty"`
	Delta      string          `json:"delta,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	Transcript string          `json:"transcript,omitempty"`
}

// NewClient creates a new OpenAI client
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
	}
}

// Connect connects to OpenAI's Realtime API
func (c *Client) Connect(wsConn *ws.Connection) (ws.OpenAIConnectionInterface, error) {
	// Create a context with timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create WebSocket connection to OpenAI
	dialer := websocket.Dialer{}
	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", c.APIKey)},
	}

	conn, _, err := dialer.DialContext(ctx, OpenAIRealtimeEndpoint, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OpenAI: %w", err)
	}

	// Create Realtime connection
	rtConn := &RealtimeConnection{
		conn:      conn,
		eventChan: make(chan types.OpenAIEvent, 100),
	}

	// Start listening for messages
	go rtConn.listenForMessages()

	// Configure server-side VAD
	err = rtConn.setupServerVAD()
	if err != nil {
		rtConn.Close()
		return nil, fmt.Errorf("failed to set up server VAD: %w", err)
	}

	return rtConn, nil
}

// setupServerVAD sets up server-side Voice Activity Detection
func (c *RealtimeConnection) setupServerVAD() error {
	// Create update session message
	sessionUpdate := map[string]interface{}{
		"turn_detection": map[string]string{
			"type": "server_vad",
		},
		"input_audio_transcription": map[string]string{
			"model": "whisper-1",
		},
	}

	// Marshal to JSON
	content, err := json.Marshal(sessionUpdate)
	if err != nil {
		return fmt.Errorf("failed to marshal session update: %w", err)
	}

	// Create message
	message := OpenAIMessage{
		Type:    "session.update",
		Content: content,
	}

	// Send message
	return c.sendMessage(message)
}

// SendAudio sends audio data to OpenAI
func (c *RealtimeConnection) SendAudio(audioData string) error {
	// Create message
	message := OpenAIMessage{
		Type:    "input_audio.data",
		Content: json.RawMessage(fmt.Sprintf(`{"audio": "%s"}`, audioData)),
	}

	// Send message
	return c.sendMessage(message)
}

// CommitAudio commits the audio buffer
func (c *RealtimeConnection) CommitAudio() error {
	// Create message
	message := OpenAIMessage{
		Type: "input_audio.commit",
	}

	// Send message
	if err := c.sendMessage(message); err != nil {
		return err
	}

	// Create response
	message = OpenAIMessage{
		Type: "response.create",
	}

	// Send message
	return c.sendMessage(message)
}

// SendText sends a text message to OpenAI
func (c *RealtimeConnection) SendText(text string) error {
	// Create content
	content := map[string]interface{}{
		"item": map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []map[string]interface{}{
				{
					"type": "input_text",
					"text": text,
				},
			},
		},
	}

	// Marshal to JSON
	contentJson, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal text message: %w", err)
	}

	// Create message
	message := OpenAIMessage{
		Type:    "conversation.item.create",
		Content: contentJson,
	}

	// Send message
	if err := c.sendMessage(message); err != nil {
		return err
	}

	// Create response
	responseMessage := OpenAIMessage{
		Type: "response.create",
	}

	// Send message
	return c.sendMessage(responseMessage)
}

// sendMessage sends a message to OpenAI
func (c *RealtimeConnection) sendMessage(message OpenAIMessage) error {
	// Check if connection is closed
	c.closeMutex.Lock()
	defer c.closeMutex.Unlock()

	if c.closed {
		return fmt.Errorf("connection is closed")
	}

	// Marshal message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send message
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// listenForMessages listens for messages from OpenAI
func (c *RealtimeConnection) listenForMessages() {
	for {
		// Check if connection is closed
		c.closeMutex.Lock()
		if c.closed {
			c.closeMutex.Unlock()
			close(c.eventChan)
			return
		}
		c.closeMutex.Unlock()

		// Read message
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading from OpenAI: %v", err)
			c.Close()
			return
		}

		// Parse message
		var response OpenAIResponse
		if err := json.Unmarshal(data, &response); err != nil {
			log.Printf("Error parsing OpenAI response: %v", err)
			continue
		}

		// Process message based on its type
		switch response.Type {
		case "session.created":
			// Extract session ID
			var session struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(response.Content, &session); err != nil {
				log.Printf("Error parsing session ID: %v", err)
				continue
			}

			// Store session ID
			c.sessionID = session.ID

			// Send event
			c.eventChan <- types.OpenAIEvent{
				Type:      "session.created",
				SessionID: session.ID,
			}

		case "session.updated":
			// Send event
			c.eventChan <- types.OpenAIEvent{
				Type: "session.updated",
				Data: response.Content,
			}

		case "conversation.item.input_audio_transcription.completed":
			// Send event
			c.eventChan <- types.OpenAIEvent{
				Type:   "user_transcript",
				ItemID: response.ItemID,
				Text:   response.Transcript,
			}

		case "response.audio.delta":
			// Send event
			c.eventChan <- types.OpenAIEvent{
				Type:   "audio_data",
				ItemID: response.ItemID,
				Data:   response.Delta,
			}

			// If this is the first audio delta for this item, send audio stream start event
			if c.isFirstAudioDelta(response.ItemID) {
				c.eventChan <- types.OpenAIEvent{
					Type:   "audio_stream_start",
					ItemID: response.ItemID,
				}
			}

		case "response.audio_transcript.delta":
			// Send event
			c.eventChan <- types.OpenAIEvent{
				Type:    "transcript",
				ItemID:  response.ItemID,
				Text:    response.Delta,
				IsFinal: false,
			}

		case "response.audio_transcript.completed":
			// Send event
			c.eventChan <- types.OpenAIEvent{
				Type:    "transcript",
				ItemID:  response.ItemID,
				Text:    response.Transcript,
				IsFinal: true,
			}
		}
	}
}

// isFirstAudioDelta checks if this is the first audio delta for an item
// This is a simplified implementation - in a real app, you would track audio items
func (c *RealtimeConnection) isFirstAudioDelta(itemID string) bool {
	// This is a simplified implementation
	// In a real app, you would track audio items with a map
	return true
}

// Events returns the event channel
func (c *RealtimeConnection) Events() <-chan types.OpenAIEvent {
	return c.eventChan
}

// Close closes the connection
func (c *RealtimeConnection) Close() {
	c.closeMutex.Lock()
	defer c.closeMutex.Unlock()

	if !c.closed {
		c.closed = true
		c.conn.Close()
	}
}
