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
	// Create a persistent store (data survives restarts)
	dataDir := "./agent-data"
	store, err := agentstore.New(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close store: %v", err)
		}
	}()
	// For in-memory (testing): agentstore.New("", agentstore.WithInMemory())

	ctx := context.Background()

	// Start a new agent session
	session, err := store.CreateSession(ctx,
		agentstore.WithSessionName("flight-search"),
		agentstore.WithLabels(map[string]string{"agent": "travel", "env": "demo"}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Session created: %s\n\n", session.ID)

	// 1. User sends a message
	e, err := agentstore.NewEvent(agentstore.EventUserMessage, map[string]string{
		"content": "Find flights from Berlin to Munich for next Friday",
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Append(ctx, session.ID, e); err != nil {
		log.Fatal(err)
	}

	// 2. Agent creates a plan
	e, err = agentstore.NewEvent(agentstore.EventPlanCreated, map[string]interface{}{
		"steps": []string{"search_flights", "filter_results", "present_options"},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Append(ctx, session.ID, e); err != nil {
		log.Fatal(err)
	}

	// 3. Agent calls a tool
	e, err = agentstore.NewEvent(agentstore.EventToolCalled, map[string]string{
		"tool": "search_flights",
		"args": `{"from":"BER","to":"MUC","date":"2026-03-13"}`,
	})
	if err != nil {
		log.Fatal(err)
	}
	e.WithMetadata(agentstore.Metadata{ToolName: "search_flights"})
	if err := store.Append(ctx, session.ID, e); err != nil {
		log.Fatal(err)
	}

	// 4. Tool returns results
	e, err = agentstore.NewEvent(agentstore.EventToolResult, map[string]interface{}{
		"flights": []map[string]string{
			{"airline": "Lufthansa", "flight": "LH1234", "price": "€89"},
			{"airline": "EuroWings", "flight": "EW456", "price": "€65"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	e.WithMetadata(agentstore.Metadata{ToolName: "search_flights", DurationMs: 1200})
	if err := store.Append(ctx, session.ID, e); err != nil {
		log.Fatal(err)
	}

	// 5. LLM processes results
	e, err = agentstore.NewEvent(agentstore.EventLLMRequest, map[string]string{
		"prompt": "Given these flights, recommend the best option...",
	})
	if err != nil {
		log.Fatal(err)
	}
	e.WithMetadata(agentstore.Metadata{Model: "gpt-4", TokensIn: 850})
	if err := store.Append(ctx, session.ID, e); err != nil {
		log.Fatal(err)
	}

	e, err = agentstore.NewEvent(agentstore.EventLLMResponse, map[string]string{
		"content": "I found 2 flights. The EuroWings EW456 at €65 is the best value.",
	})
	if err != nil {
		log.Fatal(err)
	}
	e.WithMetadata(agentstore.Metadata{Model: "gpt-4", TokensOut: 320, CostUSD: 0.018})
	if err := store.Append(ctx, session.ID, e); err != nil {
		log.Fatal(err)
	}

	// ── Replay the session ──────────────────────────────────────────────
	fmt.Println("=== Full Session Replay ===")
	events, err := store.Replay(ctx, session.ID)
	if err != nil {
		log.Fatal(err)
	}
	for _, ev := range events {
		var payload map[string]interface{}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			log.Printf("unmarshal payload: %v", err)
		}
		fmt.Printf("  [%d] %-16s %v\n", ev.SequenceNumber, ev.Type, payload)
	}

	// ── Replay only tool events ─────────────────────────────────────────
	fmt.Println("\n=== Tool Events Only ===")
	toolEvents, err := store.Replay(ctx, session.ID,
		agentstore.WithTypeFilter(agentstore.EventToolCalled, agentstore.EventToolResult),
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, ev := range toolEvents {
		fmt.Printf("  [%d] %-16s tool=%s duration=%dms\n",
			ev.SequenceNumber, ev.Type, ev.Metadata.ToolName, ev.Metadata.DurationMs)
	}

	// ── Get materialized state ──────────────────────────────────────────
	fmt.Println("\n=== Session State ===")
	state, err := store.GetState(ctx, session.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  Events:     %d\n", state.EventCount)
	fmt.Printf("  Tokens in:  %d\n", state.Tokens.In)
	fmt.Printf("  Tokens out: %d\n", state.Tokens.Out)
	fmt.Printf("  Cost:       $%.3f\n", state.Tokens.CostUSD)
	fmt.Printf("  Tool calls: %d\n", state.ToolCalls)
	fmt.Printf("  Errors:     %d\n", state.Errors)

	// ── Demonstrate persistence ─────────────────────────────────────────
	fmt.Printf("\nData persisted to %s/\n", dataDir)
	fmt.Println("Run 'agentstore sessions' or 'agentstore replay <id>' to inspect.")
}
