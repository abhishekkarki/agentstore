# Storage Backends

AgentStore supports two storage backends. Both implement the same internal interface, so switching between them requires changing only the `New` call.

## File backend (persistent)

The default backend. Data survives process restarts and crashes.

```go
store, err := agentstore.New("./agent-data")
```

### Directory layout

```
agent-data/
├── sessions/
│   └── <session-id>.json        # session metadata
├── events/
│   └── <session-id>.jsonl       # append-only event log
└── snapshots/
    └── <session-id>.json        # latest materialized state snapshot
```

### Sessions

Each session is a JSON file named by its UUID. Written atomically with fsync.

```json
{
  "id": "a1b2c3d4-...",
  "name": "flight-search",
  "created_at": "2026-03-07T12:00:00Z",
  "updated_at": "2026-03-07T12:01:30Z",
  "event_count": 6,
  "labels": {
    "agent": "travel",
    "env": "demo"
  }
}
```

### Events

Events are stored as JSONL (one JSON object per line) in an append-only file named by session ID. Each `Append` call writes one line and calls `fsync` — this is what makes the log crash-safe.

```jsonl
{"session_id":"a1b2c3d4-...","sequence_number":1,"type":"user_message","payload":{"content":"Find flights"},"timestamp":"2026-03-07T12:00:00.123Z","metadata":{}}
{"session_id":"a1b2c3d4-...","sequence_number":2,"type":"tool_called","payload":{"tool":"search_flights"},"timestamp":"2026-03-07T12:00:00.456Z","metadata":{"tool_name":"search_flights"}}
```

Because it's plain JSONL, you can inspect it directly:

```bash
# View all events for a session
cat agent-data/events/<session-id>.jsonl | jq .

# Find all errors
grep '"type":"error"' agent-data/events/<session-id>.jsonl | jq .

# Count tool calls
grep -c '"type":"tool_called"' agent-data/events/<session-id>.jsonl
```

### Snapshots

Snapshots are JSON files written atomically: the store writes to a `.tmp` file, then renames it over the real file. This ensures that a crash mid-write never produces a corrupt snapshot — the worst case is the old snapshot is kept intact.

```json
{
  "session_id": "a1b2c3d4-...",
  "version": 100,
  "state": {
    "session_id": "a1b2c3d4-...",
    "version": 100,
    "event_count": 100,
    "tool_calls": 12,
    "errors": 0,
    "tokens": {"in": 8500, "out": 3200, "cost_usd": 0.15},
    ...
  },
  "created_at": "2026-03-07T12:10:00Z",
  "event_count": 100
}
```

Only one snapshot is kept per session (the latest). Older snapshots are replaced on each write.

### Concurrency model

- A global `sync.RWMutex` protects session and snapshot file operations
- A per-session `sync.Mutex` (via `sync.Map`) serializes concurrent appends to the same session's event file
- Appending to different sessions is fully concurrent — they never block each other

### Durability guarantees

| Operation | Guarantee |
|-----------|-----------|
| `Append` | Event is fsynced before returning — durable on success |
| `CreateSession` | Session file is fsynced before returning |
| `SaveSnapshot` | Atomic rename — crash-safe, never partial |

---

## In-memory backend

An in-process map-based store. No files are written. Data is lost when the store is closed.

```go
store, err := agentstore.New("", agentstore.WithInMemory())
```

Use this for:

- Unit tests
- Ephemeral agents that don't need persistence
- Benchmarks

The in-memory backend has the same concurrency guarantees as the file backend. It is safe for concurrent use from multiple goroutines.

### Performance

In-memory operations are significantly faster than file backend operations because there are no disk writes. See the [benchmarks section in the README](../README.md#performance) for numbers.

---

## Choosing a backend

| | File backend | In-memory backend |
|---|---|---|
| **Persistence** | Yes — survives restarts | No |
| **Crash safety** | Yes — fsynced writes | N/A |
| **Human-readable data** | Yes — JSON/JSONL files | No |
| **Performance** | ~2.8μs per append | ~1μs per append |
| **Use case** | Production agents | Tests, ephemeral runs |

---

## Backup and recovery

Because the file backend uses plain files, backup is straightforward:

```bash
# Backup
cp -r ./agent-data ./agent-data-backup

# Restore
cp -r ./agent-data-backup ./agent-data
```

If the process crashes mid-append, the partial write will either be a valid JSON line (recovered normally) or an invalid line (skipped on read — the scanner silently ignores corrupt lines and continues). No data from previous successful appends is lost.

If the process crashes mid-snapshot-write, the `.tmp` file is left behind. On the next snapshot write for that session, it is overwritten. The existing snapshot file is untouched.
