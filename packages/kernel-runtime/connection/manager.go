package connection

import (
	"context"
	"time"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// Manager manages connections
type Manager struct {
	connections map[string]*contracts.Connection
}

// New creates a new connection manager
func New() *Manager {
	return &Manager{connections: make(map[string]*contracts.Connection)}
}

// Create creates a new connection
func (m *Manager) Create(conn *contracts.Connection) error {
	if conn.ID == "" {
		return ErrMissingID
	}
	if _, exists := m.connections[conn.ID]; exists {
		return ErrDuplicate
	}
	m.connections[conn.ID] = conn
	return nil
}

// Get retrieves a connection by ID
func (m *Manager) Get(id string) (*contracts.Connection, bool) {
	conn, ok := m.connections[id]
	return conn, ok
}

// List returns all connections for a user
func (m *Manager) List(userID string) []*contracts.Connection {
	var result []*contracts.Connection
	for _, conn := range m.connections {
		// In a real implementation, filter by user ownership
		result = append(result, conn)
	}
	return result
}

// Update updates a connection
func (m *Manager) Update(conn *contracts.Connection) error {
	if _, ok := m.connections[conn.ID]; !ok {
		return ErrNotFound
	}
	m.connections[conn.ID] = conn
	return nil
}

// Delete removes a connection
func (m *Manager) Delete(id string) error {
	if _, ok := m.connections[id]; !ok {
		return ErrNotFound
	}
	delete(m.connections, id)
	return nil
}

// Test tests a connection
func (m *Manager) Test(ctx context.Context, id string) (*contracts.ConnectionStatus, error) {
	_, ok := m.connections[id]
	if !ok {
		return nil, ErrNotFound
	}
	// In a real implementation, delegate to the integration's ValidateConnection
	return &contracts.ConnectionStatus{
		Healthy:     true,
		LastChecked: time.Now(),
	}, nil
}
