package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// connectionManager handles the WebSocket connection
type connectionManager struct {
	client     *clientImpl
	conn       *websocket.Conn
	connMutex  sync.RWMutex
	pingTicker *time.Ticker
	logger     zerolog.Logger
}

// connect establishes a WebSocket connection with the API
func (cm *connectionManager) connect(ctx context.Context) error {
	cm.logger.Debug().Str("url", BaseURL).Str("model", cm.client.model).Msg("Dialing WebSocket")

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+cm.client.apiKey)
	headers.Add("OpenAI-Beta", "realtime")

	// Create WebSocket Dialer
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: ConnectionTimeout,
	}

	// Add query params
	url := fmt.Sprintf("%s?model=%s", BaseURL, cm.client.model)

	// Establish connection
	conn, resp, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		// Connection failed, check for HTTP response
		if resp != nil {
			cm.logger.Error().
				Int("status_code", resp.StatusCode).
				Str("status", resp.Status).
				Msg("WebSocket connection failed")

			// Try to extract detailed API error if available
			var apiErr struct {
				Error struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				} `json:"error"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
				return fmt.Errorf("OpenAI API error: %s (type: %s, code: %s)",
					apiErr.Error.Message, apiErr.Error.Type, apiErr.Error.Code)
			}

			return fmt.Errorf("connection failed with status %s", resp.Status)
		}
		return fmt.Errorf("websocket dial error: %w", err)
	}

	// Store the connection
	cm.connMutex.Lock()
	cm.conn = conn
	cm.connMutex.Unlock()

	// Configure WebSocket behaviors
	conn.SetPingHandler(nil) // Use default Ping handler
	conn.SetPongHandler(func(string) error {
		// Reset the read deadline when we get a pong
		return conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	// Start regular pings
	cm.pingTicker = time.NewTicker(PingInterval)

	// Listen for session.created event to capture session ID
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				cm.logger.Warn().Err(err).Msg("Error reading initial messages")
				return
			}

			// Try to parse the message to see if it's a session.created event
			var eventData struct {
				Type string `json:"type"`
				Raw  json.RawMessage
			}

			if err := json.Unmarshal(message, &eventData); err != nil {
				cm.logger.Warn().Err(err).Msg("Error parsing initial message")
				continue
			}

			// Log the full raw JSON to diagnose unmarshal issues
			cm.logger.Debug().RawJSON("raw_event", message).Str("event_type", eventData.Type).Msg("Received event JSON")

			if eventData.Type == EventSessionCreated {
				var sessionEvent SessionCreatedEvent
				if err = json.Unmarshal(message, &sessionEvent); err != nil {
					cm.logger.Error().Err(err).RawJSON("raw_event", message).Str("event_type", eventData.Type).Msg("Failed to parse session.created event")
					continue
				}
				cm.client.sessionID.Store(sessionEvent.Session.ID)
				cm.logger.Info().Str("session_id", sessionEvent.Session.ID).Msg("Session created")
				break
			}
		}
	}()

	cm.logger.Debug().Msg("WebSocket connection established")
	return nil
}

// Start begins the connection manager operation
func (cm *connectionManager) Start(ctx context.Context) error {
	// Set up ping interval for keepalive
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-cm.pingTicker.C:
				cm.connMutex.Lock()
				if cm.conn != nil {
					if err := cm.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(ConnectionTimeout)); err != nil {
						cm.logger.Warn().Err(err).Msg("Failed to send ping")
					}
				}
				cm.connMutex.Unlock()
			}
		}
	}()

	return nil
}

// Stop ends the connection manager operation
func (cm *connectionManager) Stop(ctx context.Context) error {
	if cm.pingTicker != nil {
		cm.pingTicker.Stop()
	}

	cm.connMutex.Lock()
	defer cm.connMutex.Unlock()

	if cm.conn != nil {
		// Send close message
		deadline := time.Now().Add(time.Second)
		err := cm.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), deadline)
		if err != nil {
			cm.logger.Warn().Err(err).Msg("Error sending close message")
		}

		// Close the connection
		if err := cm.conn.Close(); err != nil {
			return fmt.Errorf("error closing websocket: %w", err)
		}
		cm.conn = nil
	}

	return nil
}
