# AgentStore

Embeddable event store for AI agent state. Import it. Use it. Replay everything.

[![Go Tests](https://github.com/abhishekkarki/agentstore/actions/workflows/ci.yml/badge.svg)](https://github.com/abhishekkarki/agentstore/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/abhishekkarki/agentstore.svg)](https://pkg.go.dev/github.com/abhishekkarki/agentstore)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

---

AgentStore gives your AI agents **durable, replayable state** with zero infrastructure. It's a Go library — not a platform, not a server, not something you deploy. Just `go get` and use.

Every agent decision, tool call, LLM response, and error is captured as an immutable event. State is derived from events. Crashes are recoverable. Debugging is a replay command away.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "github.com/abhishekkarki/agentstore"
)

func main() {
    store, _ := agentstore.New("./agent-data")  // persistent storage
    // store, _ := agentstore.New("", agentstore.WithInMemory())  // or in-memory
    defer store.Close()

    ctx := context.Background()

    // Create a session for your agent
    session, _ := store.CreateSession(ctx, agentstore.WithSessionName("my-agent"))

    // Record what your agent does
    event, _ := agentstore.NewEvent(agentstore.EventToolCalled, map[string]string{
        "tool": "search_flights", "query": "BER → MUC",
    })
    event.WithMetadata(agentstore.Metadata{ToolName: "search_flights", DurationMs: 800})
    store.Append(ctx, session.ID, event)

    // Get materialized state with token tracking
    state, _ := store.GetState(ctx, session.ID)
    fmt.Printf("Events: %d | Tokens: %d in / %d out | Cost: $%.3f\n",
        state.EventCount, state.Tokens.In, state.Tokens.Out, state.Tokens.CostUSD)

    // Replay the full session for debugging
    events, _ := store.Replay(ctx, session.ID)
    for _, e := range events {
        fmt.Printf("[%d] %s\n", e.SequenceNumber, e.Type)
    }
}
```

## Why AgentStore?

AI agents make decisions, call tools, talk to LLMs, and fail. When they fail, you need to know exactly what happened. When they succeed, you need to know what it cost.

Most agent frameworks store state in Redis or Postgres — mutable, overwritten, gone. AgentStore keeps everything as an append-only event log:

```
user_message → plan_created → tool_called → tool_result → llm_response → state_updated
```

This gives you **crash recovery**, **full replay**, **token cost tracking**, and **deterministic debugging** — for free.

## When to Use AgentStore vs Temporal

| | **AgentStore** | **Temporal** |
|---|---|---|
| **What it is** | Embeddable Go library | Distributed workflow platform |
| **Deployment** | `go get` — lives in your process | Cluster + database (Cassandra/MySQL) |
| **Setup time** | Seconds | Hours |
| **Best for** | Agent state & debugging | Enterprise workflow orchestration |
| **Agent awareness** | Native (tool calls, tokens, costs) | Generic workflow events |
| **Minimum infra** | Zero | 3+ services |

**Use AgentStore** when you're building an agent and want durable state without running infrastructure.

**Use Temporal** when you need enterprise-scale distributed workflow orchestration across services.

They're complementary, not competing.

## Features

**Agent-Native Events** — First-class types for `user_message`, `tool_called`, `llm_response`, `plan_created`, and more. Not generic strings.

**Token & Cost Tracking** — Every LLM call's token usage and cost is accumulated automatically in session state.

**Replay & Debugging** — Replay any session step-by-step. Filter by event type or time range. Export to JSON.

**Snapshots** — Automatic periodic snapshots so state reconstruction stays fast even with thousands of events.

**Custom Reducers** — Plug in your own state materialization logic.

**Zero Dependencies** — Pure Go, stdlib only. No CGO, no external databases.

**Thread-Safe** — Safe for concurrent use from multiple goroutines.

## Performance

In-memory backend on a single core (Intel Xeon):

| Operation | Throughput | Latency |
|-----------|-----------|---------|
| Append | ~358k ops/sec | ~2.8μs |
| GetState (with snapshot) | ~207k ops/sec | ~6.3μs |
| GetState (1000 events, no snapshot) | ~594 ops/sec | ~1.9ms |
| NewEvent | ~1M ops/sec | ~1.1μs |

## Storage

AgentStore supports two backends:

**In-memory** — For testing, ephemeral agents, or when you don't need persistence:
```go
store, _ := agentstore.New("", agentstore.WithInMemory())
```

**File-based** — Durable, human-readable, crash-safe (fsync'd writes):
```go
store, _ := agentstore.New("./agent-data")
```

The file backend stores sessions as JSON, events as append-only JSONL, and snapshots as atomic JSON files. You can `cat` and `grep` the data directly.

## API

```go
// Core interface
type Store interface {
    CreateSession(ctx, ...SessionOption) (*Session, error)
    GetSession(ctx, id) (*Session, error)
    ListSessions(ctx, ...ListOption) ([]*Session, error)
    Append(ctx, sessionID, *Event) error
    GetEvents(ctx, sessionID, fromSeq) ([]*Event, error)
    GetState(ctx, sessionID) (*State, error)
    Replay(ctx, sessionID, ...ReplayOption) ([]*Event, error)
    Close() error
}
```

## Agent-Native Event Types

```go
EventUserMessage   // User input
EventPlanCreated   // Agent's execution plan
EventToolCalled    // Tool invocation
EventToolResult    // Tool response
EventLLMRequest    // LLM API call
EventLLMResponse   // LLM output
EventStateUpdated  // Explicit state mutation
EventError         // Errors and failures
EventCustom        // Your own event types
```

## CLI Tool

AgentStore includes a CLI for inspecting and debugging sessions:

```bash
# Install
go install github.com/abhishekkarki/agentstore/cmd/agentstore@latest

# List all sessions
agentstore sessions --data-dir ./agent-data

# Replay a session timeline
agentstore replay <session-id>

# Filter by event type
agentstore replay <session-id> --type=tool_called,tool_result

# Export as JSON
agentstore replay <session-id> --format=json

# Session statistics
agentstore stats <session-id>
```

## Roadmap

- [x] **v0.1.0** — Core library: event log, state, snapshots, replay
- [ ] **v0.2.0** — Optional gRPC server mode + Python client
- [ ] **v0.3.0** — Raft-based distributed replication
- [ ] **v1.0.0** — Web UI, OpenTelemetry, framework integrations

## License

Apache 2.0
