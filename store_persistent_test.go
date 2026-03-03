package agentstore_test

import (
	"context"
	"testing"

	"github.com/abhishek/agentstore"
)

func TestPersistentStore(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Write events in one store instance
	var sessionID string
	{
		s, err := agentstore.New(dir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		session, _ := s.CreateSession(ctx,
			agentstore.WithSessionName("persist-test"),
			agentstore.WithLabels(map[string]string{"env": "test"}),
		)
		sessionID = session.ID

		e1, _ := agentstore.NewEvent(agentstore.EventUserMessage, map[string]string{"content": "hello"})
		if err := s.Append(ctx, sessionID, e1); err != nil {
			t.Fatal(err)
		}

		e2, _ := agentstore.NewEvent(agentstore.EventLLMResponse, map[string]string{"content": "hi there"})
		e2.WithMetadata(agentstore.Metadata{Model: "gpt-4", TokensIn: 100, TokensOut: 50, CostUSD: 0.01})
		if err := s.Append(ctx, sessionID, e2); err != nil {
			t.Fatal(err)
		}

		e3, _ := agentstore.NewEvent(agentstore.EventToolCalled, map[string]string{"tool": "search"})
		e3.WithMetadata(agentstore.Metadata{ToolName: "search"})
		if err := s.Append(ctx, sessionID, e3); err != nil {
			t.Fatal(err)
		}

		s.Close()
	}

	// Reopen and verify everything persisted
	{
		s, err := agentstore.New(dir)
		if err != nil {
			t.Fatalf("New (reopen): %v", err)
		}
		defer s.Close()

		// Session persists
		session, err := s.GetSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetSession after reopen: %v", err)
		}
		if session.Name != "persist-test" {
			t.Fatalf("expected name 'persist-test', got %q", session.Name)
		}
		if session.Labels["env"] != "test" {
			t.Fatalf("expected label env=test, got %v", session.Labels)
		}
		if session.EventCount != 3 {
			t.Fatalf("expected 3 events, got %d", session.EventCount)
		}

		// Events persist
		events, err := s.GetEvents(ctx, sessionID, 0)
		if err != nil {
			t.Fatalf("GetEvents after reopen: %v", err)
		}
		if len(events) != 3 {
			t.Fatalf("expected 3 events, got %d", len(events))
		}
		if events[0].Type != agentstore.EventUserMessage {
			t.Fatalf("expected first event type user_message, got %s", events[0].Type)
		}
		if events[1].Metadata.Model != "gpt-4" {
			t.Fatalf("expected model gpt-4, got %s", events[1].Metadata.Model)
		}

		// State reconstructs correctly
		state, err := s.GetState(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetState after reopen: %v", err)
		}
		if state.EventCount != 3 {
			t.Fatalf("expected state.EventCount=3, got %d", state.EventCount)
		}
		if state.Tokens.In != 100 {
			t.Fatalf("expected 100 tokens in, got %d", state.Tokens.In)
		}
		if state.Tokens.Out != 50 {
			t.Fatalf("expected 50 tokens out, got %d", state.Tokens.Out)
		}
		if state.ToolCalls != 1 {
			t.Fatalf("expected 1 tool call, got %d", state.ToolCalls)
		}

		// Can append more events after reopen
		e4, _ := agentstore.NewEvent(agentstore.EventToolResult, map[string]string{"result": "found"})
		if err := s.Append(ctx, sessionID, e4); err != nil {
			t.Fatalf("Append after reopen: %v", err)
		}

		events, _ = s.GetEvents(ctx, sessionID, 0)
		if len(events) != 4 {
			t.Fatalf("expected 4 events after additional append, got %d", len(events))
		}
		if events[3].SequenceNumber != 4 {
			t.Fatalf("expected seq 4, got %d", events[3].SequenceNumber)
		}
	}
}

func TestPersistentStoreWithSnapshots(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	var sessionID string

	// Write 15 events with snapshot every 10
	{
		s, _ := agentstore.New(dir, agentstore.WithSnapshotInterval(10))

		session, _ := s.CreateSession(ctx)
		sessionID = session.ID

		for i := 0; i < 15; i++ {
			e, _ := agentstore.NewEvent(agentstore.EventLLMResponse, nil)
			e.WithMetadata(agentstore.Metadata{TokensIn: 10, TokensOut: 5, CostUSD: 0.001})
			s.Append(ctx, sessionID, e)
		}
		s.Close()
	}

	// Reopen and verify state reconstructs from snapshot + tail events
	{
		s, _ := agentstore.New(dir, agentstore.WithSnapshotInterval(10))
		defer s.Close()

		state, err := s.GetState(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetState with snapshot: %v", err)
		}

		if state.EventCount != 15 {
			t.Fatalf("expected 15 events, got %d", state.EventCount)
		}
		if state.Tokens.In != 150 {
			t.Fatalf("expected 150 tokens in, got %d", state.Tokens.In)
		}
		if state.Tokens.CostUSD < 0.014 || state.Tokens.CostUSD > 0.016 {
			t.Fatalf("expected cost ~$0.015, got %f", state.Tokens.CostUSD)
		}
	}
}

func TestPersistentStoreListSessions(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	{
		s, _ := agentstore.New(dir)
		if _, err := s.CreateSession(ctx, agentstore.WithSessionName("alpha")); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateSession(ctx, agentstore.WithSessionName("beta")); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateSession(ctx, agentstore.WithSessionName("gamma")); err != nil {
			t.Fatal(err)
		}
		s.Close()
	}

	{
		s, _ := agentstore.New(dir)
		defer s.Close()

		sessions, err := s.ListSessions(ctx)
		if err != nil {
			t.Fatalf("ListSessions: %v", err)
		}
		if len(sessions) != 3 {
			t.Fatalf("expected 3 sessions, got %d", len(sessions))
		}
	}
}
