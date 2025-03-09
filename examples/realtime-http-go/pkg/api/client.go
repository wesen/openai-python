package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openai/realtime-http-go/pkg/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// OpenAIRealtimeClient is the interface for communicating with the OpenAI Realtime API
type OpenAIRealtimeClient interface {
	// Connect connects to the OpenAI Realtime API
	Connect(ctx context.Context) error
	// Disconnect disconnects from the OpenAI Realtime API
	Disconnect() error
	// SendAudio sends audio data to the OpenAI Realtime API
	SendAudio(audioData string) error
	// CommitAudio commits the audio buffer
	CommitAudio() error
	// SendText sends a text message to the OpenAI Realtime API
	SendText(text string) error
	// IsConnected returns true if the client is connected
	IsConnected() bool
	// RegisterEventHandler registers an event handler for a specific event type
	RegisterEventHandler(eventType string, handler EventHandler)
}

// Event types for OpenAI Realtime API
const (
	EventTypeSessionCreated                     = "session.created"
	EventTypeSessionUpdated                     = "session.updated"
	EventTypeConversationItemCreated            = "conversation.item.created"
	EventTypeConversationItemInputAudioTransComplete = "conversation.item.input_audio_transcription.completed"
	EventTypeConversationItemInputAudioTransFailed  = "conversation.item.input_audio_transcription.failed"
	EventTypeResponseCreated                    = "response.created"
	EventTypeResponseTextDelta                  = "response.text.delta"
	EventTypeResponseTextDone                   = "response.text.done"
	EventTypeResponseAudioDelta                 = "response.audio.delta"
	EventTypeResponseAudioDone                  = "response.audio.done"
	EventTypeResponseAudioTranscriptDelta       = "response.audio_transcript.delta"
	EventTypeResponseAudioTranscriptDone        = "response.audio_transcript.done"
	EventTypeResponseContentPartAdded           = "response.content_part.added"
	EventTypeResponseContentPartDone            = "response.content_part.done"
	EventTypeResponseDone                       = "response.done"
	EventTypeError                              = "error"
)

// EventHandler is a function that handles OpenAI Realtime API events
type EventHandler func(event map[string]interface{}) error

//------------------------------------------------------------------------------
// WebSocket-based OpenAI Realtime client
//------------------------------------------------------------------------------

// DefaultOpenAIRealtimeClient is the WebSocket implementation of OpenAIRealtimeClient
type DefaultOpenAIRealtimeClient struct {
	// Configuration
	config *config.Config

	// WebSocket connection
	conn *websocket.Conn

	// Session information
	sessionID string

	// Event handlers
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex

	// Connection status
	connected bool
	connMutex sync.RWMutex

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewOpenAIRealtimeClient creates a new WebSocket-based OpenAI Realtime client
func NewOpenAIRealtimeClient(cfg *config.Config) OpenAIRealtimeClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &DefaultOpenAIRealtimeClient{
		config:        cfg,
		eventHandlers: make(map[string][]EventHandler),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// RegisterEventHandler registers an event handler for a specific event type
func (c *DefaultOpenAIRealtimeClient) RegisterEventHandler(eventType string, handler EventHandler) {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()

	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
}

// Connect connects to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) Connect(ctx context.Context) error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.connected {
		log.Debug().Msg("Already connected to OpenAI Realtime API")
		return nil
	}

	// Determine API base URL - The correct realtime API endpoint is /v1/realtime
	apiBase := "wss://api.openai.com/v1/realtime"
	if c.config.OpenAIAPIBase != "" {
		log.Debug().Str("base_url", c.config.OpenAIAPIBase).Msg("Using custom API base URL")
		if strings.HasPrefix(c.config.OpenAIAPIBase, "https://") {
			// Convert https:// to wss://
			apiBase = "wss://" + strings.TrimPrefix(c.config.OpenAIAPIBase, "https://") + "/v1/realtime"
			log.Debug().Str("converted_url", apiBase).Msg("Converted HTTPS to WSS")
		} else if strings.HasPrefix(c.config.OpenAIAPIBase, "http://") {
			// Convert http:// to ws://
			apiBase = "ws://" + strings.TrimPrefix(c.config.OpenAIAPIBase, "http://") + "/v1/realtime"
			log.Debug().Str("converted_url", apiBase).Msg("Converted HTTP to WS")
		} else {
			apiBase = c.config.OpenAIAPIBase
			log.Debug().Str("using_url", apiBase).Msg("Using API base URL as-is")
		}
	}
	
	// Check for duplicate v1 path issue - this can cause bad handshake errors
	if strings.Contains(apiBase, "/v1/v1/") {
		log.Warn().Str("original_url", apiBase).Msg("Found duplicate v1 in API path, fixing URL")
		apiBase = strings.Replace(apiBase, "/v1/v1/", "/v1/", 1)
		log.Debug().Str("fixed_url", apiBase).Msg("Fixed API URL")
	}

	// Add model as query parameter
	u, err := url.Parse(apiBase)
	if err != nil {
		log.Error().Err(err).Str("api_base", apiBase).Msg("Failed to parse API base URL")
		return fmt.Errorf("failed to parse API base URL: %w", err)
	}

	q := u.Query()
	q.Set("model", c.config.OpenAIModelName)
	u.RawQuery = q.Encode()

	log.Debug().Str("model", c.config.OpenAIModelName).Str("full_url", u.String()).Msg("Setting up connection with model")

	// Set up headers
	headers := http.Header{}
	headers.Add("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	headers.Add("Content-Type", "application/json")
	headers.Add("OpenAI-Beta", "realtime=v1")

	// Debug headers (without showing the full API key)
	apiKeyDebug := "********"
	if len(c.config.OpenAIAPIKey) > 4 {
		apiKeyDebug = c.config.OpenAIAPIKey[:4] + "********"
	}
	log.Debug().
		Str("auth", "Bearer "+apiKeyDebug).
		Str("content_type", headers.Get("Content-Type")).
		Str("beta", headers.Get("OpenAI-Beta")).
		Msg("Request headers")

	log.Info().Str("url", u.String()).Msg("Connecting to OpenAI Realtime API")

	// Connect to WebSocket with debug logging
	dialer := websocket.DefaultDialer
	// Increase handshake timeout for potentially slow networks
	dialer.HandshakeTimeout = 45 * time.Second
	log.Debug().
		Int("handshake_timeout_secs", int(dialer.HandshakeTimeout.Seconds())).
		Bool("enable_compression", dialer.EnableCompression).
		Msg("WebSocket dialer configuration")

	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil {
			// Read the response body for more detailed error information
			var responseBody []byte
			if resp.Body != nil {
				responseBody, _ = io.ReadAll(resp.Body)
				resp.Body.Close()
			}
			
			log.Error().
				Err(err).
				Int("status_code", resp.StatusCode).
				Str("status", resp.Status).
				Str("url", u.String()).
				Str("response_body", string(responseBody)).
				Msg("WebSocket connection failed with HTTP error")
			
			// Log response headers for debugging
			log.Debug().Fields(func(e *zerolog.Event) {
				for k, v := range resp.Header {
					e.Strs(k, v)
				}
			}).Msg("Response headers")
		} else {
			log.Error().
				Err(err).
				Str("url", u.String()).
				Msg("WebSocket connection failed without HTTP response")
				
			// Check for common connection errors
			if strings.Contains(err.Error(), "certificate") {
				return fmt.Errorf("failed to connect to OpenAI Realtime API: SSL/TLS certificate error: %w", err)
			} else if strings.Contains(err.Error(), "timeout") {
				return fmt.Errorf("failed to connect to OpenAI Realtime API: connection timeout: %w", err)
			} else if strings.Contains(err.Error(), "no such host") {
				return fmt.Errorf("failed to connect to OpenAI Realtime API: host not found: %w", err)
			} else if strings.Contains(err.Error(), "bad handshake") {
				return fmt.Errorf("failed to connect to OpenAI Realtime API: bad handshake, check API URL and ensure you have correct access: %w", err)
			}
		}
		return fmt.Errorf("failed to connect to OpenAI Realtime API: %w", err)
	}

	log.Debug().Msg("WebSocket connection established successfully")
	c.conn = conn
	c.connected = true

	// Start event reader
	go c.readEvents()

	// Set up server-side Voice Activity Detection
	log.Debug().Msg("Setting up server-side Voice Activity Detection")
	err = c.setupServerVAD()
	if err != nil {
		log.Error().Err(err).Msg("Failed to set up server VAD")
		c.Disconnect()
		return fmt.Errorf("failed to set up server VAD: %w", err)
	}
	log.Debug().Msg("Server VAD setup complete")

	return nil
}

// setupServerVAD sets up server-side Voice Activity Detection
func (c *DefaultOpenAIRealtimeClient) setupServerVAD() error {
	message := map[string]interface{}{
		"type": "session.update",
		"session": map[string]interface{}{
			"turn_detection": map[string]interface{}{
				"type": "server_vad",
			},
			"input_audio_transcription": map[string]interface{}{
				"model": "whisper-1",
			},
		},
	}

	log.Debug().Interface("message", message).Msg("Sending session.update message for server VAD setup")
	
	err := c.sendJSON(message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send server VAD setup message")
		return err
	}
	
	log.Debug().Msg("Server VAD setup message sent successfully")
	return nil
}

// Disconnect disconnects from the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) Disconnect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	// Close WebSocket connection
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Error().Err(err).Msg("Error sending close message")
	}

	// Wait a bit for the close to be sent
	time.Sleep(100 * time.Millisecond)

	// Close the connection
	err = c.conn.Close()
	c.conn = nil
	c.connected = false

	// Signal cancellation
	c.cancel()

	return err
}

// SendAudio sends audio data to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) SendAudio(audioData string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Send audio buffer append event
	message := map[string]interface{}{
		"type": "input_audio_buffer.append",
		"audio": audioData,
	}

	return c.sendJSON(message)
}

// CommitAudio commits the audio buffer and triggers a response
func (c *DefaultOpenAIRealtimeClient) CommitAudio() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Send audio buffer commit event
	commitMsg := map[string]interface{}{
		"type": "input_audio_buffer.commit",
	}

	if err := c.sendJSON(commitMsg); err != nil {
		return err
	}

	// Send response create event - not needed with server VAD but included for compatibility
	responseMsg := map[string]interface{}{
		"type": "response.create",
	}

	return c.sendJSON(responseMsg)
}

// SendText sends a text message to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) SendText(text string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Send conversation item create event
	itemMsg := map[string]interface{}{
		"type": "conversation.item.create",
		"item": map[string]interface{}{
			"role": "user",
			"content": map[string]string{
				"text": text,
			},
		},
	}

	if err := c.sendJSON(itemMsg); err != nil {
		return err
	}

	// Send response create event
	responseMsg := map[string]interface{}{
		"type": "response.create",
	}

	return c.sendJSON(responseMsg)
}

// IsConnected returns true if the client is connected
func (c *DefaultOpenAIRealtimeClient) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()

	return c.connected && c.conn != nil
}

// sendJSON sends a JSON message to the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) sendJSON(message interface{}) error {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()

	if !c.connected || c.conn == nil {
		log.Error().Interface("message_type", message).Msg("Cannot send JSON: not connected to OpenAI Realtime API")
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Convert to JSON for logging
	jsonBytes, err := json.Marshal(message)
	if err != nil {
		log.Error().Err(err).Interface("message", message).Msg("Failed to marshal message for logging")
	} else {
		preview := string(jsonBytes)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		log.Debug().Str("message", preview).Msg("Sending WebSocket message")
	}

	// Send the message
	err = c.conn.WriteJSON(message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send WebSocket message")
		return err
	}

	log.Debug().Msg("WebSocket message sent successfully")
	return nil
}

// readEvents reads events from the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) readEvents() {
	defer func() {
		c.connMutex.Lock()
		c.connected = false
		c.connMutex.Unlock()
		log.Debug().Msg("readEvents goroutine exiting, connection marked as disconnected")
	}()

	log.Debug().Msg("Starting WebSocket read event loop")

	for {
		select {
		case <-c.ctx.Done():
			log.Debug().Msg("Context cancelled, exiting read event loop")
			return
		default:
			// Continue reading
		}

		// Read a message with detailed logging
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("WebSocket closed unexpectedly")
			} else if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Debug().Msg("WebSocket closed normally")
			} else {
				log.Error().Err(err).Msg("Error reading from WebSocket")
			}
			return
		}

		// Log basic info about received message
		log.Debug().
			Int("message_type", messageType).
			Int("message_length", len(message)).
			Str("message_preview", func() string {
				if len(message) > 100 {
					return string(message[:100]) + "..."
				}
				return string(message)
			}()).
			Msg("Received WebSocket message")

		// Parse the message
		var event map[string]interface{}
		if err := json.Unmarshal(message, &event); err != nil {
			log.Error().
				Err(err).
				Str("message", string(message)).
				Msg("Failed to unmarshal event")
			continue
		}

		// Extract event type
		eventType, ok := event["type"].(string)
		if !ok {
			log.Error().Interface("event", event).Msg("Event has no type")
			continue
		}

		log.Debug().
			Str("event_type", eventType).
			Interface("event_data", event).
			Msg("Received event from OpenAI")

		// Extract session ID from session.created event
		if eventType == EventTypeSessionCreated {
			if session, ok := event["session"].(map[string]interface{}); ok {
				if id, ok := session["id"].(string); ok {
					c.sessionID = id
					log.Info().Str("session_id", id).Msg("Session created")
				} else {
					log.Error().Interface("session", session).Msg("Session created but no ID found")
				}
			} else {
				log.Error().Interface("event", event).Msg("Session created but no session object found")
			}
		}

		// Handle the event
		c.handleEvent(eventType, event)
	}
}

// handleEvent handles an event from the OpenAI Realtime API
func (c *DefaultOpenAIRealtimeClient) handleEvent(eventType string, event map[string]interface{}) {
	c.handlersMutex.RLock()
	handlers := c.eventHandlers[eventType]
	// Also get generic handlers (those that handle all events)
	genericHandlers := c.eventHandlers["*"]
	c.handlersMutex.RUnlock()

	// Log event type but not entire event (which may be large)
	log.Debug().
		Str("event_type", eventType).
		Int("handlers", len(handlers)).
		Int("generic_handlers", len(genericHandlers)).
		Msg("Received event")

	// Call handlers for this event type
	for _, handler := range handlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling event")
		}
	}

	// Call generic handlers
	for _, handler := range genericHandlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling event (generic handler)")
		}
	}

	// Special handling for error events
	if eventType == EventTypeError {
		errorMsg := "Unknown error"
		if errObj, ok := event["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok {
				errorMsg = msg
			}
		}
		log.Error().Str("error", errorMsg).Msg("OpenAI API error")
	}
}

//------------------------------------------------------------------------------
// HTTP-based OpenAI Realtime client
//------------------------------------------------------------------------------

// HTTPRealtimeClient implements the OpenAI Realtime protocol using HTTP instead of WebSockets
type HTTPRealtimeClient struct {
	// Configuration
	config *config.Config
	
	// HTTP client
	client *http.Client
	
	// Session information
	sessionID string
	
	// Event handlers
	eventHandlers map[string][]EventHandler
	handlersMutex sync.RWMutex
	
	// Connection status
	connected bool
	connMutex sync.RWMutex
	
	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
	
	// Audio buffer
	audioBuffer []string
	bufferMutex sync.Mutex
}

// NewHTTPRealtimeClient creates a new HTTP-based OpenAI Realtime client
func NewHTTPRealtimeClient(cfg *config.Config) OpenAIRealtimeClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &HTTPRealtimeClient{
		config:        cfg,
		client:        &http.Client{Timeout: 60 * time.Second},
		eventHandlers: make(map[string][]EventHandler),
		ctx:           ctx,
		cancel:        cancel,
		audioBuffer:   make([]string, 0),
	}
}

// RegisterEventHandler registers an event handler for a specific event type
func (c *HTTPRealtimeClient) RegisterEventHandler(eventType string, handler EventHandler) {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()
	
	c.eventHandlers[eventType] = append(c.eventHandlers[eventType], handler)
}

// Connect connects to the OpenAI Realtime API
func (c *HTTPRealtimeClient) Connect(ctx context.Context) error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	
	if c.connected {
		return nil
	}
	
	// Create a session
	baseURL := "https://api.openai.com"
	if c.config.OpenAIAPIBase != "" {
		baseURL = c.config.OpenAIAPIBase
		// Remove trailing /v1 if present
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
	}
	
	// Create the session
	u := fmt.Sprintf("%s/v1/realtime/sessions", baseURL)
	
	// Create session request body
	body := map[string]interface{}{
		"model":      c.config.OpenAIModelName,
		"modalities": []string{"text", "audio"},
		"voice":      "alloy", // Default voice, could be configurable
	}
	
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal session create request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create session request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "realtime=v1")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create session: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	
	var sessionResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return fmt.Errorf("failed to decode session response: %w", err)
	}
	
	// Extract session ID
	if sessionID, ok := sessionResp["id"].(string); ok {
		c.sessionID = sessionID
		log.Info().Str("session_id", sessionID).Msg("Session created")
		
		// Emit session created event
		c.handleEvent(EventTypeSessionCreated, map[string]interface{}{
			"type": EventTypeSessionCreated,
			"session": sessionResp,
		})
	} else {
		return fmt.Errorf("session response has no ID")
	}
	
	c.connected = true
	
	// Update session with VAD settings
	err = c.updateSession()
	if err != nil {
		c.Disconnect()
		return fmt.Errorf("failed to set up server VAD: %w", err)
	}
	
	return nil
}

// updateSession sets up session parameters
func (c *HTTPRealtimeClient) updateSession() error {
	baseURL := "https://api.openai.com"
	if c.config.OpenAIAPIBase != "" {
		baseURL = c.config.OpenAIAPIBase
		// Remove trailing /v1 if present
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
	}
	
	// Update session settings
	u := fmt.Sprintf("%s/v1/realtime/sessions/%s", baseURL, c.sessionID)
	
	body := map[string]interface{}{
		"turn_detection": map[string]interface{}{
			"type": "server_vad",
		},
		"input_audio_transcription": map[string]interface{}{
			"model": "whisper-1",
		},
	}
	
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal session update: %w", err)
	}
	
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPatch, u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create session update request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "realtime=v1")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update session: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	
	var updatedSession map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&updatedSession); err != nil {
		return fmt.Errorf("failed to decode session update response: %w", err)
	}
	
	// Emit session updated event
	c.handleEvent(EventTypeSessionUpdated, map[string]interface{}{
		"type": EventTypeSessionUpdated,
		"session": updatedSession,
	})
	
	return nil
}

// Disconnect disconnects from the OpenAI Realtime API
func (c *HTTPRealtimeClient) Disconnect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	
	if !c.connected {
		return nil
	}
	
	// Delete the session
	baseURL := "https://api.openai.com"
	if c.config.OpenAIAPIBase != "" {
		baseURL = c.config.OpenAIAPIBase
		// Remove trailing /v1 if present
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
	}
	
	u := fmt.Sprintf("%s/v1/realtime/sessions/%s", baseURL, c.sessionID)
	req, err := http.NewRequestWithContext(c.ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	req.Header.Set("OpenAI-Beta", "realtime=v1")
	
	resp, err := c.client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Error deleting session")
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			log.Error().Int("status", resp.StatusCode).Msg("Failed to delete session")
		}
	}
	
	c.connected = false
	c.cancel()
	
	return nil
}

// SendAudio sends audio data to the OpenAI Realtime API
func (c *HTTPRealtimeClient) SendAudio(audioData string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}
	
	// Add to buffer
	c.bufferMutex.Lock()
	c.audioBuffer = append(c.audioBuffer, audioData)
	c.bufferMutex.Unlock()
	
	return nil
}

// CommitAudio commits the audio buffer and triggers a response
func (c *HTTPRealtimeClient) CommitAudio() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}
	
	// Get the audio buffer
	c.bufferMutex.Lock()
	audioBuffer := c.audioBuffer
	c.audioBuffer = make([]string, 0)
	c.bufferMutex.Unlock()
	
	if len(audioBuffer) == 0 {
		return nil
	}
	
	// Combine all audio data
	audioData := strings.Join(audioBuffer, "")
	
	// Create a conversation item (user message with audio)
	baseURL := "https://api.openai.com"
	if c.config.OpenAIAPIBase != "" {
		baseURL = c.config.OpenAIAPIBase
		// Remove trailing /v1 if present
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
	}
	
	u := fmt.Sprintf("%s/v1/realtime/sessions/%s/conversations/items", baseURL, c.sessionID)
	
	// Create input item
	body := map[string]interface{}{
		"type": "message",
		"role": "user",
		"content": []map[string]interface{}{
			{
				"type":  "input_audio",
				"audio": audioData,
			},
		},
	}
	
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal input item: %w", err)
	}
	
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create input item request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "realtime=v1")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send input item: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send input item: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	
	var itemResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&itemResp); err != nil {
		return fmt.Errorf("failed to decode input item response: %w", err)
	}
	
	// Emit input item created event
	// Get item ID
	c.handleEvent(EventTypeConversationItemCreated, map[string]interface{}{
		"type": EventTypeConversationItemCreated,
		"item": itemResp,
	})
	
	// Create a response
	go c.createResponse()
	
	return nil
}

// SendText sends a text message to the OpenAI Realtime API
func (c *HTTPRealtimeClient) SendText(text string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}
	
	// Create a conversation item (user message with text)
	baseURL := "https://api.openai.com"
	if c.config.OpenAIAPIBase != "" {
		baseURL = c.config.OpenAIAPIBase
		// Remove trailing /v1 if present
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
	}
	
	u := fmt.Sprintf("%s/v1/realtime/sessions/%s/conversations/items", baseURL, c.sessionID)
	
	// Create input item
	body := map[string]interface{}{
		"type": "message",
		"role": "user",
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
	
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal input item: %w", err)
	}
	
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create input item request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "realtime=v1")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send input item: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send input item: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	
	var itemResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&itemResp); err != nil {
		return fmt.Errorf("failed to decode input item response: %w", err)
	}
	
	// Emit input item created event
	// Get item ID
	c.handleEvent(EventTypeConversationItemCreated, map[string]interface{}{
		"type": EventTypeConversationItemCreated,
		"item": itemResp,
	})
	
	// Create a response
	go c.createResponse()
	
	return nil
}

// createResponse creates a response for the current conversation
func (c *HTTPRealtimeClient) createResponse() {
	baseURL := "https://api.openai.com"
	if c.config.OpenAIAPIBase != "" {
		baseURL = c.config.OpenAIAPIBase
		// Remove trailing /v1 if present
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
	}
	
	u := fmt.Sprintf("%s/v1/realtime/sessions/%s/responses", baseURL, c.sessionID)
	
	// Create response request
	body := map[string]interface{}{
		"modalities": []string{"text", "audio"},
	}
	
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response request")
		return
	}
	
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		log.Error().Err(err).Msg("Failed to create response request")
		return
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "realtime=v1")
	
	resp, err := c.client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send response request")
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Error().Int("status", resp.StatusCode).Str("body", string(bodyBytes)).Msg("Failed to create response")
		return
	}
	
	var responseResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseResp); err != nil {
		log.Error().Err(err).Msg("Failed to decode response request response")
		return
	}
	
	responseID, _ := responseResp["id"].(string)
	if responseID == "" {
		log.Error().Msg("Response ID is empty")
		return
	}
	
	// Emit response created event
	c.handleEvent(EventTypeResponseCreated, map[string]interface{}{
		"type": EventTypeResponseCreated,
		"response": responseResp,
	})
	
	// Poll for response events
	go c.pollResponseEvents(responseID)
}

// pollResponseEvents polls for response events
func (c *HTTPRealtimeClient) pollResponseEvents(responseID string) {
	baseURL := "https://api.openai.com"
	if c.config.OpenAIAPIBase != "" {
		baseURL = c.config.OpenAIAPIBase
		// Remove trailing /v1 if present
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
	}
	
	u := fmt.Sprintf("%s/v1/realtime/sessions/%s/responses/%s/events", baseURL, c.sessionID, responseID)
	
	lastEventID := ""
	done := false
	
	for !done {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Continue polling
		}
		
		req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, u, nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create response events request")
			return
		}
		
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.OpenAIAPIKey))
		req.Header.Set("OpenAI-Beta", "realtime=v1")
		
		// Add after parameter if we have a last event ID
		if lastEventID != "" {
			q := req.URL.Query()
			q.Add("after", lastEventID)
			req.URL.RawQuery = q.Encode()
		}
		
		resp, err := c.client.Do(req)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get response events")
			time.Sleep(500 * time.Millisecond)
			continue
		}
		
		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Error().Int("status", resp.StatusCode).Str("body", string(bodyBytes)).Msg("Failed to get response events")
			time.Sleep(500 * time.Millisecond)
			continue
		}
		
		var eventsResp struct {
			Events []map[string]interface{} `json:"events"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
			resp.Body.Close()
			log.Error().Err(err).Msg("Failed to decode response events")
			time.Sleep(500 * time.Millisecond)
			continue
		}
		resp.Body.Close()
		
		if len(eventsResp.Events) == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		
		// Process events
		for _, event := range eventsResp.Events {
			// Update last event ID
			if id, ok := event["id"].(string); ok {
				lastEventID = id
			}
			
			// Process the event
			if eventType, ok := event["type"].(string); ok {
				// Convert certain event types to match the WebSocket client
				switch eventType {
				case "response.content_part.added":
					// Extract text and emit response.text.delta
					if content, ok := event["content"].(map[string]interface{}); ok {
						if text, ok := content["text"].(string); ok && text != "" {
							textEvent := map[string]interface{}{
								"type":   EventTypeResponseTextDelta,
								"delta":  text,
								"itemId": event["item_id"],
							}
							c.handleEvent(EventTypeResponseTextDelta, textEvent)
						}
					}
					
					// Process the original event
					c.handleEvent(eventType, event)
				case "response.audio.delta":
					// Process as is
					c.handleEvent(EventTypeResponseAudioDelta, event)
				case "response.done":
					// Process and mark as done
					c.handleEvent(EventTypeResponseDone, event)
					done = true
				default:
					// Process other events normally
					c.handleEvent(eventType, event)
				}
			}
		}
		
		// Sleep briefly before next poll
		time.Sleep(100 * time.Millisecond)
	}
}

// IsConnected returns true if the client is connected
func (c *HTTPRealtimeClient) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	
	return c.connected
}

// handleEvent handles an event from the OpenAI Realtime API
func (c *HTTPRealtimeClient) handleEvent(eventType string, event map[string]interface{}) {
	c.handlersMutex.RLock()
	handlers := c.eventHandlers[eventType]
	// Also get generic handlers (those that handle all events)
	genericHandlers := c.eventHandlers["*"]
	c.handlersMutex.RUnlock()
	
	// Log event type but not entire event (which may be large)
	log.Debug().
		Str("event_type", eventType).
		Int("handlers", len(handlers)).
		Int("generic_handlers", len(genericHandlers)).
		Msg("Received event")
	
	// Call handlers for this event type
	for _, handler := range handlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling event")
		}
	}
	
	// Call generic handlers
	for _, handler := range genericHandlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling event (generic handler)")
		}
	}
	
	// Special handling for error events
	if eventType == EventTypeError {
		errorMsg := "Unknown error"
		if errObj, ok := event["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok {
				errorMsg = msg
			}
		}
		log.Error().Str("error", errorMsg).Msg("OpenAI API error")
	}
}

//------------------------------------------------------------------------------
// Mock OpenAI Realtime client for testing
//------------------------------------------------------------------------------

// MockOpenAIRealtimeClient is a mock implementation of OpenAIRealtimeClient for testing
type MockOpenAIRealtimeClient struct {
	connected bool
	handlers  map[string][]EventHandler
	mutex     sync.RWMutex
}

// NewMockOpenAIRealtimeClient creates a new mock OpenAI Realtime client
func NewMockOpenAIRealtimeClient() OpenAIRealtimeClient {
	return &MockOpenAIRealtimeClient{
		handlers: make(map[string][]EventHandler),
	}
}

// RegisterEventHandler registers an event handler for a specific event type
func (m *MockOpenAIRealtimeClient) RegisterEventHandler(eventType string, handler EventHandler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.handlers[eventType] = append(m.handlers[eventType], handler)
}

// Connect connects to the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) Connect(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.connected = true
	return nil
}

// Disconnect disconnects from the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) Disconnect() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.connected = false
	return nil
}

// SendAudio sends audio data to the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) SendAudio(audioData string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}
	return nil
}

// CommitAudio commits the audio buffer
func (m *MockOpenAIRealtimeClient) CommitAudio() error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Simulate a response for testing
	go func() {
		// Wait a bit to simulate processing
		time.Sleep(500 * time.Millisecond)

		// Simulate a transcript
		m.mockEvent(EventTypeConversationItemInputAudioTransComplete, map[string]interface{}{
			"transcript": "Hello, world!",
			"item_id":    "transcript_1",
		})

		// Simulate a text response
		m.mockEvent(EventTypeResponseTextDelta, map[string]interface{}{
			"delta":   "Hello, ",
			"item_id": "response_1",
		})

		time.Sleep(100 * time.Millisecond)

		m.mockEvent(EventTypeResponseTextDelta, map[string]interface{}{
			"delta":   "how can I help you?",
			"item_id": "response_1",
		})

		time.Sleep(100 * time.Millisecond)

		// Simulate done event
		m.mockEvent(EventTypeResponseDone, map[string]interface{}{})
	}()

	return nil
}

// SendText sends a text message to the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) SendText(text string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OpenAI Realtime API")
	}

	// Simulate a response for testing
	go func() {
		// Wait a bit to simulate processing
		time.Sleep(300 * time.Millisecond)

		response := fmt.Sprintf("You said: %s. I'm a mock OpenAI client.", text)
		words := strings.Split(response, " ")

		for i, word := range words {
			// Simulate text delta events
			m.mockEvent(EventTypeResponseTextDelta, map[string]interface{}{
				"delta":   word + " ",
				"item_id": "response_1",
			})
			time.Sleep(50 * time.Millisecond)

			// Every few words, simulate an audio delta event
			if i%3 == 0 {
				m.mockEvent(EventTypeResponseAudioDelta, map[string]interface{}{
					"delta":   "base64-encoded-audio-data",
					"item_id": "audio_1",
				})
			}
		}

		// Simulate done event
		m.mockEvent(EventTypeResponseDone, map[string]interface{}{})
	}()

	return nil
}

// IsConnected returns true if the client is connected
func (m *MockOpenAIRealtimeClient) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.connected
}

// mockEvent simulates an event from the OpenAI Realtime API
func (m *MockOpenAIRealtimeClient) mockEvent(eventType string, data map[string]interface{}) {
	m.mutex.RLock()
	handlers := m.handlers[eventType]
	genericHandlers := m.handlers["*"]
	m.mutex.RUnlock()

	event := data
	event["type"] = eventType

	// Call handlers for this event type
	for _, handler := range handlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling mock event")
		}
	}

	// Call generic handlers
	for _, handler := range genericHandlers {
		if err := handler(event); err != nil {
			log.Error().Err(err).Str("event_type", eventType).Msg("Error handling mock event (generic handler)")
		}
	}
}