package file_test

import (
	"testing"

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
