# CLI Tool

AgentStore ships a CLI for inspecting and debugging sessions without writing any code.

## Installation

```bash
go install github.com/abhishek/agentstore/cmd/agentstore@latest
```

Or build from source:

```bash
make install
```

The binary is installed to `$(go env GOPATH)/bin/agentstore`. Make sure that directory is in your `PATH`.

## Global flag

All commands accept:

| Flag | Default | Description |
|------|---------|-------------|
| `--data-dir DIR` | `./agent-data` | Path to the agent data directory |

---

## `sessions` — list all sessions

```
agentstore sessions [--data-dir DIR]
```

Lists all sessions in the data directory, ordered newest first.

**Output:**
```
ID                                    NAME                    EVENTS  CREATED
──────────────────────────────────────────────────────────────────────────────────────────
a1b2c3d4-e5f6-7890-abcd-ef1234567890  flight-search               6  2026-03-07 12:00:00
9f8e7d6c-...                          (unnamed)                    2  2026-03-07 11:45:00
```

Columns:

| Column | Description |
|--------|-------------|
| `ID` | Full session UUID |
| `NAME` | Session name, or `(unnamed)` if none was set |
| `EVENTS` | Total number of events |
| `CREATED` | Creation time in local timezone |

**Example:**
```bash
agentstore sessions --data-dir ./my-agent-data
```

---

## `replay` — replay a session timeline

```
agentstore replay <session-id> [--data-dir DIR] [--format timeline|json] [--type TYPE,...]
```

Replays a session and displays its events.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `timeline` | Output format: `timeline` (human-readable box) or `json` (raw JSON array) |
| `--type` | (all) | Comma-separated event types to include |

### Timeline format (default)

Displays events in a formatted box with elapsed timestamps:

```
┌────────────────── Session a1b2c3d4… · flight-search ───────────────────┐
│  00:00.000  user_message      "Find flights from Berlin to Munich…"  │
│  00:00.012  plan_created      3 steps                                │
│  00:00.034  tool_called       search_flights                         │
│  00:01.234  tool_result       search_flights (1200ms)                │
│  00:01.245  llm_request       gpt-4 · 850 tokens in                  │
│  00:01.890  llm_response      gpt-4 · 320 out · $0.018               │
└──────────────────────────────────────────────────────────────────────┘
Total: 6 events | 1170 tokens | $0.018 | 1 tool calls | 1.89s
```

Elapsed time is relative to the first event in the session (or filtered set).

### JSON format

Returns a JSON array of raw event objects, suitable for piping to `jq`:

```bash
agentstore replay <id> --format json | jq '.[].type'
agentstore replay <id> --format json | jq '[.[] | select(.type == "llm_response")] | length'
```

### Filtering by event type

```bash
# Only tool events
agentstore replay <id> --type tool_called,tool_result

# Only LLM events
agentstore replay <id> --type llm_request,llm_response

# Single type
agentstore replay <id> --type error
```

Type names match the string values exactly: `user_message`, `plan_created`, `tool_called`, `tool_result`, `llm_request`, `llm_response`, `state_updated`, `error`, `custom`.

### Flag order

Flags can appear before or after the session ID:

```bash
# Both forms are equivalent
agentstore replay <id> --format json --type tool_called
agentstore replay --format json --type tool_called <id>
```

---

## `stats` — session statistics

```
agentstore stats <session-id> [--data-dir DIR]
```

Displays a statistics summary for a session.

**Output:**
```
────────────────────────────────────────────────────
  Session:          a1b2c3d4-e5f6-7890-abcd-ef1234567890
────────────────────────────────────────────────────
  Name:             flight-search
  Created:          2026-03-07 12:00:00
  Duration:         1.890s
  Labels:           agent=travel  env=demo
────────────────────────────────────────────────────
  Events:           6
  Tool calls:       1
  Errors:           0
────────────────────────────────────────────────────
  Tokens in:        850
  Tokens out:       320
  Tokens total:     1170
  Cost (USD):       $0.0180
────────────────────────────────────────────────────
  llm_request:      1
  llm_response:     1
  plan_created:     1
  tool_called:      1
  tool_result:      1
  user_message:     1
────────────────────────────────────────────────────
```

The event-type breakdown section lists all event types present in the session, sorted alphabetically.

---

## Common workflows

### Investigate a failed agent run

```bash
# Find recent sessions
agentstore sessions --data-dir ./agent-data

# Replay the full session
agentstore replay <session-id>

# Look at just the errors
agentstore replay <session-id> --type error

# Get full error payloads as JSON
agentstore replay <session-id> --type error --format json | jq '.[] | .payload'
```

### Audit token usage

```bash
# Quick overview
agentstore stats <session-id>

# All LLM calls with their costs
agentstore replay <session-id> --type llm_response --format json \
  | jq '.[] | {model: .metadata.model, tokens_out: .metadata.tokens_out, cost: .metadata.cost_usd}'
```

### Compare multiple sessions

```bash
# List sessions, then stats for each
agentstore sessions --data-dir ./agent-data \
  | awk 'NR>2 {print $1}' \
  | xargs -I{} agentstore stats {} --data-dir ./agent-data
```

### Export a session for offline analysis

```bash
agentstore replay <session-id> --format json > session-export.json
```
