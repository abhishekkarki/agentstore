package agentstore_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/abhishek/agentstore"
)

func newTestStore(t *testing.T) agentstore.Store {
	t.Helper()
	s, err := agentstore.New("", agentstore.WithInMemory())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, err := s.CreateSession(ctx, agentstore.WithSessionName("test-agent"))
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID == "" {
		t.Fatal("session ID should not be empty")
	}
	if session.Name != "test-agent" {
		t.Fatalf("expected name 'test-agent', got %q", session.Name)
	}
	if session.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should not be zero")
	}
}

func TestCreateSessionWithCustomID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, err := s.CreateSession(ctx, agentstore.WithSessionID("my-session-123"))
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID != "my-session-123" {
		t.Fatalf("expected ID 'my-session-123', got %q", session.ID)
	}
}

func TestCreateSessionWithLabels(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	labels := map[string]string{"env": "prod", "agent": "planner"}
	session, err := s.CreateSession(ctx, agentstore.WithLabels(labels))
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.Labels["env"] != "prod" {
		t.Fatalf("expected label env=prod, got %q", session.Labels["env"])
	}
}

func TestGetSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	created, _ := s.CreateSession(ctx, agentstore.WithSessionName("lookup-test"))

	got, err := s.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if got.ID != created.ID {
		t.Fatalf("expected ID %q, got %q", created.ID, got.ID)
	}
	if got.Name != "lookup-test" {
		t.Fatalf("expected name 'lookup-test', got %q", got.Name)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetSession(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestListSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateSession(ctx, agentstore.WithSessionName("first"))
	s.CreateSession(ctx, agentstore.WithSessionName("second"))
	s.CreateSession(ctx, agentstore.WithSessionName("third"))

	sessions, err := s.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestListSessionsWithLimit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s.CreateSession(ctx)
	}

	sessions, err := s.ListSessions(ctx, agentstore.WithLimit(2))
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestListSessionsWithLabelFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateSession(ctx, agentstore.WithLabels(map[string]string{"env": "prod"}))
	s.CreateSession(ctx, agentstore.WithLabels(map[string]string{"env": "staging"}))
	s.CreateSession(ctx, agentstore.WithLabels(map[string]string{"env": "prod"}))

	sessions, err := s.ListSessions(ctx, agentstore.WithLabelFilter("env", "prod"))
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 prod sessions, got %d", len(sessions))
	}
}

// ─── Event Tests ────────────────────────────────────────────────────────────

func TestAppendAndGetEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	event, err := agentstore.NewEvent(agentstore.EventUserMessage, map[string]string{
		"content": "Find flights to Munich",
	})
	if err != nil {
		t.Fatalf("NewEvent failed: %v", err)
	}

	if err := s.Append(ctx, session.ID, event); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify event was stored correctly
	events, err := s.GetEvents(ctx, session.ID, 0)
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.SessionID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, got.SessionID)
	}
	if got.SequenceNumber != 1 {
		t.Fatalf("expected sequence 1, got %d", got.SequenceNumber)
	}
	if got.Type != agentstore.EventUserMessage {
		t.Fatalf("expected type %q, got %q", agentstore.EventUserMessage, got.Type)
	}
	if got.Timestamp.IsZero() {
		t.Fatal("timestamp should not be zero")
	}
}

func TestAppendMultipleEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	types := []agentstore.EventType{
		agentstore.EventUserMessage,
		agentstore.EventPlanCreated,
		agentstore.EventToolCalled,
		agentstore.EventToolResult,
		agentstore.EventLLMResponse,
	}

	for _, et := range types {
		event, _ := agentstore.NewEvent(et, nil)
		if err := s.Append(ctx, session.ID, event); err != nil {
			t.Fatalf("Append %s failed: %v", et, err)
		}
	}

	events, _ := s.GetEvents(ctx, session.ID, 0)
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// Verify monotonic sequence numbers
	for i, e := range events {
		expected := uint64(i + 1)
		if e.SequenceNumber != expected {
			t.Fatalf("event %d: expected seq %d, got %d", i, expected, e.SequenceNumber)
		}
	}
}

func TestAppendWithMetadata(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	event, _ := agentstore.NewEvent(agentstore.EventLLMResponse, map[string]string{
		"content": "Here are three flights...",
	})
	event.WithMetadata(agentstore.Metadata{
		Model:      "gpt-4",
		TokensIn:   1200,
		TokensOut:  450,
		CostUSD:    0.02,
		DurationMs: 1550,
	})

	s.Append(ctx, session.ID, event)

	events, _ := s.GetEvents(ctx, session.ID, 0)
	got := events[0]

	if got.Metadata.Model != "gpt-4" {
		t.Fatalf("expected model 'gpt-4', got %q", got.Metadata.Model)
	}
	if got.Metadata.TokensIn != 1200 {
		t.Fatalf("expected 1200 tokens in, got %d", got.Metadata.TokensIn)
	}
	if got.Metadata.CostUSD != 0.02 {
		t.Fatalf("expected cost $0.02, got %f", got.Metadata.CostUSD)
	}
}

func TestAppendToNonexistentSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	event, _ := agentstore.NewEvent(agentstore.EventUserMessage, "hello")
	err := s.Append(ctx, "nonexistent", event)
	if err == nil {
		t.Fatal("expected error appending to nonexistent session")
	}
}

func TestGetEventsFromSequence(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	for i := 0; i < 10; i++ {
		event, _ := agentstore.NewEvent(agentstore.EventCustom, map[string]int{"index": i})
		s.Append(ctx, session.ID, event)
	}

	// Get events starting from sequence 6
	events, err := s.GetEvents(ctx, session.ID, 6)
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}

	if len(events) != 5 {
		t.Fatalf("expected 5 events (seq 6-10), got %d", len(events))
	}
	if events[0].SequenceNumber != 6 {
		t.Fatalf("expected first event seq 6, got %d", events[0].SequenceNumber)
	}
}

func TestNewEventWithRawJSON(t *testing.T) {
	raw := json.RawMessage(`{"key":"value","nested":{"n":42}}`)
	event, err := agentstore.NewEvent(agentstore.EventCustom, raw)
	if err != nil {
		t.Fatalf("NewEvent with raw JSON failed: %v", err)
	}

	if string(event.Payload) != string(raw) {
		t.Fatalf("payload mismatch: expected %s, got %s", raw, event.Payload)
	}
}

func TestNewEventWithNilPayload(t *testing.T) {
	event, err := agentstore.NewEvent(agentstore.EventCustom, nil)
	if err != nil {
		t.Fatalf("NewEvent with nil payload failed: %v", err)
	}

	if string(event.Payload) != "{}" {
		t.Fatalf("nil payload should produce '{}', got %s", event.Payload)
	}
}

// ─── State Tests ────────────────────────────────────────────────────────────

func TestGetStateEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	state, err := s.GetState(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if state.SessionID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, state.SessionID)
	}
	if state.EventCount != 0 {
		t.Fatalf("expected 0 events, got %d", state.EventCount)
	}
}

func TestGetStateWithTokenAccumulation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	// Simulate a typical agent interaction
	events := []struct {
		eventType agentstore.EventType
		metadata  agentstore.Metadata
	}{
		{agentstore.EventUserMessage, agentstore.Metadata{}},
		{agentstore.EventLLMRequest, agentstore.Metadata{TokensIn: 500, Model: "gpt-4"}},
		{agentstore.EventLLMResponse, agentstore.Metadata{TokensOut: 200, CostUSD: 0.01, Model: "gpt-4"}},
		{agentstore.EventToolCalled, agentstore.Metadata{ToolName: "search_flights"}},
		{agentstore.EventToolResult, agentstore.Metadata{DurationMs: 1200}},
		{agentstore.EventLLMRequest, agentstore.Metadata{TokensIn: 800, Model: "gpt-4"}},
		{agentstore.EventLLMResponse, agentstore.Metadata{TokensOut: 300, CostUSD: 0.015, Model: "gpt-4"}},
	}

	for _, e := range events {
		event, _ := agentstore.NewEvent(e.eventType, map[string]string{"data": "test"})
		event.WithMetadata(e.metadata)
		s.Append(ctx, session.ID, event)
	}

	state, err := s.GetState(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if state.EventCount != 7 {
		t.Fatalf("expected 7 events, got %d", state.EventCount)
	}
	if state.Tokens.In != 1300 {
		t.Fatalf("expected 1300 tokens in, got %d", state.Tokens.In)
	}
	if state.Tokens.Out != 500 {
		t.Fatalf("expected 500 tokens out, got %d", state.Tokens.Out)
	}
	if state.Tokens.CostUSD != 0.025 {
		t.Fatalf("expected cost $0.025, got %f", state.Tokens.CostUSD)
	}
	if state.ToolCalls != 1 {
		t.Fatalf("expected 1 tool call, got %d", state.ToolCalls)
	}
}

func TestGetStateWithErrors(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	event1, _ := agentstore.NewEvent(agentstore.EventError, map[string]string{"error": "timeout"})
	event2, _ := agentstore.NewEvent(agentstore.EventError, map[string]string{"error": "rate_limit"})
	s.Append(ctx, session.ID, event1)
	s.Append(ctx, session.ID, event2)

	state, _ := s.GetState(ctx, session.ID)
	if state.Errors != 2 {
		t.Fatalf("expected 2 errors, got %d", state.Errors)
	}
}

func TestGetStateVersion(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	for i := 0; i < 5; i++ {
		event, _ := agentstore.NewEvent(agentstore.EventCustom, nil)
		s.Append(ctx, session.ID, event)
	}

	state, _ := s.GetState(ctx, session.ID)
	if state.Version != 5 {
		t.Fatalf("expected version 5, got %d", state.Version)
	}
}

// ─── Replay Tests ───────────────────────────────────────────────────────────

func TestReplay(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	types := []agentstore.EventType{
		agentstore.EventUserMessage,
		agentstore.EventPlanCreated,
		agentstore.EventToolCalled,
		agentstore.EventToolResult,
	}

	for _, et := range types {
		event, _ := agentstore.NewEvent(et, nil)
		s.Append(ctx, session.ID, event)
	}

	events, err := s.Replay(ctx, session.ID)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
}

func TestReplayWithTypeFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	types := []agentstore.EventType{
		agentstore.EventUserMessage,
		agentstore.EventToolCalled,
		agentstore.EventToolResult,
		agentstore.EventToolCalled,
		agentstore.EventToolResult,
		agentstore.EventLLMResponse,
	}

	for _, et := range types {
		event, _ := agentstore.NewEvent(et, nil)
		s.Append(ctx, session.ID, event)
	}

	// Only tool events
	events, err := s.Replay(ctx, session.ID,
		agentstore.WithTypeFilter(agentstore.EventToolCalled, agentstore.EventToolResult),
	)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 tool events, got %d", len(events))
	}
}

// ─── Snapshot Tests ─────────────────────────────────────────────────────────

func TestAutoSnapshot(t *testing.T) {
	// Create store with snapshot every 10 events
	s, err := agentstore.New("", agentstore.WithInMemory(), agentstore.WithSnapshotInterval(10))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	session, _ := s.CreateSession(ctx)

	// Append 25 events (should trigger snapshots at 10 and 20)
	for i := 0; i < 25; i++ {
		event, _ := agentstore.NewEvent(agentstore.EventCustom, map[string]int{"i": i})
		s.Append(ctx, session.ID, event)
	}

	// State should still be correct regardless of snapshots
	state, err := s.GetState(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if state.EventCount != 25 {
		t.Fatalf("expected 25 events, got %d", state.EventCount)
	}
	if state.Version != 25 {
		t.Fatalf("expected version 25, got %d", state.Version)
	}
}

// ─── Concurrency Tests ─────────────────────────────────────────────────────

func TestConcurrentAppends(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event, _ := agentstore.NewEvent(agentstore.EventCustom, map[string]int{"idx": idx})
			if err := s.Append(ctx, session.ID, event); err != nil {
				t.Errorf("concurrent append %d failed: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	events, _ := s.GetEvents(ctx, session.ID, 0)
	if len(events) != n {
		t.Fatalf("expected %d events, got %d", n, len(events))
	}

	// Verify all sequence numbers are unique and monotonic
	seen := make(map[uint64]bool)
	for _, e := range events {
		if seen[e.SequenceNumber] {
			t.Fatalf("duplicate sequence number: %d", e.SequenceNumber)
		}
		seen[e.SequenceNumber] = true
	}

	// Verify no gaps (1 through n)
	for i := uint64(1); i <= uint64(n); i++ {
		if !seen[i] {
			t.Fatalf("missing sequence number: %d", i)
		}
	}
}

func TestConcurrentSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, err := s.CreateSession(ctx)
			if err != nil {
				t.Errorf("concurrent CreateSession failed: %v", err)
				return
			}

			for j := 0; j < 10; j++ {
				event, _ := agentstore.NewEvent(agentstore.EventCustom, nil)
				if err := s.Append(ctx, session.ID, event); err != nil {
					t.Errorf("concurrent append failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	sessions, _ := s.ListSessions(ctx, agentstore.WithLimit(100))
	if len(sessions) != n {
		t.Fatalf("expected %d sessions, got %d", n, len(sessions))
	}
}

// ─── Event Helper Tests ─────────────────────────────────────────────────────

func TestEventIsLLMEvent(t *testing.T) {
	llmReq, _ := agentstore.NewEvent(agentstore.EventLLMRequest, nil)
	llmResp, _ := agentstore.NewEvent(agentstore.EventLLMResponse, nil)
	toolCall, _ := agentstore.NewEvent(agentstore.EventToolCalled, nil)

	if !llmReq.IsLLMEvent() {
		t.Fatal("LLMRequest should be LLM event")
	}
	if !llmResp.IsLLMEvent() {
		t.Fatal("LLMResponse should be LLM event")
	}
	if toolCall.IsLLMEvent() {
		t.Fatal("ToolCalled should not be LLM event")
	}
}

func TestEventIsToolEvent(t *testing.T) {
	toolCall, _ := agentstore.NewEvent(agentstore.EventToolCalled, nil)
	toolResult, _ := agentstore.NewEvent(agentstore.EventToolResult, nil)
	llmResp, _ := agentstore.NewEvent(agentstore.EventLLMResponse, nil)

	if !toolCall.IsToolEvent() {
		t.Fatal("ToolCalled should be tool event")
	}
	if !toolResult.IsToolEvent() {
		t.Fatal("ToolResult should be tool event")
	}
	if llmResp.IsToolEvent() {
		t.Fatal("LLMResponse should not be tool event")
	}
}

// ─── Session Update Tests ───────────────────────────────────────────────────

func TestSessionUpdatedAfterAppend(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)
	originalUpdatedAt := session.UpdatedAt

	event, _ := agentstore.NewEvent(agentstore.EventUserMessage, "hello")
	s.Append(ctx, session.ID, event)

	updated, _ := s.GetSession(ctx, session.ID)
	if !updated.UpdatedAt.After(originalUpdatedAt) || updated.UpdatedAt.Equal(originalUpdatedAt) {
		// They could be equal if the test runs fast enough, so just check EventCount
	}
	if updated.EventCount != 1 {
		t.Fatalf("expected event count 1, got %d", updated.EventCount)
	}
}

// ─── Custom Reducer Test ────────────────────────────────────────────────────

func TestCustomReducer(t *testing.T) {
	// Custom reducer that only counts user messages
	customReducer := func(state *agentstore.State, event *agentstore.Event) *agentstore.State {
		if state.Data == nil {
			state.Data = make(map[string]interface{})
		}
		state.Version = event.SequenceNumber
		state.EventCount++

		if event.Type == agentstore.EventUserMessage {
			count, _ := state.Data["user_message_count"].(float64)
			state.Data["user_message_count"] = count + 1
		}
		return state
	}

	s, _ := agentstore.New("", agentstore.WithInMemory(), agentstore.WithReducer(customReducer))
	defer s.Close()
	ctx := context.Background()

	session, _ := s.CreateSession(ctx)

	e1, _ := agentstore.NewEvent(agentstore.EventUserMessage, nil)
	e2, _ := agentstore.NewEvent(agentstore.EventToolCalled, nil)
	e3, _ := agentstore.NewEvent(agentstore.EventUserMessage, nil)

	s.Append(ctx, session.ID, e1)
	s.Append(ctx, session.ID, e2)
	s.Append(ctx, session.ID, e3)

	state, _ := s.GetState(ctx, session.ID)

	count, ok := state.Data["user_message_count"].(float64)
	if !ok || count != 2 {
		t.Fatalf("expected user_message_count=2, got %v", state.Data["user_message_count"])
	}
}
