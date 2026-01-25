package indexing

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

func TestSemanticChunker_Chunk(t *testing.T) {
	c := NewSemanticChunker(100, 20)
	ctx := context.Background()

	t.Run("small chunk stays as is", func(t *testing.T) {
		chunks := []*domain.CodeChunk{
			{Content: "short content", FilePath: "test.go", StartLine: 1},
		}
		result, err := c.Chunk(ctx, chunks, 0)
		if err != nil {
			t.Fatalf("Chunk() error = %v", err)
		}
		if len(result) != 1 {
			t.Errorf("Expected 1 chunk, got %d", len(result))
		}
		if result[0].Content != "short content" {
			t.Errorf("Content mismatch")
		}
	})

	t.Run("large chunk is split", func(t *testing.T) {
		// Create content larger than 100
		content := ""
		for i := 0; i < 20; i++ {
			content += "line of code " + string(rune(65+i)) + "\n"
		}
		// Content is around 300 chars
		chunks := []*domain.CodeChunk{
			{Content: content, FilePath: "test.go", StartLine: 1},
		}
		result, err := c.Chunk(ctx, chunks, 100)
		if err != nil {
			t.Fatalf("Chunk() error = %v", err)
		}
		if len(result) <= 1 {
			t.Errorf("Expected multiple chunks, got %d", len(result))
		}

		// Check overlap
		// The second chunk should start with the end of the first
	})
}

func TestSemanticChunker_GenerateChunkID(t *testing.T) {
	c := NewSemanticChunker(100, 20)
	chunk := &domain.CodeChunk{
		FilePath:  "test.go",
		StartLine: 1,
		EndLine:   10,
		Content:   "content",
	}
	id1 := c.generateChunkID(chunk)
	id2 := c.generateChunkID(chunk)
	if id1 != id2 {
		t.Error("IDs should be deterministic")
	}

	chunk.Content = "diff"
	id3 := c.generateChunkID(chunk)
	if id1 == id3 {
		t.Error("IDs should change with content")
	}
}
