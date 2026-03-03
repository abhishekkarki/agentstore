// Example: A simple agent that searches for flights using AgentStore
// for durable, replayable state management.
//
// Run: go run main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/abhishek/agentstore"
)

func main() {
	// Create an in-memory store (use a path for persistent storage)
	store, err := agentstore.New("", agentstore.WithInMemory())
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// Start a new agent session
	session, _ := store.CreateSession(ctx,
		agentstore.WithSessionName("flight-search"),
		agentstore.WithLabels(map[string]string{"agent": "travel", "env": "demo"}),
	)
	fmt.Printf("Session created: %s\n\n", session.ID)

	// 1. User sends a message
	e, _ := agentstore.NewEvent(agentstore.EventUserMessage, map[string]string{
		"content": "Find flights from Berlin to Munich for next Friday",
	})
	store.Append(ctx, session.ID, e)

	// 2. Agent creates a plan
	e, _ = agentstore.NewEvent(agentstore.EventPlanCreated, map[string]interface{}{
		"steps": []string{"search_flights", "filter_results", "present_options"},
	})
	store.Append(ctx, session.ID, e)

	// 3. Agent calls a tool
	e, _ = agentstore.NewEvent(agentstore.EventToolCalled, map[string]string{
		"tool": "search_flights",
		"args": `{"from":"BER","to":"MUC","date":"2026-03-13"}`,
	})
	e.WithMetadata(agentstore.Metadata{ToolName: "search_flights"})
	store.Append(ctx, session.ID, e)

	// 4. Tool returns results
	e, _ = agentstore.NewEvent(agentstore.EventToolResult, map[string]interface{}{
		"flights": []map[string]string{
			{"airline": "Lufthansa", "flight": "LH1234", "price": "€89"},
			{"airline": "EuroWings", "flight": "EW456", "price": "€65"},
		},
	})
	e.WithMetadata(agentstore.Metadata{ToolName: "search_flights", DurationMs: 1200})
	store.Append(ctx, session.ID, e)

	// 5. LLM processes results
	e, _ = agentstore.NewEvent(agentstore.EventLLMRequest, map[string]string{
		"prompt": "Given these flights, recommend the best option...",
	})
	e.WithMetadata(agentstore.Metadata{Model: "gpt-4", TokensIn: 850})
	store.Append(ctx, session.ID, e)

	e, _ = agentstore.NewEvent(agentstore.EventLLMResponse, map[string]string{
		"content": "I found 2 flights. The EuroWings EW456 at €65 is the best value.",
	})
	e.WithMetadata(agentstore.Metadata{Model: "gpt-4", TokensOut: 320, CostUSD: 0.018})
	store.Append(ctx, session.ID, e)

	// ── Replay the session ──────────────────────────────────────────────
	fmt.Println("=== Full Session Replay ===")
	events, _ := store.Replay(ctx, session.ID)
	for _, ev := range events {
		var payload map[string]interface{}
		json.Unmarshal(ev.Payload, &payload)
		fmt.Printf("  [%d] %-16s %v\n", ev.SequenceNumber, ev.Type, payload)
	}

	// ── Replay only tool events ─────────────────────────────────────────
	fmt.Println("\n=== Tool Events Only ===")
	toolEvents, _ := store.Replay(ctx, session.ID,
		agentstore.WithTypeFilter(agentstore.EventToolCalled, agentstore.EventToolResult),
	)
	for _, ev := range toolEvents {
		fmt.Printf("  [%d] %-16s tool=%s duration=%dms\n",
			ev.SequenceNumber, ev.Type, ev.Metadata.ToolName, ev.Metadata.DurationMs)
	}

	// ── Get materialized state ──────────────────────────────────────────
	fmt.Println("\n=== Session State ===")
	state, _ := store.GetState(ctx, session.ID)
	fmt.Printf("  Events:     %d\n", state.EventCount)
	fmt.Printf("  Tokens in:  %d\n", state.Tokens.In)
	fmt.Printf("  Tokens out: %d\n", state.Tokens.Out)
	fmt.Printf("  Cost:       $%.3f\n", state.Tokens.CostUSD)
	fmt.Printf("  Tool calls: %d\n", state.ToolCalls)
	fmt.Printf("  Errors:     %d\n", state.Errors)
}
