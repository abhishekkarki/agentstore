# Getting Started

## Installation

AgentStore requires Go 1.21 or later.

```bash
go get github.com/abhishek/agentstore
```

No other dependencies. No CGO. No external services.

## Your first agent session

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/abhishek/agentstore"
)

func main() {
    // Open a persistent store. Creates ./agent-data/ if it doesn't exist.
    store, err := agentstore.New("./agent-data")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    ctx := context.Background()

    // Create a session for your agent run.
    session, err := store.CreateSession(ctx,
        agentstore.WithSessionName("my-agent"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Record events as your agent works.
    ev, _ := agentstore.NewEvent(agentstore.EventUserMessage, map[string]string{
        "content": "Summarise today's news",
    })
    store.Append(ctx, session.ID, ev)

    ev, _ = agentstore.NewEvent(agentstore.EventToolCalled, map[string]string{
        "tool": "fetch_news",
    })
    ev.WithMetadata(agentstore.Metadata{ToolName: "fetch_news"})
    store.Append(ctx, session.ID, ev)

    // Get materialized state at any point.
    state, _ := store.GetState(ctx, session.ID)
    fmt.Printf("Events: %d | Tool calls: %d\n", state.EventCount, state.ToolCalls)

    // Replay the full session.
    events, _ := store.Replay(ctx, session.ID)
    for _, e := range events {
        fmt.Printf("[%d] %s\n", e.SequenceNumber, e.Type)
    }
}
```

## In-memory mode (for tests)

If you don't need persistence — unit tests, one-shot scripts — use the in-memory backend:

```go
store, err := agentstore.New("", agentstore.WithInMemory())
```

Data is lost when the store is closed. Everything else behaves identically.

## Running the example

The repo ships a complete working example:

```bash
go run examples/simple/main.go
```

This runs a simulated flight-search agent, appends events across the full lifecycle, then replays and prints state.

## Next steps

- [Concepts](concepts.md) - understand the event-sourcing model
- [Event Types](event-types.md) - all built-in event types and their payloads
- [API Reference](api-reference.md) - full interface documentation
- [CLI](cli.md) - inspect and debug sessions from the terminal
