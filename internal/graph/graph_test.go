package graph

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
)

func init() {
	// Initialize logger for tests
	logger.Init(logger.Config{
		Level: logger.LevelError, // Use error level to reduce test noise
	})
}

func TestGraph_AddNode(t *testing.T) {
	g := NewGraph()

	node := &Node{
		ID:       "func1",
		Type:     "function",
		Name:     "TestFunc",
		FilePath: "/test/file.go",
		Metadata: map[string]string{"key": "value"},
	}

	g.AddNode(node)

	retrieved, exists := g.GetNode("func1")
	if !exists {
		t.Fatal("Expected node to exist")
	}

	if retrieved.Name != "TestFunc" {
		t.Errorf("Expected name TestFunc, got %s", retrieved.Name)
	}
}

func TestGraph_AddEdge(t *testing.T) {
	g := NewGraph()

	node1 := &Node{ID: "func1", Name: "Caller"}
	node2 := &Node{ID: "func2", Name: "Callee"}

	g.AddNode(node1)
	g.AddNode(node2)
	g.AddEdge("func1", "func2", RelationCall)

	related := g.GetRelated("func1", RelationCall)
	if len(related) != 1 {
		t.Fatalf("Expected 1 related node, got %d", len(related))
	}

	if related[0].Name != "Callee" {
		t.Errorf("Expected Callee, got %s", related[0].Name)
	}
}

func TestGraph_GetNodesByName(t *testing.T) {
	g := NewGraph()

	g.AddNode(&Node{ID: "1", Name: "TestFunc", FilePath: "/a.go"})
	g.AddNode(&Node{ID: "2", Name: "TestFunc", FilePath: "/b.go"})
	g.AddNode(&Node{ID: "3", Name: "OtherFunc", FilePath: "/c.go"})

	nodes := g.GetNodesByName("TestFunc")
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestGraph_GetAllRelated(t *testing.T) {
	g := NewGraph()

	g.AddNode(&Node{ID: "1", Name: "Main"})
	g.AddNode(&Node{ID: "2", Name: "Func1"})
	g.AddNode(&Node{ID: "3", Name: "Func2"})

	g.AddEdge("1", "2", RelationCall)
	g.AddEdge("1", "3", RelationImport)

	related := g.GetAllRelated("1")
	if len(related) != 2 {
		t.Errorf("Expected 2 related nodes, got %d", len(related))
	}
}

func TestGraph_Clear(t *testing.T) {
	g := NewGraph()

	g.AddNode(&Node{ID: "1", Name: "Test"})
	g.AddEdge("1", "2", RelationCall)

	stats := g.Stats()
	if stats["nodes"] != 1 {
		t.Errorf("Expected 1 node before clear, got %d", stats["nodes"])
	}

	g.Clear()

	stats = g.Stats()
	if stats["nodes"] != 0 {
		t.Errorf("Expected 0 nodes after clear, got %d", stats["nodes"])
	}
}

func TestBuilder_Build(t *testing.T) {
	builder := NewBuilder()

	chunks := []*domain.CodeChunk{
		{
			ID:        "chunk1",
			ChunkType: domain.ChunkTypeFunction,
			FilePath:  "/test/a.go",
			Metadata: map[string]string{
				"name":  "main",
				"calls": "helper,fmt.Println",
			},
		},
		{
			ID:        "chunk2",
			ChunkType: domain.ChunkTypeFunction,
			FilePath:  "/test/b.go",
			Metadata: map[string]string{
				"name": "helper",
			},
		},
		{
			ID:        "chunk3",
			ChunkType: domain.ChunkTypeImport,
			FilePath:  "/test/a.go",
			Metadata: map[string]string{
				"imports": "fmt,strings",
			},
		},
	}

	graph := builder.Build(context.Background(), chunks)

	// Check nodes were added
	stats := graph.Stats()
	if stats["nodes"] != 3 {
		t.Errorf("Expected 3 nodes, got %d", stats["nodes"])
	}

	// Check call edge was created
	related := graph.GetRelated("chunk1", RelationCall)
	found := false
	for _, node := range related {
		if node.ID == "chunk2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected call edge from chunk1 to chunk2")
	}
}

func TestBuilder_Rebuild(t *testing.T) {
	builder := NewBuilder()

	chunks1 := []*domain.CodeChunk{
		{ID: "1", ChunkType: domain.ChunkTypeFunction, Metadata: map[string]string{"name": "func1"}},
	}

	graph := builder.Build(context.Background(), chunks1)
	if stats := graph.Stats(); stats["nodes"] != 1 {
		t.Errorf("Expected 1 node, got %d", stats["nodes"])
	}

	chunks2 := []*domain.CodeChunk{
		{ID: "2", ChunkType: domain.ChunkTypeFunction, Metadata: map[string]string{"name": "func2"}},
		{ID: "3", ChunkType: domain.ChunkTypeFunction, Metadata: map[string]string{"name": "func3"}},
	}

	graph = builder.Rebuild(context.Background(), chunks2)
	if stats := graph.Stats(); stats["nodes"] != 2 {
		t.Errorf("Expected 2 nodes after rebuild, got %d", stats["nodes"])
	}

	// Old node should not exist
	if _, exists := graph.GetNode("1"); exists {
		t.Error("Old node should not exist after rebuild")
	}
}
