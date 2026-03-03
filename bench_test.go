package agentstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/abhishek/agentstore"
)

func BenchmarkAppend(b *testing.B) {
	s, _ := agentstore.New("", agentstore.WithInMemory(), agentstore.WithSnapshotInterval(0))
	defer s.Close()
	ctx := context.Background()
	session, _ := s.CreateSession(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e, _ := agentstore.NewEvent(agentstore.EventLLMResponse, map[string]string{"content": "test"})
		_ = s.Append(ctx, session.ID, e)
	}
}

func BenchmarkGetState(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("events=%d", n), func(b *testing.B) {
			s, _ := agentstore.New("", agentstore.WithInMemory(), agentstore.WithSnapshotInterval(0))
			defer s.Close()
			ctx := context.Background()
			session, _ := s.CreateSession(ctx)

			for i := 0; i < n; i++ {
				e, _ := agentstore.NewEvent(agentstore.EventCustom, map[string]int{"i": i})
				if err := s.Append(ctx, session.ID, e); err != nil {
					b.Fatal(err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = s.GetState(ctx, session.ID)
			}
		})
	}
}

func BenchmarkGetStateWithSnapshots(b *testing.B) {
	s, _ := agentstore.New("", agentstore.WithInMemory(), agentstore.WithSnapshotInterval(100))
	defer s.Close()
	ctx := context.Background()
	session, _ := s.CreateSession(ctx)

	for i := 0; i < 1000; i++ {
		e, _ := agentstore.NewEvent(agentstore.EventCustom, map[string]int{"i": i})
		if err := s.Append(ctx, session.ID, e); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.GetState(ctx, session.ID)
	}
}

func BenchmarkReplay(b *testing.B) {
	s, _ := agentstore.New("", agentstore.WithInMemory())
	defer s.Close()
	ctx := context.Background()
	session, _ := s.CreateSession(ctx)

	for i := 0; i < 1000; i++ {
		e, _ := agentstore.NewEvent(agentstore.EventCustom, nil)
		if err := s.Append(ctx, session.ID, e); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Replay(ctx, session.ID)
	}
}

func BenchmarkNewEvent(b *testing.B) {
	payload := map[string]interface{}{
		"content": "Find flights to Munich",
		"tools":   []string{"search", "book", "notify"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = agentstore.NewEvent(agentstore.EventUserMessage, payload)
	}
}
