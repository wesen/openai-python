package server

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/openai/realtime-http-go/internal/config"
	"github.com/openai/realtime-http-go/internal/openai"
	"github.com/openai/realtime-http-go/internal/ws"
	"github.com/openai/realtime-http-go/templates"
)

// Server represents the HTTP server for the OpenAI Realtime API
type Server struct {
	Router     *mux.Router
	wsUpgrader *websocket.Upgrader
	config     *config.Config
	wsManager  *ws.ConnectionManager
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config) *Server {
	r := mux.NewRouter()

	// Create WebSocket upgrader
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins
		},
	}

	// Create OpenAI client
	openaiClient := openai.NewClient(cfg.OpenAIAPIKey)

	// Create WebSocket connection manager
	wsManager := ws.NewConnectionManager(openaiClient)

	// Create server
	srv := &Server{
		Router:     r,
		wsUpgrader: upgrader,
		config:     cfg,
		wsManager:  wsManager,
	}

	// Set up routes
	srv.setupRoutes()

	return srv
}

// setupRoutes sets up the routes for the server
func (s *Server) setupRoutes() {
	// Static file server
	s.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Main page route
	s.Router.HandleFunc("/", s.handleIndex)

	// WebSocket route
	s.Router.HandleFunc("/ws", s.handleWebSocket)

	// Debug routes
	s.Router.HandleFunc("/debug/audio-format", s.handleDebugAudioFormat)
	s.Router.HandleFunc("/debug/audio", s.handleDebugAudio)

	// API routes
	s.Router.HandleFunc("/send-text", s.handleSendText).Methods("POST")
}

// handleIndex handles requests to the index page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	component := templates.Index()
	component.Render(r.Context(), w)
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("[WebSocket Server] New WebSocket connection request from %s", r.RemoteAddr)

	// Log request headers for debugging
	log.Printf("[WebSocket Server] Request headers: %v", r.Header)

	// Upgrade HTTP connection to WebSocket
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket Server] Error upgrading to WebSocket: %v", err)
		http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)
		return
	}

	log.Printf("[WebSocket Server] WebSocket connection established with %s", conn.RemoteAddr().String())

	// Add connection to manager
	s.wsManager.Connect(conn)

	log.Printf("[WebSocket Server] Added connection to WebSocket manager")
}

// handleDebugAudioFormat returns information about the expected audio format
func (s *Server) handleDebugAudioFormat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
		"status": "ok",
		"sample_rate": 24000,
		"channels": 1,
		"format": "PCM 16-bit"
	}`))
}

// handleDebugAudio returns information about audio processing
func (s *Server) handleDebugAudio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
		"status": "ok",
		"audio_processing": {
			"sample_rate": 24000,
			"channels": 1,
			"openai_api_key_set": true,
			"websocket_manager": {
				"active_connections": ` + string(s.wsManager.ConnectionCount()) + `
			}
		}
	}`))
}

// handleSendText handles text message submissions
func (s *Server) handleSendText(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "Missing text parameter", http.StatusBadRequest)
		return
	}

	// Send text to all WebSocket connections (broadcast)
	s.wsManager.BroadcastTextMessage(text)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}
