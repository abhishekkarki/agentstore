package storage

import (
	"context"
	"encoding/json"
	"time"
)

// EventRecord is the internal storage representation of an event.
// It's separate from the public Event type to keep storage concerns isolated.
type EventRecord struct {
	SessionID      string          `json:"session_id"`
	SequenceNumber uint64          `json:"sequence_number"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	Timestamp      time.Time       `json:"timestamp"`
	Metadata       json.RawMessage `json:"metadata"`
}

// SessionRecord is the internal storage representation of a session.
type SessionRecord struct {
	ID         string            `json:"id"`
	Name       string            `json:"name,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	EventCount uint64            `json:"event_count"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// SnapshotRecord is the internal storage representation of a snapshot.
type SnapshotRecord struct {
	SessionID  string          `json:"session_id"`
	Version    uint64          `json:"version"`
	State      json.RawMessage `json:"state"`
	CreatedAt  time.Time       `json:"created_at"`
	EventCount uint64          `json:"event_count"`
}

// Backend defines the operations that any storage implementation must provide.
// Implementations must be safe for concurrent use.
type Backend interface {
	// Session operations
	SaveSession(ctx context.Context, session *SessionRecord) error
	GetSession(ctx context.Context, id string) (*SessionRecord, error)
	ListSessions(ctx context.Context, limit, offset int) ([]*SessionRecord, error)
	UpdateSession(ctx context.Context, session *SessionRecord) error

	// Event operations
	AppendEvent(ctx context.Context, event *EventRecord) error
	GetEvents(ctx context.Context, sessionID string, fromSeq uint64) ([]*EventRecord, error)
	GetLatestSequence(ctx context.Context, sessionID string) (uint64, error)

	// Snapshot operations
	SaveSnapshot(ctx context.Context, snapshot *SnapshotRecord) error
	GetLatestSnapshot(ctx context.Context, sessionID string) (*SnapshotRecord, error)

	// Lifecycle
	Close() error
}
