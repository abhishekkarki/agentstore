// Package file implements a persistent storage backend using the filesystem.
//
// Design:
//   - Events are stored as append-only JSONL files (one per session)
//   - Sessions are individual JSON files
//   - Snapshots are individual JSON files (atomic replace via rename)
//   - All writes are fsynced for crash safety
//   - Human-readable: you can cat/grep the files directly
//
// Directory layout:
//
//	datadir/
//	├── sessions/
//	│   └── <session_id>.json
//	├── events/
//	│   └── <session_id>.jsonl
//	└── snapshots/
//	    └── <session_id>.json
package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/abhishek/agentstore/internal/storage"
)

// Backend is a filesystem-backed persistent storage implementation.
// All operations are goroutine-safe. Writes are fsynced for durability.
type Backend struct {
	dir string

	mu     sync.RWMutex
	closed bool

	// fileMu provides per-session file locking to avoid contention
	// across different sessions while still protecting concurrent
	// writes to the same session file.
	fileMu sync.Map // map[string]*sync.Mutex
}

// New creates a new file-based storage backend at the given directory.
// The directory and subdirectories are created if they don't exist.
func New(dir string) (*Backend, error) {
	dirs := []string{
		filepath.Join(dir, "sessions"),
		filepath.Join(dir, "events"),
		filepath.Join(dir, "snapshots"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	return &Backend{dir: dir}, nil
}

// sessionFileMu returns a per-session mutex.
func (b *Backend) sessionFileMu(sessionID string) *sync.Mutex {
	v, _ := b.fileMu.LoadOrStore(sessionID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// ─── Session Operations ─────────────────────────────────────────────────────

func (b *Backend) sessionPath(id string) string {
	return filepath.Join(b.dir, "sessions", id+".json")
}

func (b *Backend) SaveSession(_ context.Context, session *storage.SessionRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("store is closed")
	}

	path := b.sessionPath(session.ID)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	return b.writeJSON(path, session)
}

func (b *Backend) GetSession(_ context.Context, id string) (*storage.SessionRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, fmt.Errorf("store is closed")
	}

	var s storage.SessionRecord
	if err := b.readJSON(b.sessionPath(id), &s); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %s not found", id)
		}
		return nil, err
	}
	return &s, nil
}

func (b *Backend) ListSessions(_ context.Context, limit, offset int) ([]*storage.SessionRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, fmt.Errorf("store is closed")
	}

	dir := filepath.Join(b.dir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var sessions []*storage.SessionRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var s storage.SessionRecord
		if err := b.readJSON(filepath.Join(dir, entry.Name()), &s); err != nil {
			continue // skip corrupt files
		}
		sessions = append(sessions, &s)
	}

	// Sort by creation time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	// Apply pagination
	if offset >= len(sessions) {
		return nil, nil
	}
	end := offset + limit
	if end > len(sessions) {
		end = len(sessions)
	}
	return sessions[offset:end], nil
}

func (b *Backend) UpdateSession(_ context.Context, session *storage.SessionRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("store is closed")
	}

	path := b.sessionPath(session.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("session %s not found", session.ID)
	}

	return b.writeJSON(path, session)
}

// ─── Event Operations ───────────────────────────────────────────────────────

func (b *Backend) eventsPath(sessionID string) string {
	return filepath.Join(b.dir, "events", sessionID+".jsonl")
}

func (b *Backend) AppendEvent(_ context.Context, event *storage.EventRecord) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	b.mu.RUnlock()

	// Verify session exists
	if _, err := os.Stat(b.sessionPath(event.SessionID)); os.IsNotExist(err) {
		return fmt.Errorf("session %s not found", event.SessionID)
	}

	// Per-session lock for concurrent appends to different sessions
	fmu := b.sessionFileMu(event.SessionID)
	fmu.Lock()
	defer fmu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	data = append(data, '\n')

	path := b.eventsPath(event.SessionID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write event: %w", err)
	}

	// Fsync for durability — this is what makes it a real WAL
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync events file: %w", err)
	}

	return nil
}

func (b *Backend) GetEvents(_ context.Context, sessionID string, fromSeq uint64) ([]*storage.EventRecord, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	b.mu.RUnlock()

	path := b.eventsPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no events yet
		}
		return nil, fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	var events []*storage.EventRecord
	scanner := bufio.NewScanner(f)

	// Increase buffer for large payloads (1MB max line)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var e storage.EventRecord
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip corrupt lines
		}

		if e.SequenceNumber >= fromSeq {
			events = append(events, &e)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan events file: %w", err)
	}

	return events, nil
}

func (b *Backend) GetLatestSequence(_ context.Context, sessionID string) (uint64, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return 0, fmt.Errorf("store is closed")
	}
	b.mu.RUnlock()

	path := b.eventsPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // no events yet
		}
		return 0, fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	var lastSeq uint64
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Fast path: extract just sequence_number without full unmarshal
		var partial struct {
			SequenceNumber uint64 `json:"sequence_number"`
		}
		if err := json.Unmarshal(line, &partial); err == nil {
			if partial.SequenceNumber > lastSeq {
				lastSeq = partial.SequenceNumber
			}
		}
	}

	return lastSeq, scanner.Err()
}

// ─── Snapshot Operations ────────────────────────────────────────────────────

func (b *Backend) snapshotPath(sessionID string) string {
	return filepath.Join(b.dir, "snapshots", sessionID+".json")
}

func (b *Backend) SaveSnapshot(_ context.Context, snapshot *storage.SnapshotRecord) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	b.mu.RUnlock()

	// Atomic write: write to temp file, then rename
	path := b.snapshotPath(snapshot.SessionID)
	tmpPath := path + ".tmp"

	if err := b.writeJSON(tmpPath, snapshot); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename snapshot: %w", err)
	}

	return nil
}

func (b *Backend) GetLatestSnapshot(_ context.Context, sessionID string) (*storage.SnapshotRecord, error) {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	b.mu.RUnlock()

	var snap storage.SnapshotRecord
	if err := b.readJSON(b.snapshotPath(sessionID), &snap); err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no snapshot
		}
		return nil, err
	}
	return &snap, nil
}

// ─── Lifecycle ──────────────────────────────────────────────────────────────

func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// writeJSON writes a value as JSON to a file with fsync.
func (b *Backend) writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return f.Sync()
}

// readJSON reads a JSON file into a value.
func (b *Backend) readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
