package reranker

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

func TestHeuristicReranker_Rerank(t *testing.T) {
	r := NewHeuristicReranker()
	ctx := context.Background()

	results := []*domain.SearchResult{
		{
			Chunk: &domain.CodeChunk{
				ID:        "1",
				Content:   "func solve() {}",
				ChunkType: domain.ChunkTypeFunction,
				FilePath:  "solver.go",
			},
			Score: 0.8,
		},
		{
			Chunk: &domain.CodeChunk{
				ID:        "2",
				Content:   "// this solves nothing",
				ChunkType: domain.ChunkTypeComment,
				FilePath:  "helper.go",
			},
			Score: 0.9,
		},
		{
			Chunk: &domain.CodeChunk{
				ID:        "3",
				Content:   "type Solver struct {}",
				ChunkType: domain.ChunkTypeClass,
				FilePath:  "models.go",
			},
			Score: 0.7,
		},
	}

	query := "solve"
	reranked, err := r.Rerank(ctx, query, results)
	if err != nil {
		t.Fatalf("Rerank failed: %v", err)
	}

	if len(reranked) != 3 {
		t.Errorf("Expected 3 results, got %d", len(reranked))
	}

	// First result should be the function because it has "solve" in content and is a function
	if reranked[0].Chunk.ID != "1" {
		t.Errorf("Expected first result to be ID 1, got %s (Score: %f, Relevance: %f)",
			reranked[0].Chunk.ID, reranked[0].Score, reranked[0].RelevanceScore)
		for i, res := range reranked {
			t.Logf("Result %d: ID=%s, Score=%f, Relevance=%f, Type=%s", i, res.Chunk.ID, res.Score, res.RelevanceScore, res.Chunk.ChunkType)
		}
	}

	// The comment (ID 2) had higher initial score (0.9) but should be penalized by weight (0.5)
	// Score 2: 0.9 * 0.5 (comment) * 1.5 (contains "solve") = 0.675
	// Score 1: 0.8 * 1.2 (func) * 1.5 (contains "solve") * 1.1 (path contains "solve") = 1.584
	// Score 3: 0.7 * 1.1 (class) = 0.77
}

func TestHeuristicReranker_EmptyResults(t *testing.T) {
	r := NewHeuristicReranker()
	reranked, err := r.Rerank(context.Background(), "query", nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(reranked) != 0 {
		t.Errorf("Expected 0 results, got %d", len(reranked))
	}
}
