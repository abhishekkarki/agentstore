package agentstore

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Session represents a logical agent instance. All events belong to a session.
type Session struct {
	// ID uniquely identifies this session.
	ID string `json:"id"`

	// Name is an optional human-readable label.
	Name string `json:"name,omitempty"`

	// CreatedAt records when the session was created (UTC).
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt records when the last event was appended (UTC).
	UpdatedAt time.Time `json:"updated_at"`

	// EventCount tracks the total number of events in this session.
	EventCount uint64 `json:"event_count"`

	// Labels are user-defined key-value pairs for organization and filtering.
	Labels map[string]string `json:"labels,omitempty"`
}

// SessionOption configures session creation.
type SessionOption func(*Session)

// WithSessionID sets a custom session ID instead of generating one.
func WithSessionID(id string) SessionOption {
	return func(s *Session) {
		s.ID = id
	}
}

// WithSessionName sets a human-readable name for the session.
func WithSessionName(name string) SessionOption {
	return func(s *Session) {
		s.Name = name
	}
}

// WithLabels attaches key-value labels to the session.
func WithLabels(labels map[string]string) SessionOption {
	return func(s *Session) {
		s.Labels = labels
	}
}

// newID generates a UUID v4 string using crypto/rand.
// Zero external dependencies.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// newSession creates a session with the given options applied.
func newSession(opts ...SessionOption) *Session {
	now := time.Now().UTC()
	s := &Session{
		ID:        newID(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
