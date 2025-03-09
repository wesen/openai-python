package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openai/realtime-http-go/internal/types"
	"github.com/openai/realtime-http-go/internal/ws"
)

// OpenAI API endpoints
const (
	// The OpenAI Realtime API endpoint follows the same pattern as other OpenAI APIs
	// but uses wss:// protocol and /realtime path
	OpenAIRealtimeEndpoint = "wss://api.openai.com/v1/realtime"
)

// Client represents an OpenAI client
type Client struct {
	APIKey string
	// Allow for a custom base URL
	BaseURL string
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
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
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
	// Check for custom base URL from environment variables
	baseURL := getEnv("OPENAI_API_BASE", "api.openai.com")

	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
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
		"OpenAI-Beta":   {"realtime=v1"},
	}

	// Construct the URL with the model name from the config
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-realtime-preview"
	}

	// Construct the full WebSocket URL
	url := fmt.Sprintf("wss://%s/v1/realtime?model=%s", c.BaseURL, model)

	log.Printf("[OpenAI Client] Connecting to OpenAI Realtime at: %s", url)
	log.Printf("[OpenAI Client] Headers: %v", headers)

	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		if resp != nil {
			log.Printf("[OpenAI Client] Connection failed with status: %d, headers: %v",
				resp.StatusCode, resp.Header)
		}
		return nil, fmt.Errorf("failed to connect to OpenAI: %w", err)
	}
	log.Printf("[OpenAI Client] WebSocket connection established successfully")

	// Create Realtime connection
	rtConn := &RealtimeConnection{
		conn:      conn,
		eventChan: make(chan types.OpenAIEvent, 100),
	}

	// Start listening for messages
	go rtConn.listenForMessages()

	// Configure server-side VAD
	log.Printf("[OpenAI Client] Setting up server VAD...")
	err = rtConn.setupServerVAD()
	if err != nil {
		rtConn.Close()
		return nil, fmt.Errorf("failed to set up server VAD: %w", err)
	}
	log.Printf("[OpenAI Client] Server VAD setup completed")

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

	// Create message
	message := map[string]interface{}{
		"type":    "session.update",
		"session": sessionUpdate,
	}

	// Marshal to JSON
	messageJson, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Log the full JSON payload
	log.Printf("[OpenAI Client] Sending message to OpenAI: %s", string(messageJson))

	// Write directly to the connection
	return c.conn.WriteMessage(websocket.TextMessage, messageJson)
}

// updateSession updates the session with the given session ID
func (c *RealtimeConnection) updateSession() error {
	if c.sessionID == "" {
		return fmt.Errorf("cannot update session: no session ID available")
	}

	// Create update session message with proper structure based on API docs
	sessionUpdate := map[string]interface{}{
		"turn_detection": map[string]string{
			"type": "server_vad",
		},
		"input_audio_transcription": map[string]string{
			"model": "whisper-1",
		},
	}

	// Create message with session in the correct format
	message := map[string]interface{}{
		"type":    "session.update",
		"session": sessionUpdate,
	}

	// Marshal message to JSON
	messageJson, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Log the full JSON payload
	log.Printf("[OpenAI Client] Sending message to OpenAI: %s", string(messageJson))

	// Write directly to the connection
	return c.conn.WriteMessage(websocket.TextMessage, messageJson)
}

// truncateForLogging truncates long strings for logging purposes
func truncateForLogging(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen/2] + "..." + s[len(s)-maxLen/2:]
}

// SendAudio sends audio data to OpenAI
func (c *RealtimeConnection) SendAudio(audioData string) error {
	// Check if we have a session ID
	if c.sessionID == "" {
		log.Printf("[OpenAI Client] Warning: Sending audio without session ID")
	}

	// Create message in correct format
	message := map[string]interface{}{
		"type":  "input_audio_buffer.append",
		"audio": audioData,
	}

	// Send message
	messageJson, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Log the full JSON payload (truncated for audio data to avoid huge logs)
	truncatedJson := string(messageJson)
	if len(truncatedJson) > 200 {
		audioLength := len(audioData)
		truncatedJson = truncatedJson[:100] + "..." + fmt.Sprintf("[AUDIO DATA: %d bytes]", audioLength) + "..." + truncatedJson[len(truncatedJson)-50:]
	}
	log.Printf("[OpenAI Client] Sending audio message to OpenAI: %s", truncatedJson)

	// Write directly to the connection
	return c.conn.WriteMessage(websocket.TextMessage, messageJson)
}

// CommitAudio commits the audio buffer
func (c *RealtimeConnection) CommitAudio() error {
	// Check if we have a session ID
	if c.sessionID == "" {
		log.Printf("[OpenAI Client] Warning: Committing audio without session ID")
	}

	// Create message in correct format
	message := map[string]interface{}{
		"type": "input_audio_buffer.commit",
	}

	// Send message
	messageJson, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Log the full JSON payload
	log.Printf("[OpenAI Client] Sending commit message to OpenAI: %s", string(messageJson))

	// Write directly to the connection
	if err := c.conn.WriteMessage(websocket.TextMessage, messageJson); err != nil {
		return err
	}

	log.Printf("[OpenAI Client] Audio buffer committed successfully")
	return nil
}

// SendText sends a text message to OpenAI
func (c *RealtimeConnection) SendText(text string) error {
	// Check if we have a session ID
	if c.sessionID == "" {
		log.Printf("[OpenAI Client] Warning: Sending text without session ID")
	}

	// Create message in correct format
	message := map[string]interface{}{
		"type": "conversation.item.create",
		"item": map[string]interface{}{
			"role": "user",
			"content": map[string]string{
				"text": text,
			},
		},
	}

	// Marshal to JSON
	messageJson, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal text message: %w", err)
	}

	// Log the full JSON payload
	log.Printf("[OpenAI Client] Sending text message to OpenAI: %s", string(messageJson))

	// Write directly to the connection
	return c.conn.WriteMessage(websocket.TextMessage, messageJson)
}

// listenForMessages listens for messages from OpenAI
func (c *RealtimeConnection) listenForMessages() {
	log.Printf("[OpenAI Client] Started listening for messages from OpenAI")
	for {
		// Check if connection is closed
		c.closeMutex.Lock()
		if c.closed {
			c.closeMutex.Unlock()
			log.Printf("[OpenAI Client] Connection closed, stopping message listener")
			close(c.eventChan)
			return
		}
		c.closeMutex.Unlock()

		// Read message
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[OpenAI Client] Error reading from OpenAI: %v", err)
			c.Close()
			return
		}

		// Log the raw incoming message, with appropriate handling for different message types
		logData := string(data)
		messageTypeStr := "TEXT"
		if messageType == websocket.BinaryMessage {
			messageTypeStr = "BINARY"
		}

		// Check if it's an audio response to truncate it appropriately
		if strings.Contains(logData, "\"type\":\"response.audio.delta\"") {
			// For audio responses, extract type and truncate the audio data
			var partialParse struct {
				Type  string `json:"type"`
				Audio string `json:"audio,omitempty"`
			}
			if err := json.Unmarshal(data, &partialParse); err == nil {
				audioLength := len(partialParse.Audio)
				log.Printf("[OpenAI Client] Received %s message: type=%s with %d bytes of audio data",
					messageTypeStr, partialParse.Type, audioLength)
			} else {
				// Fall back to basic truncation if parsing fails
				prefix := logData
				if len(logData) > 100 {
					prefix = logData[:100] + "..."
				}
				log.Printf("[OpenAI Client] Received %s message (truncated, total length: %d): %s",
					messageTypeStr, len(logData), prefix)
			}
		} else {
			// For non-audio messages, log the complete response
			log.Printf("[OpenAI Client] Received %s message from OpenAI: %s", messageTypeStr, logData)
		}

		// Parse message
		var response OpenAIResponse
		if err := json.Unmarshal(data, &response); err != nil {
			log.Printf("[OpenAI Client] Error parsing OpenAI response: %v - Raw data: %s", err, string(data))
			continue
		}

		// Log the parsed response type
		log.Printf("[OpenAI Client] Parsed response type: %s", response.Type)

		// Process message based on its type
		switch response.Type {
		case "session.created":
			// Extract session information from the response
			// The session.created event contains a "session" object with an "id" field
			var sessionResponse struct {
				Session struct {
					ID string `json:"id"`
				} `json:"session"`
			}

			if err := json.Unmarshal(data, &sessionResponse); err != nil {
				log.Printf("[OpenAI Client] Error parsing session response: %v", err)
				continue
			}

			if sessionResponse.Session.ID == "" {
				log.Printf("[OpenAI Client] Error: received empty session ID")
				continue
			}

			// Store the session ID
			c.sessionID = sessionResponse.Session.ID
			log.Printf("[OpenAI Client] Session created with ID: %s", c.sessionID)

			// Update session with proper session ID to ensure subsequent operations work
			if err := c.updateSession(); err != nil {
				log.Printf("[OpenAI Client] Warning: Failed to update session after creation: %v", err)
			}

			// Send event
			c.eventChan <- types.OpenAIEvent{
				Type:      "session.created",
				SessionID: c.sessionID,
			}

		case "error":
			// Extract error information
			var errorResponse struct {
				Error struct {
					Type    string `json:"type"`
					Code    string `json:"code"`
					Message string `json:"message"`
					Param   string `json:"param"`
				} `json:"error"`
			}

			if err := json.Unmarshal(data, &errorResponse); err != nil {
				log.Printf("[OpenAI Client] Error parsing error response: %v", err)
			} else {
				log.Printf("[OpenAI Client] Received error from OpenAI: %s - %s",
					errorResponse.Error.Code, errorResponse.Error.Message)

				// Handle "missing_required_parameter" errors with session
				if errorResponse.Error.Code == "missing_required_parameter" &&
					errorResponse.Error.Param == "session" && c.sessionID != "" {
					log.Printf("[OpenAI Client] Attempting to recover from session error by updating session...")
					if err := c.updateSession(); err != nil {
						log.Printf("[OpenAI Client] Failed to recover session: %v", err)
					}
				}
			}

			// Send error event
			c.eventChan <- types.OpenAIEvent{
				Type:    "error",
				Message: fmt.Sprintf("OpenAI API error: %s", string(data)),
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
