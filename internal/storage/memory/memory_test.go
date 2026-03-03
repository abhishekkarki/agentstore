package memory_test

import (
	"testing"

	"github.com/abhishek/agentstore/internal/storage"
	"github.com/abhishek/agentstore/internal/storage/memory"
	"github.com/abhishek/agentstore/internal/storage/storagetest"
)

func TestMemoryBackend(t *testing.T) {
	storagetest.RunAll(t, func(t *testing.T) storage.Backend {
		return memory.New()
	})
}
