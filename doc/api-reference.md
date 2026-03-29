# API Reference

## Store interface

```go
type Store interface {
    CreateSession(ctx context.Context, opts ...SessionOption) (*Session, error)
    GetSession(ctx context.Context, id string) (*Session, error)
    ListSessions(ctx context.Context, opts ...ListOption) ([]*Session, error)
    Append(ctx context.Context, sessionID string, event *Event) error
    GetEvents(ctx context.Context, sessionID string, fromSeq uint64) ([]*Event, error)
    GetState(ctx context.Context, sessionID string) (*State, error)
    Replay(ctx context.Context, sessionID string, opts ...ReplayOption) ([]*Event, error)
    Close() error
}
```

All methods are safe for concurrent use.

---

### `New`

```go
func New(dataDir string, opts ...StoreOption) (Store, error)
```

Creates a new store. If `dataDir` is a non-empty path, uses the file backend (persistent). If `dataDir` is empty or `WithInMemory()` is passed, uses the in-memory backend.

The file backend creates `dataDir` and its subdirectories if they don't exist.

```go
// Persistent
store, err := agentstore.New("./agent-data")

// In-memory
store, err := agentstore.New("", agentstore.WithInMemory())

// Custom options
store, err := agentstore.New("./data",
    agentstore.WithSnapshotInterval(50),
    agentstore.WithReducer(myReducer),
)
```

---

### `CreateSession`

```go
func (s Store) CreateSession(ctx context.Context, opts ...SessionOption) (*Session, error)
```

Creates and persists a new session. Returns the created `*Session`.

```go
session, err := store.CreateSession(ctx,
    agentstore.WithSessionName("my-agent"),
    agentstore.WithSessionID("custom-id"),          // optional: override generated UUID
    agentstore.WithLabels(map[string]string{
        "env":   "production",
        "model": "gpt-4",
    }),
)
```

**Errors:** Returns an error if the session ID already exists.

---

### `GetSession`

```go
func (s Store) GetSession(ctx context.Context, id string) (*Session, error)
```

Retrieves a session by ID.

**Errors:** Returns an error if the session is not found.

---

### `ListSessions`

```go
func (s Store) ListSessions(ctx context.Context, opts ...ListOption) ([]*Session, error)
```

Returns sessions ordered by creation time (newest first).

```go
// First 100 sessions (default)
sessions, _ := store.ListSessions(ctx)

// Paginate
sessions, _ := store.ListSessions(ctx,
    agentstore.WithLimit(20),
    agentstore.WithOffset(40),
)

// Filter by label
sessions, _ := store.ListSessions(ctx,
    agentstore.WithLabelFilter("env", "production"),
)
```

---

### `Append`

```go
func (s Store) Append(ctx context.Context, sessionID string, event *Event) error
```

Appends an event to the session's log. The store sets `SequenceNumber` and `Timestamp` automatically — do not set these on the event before calling `Append`.

```go
ev, err := agentstore.NewEvent(agentstore.EventToolCalled, map[string]string{
    "tool": "search",
    "query": "latest AI news",
})
ev.WithMetadata(agentstore.Metadata{
    ToolName:   "search",
    DurationMs: 340,
})
err = store.Append(ctx, session.ID, ev)
```

After `Append` returns, the event is durable (fsynced to disk on the file backend).

If the snapshot interval is configured, `Append` may trigger an automatic snapshot. Snapshot errors are non-fatal — they do not cause `Append` to fail.

**Errors:** Returns an error if the session does not exist.

---

### `GetEvents`

```go
func (s Store) GetEvents(ctx context.Context, sessionID string, fromSeq uint64) ([]*Event, error)
```

Returns events for a session starting from `fromSeq`. Use `fromSeq=0` to get all events.

```go
// All events
events, _ := store.GetEvents(ctx, sessionID, 0)

// Events after sequence number 50
events, _ := store.GetEvents(ctx, sessionID, 51)
```

---

### `GetState`

```go
func (s Store) GetState(ctx context.Context, sessionID string) (*State, error)
```

Returns the materialized state for a session. Loads the latest snapshot (if any) and replays subsequent events through the reducer.

```go
state, err := store.GetState(ctx, sessionID)

fmt.Println(state.EventCount)
fmt.Println(state.ToolCalls)
fmt.Println(state.Errors)
fmt.Println(state.Tokens.In, state.Tokens.Out, state.Tokens.CostUSD)
fmt.Println(state.Data["llm_response"]) // latest payload for that event type
```

---

### `Replay`

```go
func (s Store) Replay(ctx context.Context, sessionID string, opts ...ReplayOption) ([]*Event, error)
```

Returns events in order, with optional filtering. Reads from the raw event log — snapshots do not affect this.

```go
// All events
events, _ := store.Replay(ctx, sessionID)

// Filter by event type
events, _ := store.Replay(ctx, sessionID,
    agentstore.WithTypeFilter(agentstore.EventToolCalled, agentstore.EventToolResult),
)

// Filter by time range
events, _ := store.Replay(ctx, sessionID,
    agentstore.WithTimeRange(start, end),
)

// Combine filters
events, _ := store.Replay(ctx, sessionID,
    agentstore.WithTypeFilter(agentstore.EventLLMResponse),
    agentstore.WithTimeRange(start, end),
)
```

---

### `Close`

```go
func (s Store) Close() error
```

Shuts down the store and releases resources. After `Close`, all operations return an error. Always call `Close` (typically via `defer`).

---

## Types

### `Session`

```go
type Session struct {
    ID         string            `json:"id"`
    Name       string            `json:"name,omitempty"`
    CreatedAt  time.Time         `json:"created_at"`
    UpdatedAt  time.Time         `json:"updated_at"`
    EventCount uint64            `json:"event_count"`
    Labels     map[string]string `json:"labels,omitempty"`
}
```

### `Event`

```go
type Event struct {
    SessionID      string          `json:"session_id"`
    SequenceNumber uint64          `json:"sequence_number"`
    Type           EventType       `json:"type"`
    Payload        json.RawMessage `json:"payload"`
    Timestamp      time.Time       `json:"timestamp"`
    Metadata       Metadata        `json:"metadata,omitempty"`
}
```

Helper methods:

```go
func (e *Event) WithMetadata(m Metadata) *Event  // chainable
func (e *Event) IsLLMEvent() bool                 // true for llm_request / llm_response
func (e *Event) IsToolEvent() bool                // true for tool_called / tool_result
```

### `Metadata`

```go
type Metadata struct {
    WorkerID   string            `json:"worker_id,omitempty"`
    TokensIn   int               `json:"tokens_in,omitempty"`
    TokensOut  int               `json:"tokens_out,omitempty"`
    Model      string            `json:"model,omitempty"`
    ToolName   string            `json:"tool_name,omitempty"`
    DurationMs int64             `json:"duration_ms,omitempty"`
    CostUSD    float64           `json:"cost_usd,omitempty"`
    Extra      map[string]string `json:"extra,omitempty"`
}
```

All fields are optional.

### `State`

```go
type State struct {
    SessionID   string                 `json:"session_id"`
    Version     uint64                 `json:"version"`
    Data        map[string]interface{} `json:"data"`
    LastEventAt time.Time              `json:"last_event_at"`
    EventCount  uint64                 `json:"event_count"`
    Tokens      TokenUsage             `json:"tokens"`
    ToolCalls   int                    `json:"tool_calls"`
    Errors      int                    `json:"errors"`
}

type TokenUsage struct {
    In      int     `json:"in"`
    Out     int     `json:"out"`
    CostUSD float64 `json:"cost_usd"`
}
```

---

## Store options (`StoreOption`)

| Option | Default | Description |
|--------|---------|-------------|
| `WithInMemory()` | false | Use in-memory backend instead of file backend |
| `WithSnapshotInterval(n)` | 100 | Create a snapshot every N events. 0 disables snapshots |
| `WithReducer(r)` | `DefaultReducer()` | Custom state reducer function |

## Session options (`SessionOption`)

| Option | Description |
|--------|-------------|
| `WithSessionName(name)` | Human-readable label |
| `WithSessionID(id)` | Override the auto-generated UUID |
| `WithLabels(labels)` | Attach key-value labels |

## List options (`ListOption`)

| Option | Default | Description |
|--------|---------|-------------|
| `WithLimit(n)` | 100 | Max sessions to return |
| `WithOffset(n)` | 0 | Pagination offset |
| `WithLabelFilter(key, value)` | — | Filter by label |

## Replay options (`ReplayOption`)

| Option | Description |
|--------|-------------|
| `WithTypeFilter(types...)` | Only return events of the given types |
| `WithTimeRange(from, to)` | Only return events within the time range |
