package types

import (
	"context"
	"net"
	"time"
)

// ConnectionState represents the state of a connection
type ConnectionState int

const (
	ConnectionStateConnected ConnectionState = iota
	ConnectionStateAuthenticated
	ConnectionStateActive
	ConnectionStateClosing
	ConnectionStateClosed
)

// Connection represents a client connection
type Connection struct {
	ID           string
	RemoteAddr   net.Addr
	UserID       string
	State        ConnectionState
	CreatedAt    time.Time
	LastActivity time.Time
}

// ConnectionStats represents connection statistics
type ConnectionStats struct {
	Active     int
	Total      int
	ByState    map[ConnectionState]int
	ByUserID   map[string]int
	ByRemoteIP map[string]int
}

// ConnectionHandler handles connection lifecycle
type ConnectionHandler interface {
	Handle(ctx context.Context) error
	Close() error
	GetConnection() *Connection
}
