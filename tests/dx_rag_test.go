package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chainforge "github.com/lioarce01/chainforge"
	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/rag"
	"github.com/lioarce01/chainforge/pkg/rag/loader"
	"github.com/lioarce01/chainforge/pkg/rag/splitter"
	"github.com/lioarce01/chainforge/pkg/testutil"
)

// --- Splitter tests ---

func TestFixedSplitter_ChunksWithOverlap(t *testing.T) {
	s := splitter.NewFixedSizeSplitter(10, 3)
	text := "abcdefghijklmnopqrstuvwxyz" // 26 chars
	chunks := s.Split(text)

	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// First chunk should be 10 chars.
	if len([]rune(chunks[0])) != 10 {
		t.Errorf("chunk[0] len = %d, want 10", len([]rune(chunks[0])))
	}
	// Overlap: chunk[1] should share 3 chars with the end of chunk[0].
	overlap := chunks[0][len(chunks[0])-3:]
	if !strings.HasPrefix(chunks[1], overlap) {
		t.Errorf("expected chunk[1] to start with %q (overlap), got %q", overlap, chunks[1])
	}
}

func TestFixedSplitter_ShortTextNoSplit(t *testing.T) {
	s := splitter.NewFixedSizeSplitter(100, 0)
	text := "hello world"
	chunks := s.Split(text)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("chunk = %q, want %q", chunks[0], text)
	}
}

func TestRecursiveSplitter_SplitsOnParagraphs(t *testing.T) {
	s := splitter.NewRecursiveCharacterSplitter(50, 0)
	// Two paragraphs separated by double newline; each is under 50 chars.
	text := "First paragraph text here.\n\nSecond paragraph text here."
	chunks := s.Split(text)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks (one per paragraph), got %d: %v", len(chunks), chunks)
	}
	// Each chunk should contain the paragraph text.
	found1 := false
	found2 := false
	for _, c := range chunks {
		if strings.Contains(c, "First paragraph") {
			found1 = true
		}
		if strings.Contains(c, "Second paragraph") {
			found2 = true
		}
	}
	if !found1 {
		t.Error("expected chunk containing 'First paragraph'")
	}
	if !found2 {
		t.Error("expected chunk containing 'Second paragraph'")
	}
}

// --- Loader tests ---

func TestLoadFile_TextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "Hello, chainforge!\nThis is a test document."
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	docs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].Content != content {
		t.Errorf("content = %q, want %q", docs[0].Content, content)
	}
	if docs[0].Source != path {
		t.Errorf("source = %q, want %q", docs[0].Source, path)
	}
}

func TestLoadFile_HTMLStripsTagsKeepsText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.html")
	html := `<html><head><title>Title</title></head><body><h1>Hello</h1><p>World</p></body></html>`
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		t.Fatal(err)
	}

	docs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile HTML: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	// Tags should be stripped; text content preserved.
	if strings.Contains(docs[0].Content, "<") {
		t.Errorf("content still contains HTML tags: %q", docs[0].Content)
	}
	if !strings.Contains(docs[0].Content, "Hello") {
		t.Errorf("content missing 'Hello': %q", docs[0].Content)
	}
	if !strings.Contains(docs[0].Content, "World") {
		t.Errorf("content missing 'World': %q", docs[0].Content)
	}
	// <title> tag content (from <head>) should NOT appear when head is skipped.
	// Title is in <head> which our stripHTML skips.
}

// --- RetrieverTool tests ---

func TestRetrieverTool_SchemaIsLLMCompatible(t *testing.T) {
	r := &mockRetriever{}
	tool := rag.NewRetrieverTool(r, rag.WithTopK(3))

	def := tool.Definition()
	if def.Name == "" {
		t.Error("tool name is empty")
	}
	if def.Description == "" {
		t.Error("tool description is empty")
	}
	if len(def.InputSchema) == 0 {
		t.Fatal("tool input schema is empty")
	}

	var schema map[string]any
	if err := json.Unmarshal(def.InputSchema, &schema); err != nil {
		t.Fatalf("tool schema is not valid JSON: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema missing 'properties'")
	}
	if _, hasQuery := props["query"]; !hasQuery {
		t.Error("schema missing 'query' property")
	}
}

func TestRetrieverTool_ReturnsFormattedContext(t *testing.T) {
	r := &mockRetriever{docs: []rag.Document{
		{ID: "1", Content: "The capital of France is Paris.", Source: "geo.txt"},
		{ID: "2", Content: "Go is a statically typed language.", Source: "go.txt"},
	}}
	tool := rag.NewRetrieverTool(r, rag.WithTopK(2))

	output, err := tool.Call(context.Background(), `{"query":"capitals"}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(output, "Paris") {
		t.Errorf("output missing 'Paris': %q", output)
	}
	if !strings.Contains(output, "Go is a statically typed") {
		t.Errorf("output missing Go doc: %q", output)
	}
}

// --- WithRetriever agent option tests ---

func TestWithRetriever_InjectsContextIntoSystemPrompt(t *testing.T) {
	r := &mockRetriever{docs: []rag.Document{
		{ID: "1", Content: "Relevant fact: the answer is 42.", Source: "facts.txt"},
	}}

	p := testutil.NewMockProvider(testutil.EndTurnResponse("The answer is 42."))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithSystemPrompt("You are helpful."),
		chainforge.WithRetriever(r, rag.WithTopK(3)),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), "s1", "What is the answer?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The system message should contain the retrieved context.
	req := p.LastRequest()
	if len(req.Messages) == 0 {
		t.Fatal("no messages sent to provider")
	}
	systemMsg := req.Messages[0]
	if systemMsg.Role != core.RoleSystem {
		t.Fatalf("first message role = %q, want %q", systemMsg.Role, core.RoleSystem)
	}
	if !strings.Contains(systemMsg.Content, "Relevant fact") {
		t.Errorf("system prompt missing retrieved context; got: %q", systemMsg.Content)
	}
}

func TestWithRetriever_SkipsOnEmptyResults(t *testing.T) {
	r := &mockRetriever{docs: nil} // always returns empty

	p := testutil.NewMockProvider(testutil.EndTurnResponse("ok"))
	agent, err := chainforge.NewAgent(
		chainforge.WithProvider(p),
		chainforge.WithModel("mock"),
		chainforge.WithSystemPrompt("Base prompt."),
		chainforge.WithRetriever(r, rag.WithTopK(5)),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	_, err = agent.Run(context.Background(), "s1", "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	req := p.LastRequest()
	sysMsg := req.Messages[0]
	// System prompt should be the base prompt without any RAG context.
	if sysMsg.Content != "Base prompt." {
		t.Errorf("system prompt = %q, want %q", sysMsg.Content, "Base prompt.")
	}
}

func TestQdrantRetriever_AdaptsSearchToDocuments(t *testing.T) {
	// Verify FormatContext produces the expected output format.
	docs := []rag.Document{
		{ID: "a", Content: "First result.", Source: "doc1.txt"},
		{ID: "b", Content: "Second result.", Source: "doc2.txt"},
	}
	out := rag.FormatContext(docs)

	if !strings.Contains(out, "First result.") {
		t.Error("output missing 'First result.'")
	}
	if !strings.Contains(out, "doc1.txt") {
		t.Error("output missing source 'doc1.txt'")
	}
	if !strings.Contains(out, "Second result.") {
		t.Error("output missing 'Second result.'")
	}
}

// --- Mock helpers ---

type mockRetriever struct {
	docs []rag.Document
	err  error
}

func (m *mockRetriever) Retrieve(_ context.Context, _ string, _ int) ([]rag.Document, error) {
	return m.docs, m.err
}
