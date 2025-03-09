package websocket

import (
	"sync"

	"github.com/rs/zerolog/log"
)

// No need for an Upgrader type anymore as we're using the gorilla one directly

// Manager manages WebSocket connections
type Manager struct {
	// Channels for managing connections
	Register   chan *Connection
	Unregister chan *Connection
	Broadcast  chan []byte

	// Map of connection ID to connection
	connections      map[string]*Connection
	connectionsMutex sync.RWMutex

	// Message handler
	messageHandler func(*Connection, Message) error
}

// NewManager creates a new connection manager
func NewManager() *Manager {
	return &Manager{
		Register:     make(chan *Connection),
		Unregister:   make(chan *Connection),
		Broadcast:    make(chan []byte),
		connections:  make(map[string]*Connection),
	}
}

// Run starts the manager goroutine
func (m *Manager) Run() {
	for {
		select {
		case conn := <-m.Register:
			m.connectionsMutex.Lock()
			m.connections[conn.ID] = conn
			m.connectionsMutex.Unlock()
			log.Info().Str("conn_id", conn.ID).Msg("Client registered")
			
		case conn := <-m.Unregister:
			m.connectionsMutex.Lock()
			if _, ok := m.connections[conn.ID]; ok {
				delete(m.connections, conn.ID)
				close(conn.Send)
			}
			m.connectionsMutex.Unlock()
			log.Info().Str("conn_id", conn.ID).Msg("Client unregistered")
			
		case message := <-m.Broadcast:
			m.connectionsMutex.RLock()
			for _, conn := range m.connections {
				select {
				case conn.Send <- message:
				default:
					close(conn.Send)
					delete(m.connections, conn.ID)
				}
			}
			m.connectionsMutex.RUnlock()
		}
	}
}

// SendToClient sends a message to a specific client
func (m *Manager) SendToClient(id string, message []byte) {
	m.connectionsMutex.RLock()
	defer m.connectionsMutex.RUnlock()

	if conn, ok := m.connections[id]; ok {
		select {
		case conn.Send <- message:
		default:
			close(conn.Send)
			delete(m.connections, id)
		}
	}
}

// GetConnectionCount returns the number of active connections
func (m *Manager) GetConnectionCount() int {
	m.connectionsMutex.RLock()
	defer m.connectionsMutex.RUnlock()

	return len(m.connections)
}

// CloseAllConnections closes all connections
func (m *Manager) CloseAllConnections() {
	m.connectionsMutex.Lock()
	defer m.connectionsMutex.Unlock()

	for id, conn := range m.connections {
		close(conn.Send)
		delete(m.connections, id)
	}
	log.Info().Msg("All connections closed")
}