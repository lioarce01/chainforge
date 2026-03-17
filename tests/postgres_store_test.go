package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/postgres"
)

func pgDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set")
	}
	return dsn
}

func TestPostgres_ErrNoDSN(t *testing.T) {
	_, err := postgres.New("")
	if !errors.Is(err, postgres.ErrNoDSN) {
		t.Fatalf("want ErrNoDSN, got %v", err)
	}
}

func TestPostgres_Roundtrip(t *testing.T) {
	store, err := postgres.New(pgDSN(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "pg-roundtrip") }) //nolint:errcheck

	err = store.Append(ctx, "pg-roundtrip",
		core.Message{Role: core.RoleUser, Content: "hello"},
		core.Message{Role: core.RoleAssistant, Content: "world"},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	msgs, err := store.Get(ctx, "pg-roundtrip")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" || msgs[1].Content != "world" {
		t.Fatalf("unexpected messages: %v", msgs)
	}
}

func TestPostgres_Ordering(t *testing.T) {
	store, err := postgres.New(pgDSN(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "pg-ordering") }) //nolint:errcheck

	for _, content := range []string{"A", "B", "C"} {
		_ = store.Append(ctx, "pg-ordering", core.Message{Role: core.RoleUser, Content: content})
	}

	msgs, err := store.Get(ctx, "pg-ordering")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := []string{"A", "B", "C"}
	for i, w := range want {
		if msgs[i].Content != w {
			t.Errorf("index %d: want %q, got %q", i, w, msgs[i].Content)
		}
	}
}

func TestPostgres_Clear(t *testing.T) {
	store, err := postgres.New(pgDSN(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "pg-clear") }) //nolint:errcheck

	_ = store.Append(ctx, "pg-clear", core.Message{Role: core.RoleUser, Content: "hello"})

	if err := store.Clear(ctx, "pg-clear"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	msgs, err := store.Get(ctx, "pg-clear")
	if err != nil {
		t.Fatalf("Get after Clear: %v", err)
	}
	if msgs != nil {
		t.Fatalf("want nil after Clear, got %v", msgs)
	}
}

func TestPostgres_ToolCallsRoundtrip(t *testing.T) {
	store, err := postgres.New(pgDSN(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "pg-toolcalls") }) //nolint:errcheck

	original := core.Message{
		Role: core.RoleAssistant,
		ToolCalls: []core.ToolCall{
			{ID: "tc1", Name: "search", Input: `{"query":"postgres"}`},
		},
	}
	_ = store.Append(ctx, "pg-toolcalls", original)

	msgs, err := store.Get(ctx, "pg-toolcalls")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs[0].ToolCalls) != 1 || msgs[0].ToolCalls[0].Name != "search" {
		t.Fatalf("tool call mismatch: %+v", msgs[0])
	}
}

func TestPostgres_ConcurrentAppend(t *testing.T) {
	store, err := postgres.New(pgDSN(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	session := "pg-concurrent"
	t.Cleanup(func() { store.Clear(ctx, session) }) //nolint:errcheck

	const (
		goroutines = 5
		perGoroutine = 10
	)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				_ = store.Append(ctx, session,
					core.Message{Role: core.RoleUser, Content: fmt.Sprintf("g%d-m%d", i, j)},
				)
			}
		}(i)
	}
	wg.Wait()

	msgs, err := store.Get(ctx, session)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != goroutines*perGoroutine {
		t.Fatalf("want %d messages, got %d", goroutines*perGoroutine, len(msgs))
	}
}

func TestPostgres_WithSchemaName(t *testing.T) {
	store, err := postgres.New(pgDSN(t), postgres.WithSchemaName("public"))
	if err != nil {
		t.Fatalf("New with schema: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	t.Cleanup(func() { store.Clear(ctx, "pg-schema") }) //nolint:errcheck

	_ = store.Append(ctx, "pg-schema", core.Message{Role: core.RoleUser, Content: "schema test"})
	msgs, err := store.Get(ctx, "pg-schema")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("want 1, got %d", len(msgs))
	}
}
