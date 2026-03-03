package agentstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/abhishek/agentstore/internal/storage"
	"github.com/abhishek/agentstore/internal/storage/file"
	"github.com/abhishek/agentstore/internal/storage/memory"
)

// Store is the main interface for AgentStore.
// All methods are safe for concurrent use.
type Store interface {
	// CreateSession starts a new agent session.
	CreateSession(ctx context.Context, opts ...SessionOption) (*Session, error)

	// GetSession retrieves a session by ID.
	GetSession(ctx context.Context, id string) (*Session, error)

	// ListSessions returns sessions ordered by creation time (newest first).
	ListSessions(ctx context.Context, opts ...ListOption) ([]*Session, error)

	// Append adds an event to a session's log.
	// SequenceNumber and Timestamp are set automatically.
	Append(ctx context.Context, sessionID string, event *Event) error

	// GetEvents returns events for a session starting from the given sequence number.
	// Use fromSeq=0 to get all events.
	GetEvents(ctx context.Context, sessionID string, fromSeq uint64) ([]*Event, error)

	// GetState returns the current materialized state for a session.
	// State is reconstructed from the latest snapshot plus subsequent events.
	GetState(ctx context.Context, sessionID string) (*State, error)

	// Replay returns all events for a session in order, optionally filtered.
	Replay(ctx context.Context, sessionID string, opts ...ReplayOption) ([]*Event, error)

	// Close shuts down the store and releases resources.
	Close() error
}

// ReplayOption configures replay behavior.
type ReplayOption func(*replayConfig)

type replayConfig struct {
	types []EventType
	from  time.Time
	to    time.Time
}

// WithTypeFilter limits replay to specific event types.
func WithTypeFilter(types ...EventType) ReplayOption {
	return func(c *replayConfig) {
		c.types = types
	}
}

// WithTimeRange limits replay to events within a time range.
func WithTimeRange(from, to time.Time) ReplayOption {
	return func(c *replayConfig) {
		c.from = from
		c.to = to
	}
}

// store is the concrete implementation of Store.
type store struct {
	backend storage.Backend
	config  *storeConfig

	// mu protects sequence number assignment to ensure monotonicity.
	mu sync.Mutex
}

// New creates a new AgentStore.
//
// For persistent storage, provide a data directory path:
//
//	s, err := agentstore.New("/path/to/data")
//
// For in-memory (testing/ephemeral), use WithInMemory():
//
//	s, err := agentstore.New("", agentstore.WithInMemory())
func New(dataDir string, opts ...StoreOption) (Store, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var backend storage.Backend

	if cfg.inMemory || dataDir == "" {
		backend = memory.New()
	} else {
		fb, err := file.New(dataDir)
		if err != nil {
			return nil, fmt.Errorf("open file storage at %s: %w", dataDir, err)
		}
		backend = fb
	}

	return &store{
		backend: backend,
		config:  cfg,
	}, nil
}

func (s *store) CreateSession(ctx context.Context, opts ...SessionOption) (*Session, error) {
	session := newSession(opts...)

	record := &storage.SessionRecord{
		ID:        session.ID,
		Name:      session.Name,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
		Labels:    session.Labels,
	}

	if err := s.backend.SaveSession(ctx, record); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

func (s *store) GetSession(ctx context.Context, id string) (*Session, error) {
	record, err := s.backend.GetSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return recordToSession(record), nil
}

func (s *store) ListSessions(ctx context.Context, opts ...ListOption) ([]*Session, error) {
	cfg := defaultListConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	records, err := s.backend.ListSessions(ctx, cfg.limit, cfg.offset)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	sessions := make([]*Session, len(records))
	for i, r := range records {
		sessions[i] = recordToSession(r)
	}

	// Apply label filter if specified
	if cfg.label != "" {
		var filtered []*Session
		for _, sess := range sessions {
			if sess.Labels != nil && sess.Labels[cfg.label] == cfg.value {
				filtered = append(filtered, sess)
			}
		}
		sessions = filtered
	}

	return sessions, nil
}

func (s *store) Append(ctx context.Context, sessionID string, event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get the current sequence number for this session
	seq, err := s.backend.GetLatestSequence(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get sequence: %w", err)
	}

	nextSeq := seq + 1
	now := time.Now().UTC()

	// Set event fields that the store controls
	event.SessionID = sessionID
	event.SequenceNumber = nextSeq
	event.Timestamp = now

	// Marshal metadata
	metaJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	record := &storage.EventRecord{
		SessionID:      sessionID,
		SequenceNumber: nextSeq,
		Type:           string(event.Type),
		Payload:        event.Payload,
		Timestamp:      now,
		Metadata:       metaJSON,
	}

	if err := s.backend.AppendEvent(ctx, record); err != nil {
		return fmt.Errorf("append event: %w", err)
	}

	// Update session's UpdatedAt and EventCount
	sessionRecord, err := s.backend.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session for update: %w", err)
	}
	sessionRecord.UpdatedAt = now
	sessionRecord.EventCount = nextSeq
	if err := s.backend.UpdateSession(ctx, sessionRecord); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	// Auto-snapshot if configured
	if s.config.snapshotInterval > 0 && nextSeq%s.config.snapshotInterval == 0 {
		if err := s.createSnapshot(ctx, sessionID); err != nil {
			// Snapshot failure is non-fatal — log but don't fail the append
			_ = err
		}
	}

	return nil
}

func (s *store) GetEvents(ctx context.Context, sessionID string, fromSeq uint64) ([]*Event, error) {
	records, err := s.backend.GetEvents(ctx, sessionID, fromSeq)
	if err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}

	events := make([]*Event, len(records))
	for i, r := range records {
		events[i] = recordToEvent(r)
	}
	return events, nil
}

func (s *store) GetState(ctx context.Context, sessionID string) (*State, error) {
	state := newState(sessionID)
	var fromSeq uint64

	// Try to load from snapshot first
	snap, err := s.backend.GetLatestSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	if snap != nil {
		var snapState State
		if err := json.Unmarshal(snap.State, &snapState); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot: %w", err)
		}
		state = &snapState
		fromSeq = snap.Version + 1
	}

	// Apply events after the snapshot
	events, err := s.GetEvents(ctx, sessionID, fromSeq)
	if err != nil {
		return nil, fmt.Errorf("get events for state: %w", err)
	}

	state = applyEvents(state, events, s.config.reducer)
	return state, nil
}

func (s *store) Replay(ctx context.Context, sessionID string, opts ...ReplayOption) ([]*Event, error) {
	cfg := &replayConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	events, err := s.GetEvents(ctx, sessionID, 0)
	if err != nil {
		return nil, err
	}

	// Apply filters
	if len(cfg.types) > 0 || !cfg.from.IsZero() || !cfg.to.IsZero() {
		typeSet := make(map[EventType]bool, len(cfg.types))
		for _, t := range cfg.types {
			typeSet[t] = true
		}

		var filtered []*Event
		for _, e := range events {
			if len(typeSet) > 0 && !typeSet[e.Type] {
				continue
			}
			if !cfg.from.IsZero() && e.Timestamp.Before(cfg.from) {
				continue
			}
			if !cfg.to.IsZero() && e.Timestamp.After(cfg.to) {
				continue
			}
			filtered = append(filtered, e)
		}
		events = filtered
	}

	return events, nil
}

func (s *store) Close() error {
	return s.backend.Close()
}

// createSnapshot materializes state and saves a snapshot.
func (s *store) createSnapshot(ctx context.Context, sessionID string) error {
	state, err := s.GetState(ctx, sessionID)
	if err != nil {
		return err
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}

	snap := &storage.SnapshotRecord{
		SessionID:  sessionID,
		Version:    state.Version,
		State:      stateJSON,
		CreatedAt:  time.Now().UTC(),
		EventCount: state.EventCount,
	}

	return s.backend.SaveSnapshot(ctx, snap)
}

// recordToSession converts a storage record to the public Session type.
func recordToSession(r *storage.SessionRecord) *Session {
	return &Session{
		ID:         r.ID,
		Name:       r.Name,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
		EventCount: r.EventCount,
		Labels:     r.Labels,
	}
}

// recordToEvent converts a storage record to the public Event type.
func recordToEvent(r *storage.EventRecord) *Event {
	e := &Event{
		SessionID:      r.SessionID,
		SequenceNumber: r.SequenceNumber,
		Type:           EventType(r.Type),
		Payload:        r.Payload,
		Timestamp:      r.Timestamp,
	}

	if r.Metadata != nil {
		_ = json.Unmarshal(r.Metadata, &e.Metadata)
	}
	return e
}
