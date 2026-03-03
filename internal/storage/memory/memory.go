package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/abhishek/agentstore/internal/storage"
)

// Backend is a thread-safe in-memory storage implementation.
// It's useful for testing and for agents that don't need persistence.
type Backend struct {
	mu        sync.RWMutex
	sessions  map[string]*storage.SessionRecord
	events    map[string][]*storage.EventRecord // keyed by session ID
	snapshots map[string]*storage.SnapshotRecord // latest snapshot per session
	closed    bool
}

// New creates a new in-memory storage backend.
func New() *Backend {
	return &Backend{
		sessions:  make(map[string]*storage.SessionRecord),
		events:    make(map[string][]*storage.EventRecord),
		snapshots: make(map[string]*storage.SnapshotRecord),
	}
}

func (b *Backend) SaveSession(_ context.Context, session *storage.SessionRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("store is closed")
	}

	if _, exists := b.sessions[session.ID]; exists {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	// Deep copy to avoid external mutation
	s := *session
	if session.Labels != nil {
		s.Labels = make(map[string]string, len(session.Labels))
		for k, v := range session.Labels {
			s.Labels[k] = v
		}
	}
	b.sessions[s.ID] = &s
	return nil
}

func (b *Backend) GetSession(_ context.Context, id string) (*storage.SessionRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, fmt.Errorf("store is closed")
	}

	s, ok := b.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}

	// Return a copy
	out := *s
	return &out, nil
}

func (b *Backend) ListSessions(_ context.Context, limit, offset int) ([]*storage.SessionRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, fmt.Errorf("store is closed")
	}

	// Collect and sort by creation time (newest first)
	all := make([]*storage.SessionRecord, 0, len(b.sessions))
	for _, s := range b.sessions {
		cp := *s
		all = append(all, &cp)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	// Apply pagination
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (b *Backend) UpdateSession(_ context.Context, session *storage.SessionRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("store is closed")
	}

	if _, ok := b.sessions[session.ID]; !ok {
		return fmt.Errorf("session %s not found", session.ID)
	}

	s := *session
	b.sessions[s.ID] = &s
	return nil
}

func (b *Backend) AppendEvent(_ context.Context, event *storage.EventRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("store is closed")
	}

	// Verify session exists
	if _, ok := b.sessions[event.SessionID]; !ok {
		return fmt.Errorf("session %s not found", event.SessionID)
	}

	// Deep copy event
	e := *event
	payload := make([]byte, len(event.Payload))
	copy(payload, event.Payload)
	e.Payload = payload

	if event.Metadata != nil {
		meta := make([]byte, len(event.Metadata))
		copy(meta, event.Metadata)
		e.Metadata = meta
	}

	b.events[event.SessionID] = append(b.events[event.SessionID], &e)
	return nil
}

func (b *Backend) GetEvents(_ context.Context, sessionID string, fromSeq uint64) ([]*storage.EventRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, fmt.Errorf("store is closed")
	}

	events := b.events[sessionID]
	var result []*storage.EventRecord
	for _, e := range events {
		if e.SequenceNumber >= fromSeq {
			cp := *e
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (b *Backend) GetLatestSequence(_ context.Context, sessionID string) (uint64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return 0, fmt.Errorf("store is closed")
	}

	events := b.events[sessionID]
	if len(events) == 0 {
		return 0, nil
	}
	return events[len(events)-1].SequenceNumber, nil
}

func (b *Backend) SaveSnapshot(_ context.Context, snapshot *storage.SnapshotRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("store is closed")
	}

	s := *snapshot
	state := make([]byte, len(snapshot.State))
	copy(state, snapshot.State)
	s.State = state

	b.snapshots[snapshot.SessionID] = &s
	return nil
}

func (b *Backend) GetLatestSnapshot(_ context.Context, sessionID string) (*storage.SnapshotRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, fmt.Errorf("store is closed")
	}

	snap, ok := b.snapshots[sessionID]
	if !ok {
		return nil, nil // No snapshot is not an error
	}

	cp := *snap
	return &cp, nil
}

func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}
