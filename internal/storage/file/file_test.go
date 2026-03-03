package file_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/abhishek/agentstore/internal/storage"
	"github.com/abhishek/agentstore/internal/storage/file"
	"github.com/abhishek/agentstore/internal/storage/storagetest"
)

func TestFileBackend(t *testing.T) {
	storagetest.RunAll(t, func(t *testing.T) storage.Backend {
		dir := t.TempDir()
		b, err := file.New(dir)
		if err != nil {
			t.Fatalf("file.New: %v", err)
		}
		return b
	})
}

func newBackend(t *testing.T) *file.Backend {
	t.Helper()
	b, err := file.New(t.TempDir())
	if err != nil {
		t.Fatalf("file.New: %v", err)
	}
	return b
}

func TestUpdateSessionNotFound(t *testing.T) {
	b := newBackend(t)
	defer b.Close()

	err := b.UpdateSession(context.Background(), &storage.SessionRecord{ID: "ghost"})
	if err == nil {
		t.Fatal("expected error updating non-existent session")
	}
}

func TestClosedBackendRejectsAllOps(t *testing.T) {
	ctx := context.Background()

	run := func(name string, fn func(*file.Backend) error) {
		t.Run(name, func(t *testing.T) {
			b := newBackend(t)
			b.Close()
			if err := fn(b); err == nil {
				t.Fatalf("%s: expected error after close", name)
			}
		})
	}

	payload, _ := json.Marshal(map[string]int{"x": 1})
	snap := &storage.SnapshotRecord{
		SessionID: "s1", Version: 1,
		State:     payload,
		CreatedAt: time.Now().UTC(),
	}

	run("SaveSession", func(b *file.Backend) error {
		return b.SaveSession(ctx, &storage.SessionRecord{ID: "s1"})
	})
	run("GetSession", func(b *file.Backend) error {
		_, err := b.GetSession(ctx, "s1")
		return err
	})
	run("ListSessions", func(b *file.Backend) error {
		_, err := b.ListSessions(ctx, 10, 0)
		return err
	})
	run("UpdateSession", func(b *file.Backend) error {
		return b.UpdateSession(ctx, &storage.SessionRecord{ID: "s1"})
	})
	run("AppendEvent", func(b *file.Backend) error {
		return b.AppendEvent(ctx, &storage.EventRecord{SessionID: "s1", SequenceNumber: 1, Type: "custom"})
	})
	run("GetEvents", func(b *file.Backend) error {
		_, err := b.GetEvents(ctx, "s1", 0)
		return err
	})
	run("GetLatestSequence", func(b *file.Backend) error {
		_, err := b.GetLatestSequence(ctx, "s1")
		return err
	})
	run("SaveSnapshot", func(b *file.Backend) error {
		return b.SaveSnapshot(ctx, snap)
	})
	run("GetLatestSnapshot", func(b *file.Backend) error {
		_, err := b.GetLatestSnapshot(ctx, "s1")
		return err
	})
}
