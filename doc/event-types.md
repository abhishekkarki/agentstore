# Event Types

AgentStore defines first-class event types that map directly to the lifecycle of an AI agent. Each type has a recommended payload schema and metadata fields.

## Built-in types

| Constant | String value | Description |
|----------|-------------|-------------|
| `EventUserMessage` | `user_message` | Input received from the user |
| `EventPlanCreated` | `plan_created` | Agent created an execution plan |
| `EventToolCalled` | `tool_called` | Agent invoked a tool |
| `EventToolResult` | `tool_result` | Tool returned a result |
| `EventLLMRequest` | `llm_request` | Request sent to an LLM API |
| `EventLLMResponse` | `llm_response` | Response received from an LLM API |
| `EventStateUpdated` | `state_updated` | Explicit state mutation |
| `EventError` | `error` | An error occurred |
| `EventCustom` | `custom` | User-defined event |

---

## `user_message`

Represents input from the user that triggers agent work.

**Recommended payload:**
```json
{
  "content": "Find flights from Berlin to Munich for next Friday",
  "role": "user"
}
```

**Example:**
```go
ev, _ := agentstore.NewEvent(agentstore.EventUserMessage, map[string]string{
    "content": "Find flights from Berlin to Munich",
    "role":    "user",
})
store.Append(ctx, sessionID, ev)
```

---

## `plan_created`

Records the agent's execution plan before it begins working.

**Recommended payload:**
```json
{
  "steps": ["search_flights", "filter_results", "present_options"],
  "goal": "find cheapest flight"
}
```

**Example:**
```go
ev, _ := agentstore.NewEvent(agentstore.EventPlanCreated, map[string]interface{}{
    "steps": []string{"search_flights", "filter_results", "present_options"},
    "goal":  "find cheapest flight",
})
store.Append(ctx, sessionID, ev)
```

---

## `tool_called`

Records a tool invocation before it executes.

**Recommended payload:**
```json
{
  "tool": "search_flights",
  "args": {"from": "BER", "to": "MUC", "date": "2026-03-13"}
}
```

**Recommended metadata:**
```go
agentstore.Metadata{
    ToolName: "search_flights",
}
```

**Example:**
```go
ev, _ := agentstore.NewEvent(agentstore.EventToolCalled, map[string]interface{}{
    "tool": "search_flights",
    "args": map[string]string{"from": "BER", "to": "MUC"},
})
ev.WithMetadata(agentstore.Metadata{ToolName: "search_flights"})
store.Append(ctx, sessionID, ev)
```

The default reducer increments `state.ToolCalls` for each `tool_called` event.

---

## `tool_result`

Records the result returned by a tool.

**Recommended payload:**
```json
{
  "result": [...],
  "error": null
}
```

**Recommended metadata:**
```go
agentstore.Metadata{
    ToolName:   "search_flights",
    DurationMs: 1200,
}
```

**Example:**
```go
ev, _ := agentstore.NewEvent(agentstore.EventToolResult, map[string]interface{}{
    "result": flights,
})
ev.WithMetadata(agentstore.Metadata{
    ToolName:   "search_flights",
    DurationMs: 1200,
})
store.Append(ctx, sessionID, ev)
```

---

## `llm_request`

Records a request sent to an LLM API.

**Recommended payload:**
```json
{
  "prompt": "Given these flights, recommend the best option...",
  "messages": [...]
}
```

**Recommended metadata:**
```go
agentstore.Metadata{
    Model:    "gpt-4",
    TokensIn: 850,
}
```

**Example:**
```go
ev, _ := agentstore.NewEvent(agentstore.EventLLMRequest, map[string]string{
    "prompt": "Given these flights, recommend the best option...",
})
ev.WithMetadata(agentstore.Metadata{
    Model:    "claude-sonnet-4-6",
    TokensIn: 850,
})
store.Append(ctx, sessionID, ev)
```

The default reducer accumulates `TokensIn` and `TokensOut` from all LLM events into `state.Tokens`.

---

## `llm_response`

Records the response received from an LLM API.

**Recommended payload:**
```json
{
  "content": "I recommend the EuroWings flight at €65...",
  "finish_reason": "stop"
}
```

**Recommended metadata:**
```go
agentstore.Metadata{
    Model:     "gpt-4",
    TokensOut: 320,
    CostUSD:   0.018,
}
```

**Example:**
```go
ev, _ := agentstore.NewEvent(agentstore.EventLLMResponse, map[string]string{
    "content":       "I recommend the EuroWings flight at €65.",
    "finish_reason": "stop",
})
ev.WithMetadata(agentstore.Metadata{
    Model:     "claude-sonnet-4-6",
    TokensOut: 320,
    CostUSD:   0.0015,
})
store.Append(ctx, sessionID, ev)
```

---

## `state_updated`

Records an explicit state mutation — useful when your agent updates application-level state that doesn't map to an LLM or tool event.

**Recommended payload:** any key-value pairs describing what changed.

```go
ev, _ := agentstore.NewEvent(agentstore.EventStateUpdated, map[string]interface{}{
    "status":        "awaiting_confirmation",
    "selected_item": "EW456",
})
store.Append(ctx, sessionID, ev)
```

---

## `error`

Records an error. The default reducer increments `state.Errors`.

**Recommended payload:**
```json
{
  "error": "timeout calling search_flights after 30s",
  "code":  "TIMEOUT"
}
```

**Example:**
```go
ev, _ := agentstore.NewEvent(agentstore.EventError, map[string]string{
    "error": "timeout calling search_flights after 30s",
    "code":  "TIMEOUT",
})
store.Append(ctx, sessionID, ev)
```

---

## `custom`

Use `EventCustom` (string value `"custom"`) as a base for your own event types. You can also use any string as an `EventType` — the type system is open.

```go
// Using EventCustom
ev, _ := agentstore.NewEvent(agentstore.EventCustom, myPayload)

// Using a fully custom type string
ev, _ := agentstore.NewEvent(agentstore.EventType("cache_hit"), myPayload)
```

Custom event types are stored and replayed normally. The default reducer stores their latest payload in `state.Data` under the event type string.

---

## Metadata reference

`Metadata` is optional on every event. Populate the fields that are relevant to your event type.

| Field | Type | Used by |
|-------|------|---------|
| `WorkerID` | `string` | Any event — identifies the goroutine/worker |
| `TokensIn` | `int` | `llm_request`, `llm_response` |
| `TokensOut` | `int` | `llm_response` |
| `Model` | `string` | `llm_request`, `llm_response` |
| `ToolName` | `string` | `tool_called`, `tool_result` |
| `DurationMs` | `int64` | `tool_result`, any timed operation |
| `CostUSD` | `float64` | `llm_response` |
| `Extra` | `map[string]string` | Any event — arbitrary key-value pairs |

The default reducer reads `TokensIn`, `TokensOut`, and `CostUSD` from metadata to build `state.Tokens`. Custom reducers can read any metadata field.
