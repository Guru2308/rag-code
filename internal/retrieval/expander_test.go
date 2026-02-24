package retrieval

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/graph"
)

// mockChunkStore is a mock implementation of ChunkStore for testing
type mockChunkStore struct {
	chunks map[string]*domain.CodeChunk
}

func newMockChunkStore() *mockChunkStore {
	return &mockChunkStore{
		chunks: make(map[string]*domain.CodeChunk),
	}
}

func (m *mockChunkStore) Store(ctx context.Context, chunks []*domain.CodeChunk) error {
	for _, chunk := range chunks {
		m.chunks[chunk.ID] = chunk
	}
	return nil
}

func (m *mockChunkStore) Get(ctx context.Context, id string) (*domain.CodeChunk, error) {
	chunk, exists := m.chunks[id]
	if !exists {
		return nil, nil
	}
	return chunk, nil
}

func (m *mockChunkStore) Delete(ctx context.Context, filePath string) error {
	// Delete all chunks for this file
	for id, chunk := range m.chunks {
		if chunk.FilePath == filePath {
			delete(m.chunks, id)
		}
	}
	return nil
}

func (m *mockChunkStore) Search(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
	// Simple mock: return empty results
	return []*domain.SearchResult{}, nil
}

func TestContextExpander_Expand(t *testing.T) {
	// Create graph
	g := graph.NewGraph()
	g.AddNode(&graph.Node{ID: "1", Name: "main"})
	g.AddNode(&graph.Node{ID: "2", Name: "helper"})
	g.AddNode(&graph.Node{ID: "3", Name: "util"})
	g.AddEdge("1", "2", graph.RelationCall)
	g.AddEdge("2", "3", graph.RelationCall)

	// Create mock store
	store := newMockChunkStore()
	store.Store(context.Background(), []*domain.CodeChunk{
		{
			ID:      "1",
			Content: "func main() {}",
		},
	})
	store.Store(context.Background(), []*domain.CodeChunk{
		{
			ID:      "2",
			Content: "func helper() {}",
		},
	})
	store.Store(context.Background(), []*domain.CodeChunk{
		{
			ID:      "3",
			Content: "func util() {}",
		},
	})

	expander := NewContextExpander(g, store)

	// Initial results with just main
	results := []*domain.SearchResult{
		{
			Chunk: &domain.CodeChunk{ID: "1", Content: "func main() {}"},
			Score: 1.0,
		},
	}

	config := ExpandConfig{
		IncludeCalledFunctions: true,
		IncludeImports:         false,
		MaxDepth:               1,
		MaxChunks:              10,
	}

	expanded, err := expander.Expand(context.Background(), results, config)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Should include main + helper (depth 1)
	if len(expanded) != 2 {
		t.Errorf("Expected 2 chunks (main + helper), got %d", len(expanded))
	}

	// Test with MaxDepth=2
	config.MaxDepth = 2
	expanded, err = expander.Expand(context.Background(), results, config)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Should include main + helper + util (depth 2)
	if len(expanded) != 3 {
		t.Errorf("Expected 3 chunks (main + helper + util), got %d", len(expanded))
	}
}

func TestContextExpander_NoDuplicates(t *testing.T) {
	g := graph.NewGraph()
	g.AddNode(&graph.Node{ID: "1", Name: "main"})
	g.AddNode(&graph.Node{ID: "2", Name: "shared"})
	g.AddNode(&graph.Node{ID: "3", Name: "func1"})
	g.AddNode(&graph.Node{ID: "4", Name: "func2"})

	// Both func1 and func2 call shared
	g.AddEdge("1", "3", graph.RelationCall)
	g.AddEdge("1", "4", graph.RelationCall)
	g.AddEdge("3", "2", graph.RelationCall)
	g.AddEdge("4", "2", graph.RelationCall)

	store := newMockChunkStore()
	for i := 1; i <= 4; i++ {
		store.Store(context.Background(), []*domain.CodeChunk{
			{
				ID:      string(rune('0' + i)),
				Content: "test",
			},
		})
	}

	expander := NewContextExpander(g, store)

	results := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "1"}},
	}

	config := ExpandConfig{
		IncludeCalledFunctions: true,
		MaxDepth:               2,
		MaxChunks:              10,
	}

	expanded, err := expander.Expand(context.Background(), results, config)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Check no duplicates
	seen := make(map[string]bool)
	for _, result := range expanded {
		if result.Chunk == nil {
			continue
		}
		if seen[result.Chunk.ID] {
			t.Errorf("Duplicate chunk ID: %s", result.Chunk.ID)
		}
		seen[result.Chunk.ID] = true
	}
}

func TestContextExpander_MaxChunks(t *testing.T) {
	g := graph.NewGraph()
	store := newMockChunkStore()

	// Create a chain: 1 -> 2 -> 3 -> 4 -> 5
	for i := 1; i <= 5; i++ {
		id := string(rune('0' + i))
		g.AddNode(&graph.Node{ID: id, Name: "func" + id})
		store.Store(context.Background(), []*domain.CodeChunk{{ID: id}})
		if i > 1 {
			prevID := string(rune('0' + i - 1))
			g.AddEdge(prevID, id, graph.RelationCall)
		}
	}

	expander := NewContextExpander(g, store)

	results := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "1"}},
	}

	config := ExpandConfig{
		IncludeCalledFunctions: true,
		MaxDepth:               10,
		MaxChunks:              3, // Limit to 3 chunks
	}

	expanded, err := expander.Expand(context.Background(), results, config)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	if len(expanded) > 3 {
		t.Errorf("Expected at most 3 chunks, got %d", len(expanded))
	}
}

func TestDefaultExpandConfig(t *testing.T) {
	config := DefaultExpandConfig()

	if !config.IncludeCalledFunctions {
		t.Error("Expected IncludeCalledFunctions to be true")
	}

	if config.IncludeImports {
		t.Error("Expected IncludeImports to be false")
	}

	if !config.IncludeParentType {
		t.Error("Expected IncludeParentType to be true")
	}

	if !config.IncludeChildMethods {
		t.Error("Expected IncludeChildMethods to be true")
	}

	if config.MaxDepth != 1 {
		t.Errorf("Expected MaxDepth=1, got %d", config.MaxDepth)
	}

	if config.MaxChunks != 50 {
		t.Errorf("Expected MaxChunks=50, got %d", config.MaxChunks)
	}
}

func TestContextExpander_RelationDefine(t *testing.T) {
	g := graph.NewGraph()
	// Class "MyStruct" with method "DoSomething"
	g.AddNode(&graph.Node{ID: "class1", Name: "MyStruct", Type: "class"})
	g.AddNode(&graph.Node{ID: "method1", Name: "DoSomething", Type: "method"})
	g.AddEdge("class1", "method1", graph.RelationDefine)

	store := newMockChunkStore()
	chunks := []*domain.CodeChunk{
		{ID: "class1", Content: "type MyStruct struct {}", ChunkType: domain.ChunkTypeClass},
		{ID: "method1", Content: "func (m *MyStruct) DoSomething() {}", ChunkType: domain.ChunkTypeMethod},
	}
	if err := store.Store(context.Background(), chunks); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	// Verify store has both chunks
	if c, _ := store.Get(context.Background(), "class1"); c == nil {
		t.Fatal("Store should have class1 chunk")
	}
	if c, _ := store.Get(context.Background(), "method1"); c == nil {
		t.Fatal("Store should have method1 chunk")
	}

	// Verify graph has correct RelationDefine edges
	parents := g.GetIncoming("method1", graph.RelationDefine)
	if len(parents) != 1 || parents[0].ID != "class1" {
		t.Fatalf("GetIncoming(method1, RelationDefine) should return [class1], got %v", parents)
	}
	children := g.GetRelated("class1", graph.RelationDefine)
	if len(children) != 1 || children[0].ID != "method1" {
		t.Fatalf("GetRelated(class1, RelationDefine) should return [method1], got %v", children)
	}

	expander := NewContextExpander(g, store)

	// Retrieve method -> should expand to include parent class
	results := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "method1", Content: "func (m *MyStruct) DoSomething() {}"}},
	}

	config := ExpandConfig{
		IncludeParentType:     true,
		IncludeChildMethods:   true,
		IncludeCallers:        false,
		IncludeCalledFunctions: false,
		MaxDepth:              1, // Must be >= 1 to allow expansion at depth 0
		MaxChunks:             10,
	}

	expanded, err := expander.Expand(context.Background(), results, config)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Should include method + parent class
	if len(expanded) != 2 {
		t.Errorf("Expected 2 chunks (method + parent class), got %d", len(expanded))
	}

	// Retrieve class -> should expand to include child method
	results2 := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "class1", Content: "type MyStruct struct {}"}},
	}
	expanded2, err := expander.Expand(context.Background(), results2, config)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	if len(expanded2) != 2 {
		t.Errorf("Expected 2 chunks (class + child method), got %d", len(expanded2))
	}
}
