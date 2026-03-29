# Custom Reducers

A **reducer** is a function that folds events into state:

```go
type Reducer func(state *State, event *Event) *State
```

The default reducer handles all built-in event types. You can replace it with your own to build domain-specific state, implement a custom state machine, or extend the default behavior.

## When to use a custom reducer

- You need to maintain a list of messages or tool results in state (the default only keeps the latest per type)
- You want to track domain-specific counters or flags
- You have custom event types that should affect state
- You want to derive structured state from payloads rather than keeping raw JSON

## Providing a custom reducer

Pass `WithReducer` when creating the store:

```go
store, err := agentstore.New("./data",
    agentstore.WithReducer(myReducer),
)
```

## Example: maintaining a message history

The default reducer stores only the latest `user_message` payload in `state.Data`. To keep all messages:

```go
func messageHistoryReducer(state *agentstore.State, event *agentstore.Event) *agentstore.State {
    if state.Data == nil {
        state.Data = make(map[string]interface{})
    }

    // Always update the built-in counters.
    state.Version = event.SequenceNumber
    state.LastEventAt = event.Timestamp
    state.EventCount++

    if event.Type == agentstore.EventLLMRequest || event.Type == agentstore.EventLLMResponse {
        state.Tokens.In += event.Metadata.TokensIn
        state.Tokens.Out += event.Metadata.TokensOut
        state.Tokens.CostUSD += event.Metadata.CostUSD
    }
    if event.Type == agentstore.EventToolCalled {
        state.ToolCalls++
    }
    if event.Type == agentstore.EventError {
        state.Errors++
    }

    // Custom: accumulate messages into a slice.
    if event.Type == agentstore.EventUserMessage || event.Type == agentstore.EventLLMResponse {
        var payload map[string]interface{}
        if err := json.Unmarshal(event.Payload, &payload); err == nil {
            messages, _ := state.Data["messages"].([]interface{})
            state.Data["messages"] = append(messages, map[string]interface{}{
                "type":    string(event.Type),
                "content": payload["content"],
                "seq":     event.SequenceNumber,
            })
        }
    }

    return state
}

store, _ := agentstore.New("./data", agentstore.WithReducer(messageHistoryReducer))
```

Now `state.Data["messages"]` is a slice of all user and LLM messages in order.

## Example: custom status tracking

```go
type AgentStatus string

const (
    StatusIdle     AgentStatus = "idle"
    StatusPlanning AgentStatus = "planning"
    StatusWorking  AgentStatus = "working"
    StatusDone     AgentStatus = "done"
    StatusError    AgentStatus = "error"
)

func statusReducer(state *agentstore.State, event *agentstore.Event) *agentstore.State {
    if state.Data == nil {
        state.Data = make(map[string]interface{})
    }

    state.Version = event.SequenceNumber
    state.LastEventAt = event.Timestamp
    state.EventCount++

    switch event.Type {
    case agentstore.EventPlanCreated:
        state.Data["status"] = string(StatusPlanning)
    case agentstore.EventToolCalled:
        state.ToolCalls++
        state.Data["status"] = string(StatusWorking)
    case agentstore.EventToolResult:
        state.Data["status"] = string(StatusWorking)
    case agentstore.EventLLMRequest, agentstore.EventLLMResponse:
        state.Tokens.In += event.Metadata.TokensIn
        state.Tokens.Out += event.Metadata.TokensOut
        state.Tokens.CostUSD += event.Metadata.CostUSD
        state.Data["status"] = string(StatusWorking)
    case agentstore.EventStateUpdated:
        var p map[string]interface{}
        if json.Unmarshal(event.Payload, &p) == nil {
            if v, ok := p["done"].(bool); ok && v {
                state.Data["status"] = string(StatusDone)
            }
        }
    case agentstore.EventError:
        state.Errors++
        state.Data["status"] = string(StatusError)
    }

    return state
}
```

## Extending the default reducer

If you want the default behavior plus your own additions, call `DefaultReducer` inside your function:

```go
base := agentstore.DefaultReducer()

myReducer := func(state *agentstore.State, event *agentstore.Event) *agentstore.State {
    // Apply all default behavior first.
    state = base(state, event)

    // Then add your own logic.
    if event.Type == agentstore.EventType("cache_hit") {
        hits, _ := state.Data["cache_hits"].(float64)
        state.Data["cache_hits"] = hits + 1
    }

    return state
}

store, _ := agentstore.New("./data", agentstore.WithReducer(myReducer))
```

## Important rules

1. **Reducers must be pure.** The same events in the same order must always produce the same state. Do not read from external systems inside a reducer.

2. **Reducers must not return nil.** Always return the (possibly modified) state.

3. **State is reused across calls.** The same `*State` pointer is passed to each reducer call. Mutate it in place and return it — do not create a new `State` from scratch (you would lose accumulated counters).

4. **Reducers run during `GetState` and snapshot creation.** They may run multiple times on the same events (once per `GetState` call if no snapshot exists). Do not have side effects.

## Snapshots and custom reducers

Snapshots serialize the full `State` struct to JSON. If your reducer stores data in `state.Data`, it is captured in snapshots and restored on the next `GetState` call. No special handling is needed.

However, if your custom state uses Go types that do not round-trip cleanly through `encoding/json` (e.g., typed structs stored as `interface{}`), you may need to re-cast when reading from `state.Data` after a snapshot reload. JSON numbers unmarshal as `float64`, not `int`.
