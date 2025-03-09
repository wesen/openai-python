package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/a-h/templ"
	gorillaWs "github.com/gorilla/websocket"
	"github.com/openai/realtime-http-go/pkg/api"
	"github.com/openai/realtime-http-go/pkg/audio"
	"github.com/openai/realtime-http-go/pkg/config"
	"github.com/openai/realtime-http-go/pkg/websocket"
	"github.com/openai/realtime-http-go/web/templates"
	"github.com/rs/zerolog/log"
)

// ClientSession represents a client session with OpenAI
type ClientSession struct {
	ID            string
	WSClient      *websocket.Connection
	OpenAIClient  api.OpenAIRealtimeClient
	AudioDecoder  *audio.FFmpegDecoder
	AudioEncoder  *audio.FFmpegDecoder  // Use for encoding responses
	LastActive    time.Time
	Context       context.Context
	Cancel        context.CancelFunc
	Lock          sync.RWMutex
}

type Handler struct {
	wsManager    *websocket.Manager
	upgrader     *gorillaWs.Upgrader
	sessionCount int
	sessions     map[string]*ClientSession
	sessionsLock sync.RWMutex
	config       *config.Config
}

// NewHandler creates a new HTTP handler with WebSocket support
func NewHandler(cfg *config.Config) *Handler {
	upgrader := &gorillaWs.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
	}

	wsManager := websocket.NewManager()
	go wsManager.Run()

	return &Handler{
		wsManager:    wsManager,
		upgrader:     upgrader,
		sessionCount: 0,
		sessions:     make(map[string]*ClientSession),
		config:       cfg,
	}
}

// IndexHandler serves the main page
func (h *Handler) IndexHandler() http.Handler {
	return templ.Handler(templates.Index())
}

// WebSocketHandler handles WebSocket connections
func (h *Handler) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade connection to WebSocket")
		return
	}

	// Create a unique session ID
	sessionID := fmt.Sprintf("session_%d", h.sessionCount)
	h.sessionCount++

	// Create a new WebSocket client
	client := websocket.NewConnection(conn, sessionID)

	// Create a new client session
	ctx, cancel := context.WithCancel(context.Background())
	session := &ClientSession{
		ID:           sessionID,
		WSClient:     client,
		OpenAIClient: h.createOpenAIClient(),
		LastActive:   time.Now(),
		Context:      ctx,
		Cancel:       cancel,
	}

	// Create audio decoder for this session
	audioDecoder, err := audio.NewFFmpegDecoder("webm", h.config.AudioSampleRate, h.config.AudioChannels, h.config.MinBufferSize)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to create audio decoder")
	} else {
		session.AudioDecoder = audioDecoder
	}

	// Store session
	h.sessionsLock.Lock()
	h.sessions[sessionID] = session
	h.sessionsLock.Unlock()

	// Register the client
	h.wsManager.Register <- client

	// Register event handlers for OpenAI client
	h.registerOpenAIEventHandlers(session)

	// Connect to OpenAI
	go func() {
		if err := session.OpenAIClient.Connect(session.Context); err != nil {
			log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to connect to OpenAI")
			errorMsg := map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			}
			jsonMsg, _ := json.Marshal(errorMsg)
			client.Send <- jsonMsg
			return
		}
		
		// Send session created message
		sessionMsg := map[string]string{
			"type":       "session.created",
			"session_id": sessionID,
		}
		jsonMsg, _ := json.Marshal(sessionMsg)
		client.Send <- jsonMsg
	}()

	// Set message handler
	client.SetMessageHandler(func(message []byte) {
		h.handleClientMessage(sessionID, message)
	})

	// Start client routines
	go client.WritePump()
	go client.ReadPump(h.wsManager)

	// Clean up routine
	go func() {
		<-session.Context.Done()
		h.cleanupSession(sessionID)
	}()
}

// createOpenAIClient creates an OpenAI client
func (h *Handler) createOpenAIClient() api.OpenAIRealtimeClient {
	// In development mode, use the mock client
	if h.config.OpenAIAPIKey == "dev" {
		return api.NewMockOpenAIRealtimeClient()
	}

	// Create a real OpenAI client
	return api.NewOpenAIRealtimeClient(h.config)
}

// registerOpenAIEventHandlers registers event handlers for OpenAI client
func (h *Handler) registerOpenAIEventHandlers(session *ClientSession) {
	// Handle all events (for logging)
	session.OpenAIClient.(*api.DefaultOpenAIRealtimeClient).RegisterEventHandler("*", func(event map[string]interface{}) error {
		// Just log the event type for debugging
		if eventType, ok := event["type"].(string); ok {
			log.Debug().Str("session_id", session.ID).Str("event_type", eventType).Msg("OpenAI event")
		}
		return nil
	})

	// Handle text delta events
	session.OpenAIClient.(*api.DefaultOpenAIRealtimeClient).RegisterEventHandler(api.EventTypeResponseTextDelta, func(event map[string]interface{}) error {
		// Forward to client
		jsonMsg, _ := json.Marshal(event)
		session.WSClient.Send <- jsonMsg
		return nil
	})

	// Handle audio delta events
	session.OpenAIClient.(*api.DefaultOpenAIRealtimeClient).RegisterEventHandler(api.EventTypeResponseAudioDelta, func(event map[string]interface{}) error {
		// Forward to client
		jsonMsg, _ := json.Marshal(event)
		session.WSClient.Send <- jsonMsg
		return nil
	})

	// Handle transcription events
	session.OpenAIClient.(*api.DefaultOpenAIRealtimeClient).RegisterEventHandler(api.EventTypeConversationItemInputAudioTransComplete, func(event map[string]interface{}) error {
		// Forward to client
		jsonMsg, _ := json.Marshal(event)
		session.WSClient.Send <- jsonMsg
		return nil
	})

	// Handle error events
	session.OpenAIClient.(*api.DefaultOpenAIRealtimeClient).RegisterEventHandler(api.EventTypeError, func(event map[string]interface{}) error {
		// Forward to client
		jsonMsg, _ := json.Marshal(event)
		session.WSClient.Send <- jsonMsg
		return nil
	})

	// Handle done events
	session.OpenAIClient.(*api.DefaultOpenAIRealtimeClient).RegisterEventHandler(api.EventTypeResponseDone, func(event map[string]interface{}) error {
		// Forward to client
		jsonMsg, _ := json.Marshal(event)
		session.WSClient.Send <- jsonMsg
		return nil
	})
}

// cleanupSession cleans up a client session
func (h *Handler) cleanupSession(sessionID string) {
	h.sessionsLock.Lock()
	defer h.sessionsLock.Unlock()

	session, ok := h.sessions[sessionID]
	if !ok {
		return
	}

	// Disconnect from OpenAI
	if session.OpenAIClient != nil && session.OpenAIClient.IsConnected() {
		session.OpenAIClient.Disconnect()
	}

	// Close audio decoder
	if session.AudioDecoder != nil {
		session.AudioDecoder.Close()
	}

	// Delete session
	delete(h.sessions, sessionID)
	log.Info().Str("session_id", sessionID).Msg("Session cleaned up")
}

// handleClientMessage handles a message from a client
func (h *Handler) handleClientMessage(sessionID string, message []byte) {
	h.sessionsLock.RLock()
	session, ok := h.sessions[sessionID]
	h.sessionsLock.RUnlock()

	if !ok {
		log.Error().Str("session_id", sessionID).Msg("Session not found")
		return
	}

	// Update last active time
	session.Lock.Lock()
	session.LastActive = time.Now()
	session.Lock.Unlock()

	// Parse message
	var data map[string]interface{}
	if err := json.Unmarshal(message, &data); err != nil {
		log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to parse message")
		return
	}

	// Extract message type
	msgType, ok := data["type"].(string)
	if !ok {
		log.Error().Str("session_id", sessionID).Interface("data", data).Msg("Message has no type")
		return
	}

	// Handle message based on type
	switch msgType {
	case "input_audio":
		// Handle audio input
		h.handleAudioInput(session, data)
	case "input_text":
		// Handle text input
		h.handleTextInput(session, data)
	case "commit_audio":
		// Commit audio buffer
		if err := session.OpenAIClient.CommitAudio(); err != nil {
			log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to commit audio")
		}
	default:
		log.Debug().Str("session_id", sessionID).Str("type", msgType).Msg("Unknown message type")
	}
}

// handleAudioInput handles audio input from client
func (h *Handler) handleAudioInput(session *ClientSession, data map[string]interface{}) {
	// Extract audio data
	audioData, ok := data["audio"].(string)
	if !ok {
		log.Error().Str("session_id", session.ID).Msg("Audio message has no audio data")
		return
	}

	// Check if OpenAI client is connected
	if !session.OpenAIClient.IsConnected() {
		log.Error().Str("session_id", session.ID).Msg("OpenAI client not connected")
		
		// Try to reconnect
		if err := session.OpenAIClient.Connect(session.Context); err != nil {
			log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to reconnect to OpenAI")
			return
		}
	}

	// Process audio (decode to PCM)
	if session.AudioDecoder != nil {
		// Decode audio (Base64 -> PCM)
		err := session.AudioDecoder.DecodeBase64Chunk(audioData)
		if err != nil {
			log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to decode audio")
			return
		}

		// If we have enough data, send to OpenAI
		decodedData, err := session.AudioDecoder.GetDecodedAudio()
		if err != nil {
			log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to get decoded audio")
			return
		}

		if decodedData != nil {
			// Encode audio to base64
			encodedData := base64.StdEncoding.EncodeToString(decodedData)
			
			// Send to OpenAI
			if err := session.OpenAIClient.SendAudio(encodedData); err != nil {
				log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to send audio to OpenAI")
			}
		}
	} else {
		// If no decoder, send raw audio to OpenAI
		if err := session.OpenAIClient.SendAudio(audioData); err != nil {
			log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to send audio to OpenAI")
		}
	}
}

// handleTextInput handles text input from client
func (h *Handler) handleTextInput(session *ClientSession, data map[string]interface{}) {
	// Extract text
	text, ok := data["text"].(string)
	if !ok {
		log.Error().Str("session_id", session.ID).Msg("Text message has no text")
		return
	}

	// Check if OpenAI client is connected
	if !session.OpenAIClient.IsConnected() {
		log.Error().Str("session_id", session.ID).Msg("OpenAI client not connected")
		
		// Try to reconnect
		if err := session.OpenAIClient.Connect(session.Context); err != nil {
			log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to reconnect to OpenAI")
			return
		}
	}

	// Send text to OpenAI
	if err := session.OpenAIClient.SendText(text); err != nil {
		log.Error().Err(err).Str("session_id", session.ID).Msg("Failed to send text to OpenAI")
	}
}

// StartRecordingHandler handles the start recording request
func (h *Handler) StartRecordingHandler(w http.ResponseWriter, r *http.Request) {
	// This would trigger audio recording in a real implementation
	// For now, we'll just send a notification to all clients
	// that recording has started
	msg := map[string]string{
		"type":   "recording_started",
		"status": "Recording started",
	}
	jsonMsg, _ := json.Marshal(msg)
	h.wsManager.Broadcast <- jsonMsg

	w.WriteHeader(http.StatusOK)
}

// StopRecordingHandler handles the stop recording request
func (h *Handler) StopRecordingHandler(w http.ResponseWriter, r *http.Request) {
	// This would stop audio recording in a real implementation
	msg := map[string]string{
		"type":   "recording_stopped",
		"status": "Recording stopped",
	}
	jsonMsg, _ := json.Marshal(msg)
	h.wsManager.Broadcast <- jsonMsg

	w.WriteHeader(http.StatusOK)
}

// SendMessageHandler handles sending text messages via HTTP
func (h *Handler) SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	userMessage := r.FormValue("message")
	if userMessage == "" {
		http.Error(w, "Empty message", http.StatusBadRequest)
		return
	}

	sessionID := r.FormValue("session_id")
	if sessionID == "" {
		// Use first session as fallback
		h.sessionsLock.RLock()
		for id := range h.sessions {
			sessionID = id
			break
		}
		h.sessionsLock.RUnlock()

		if sessionID == "" {
			http.Error(w, "No active session", http.StatusBadRequest)
			return
		}
	}

	// Get session
	h.sessionsLock.RLock()
	session, ok := h.sessions[sessionID]
	h.sessionsLock.RUnlock()

	if !ok {
		http.Error(w, "Session not found", http.StatusBadRequest)
		return
	}

	// Create input_text message
	inputMsg := map[string]interface{}{
		"type": "input_text",
		"text": userMessage,
	}

	// Handle the message
	h.handleTextInput(session, inputMsg)

	w.WriteHeader(http.StatusOK)
}

// RegisterRoutes registers all HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Static files
	fs := http.FileServer(http.Dir("./web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	
	// Main page
	mux.Handle("/", h.IndexHandler())
	
	// WebSocket
	mux.HandleFunc("/ws", h.WebSocketHandler)
	
	// API endpoints
	mux.HandleFunc("POST /start-recording", h.StartRecordingHandler)
	mux.HandleFunc("POST /stop-recording", h.StopRecordingHandler)
	mux.HandleFunc("POST /send-message", h.SendMessageHandler)
}

// Shutdown gracefully shuts down the handler
func (h *Handler) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down handler")

	// Close all sessions
	h.sessionsLock.Lock()
	for sessionID, session := range h.sessions {
		log.Info().Str("session_id", sessionID).Msg("Closing session")
		session.Cancel()
	}
	h.sessionsLock.Unlock()

	// Wait for context to be done
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}