package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// connectionHandler manages the WebSocket connection and all message sending
// It combines the functionality of the previous connectionManager and messageSender
type connectionHandler struct {
	client     *clientImpl
	conn       *websocket.Conn // Owned exclusively by the handler goroutine
	msgQueue   chan interface{}
	pingTicker *time.Ticker
	logger     zerolog.Logger
	running    atomic.Bool
}

// newConnectionHandler creates a new unified connection handler
func newConnectionHandler(client *clientImpl) *connectionHandler {
	client.logger.Debug().Msg("Creating new connection handler")
	return &connectionHandler{
		client:   client,
		msgQueue: make(chan interface{}, 100), // Buffer size of 100 messages
		logger:   client.logger.With().Str("component", "connection_handler").Logger(),
	}
}

// Connect establishes a connection to the OpenAI WebSocket API
func (ch *connectionHandler) Connect(ctx context.Context) error {
	ch.logger.Debug().Msg("Connecting to WebSocket API")
	if ch.running.Load() {
		return ErrAlreadyRunning
	}

	// Construct the URL for the WebSocket connection
	apiURL := fmt.Sprintf("%s?model=%s", BaseURL, url.QueryEscape(ch.client.model))

	ch.logger.Debug().Str("url", apiURL).Msg("Connecting to WebSocket API")

	// Initialize dialer with appropriate timeouts
	dialer := &websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  ConnectionTimeout,
		EnableCompression: false, // Disable compression to avoid fragmentation issues
	}

	// Add authorization header
	header := http.Header{}
	header.Add("Authorization", fmt.Sprintf("Bearer %s", ch.client.apiKey))
	header.Add("OpenAI-Beta", "realtime=v1")

	// Connect to the WebSocket API
	conn, resp, err := dialer.DialContext(ctx, apiURL, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			ch.logger.Error().
				Int("status", resp.StatusCode).
				Str("body", string(body)).
				Msg("WebSocket connection failed")

			return fmt.Errorf("websocket connection failed with status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("failed to connect to WebSocket API: %w", err)
	}

	// Store the connection for later use
	ch.conn = conn

	// Set read deadline for the initial connection
	if err := conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Configure WebSocket behaviors
	conn.SetPingHandler(nil) // Use default Ping handler
	conn.SetPongHandler(func(string) error {
		ch.logger.Debug().Msg("Received pong frame")
		return conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	// Disable write compression to avoid fragmentation issues
	conn.EnableWriteCompression(false)

	ch.logger.Debug().
		Bool("compression_enabled", false).
		Msg("WebSocket connection established with configuration")

	// Start the ping ticker
	ch.pingTicker = time.NewTicker(PingInterval)

	ch.running.Store(true)
	return nil
}

// Run starts the connection handler main loop
func (ch *connectionHandler) Run(ctx context.Context) error {
	ch.logger.Debug().Msg("Connection handler running")

	if ch.conn == nil {
		return fmt.Errorf("connection not established, call Connect first")
	}
	defer func() {
		ch.running.Store(false)
		if ch.conn != nil {
			ch.conn.Close()
			ch.conn = nil
		}
		if ch.pingTicker != nil {
			ch.pingTicker.Stop()
		}
		ch.logger.Debug().Msg("Connection handler stopping")
	}()

	// Create channels for communication
	readChan := make(chan []byte, 10)

	// Create error group for parallel execution
	eg, egCtx := errgroup.WithContext(ctx)

	// Start WebSocket reader goroutine
	eg.Go(func() error {
		ch.logger.Debug().Msg("WebSocket reader goroutine started")
		for {
			select {
			case <-egCtx.Done():
				return egCtx.Err()
			default:
				if ch.conn == nil {
					return errors.New("no active connection")
				}

				err := ch.conn.SetReadDeadline(time.Now().Add(PongWait))
				if err != nil {
					ch.logger.Warn().Err(err).Msg("Error setting read deadline")
				}

				ch.logger.Debug().Msg("Reading message")
				_, message, err := ch.conn.ReadMessage()
				if err != nil {
					ch.logger.Warn().Err(err).Msg("Error reading message")
					select {
					case <-egCtx.Done():
						return egCtx.Err()
					default:
						return err
					}
				}
				ch.logger.Debug().Str("message", string(message)).Msg("Message read")

				select {
				case readChan <- message:
					// Message sent successfully
				case <-egCtx.Done():
					return egCtx.Err()
				}
			}
		}
	})

	// Start main message handler goroutine
	eg.Go(func() error {
		ch.logger.Debug().Msg("Main message handler goroutine started")
		for {
			select {
			case <-egCtx.Done():
				return egCtx.Err()

			case message := <-readChan:
				ch.logger.Debug().
					Int("message_length", len(message)).
					Msg("Received WebSocket message")

				// Forward the message to the client's event channel
				select {
				case ch.client.eventChan <- message:
					// Message forwarded successfully
				case <-egCtx.Done():
					return egCtx.Err()
				default:
					ch.logger.Warn().Msg("Event channel full, dropping message")
				}

			case <-ch.pingTicker.C:
				deadline := time.Now().Add(PingWriteTimeout)
				if ch.conn != nil {
					if err := ch.conn.WriteControl(websocket.PingMessage, []byte{}, deadline); err != nil {
						ch.logger.Warn().Err(err).Msg("Failed to send ping")
					} else {
						ch.logger.Debug().Msg("Ping sent")
					}
				}

			case msg := <-ch.msgQueue:
				ch.sendMessage(msg)
			}
		}
	})

	// Wait for error from either goroutine
	err := eg.Wait()
	if err != nil {
		if websocket.IsCloseError(err,
			websocket.CloseNormalClosure,
			websocket.CloseGoingAway,
			websocket.CloseAbnormalClosure) {
			ch.logger.Info().Msg("WebSocket closed normally")
		} else {
			ch.logger.Error().
				Err(err).
				Str("error_type", fmt.Sprintf("%T", err)).
				Msg("Error reading from WebSocket")

			if closeErr, ok := err.(*websocket.CloseError); ok {
				ch.logger.Error().
					Int("close_code", closeErr.Code).
					Str("close_text", closeErr.Text).
					Msg("WebSocket close error details")
			} else if strings.Contains(err.Error(), "continuation after FIN") {
				ch.logger.Error().
					Msg("WebSocket protocol error: received continuation frame after final frame")
			} else if strings.Contains(err.Error(), "unexpected continuation") {
				ch.logger.Error().
					Msg("WebSocket protocol error: unexpected continuation frame")
			}
		}
	}
	return err
}

// Close gracefully closes the connection
func (ch *connectionHandler) Close(ctx context.Context) error {
	ch.logger.Debug().Msg("Closing connection handler")
	if !ch.running.Load() {
		return nil
	}

	// Send close message if connection exists
	if ch.conn != nil {
		deadline := time.Now().Add(CloseTimeout)
		err := ch.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			deadline)
		if err != nil {
			ch.logger.Warn().Err(err).Msg("Failed to send close message")
		}
	}

	// We don't need to do much more here since context cancellation
	// will trigger cleanup in the handler goroutine

	return nil
}

// SendMessage queues a message to be sent
func (ch *connectionHandler) SendMessage(ctx context.Context, msg interface{}) error {
	if !ch.running.Load() {
		return errors.New("connection handler is not running")
	}

	ch.logger.Debug().Msg("Sending message")

	select {
	case ch.msgQueue <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full
		return errors.New("message queue is full")
	}
}

// sendMessage sends a JSON message over the WebSocket
// Only called from the handler goroutine, so no locking needed
func (ch *connectionHandler) sendMessage(msg interface{}) {
	ch.logger.Debug().Interface("msgType", fmt.Sprintf("%T", msg)).Msg("Processing message to send")
	if ch.conn == nil {
		ch.logger.Debug().Msg("No connection for sending message")
		return
	}

	// Handle special audio message case with base64 encoding
	if audioMsg, ok := msg.(AudioBufferAppendRequest); ok {
		// Special case for audio - use the more efficient struct
		type jsonAudioBufferAppend struct {
			Type  ClientEventType `json:"type"`
			Audio string          `json:"audio"`
		}

		// Convert directly rather than using a struct literal
		jsonMsg := jsonAudioBufferAppend(audioMsg)

		jsonBytes, _ := json.Marshal(jsonMsg)
		ch.logger.Debug().RawJSON("outgoing_json", jsonBytes).Msg("Sending JSON message")

		if err := ch.conn.WriteJSON(jsonMsg); err != nil {
			ch.logger.Error().Err(err).Msg("Failed to send audio message")
		}
		return
	}

	// For all other message types
	jsonBytes, _ := json.Marshal(msg)
	ch.logger.Debug().RawJSON("outgoing_json", jsonBytes).Msg("Sending JSON message")

	if err := ch.conn.WriteJSON(msg); err != nil {
		ch.logger.Error().Err(err).Msg("Failed to send message")
	}
}
