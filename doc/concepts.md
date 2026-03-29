# Core Concepts

## Event sourcing for agents

AgentStore applies event sourcing to AI agent state. Instead of storing the current state and overwriting it on each change, every action your agent takes is stored as an immutable event. State is derived by replaying those events.

```
append-only event log -> replay -> current state
```

This gives you:

- **Full history** — every decision, tool call, and LLM response is preserved forever
- **Crash recovery** — on restart, replay the log to restore state exactly as it was
- **Deterministic debugging** — reproduce any past state by replaying up to that point
- **Auditability** — know exactly what happened, when, and in what order

## Sessions

A **session** is the top-level container for one agent run. Think of it as a conversation thread, a task execution, or a job — whatever unit of work your agent performs.

```
Session
├── Metadata (ID, name, labels, timestamps)
└── Event log (append-only, ordered by sequence number)
```

Sessions are independent. Each has its own event log and materialized state. Multiple sessions can be open and written to concurrently.

Key properties:

| Property     | Description                                              |
|--------------|----------------------------------------------------------|
| `ID`         | UUID v4, generated automatically unless overridden       |
| `Name`       | Optional human-readable label                            |
| `Labels`     | Arbitrary key-value pairs for grouping and filtering     |
| `EventCount` | Updated automatically when events are appended           |
| `CreatedAt`  | Set once at creation (UTC)                               |
| `UpdatedAt`  | Set on every append (UTC)                                |

## Events

An **event** is an immutable record of something that happened. Events are the source of truth — nothing is ever modified or deleted.

```
Event
├── SessionID        (links to parent session)
├── SequenceNumber   (monotonic, assigned by the store)
├── Type             (EventToolCalled, EventLLMResponse, etc.)
├── Payload          (JSON — event-specific data)
├── Timestamp        (UTC, set by the store on append)
└── Metadata         (optional — tokens, cost, model, tool name, duration)
```

Once appended, an event cannot be changed. `SequenceNumber` and `Timestamp` are assigned by the store — callers should not set them.

See [Event Types](event-types.md) for the full list of built-in event types.

## State

**State** is the materialized view of a session — derived by folding events through a reducer function. You never update state directly; it is always recomputed from events.

```go
state, err := store.GetState(ctx, sessionID)

state.EventCount      // total events applied
state.ToolCalls       // number of EventToolCalled events
state.Errors          // number of EventError events
state.Tokens.In       // cumulative LLM input tokens
state.Tokens.Out      // cumulative LLM output tokens
state.Tokens.CostUSD  // cumulative estimated cost
state.Data            // map[string]interface{} — latest payload per event type
state.Version         // sequence number of last applied event
```

`state.Data` stores the most recent payload for each event type. For example, after an `llm_response` event, `state.Data["llm_response"]` holds its payload.

## Reducer

The **reducer** is the function that turns events into state. The default reducer handles all built-in event types: it accumulates token counts, tracks tool calls and errors, and stores the latest payload per event type.

You can supply your own reducer to build domain-specific state — for example, maintaining a list of messages, tracking custom metrics, or implementing your own state machine. See [Custom Reducers](custom-reducers.md).

## Snapshots

Replaying thousands of events on every `GetState` call would be slow. **Snapshots** solve this: at a configurable interval (default every 100 events), the store persists the current materialized state to disk. On the next `GetState` call, it loads the snapshot and only replays events after it.

```
GetState:
  1. Load latest snapshot (if any) → state at version N
  2. Replay events from N+1 onward
  3. Return final state
```

Snapshots are written atomically (write to temp file, then rename), so a crash during snapshot creation never corrupts the event log.

The snapshot interval is configurable:

```go
// Snapshot every 50 events
store, _ := agentstore.New("./data", agentstore.WithSnapshotInterval(50))

// Disable snapshots entirely
store, _ := agentstore.New("./data", agentstore.WithSnapshotInterval(0))
```

## Replay

**Replay** returns events from the log, optionally filtered. It is the primary tool for debugging.

```go
// All events
events, _ := store.Replay(ctx, sessionID)

// Only tool events
events, _ := store.Replay(ctx, sessionID,
    agentstore.WithTypeFilter(agentstore.EventToolCalled, agentstore.EventToolResult),
)

// Events within a time range
events, _ := store.Replay(ctx, sessionID,
    agentstore.WithTimeRange(from, to),
)
```

Replay reads from the raw event log, not from snapshots, so you always get the full unmodified history.

## Concurrency

The store is safe for concurrent use from multiple goroutines. You can:

- Append events to different sessions in parallel
- Append and read state for the same session concurrently

The file backend uses a global `sync.RWMutex` for session/snapshot metadata and per-session mutexes for event file writes, so concurrent appends to different sessions do not block each other.
