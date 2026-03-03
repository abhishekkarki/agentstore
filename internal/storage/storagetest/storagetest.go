package storagetest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/abhishek/agentstore/internal/storage"
)

// BackendFactory creates a new backend instance for testing.
type BackendFactory func(t *testing.T) storage.Backend

// RunAll runs the full backend test suite against the given factory.
func RunAll(t *testing.T, factory BackendFactory) {
	t.Run("SaveAndGetSession", func(t *testing.T) { testSaveAndGetSession(t, factory) })
	t.Run("SaveSessionDuplicate", func(t *testing.T) { testSaveSessionDuplicate(t, factory) })
	t.Run("GetSessionNotFound", func(t *testing.T) { testGetSessionNotFound(t, factory) })
	t.Run("ListSessions", func(t *testing.T) { testListSessions(t, factory) })
	t.Run("ListSessionsPagination", func(t *testing.T) { testListSessionsPagination(t, factory) })
	t.Run("UpdateSession", func(t *testing.T) { testUpdateSession(t, factory) })
	t.Run("AppendAndGetEvents", func(t *testing.T) { testAppendAndGetEvents(t, factory) })
	t.Run("AppendToMissingSession", func(t *testing.T) { testAppendToMissingSession(t, factory) })
	t.Run("GetEventsFromSequence", func(t *testing.T) { testGetEventsFromSequence(t, factory) })
	t.Run("GetEventsEmpty", func(t *testing.T) { testGetEventsEmpty(t, factory) })
	t.Run("GetLatestSequence", func(t *testing.T) { testGetLatestSequence(t, factory) })
	t.Run("GetLatestSequenceEmpty", func(t *testing.T) { testGetLatestSequenceEmpty(t, factory) })
	t.Run("SaveAndGetSnapshot", func(t *testing.T) { testSaveAndGetSnapshot(t, factory) })
	t.Run("GetSnapshotEmpty", func(t *testing.T) { testGetSnapshotEmpty(t, factory) })
	t.Run("SnapshotOverwrite", func(t *testing.T) { testSnapshotOverwrite(t, factory) })
	t.Run("ClosePreventsFurtherOps", func(t *testing.T) { testClosePreventsFurtherOps(t, factory) })
}

func makeSession(id string) *storage.SessionRecord {
	now := time.Now().UTC()
	return &storage.SessionRecord{
		ID:        id,
		Name:      "test-" + id,
		CreatedAt: now,
		UpdatedAt: now,
		Labels:    map[string]string{"env": "test"},
	}
}

func makeEvent(sessionID string, seq uint64) *storage.EventRecord {
	payload, _ := json.Marshal(map[string]interface{}{"seq": seq})
	meta, _ := json.Marshal(map[string]interface{}{"worker": "w1"})
	return &storage.EventRecord{
		SessionID:      sessionID,
		SequenceNumber: seq,
		Type:           "custom",
		Payload:        payload,
		Timestamp:      time.Now().UTC(),
		Metadata:       meta,
	}
}

func testSaveAndGetSession(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	s := makeSession("sess-1")
	if err := b.SaveSession(ctx, s); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	got, err := b.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != "sess-1" {
		t.Fatalf("expected ID sess-1, got %s", got.ID)
	}
	if got.Name != "test-sess-1" {
		t.Fatalf("expected name test-sess-1, got %s", got.Name)
	}
	if got.Labels["env"] != "test" {
		t.Fatalf("expected label env=test, got %v", got.Labels)
	}
}

func testSaveSessionDuplicate(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	s := makeSession("dup-1")
	if err := b.SaveSession(ctx, s); err != nil {
		t.Fatalf("first SaveSession: %v", err)
	}

	err := b.SaveSession(ctx, s)
	if err == nil {
		t.Fatal("expected error on duplicate session")
	}
}

func testGetSessionNotFound(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()

	_, err := b.GetSession(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func testListSessions(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s := makeSession(fmt.Sprintf("list-%d", i))
		// Stagger creation times
		s.CreatedAt = time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		if err := b.SaveSession(ctx, s); err != nil {
			t.Fatalf("SaveSession list-%d: %v", i, err)
		}
	}

	sessions, err := b.ListSessions(ctx, 100, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 5 {
		t.Fatalf("expected 5 sessions, got %d", len(sessions))
	}
}

func testListSessionsPagination(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := b.SaveSession(ctx, makeSession(fmt.Sprintf("page-%d", i))); err != nil {
			t.Fatalf("SaveSession page-%d: %v", i, err)
		}
	}

	sessions, _ := b.ListSessions(ctx, 2, 0)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions with limit=2, got %d", len(sessions))
	}

	sessions, _ = b.ListSessions(ctx, 100, 10)
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions with offset=10, got %d", len(sessions))
	}
}

func testUpdateSession(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	s := makeSession("upd-1")
	if err := b.SaveSession(ctx, s); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	s.EventCount = 42
	s.Name = "updated"
	if err := b.UpdateSession(ctx, s); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	got, _ := b.GetSession(ctx, "upd-1")
	if got.EventCount != 42 {
		t.Fatalf("expected EventCount 42, got %d", got.EventCount)
	}
	if got.Name != "updated" {
		t.Fatalf("expected name 'updated', got %s", got.Name)
	}
}

func testAppendAndGetEvents(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	if err := b.SaveSession(ctx, makeSession("ev-1")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	for i := uint64(1); i <= 5; i++ {
		if err := b.AppendEvent(ctx, makeEvent("ev-1", i)); err != nil {
			t.Fatalf("AppendEvent seq=%d: %v", i, err)
		}
	}

	events, err := b.GetEvents(ctx, "ev-1", 0)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// Verify ordering
	for i, e := range events {
		expected := uint64(i + 1)
		if e.SequenceNumber != expected {
			t.Fatalf("event %d: expected seq %d, got %d", i, expected, e.SequenceNumber)
		}
		if e.SessionID != "ev-1" {
			t.Fatalf("event %d: wrong session ID %s", i, e.SessionID)
		}
	}
}

func testAppendToMissingSession(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()

	err := b.AppendEvent(context.Background(), makeEvent("missing", 1))
	if err == nil {
		t.Fatal("expected error appending to missing session")
	}
}

func testGetEventsFromSequence(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	if err := b.SaveSession(ctx, makeSession("seq-1")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	for i := uint64(1); i <= 10; i++ {
		if err := b.AppendEvent(ctx, makeEvent("seq-1", i)); err != nil {
			t.Fatalf("AppendEvent seq=%d: %v", i, err)
		}
	}

	events, _ := b.GetEvents(ctx, "seq-1", 7)
	if len(events) != 4 {
		t.Fatalf("expected 4 events from seq 7, got %d", len(events))
	}
	if events[0].SequenceNumber != 7 {
		t.Fatalf("expected first event seq=7, got %d", events[0].SequenceNumber)
	}
}

func testGetEventsEmpty(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	if err := b.SaveSession(ctx, makeSession("empty-1")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	events, err := b.GetEvents(ctx, "empty-1", 0)
	if err != nil {
		t.Fatalf("GetEvents on empty session: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func testGetLatestSequence(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	if err := b.SaveSession(ctx, makeSession("lseq-1")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	for i := uint64(1); i <= 3; i++ {
		if err := b.AppendEvent(ctx, makeEvent("lseq-1", i)); err != nil {
			t.Fatalf("AppendEvent seq=%d: %v", i, err)
		}
	}

	seq, err := b.GetLatestSequence(ctx, "lseq-1")
	if err != nil {
		t.Fatalf("GetLatestSequence: %v", err)
	}
	if seq != 3 {
		t.Fatalf("expected seq 3, got %d", seq)
	}
}

func testGetLatestSequenceEmpty(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()

	seq, err := b.GetLatestSequence(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetLatestSequence on empty: %v", err)
	}
	if seq != 0 {
		t.Fatalf("expected seq 0, got %d", seq)
	}
}

func testSaveAndGetSnapshot(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	state, _ := json.Marshal(map[string]interface{}{"tokens": 500})
	snap := &storage.SnapshotRecord{
		SessionID:  "snap-1",
		Version:    10,
		State:      state,
		CreatedAt:  time.Now().UTC(),
		EventCount: 10,
	}

	if err := b.SaveSnapshot(ctx, snap); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	got, err := b.GetLatestSnapshot(ctx, "snap-1")
	if err != nil {
		t.Fatalf("GetLatestSnapshot: %v", err)
	}
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if got.Version != 10 {
		t.Fatalf("expected version 10, got %d", got.Version)
	}
	if got.EventCount != 10 {
		t.Fatalf("expected EventCount 10, got %d", got.EventCount)
	}
}

func testGetSnapshotEmpty(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()

	snap, err := b.GetLatestSnapshot(context.Background(), "no-snap")
	if err != nil {
		t.Fatalf("GetLatestSnapshot on empty: %v", err)
	}
	if snap != nil {
		t.Fatal("expected nil snapshot")
	}
}

func testSnapshotOverwrite(t *testing.T, factory BackendFactory) {
	b := factory(t)
	defer func() { _ = b.Close() }()
	ctx := context.Background()

	state1, _ := json.Marshal(map[string]interface{}{"v": 1})
	snap1 := &storage.SnapshotRecord{SessionID: "ow-1", Version: 5, State: state1, CreatedAt: time.Now().UTC(), EventCount: 5}
	if err := b.SaveSnapshot(ctx, snap1); err != nil {
		t.Fatalf("SaveSnapshot snap1: %v", err)
	}

	state2, _ := json.Marshal(map[string]interface{}{"v": 2})
	snap2 := &storage.SnapshotRecord{SessionID: "ow-1", Version: 10, State: state2, CreatedAt: time.Now().UTC(), EventCount: 10}
	if err := b.SaveSnapshot(ctx, snap2); err != nil {
		t.Fatalf("SaveSnapshot snap2: %v", err)
	}

	got, _ := b.GetLatestSnapshot(ctx, "ow-1")
	if got.Version != 10 {
		t.Fatalf("expected version 10 after overwrite, got %d", got.Version)
	}
}

func testClosePreventsFurtherOps(t *testing.T, factory BackendFactory) {
	b := factory(t)
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err := b.GetSession(context.Background(), "any")
	if err == nil {
		t.Fatal("expected error after close")
	}
}
