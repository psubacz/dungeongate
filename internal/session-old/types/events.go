package types

import "time"

// EventType represents the type of event
type EventType string

const (
	EventTypeConnectionEstablished EventType = "connection_established"
	EventTypeConnectionClosed      EventType = "connection_closed"
	EventTypeSessionStarted        EventType = "session_started"
	EventTypeSessionEnded          EventType = "session_ended"
	EventTypeSpectatorJoined       EventType = "spectator_joined"
	EventTypeSpectatorLeft         EventType = "spectator_left"
	EventTypeTerminalResize        EventType = "terminal_resize"
	EventTypeError                 EventType = "error"
)

// Event represents a system event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// EventHandler handles events
type EventHandler interface {
	HandleEvent(event *Event) error
}

// EventBus manages event distribution
type EventBus interface {
	Publish(event *Event) error
	Subscribe(eventType EventType, handler EventHandler) error
	Unsubscribe(eventType EventType, handler EventHandler) error
}
