package reranker

import (
	"context"
	"sort"
	"strings"

	"github.com/Guru2308/rag-code/internal/domain"
)

// Reranker defines the interface for reranking search results
type Reranker interface {
	Rerank(ctx context.Context, query string, results []*domain.SearchResult) ([]*domain.SearchResult, error)
}

// HeuristicReranker implements Reranker using simple heuristics
type HeuristicReranker struct {
	weights map[domain.ChunkType]float32
}

// NewHeuristicReranker creates a new heuristic reranker
func NewHeuristicReranker() *HeuristicReranker {
	return &HeuristicReranker{
		weights: map[domain.ChunkType]float32{
			domain.ChunkTypeFunction: 1.2,
			domain.ChunkTypeClass:    1.1,
			domain.ChunkTypeMethod:   1.15,
			domain.ChunkTypeImport:   0.8,
			domain.ChunkTypeComment:  0.5,
			domain.ChunkTypeOther:    1.0,
		},
	}
}

// Rerank applies heuristics to improve search result relevance
func (r *HeuristicReranker) Rerank(ctx context.Context, query string, results []*domain.SearchResult) ([]*domain.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	queryLower := strings.ToLower(query)
	queryTokens := strings.Fields(queryLower)

	for _, res := range results {
		score := res.Score

		// 1. Code Type Bonus
		if weight, ok := r.weights[res.Chunk.ChunkType]; ok {
			score *= weight
		}

		// 2. Exact Match Bonus in Content
		contentLower := strings.ToLower(res.Chunk.Content)
		if strings.Contains(contentLower, queryLower) {
			score *= 1.5
		}

		// 3. Keyword Match in FilePath
		pathLower := strings.ToLower(res.Chunk.FilePath)
		for _, token := range queryTokens {
			if len(token) < 3 {
				continue
			}
			if strings.Contains(pathLower, token) {
				score *= 1.1
				break
			}
		}

		res.RelevanceScore = score
	}

	// Sort by the new relevance score
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results, nil
}
