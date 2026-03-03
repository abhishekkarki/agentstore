package agentstore

import (
	"encoding/json"
	"time"
)

// State is the materialized view of a session, derived by reducing events.
// It is always reconstructable from the event log.
type State struct {
	// SessionID links this state to its session.
	SessionID string `json:"session_id"`

	// Version is the sequence number of the last applied event.
	Version uint64 `json:"version"`

	// Data holds the user-defined state as a flexible map.
	Data map[string]interface{} `json:"data"`

	// LastEventAt records when the most recent event occurred.
	LastEventAt time.Time `json:"last_event_at"`

	// EventCount is the total number of events applied to produce this state.
	EventCount uint64 `json:"event_count"`

	// Tokens tracks cumulative token usage across all LLM events.
	Tokens TokenUsage `json:"tokens"`

	// ToolCalls tracks the number of tool invocations.
	ToolCalls int `json:"tool_calls"`

	// Errors tracks the number of error events.
	Errors int `json:"errors"`
}

// TokenUsage aggregates token consumption and cost.
type TokenUsage struct {
	In      int     `json:"in"`
	Out     int     `json:"out"`
	CostUSD float64 `json:"cost_usd"`
}

// Reducer is a function that folds an event into the current state.
// Users can provide custom reducers to build domain-specific state.
type Reducer func(state *State, event *Event) *State

// Snapshot is a point-in-time capture of materialized state,
// used to speed up state reconstruction by avoiding full replay.
type Snapshot struct {
	SessionID      string    `json:"session_id"`
	Version        uint64    `json:"version"`
	State          *State    `json:"state"`
	CreatedAt      time.Time `json:"created_at"`
	EventCount     uint64    `json:"event_count"`
}

// DefaultReducer returns the built-in reducer that handles all agent event types.
// It accumulates token usage, tracks tool calls, and stores the latest payload
// in state.Data keyed by event type.
func DefaultReducer() Reducer {
	return func(state *State, event *Event) *State {
		if state.Data == nil {
			state.Data = make(map[string]interface{})
		}

		state.Version = event.SequenceNumber
		state.LastEventAt = event.Timestamp
		state.EventCount++

		// Accumulate token usage from LLM events
		if event.IsLLMEvent() {
			state.Tokens.In += event.Metadata.TokensIn
			state.Tokens.Out += event.Metadata.TokensOut
			state.Tokens.CostUSD += event.Metadata.CostUSD
		}

		// Count tool calls
		if event.Type == EventToolCalled {
			state.ToolCalls++
		}

		// Count errors
		if event.Type == EventError {
			state.Errors++
		}

		// Store the latest payload for each event type in Data.
		// This gives users quick access to the most recent value.
		var payload interface{}
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state.Data[string(event.Type)] = payload
		}

		return state
	}
}

// newState creates an empty state for the given session.
func newState(sessionID string) *State {
	return &State{
		SessionID: sessionID,
		Data:      make(map[string]interface{}),
	}
}

// applyEvents reduces a slice of events onto a state using the given reducer.
func applyEvents(state *State, events []*Event, reducer Reducer) *State {
	for _, event := range events {
		state = reducer(state, event)
	}
	return state
}
