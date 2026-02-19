package hierarchy

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

func TestHierarchicalFilter_Process(t *testing.T) {
	f := NewHierarchicalFilter(2)
	ctx := context.Background()

	results := []*domain.SearchResult{
		{
			Chunk:          &domain.CodeChunk{ID: "1", FilePath: "file1.go"},
			RelevanceScore: 0.9,
		},
		{
			Chunk:          &domain.CodeChunk{ID: "2", FilePath: "file1.go"},
			RelevanceScore: 0.8,
		},
		{
			Chunk:          &domain.CodeChunk{ID: "3", FilePath: "file1.go"},
			RelevanceScore: 0.7, // Should be filtered out (max 2 per file)
		},
		{
			Chunk:          &domain.CodeChunk{ID: "4", FilePath: "file2.go"},
			RelevanceScore: 0.85,
		},
	}

	filtered, err := f.Process(ctx, results)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(filtered) != 3 {
		t.Errorf("Expected 3 results, got %d", len(filtered))
	}

	// Verify ID 3 is missing
	found3 := false
	for _, res := range filtered {
		if res.Chunk.ID == "3" {
			found3 = true
		}
	}
	if found3 {
		t.Errorf("Result ID 3 should have been filtered out")
	}

	// Verify order (by RelevanceScore)
	if filtered[0].Chunk.ID != "1" || filtered[1].Chunk.ID != "4" || filtered[2].Chunk.ID != "2" {
		t.Errorf("Wrong order in filtered results")
		for i, res := range filtered {
			t.Logf("%d: %s (%f)", i, res.Chunk.ID, res.RelevanceScore)
		}
	}
}

func TestHierarchicalFilter_Empty(t *testing.T) {
	f := NewHierarchicalFilter(2)
	filtered, err := f.Process(context.Background(), nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(filtered) != 0 {
		t.Errorf("Expected 0 results, got %d", len(filtered))
	}
}
