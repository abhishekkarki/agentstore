package agentstore

import (
	"encoding/json"
	"time"
)

// EventType represents agent-native event categories.
// These are first-class types, not generic strings — this is what
// differentiates AgentStore from generic event sourcing libraries.
type EventType string

const (
	// Core agent lifecycle events
	EventUserMessage  EventType = "user_message"
	EventPlanCreated  EventType = "plan_created"
	EventToolCalled   EventType = "tool_called"
	EventToolResult   EventType = "tool_result"
	EventLLMRequest   EventType = "llm_request"
	EventLLMResponse  EventType = "llm_response"
	EventStateUpdated EventType = "state_updated"
	EventError        EventType = "error"

	// Extensibility — users can define their own event types
	EventCustom EventType = "custom"
)

// Event is an immutable record of an action or state change within an agent session.
// Events are append-only and form the source of truth for session state.
type Event struct {
	// SessionID links this event to its agent session.
	SessionID string `json:"session_id"`

	// SequenceNumber is the monotonically increasing position within the session.
	// Assigned by the store on append — callers should leave this as zero.
	SequenceNumber uint64 `json:"sequence_number"`

	// Type categorizes the event using agent-native semantics.
	Type EventType `json:"type"`

	// Payload carries the event-specific data as arbitrary JSON.
	Payload json.RawMessage `json:"payload"`

	// Timestamp records when the event occurred (UTC).
	Timestamp time.Time `json:"timestamp"`

	// Metadata holds optional observability data (tokens, cost, model, etc.).
	Metadata Metadata `json:"metadata,omitempty"`
}

// Metadata captures agent-specific observability information.
// All fields are optional — populate what you have.
type Metadata struct {
	// WorkerID identifies which worker/process produced this event.
	WorkerID string `json:"worker_id,omitempty"`

	// Token usage for LLM events.
	TokensIn  int `json:"tokens_in,omitempty"`
	TokensOut int `json:"tokens_out,omitempty"`

	// Model name (e.g., "gpt-4", "claude-3-opus").
	Model string `json:"model,omitempty"`

	// ToolName for tool_called and tool_result events.
	ToolName string `json:"tool_name,omitempty"`

	// DurationMs records how long the operation took.
	DurationMs int64 `json:"duration_ms,omitempty"`

	// CostUSD tracks the estimated cost of LLM calls.
	CostUSD float64 `json:"cost_usd,omitempty"`

	// Extra allows arbitrary key-value pairs for user-defined metadata.
	Extra map[string]string `json:"extra,omitempty"`
}

// NewEvent creates an event with the given type and payload.
// SequenceNumber and Timestamp are set automatically by the store on append.
// The payload is marshaled to JSON if it isn't already a json.RawMessage.
func NewEvent(eventType EventType, payload interface{}) (*Event, error) {
	var raw json.RawMessage

	switch p := payload.(type) {
	case json.RawMessage:
		raw = p
	case []byte:
		raw = json.RawMessage(p)
	case nil:
		raw = json.RawMessage(`{}`)
	default:
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = data
	}

	return &Event{
		Type:    eventType,
		Payload: raw,
	}, nil
}

// WithMetadata sets metadata on an event and returns it for chaining.
func (e *Event) WithMetadata(m Metadata) *Event {
	e.Metadata = m
	return e
}

// IsLLMEvent returns true if the event is an LLM request or response.
func (e *Event) IsLLMEvent() bool {
	return e.Type == EventLLMRequest || e.Type == EventLLMResponse
}

// IsToolEvent returns true if the event is a tool call or result.
func (e *Event) IsToolEvent() bool {
	return e.Type == EventToolCalled || e.Type == EventToolResult
}
