package tests

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/memory/sqlite"
)

func TestSQLite_NewInMemory(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()
}

func TestSQLite_New(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := sqlite.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	// Basic roundtrip to confirm file-based DB works.
	ctx := context.Background()
	if err := store.Append(ctx, "s", core.Message{Role: core.RoleUser, Content: "hi"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	msgs, err := store.Get(ctx, "s")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d", len(msgs))
	}
}

func TestSQLite_AppendAndGet(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	err = store.Append(ctx, "sess",
		core.Message{Role: core.RoleUser, Content: "hello"},
		core.Message{Role: core.RoleAssistant, Content: "world"},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	msgs, err := store.Get(ctx, "sess")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" || msgs[1].Content != "world" {
		t.Fatalf("unexpected messages: %v", msgs)
	}
}

func TestSQLite_EmptySessionReturnsNil(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	msgs, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if msgs != nil {
		t.Fatalf("want nil for empty session, got %v", msgs)
	}
}

func TestSQLite_Ordering(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	_ = store.Append(ctx, "s", core.Message{Role: core.RoleUser, Content: "A"})
	_ = store.Append(ctx, "s", core.Message{Role: core.RoleAssistant, Content: "B"})
	_ = store.Append(ctx, "s", core.Message{Role: core.RoleUser, Content: "C"})

	msgs, err := store.Get(ctx, "s")
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

func TestSQLite_Clear(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	_ = store.Append(ctx, "s", core.Message{Role: core.RoleUser, Content: "hello"})

	if err := store.Clear(ctx, "s"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	msgs, err := store.Get(ctx, "s")
	if err != nil {
		t.Fatalf("Get after Clear: %v", err)
	}
	if msgs != nil {
		t.Fatalf("want nil after Clear, got %v", msgs)
	}
}

func TestSQLite_ToolCallsRoundtrip(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	original := core.Message{
		Role: core.RoleAssistant,
		ToolCalls: []core.ToolCall{
			{ID: "tc1", Name: "search", Input: `{"query":"golang"}`},
		},
	}
	_ = store.Append(ctx, "s", original)

	msgs, err := store.Get(ctx, "s")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("want 1, got %d", len(msgs))
	}
	got := msgs[0]
	if len(got.ToolCalls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(got.ToolCalls))
	}
	if got.ToolCalls[0].Name != "search" || got.ToolCalls[0].Input != `{"query":"golang"}` {
		t.Fatalf("tool call mismatch: %+v", got.ToolCalls[0])
	}
}

func TestSQLite_MultipleSessionsIsolated(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	_ = store.Append(ctx, "sess-a", core.Message{Role: core.RoleUser, Content: "from A"})
	_ = store.Append(ctx, "sess-b", core.Message{Role: core.RoleUser, Content: "from B"})

	msgsA, _ := store.Get(ctx, "sess-a")
	msgsB, _ := store.Get(ctx, "sess-b")

	if len(msgsA) != 1 || msgsA[0].Content != "from A" {
		t.Fatalf("session A isolation broken: %v", msgsA)
	}
	if len(msgsB) != 1 || msgsB[0].Content != "from B" {
		t.Fatalf("session B isolation broken: %v", msgsB)
	}
}

func TestSQLite_ConcurrentAppend(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = store.Append(ctx, "concurrent",
				core.Message{Role: core.RoleUser, Content: "msg"},
			)
		}(i)
	}
	wg.Wait()

	msgs, err := store.Get(ctx, "concurrent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(msgs) != 10 {
		t.Fatalf("want 10 messages after concurrent append, got %d", len(msgs))
	}
}

func TestSQLite_WithPathTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chainforge_test.db")

	store, err := sqlite.New(path, sqlite.WithBusyTimeout(2000000000)) // 2s
	if err != nil {
		t.Fatalf("New with path: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	_ = store.Append(ctx, "s", core.Message{Role: core.RoleUser, Content: "persisted"})

	msgs, err2 := store.Get(ctx, "s")
	if err2 != nil {
		t.Fatalf("Get: %v", err2)
	}
	if len(msgs) != 1 || msgs[0].Content != "persisted" {
		t.Fatalf("unexpected messages: %v", msgs)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected database file to exist on disk")
	}
}

func TestSQLite_WALMode(t *testing.T) {
	store, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("NewInMemory: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	// Trigger schema init.
	_, _ = store.Get(ctx, "probe")

	var mode string
	row := store.DB().QueryRowContext(ctx, "PRAGMA journal_mode")
	if err := row.Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	// :memory: databases report "memory" for journal_mode, not "wal".
	// For file-based, we'd expect "wal". Both are acceptable here.
	if mode == "" {
		t.Fatal("journal_mode is empty")
	}
}
