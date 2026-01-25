package retrieval

import (
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

func TestFuseResults(t *testing.T) {
	vecRes := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "1"}, Score: 0.9},
		{Chunk: &domain.CodeChunk{ID: "2"}, Score: 0.8},
	}
	kwRes := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "2"}, Score: 0.9},
		{Chunk: &domain.CodeChunk{ID: "3"}, Score: 0.7},
	}

	tests := []struct {
		name   string
		config FusionConfig
		check  func([]*domain.SearchResult)
	}{
		{
			"RRF",
			FusionConfig{Strategy: FusionRRF, RRFConstant: 60},
			func(res []*domain.SearchResult) {
				if len(res) != 3 {
					t.Errorf("RRF: expected 3 results, got %d", len(res))
				}
				// ID 2 should be top because it's in both
				if res[0].Chunk.ID != "2" {
					t.Errorf("RRF: expected top result ID 2, got %s", res[0].Chunk.ID)
				}
			},
		},
		{
			"Weighted",
			FusionConfig{Strategy: FusionWeighted, VectorWeight: 0.5},
			func(res []*domain.SearchResult) {
				if len(res) != 3 {
					t.Errorf("Weighted: expected 3 results, got %d", len(res))
				}
			},
		},
		{
			"Max",
			FusionConfig{Strategy: FusionMax},
			func(res []*domain.SearchResult) {
				// 2 has 0.9 from keyword, 0.8 from vector -> max 0.9
				// 1 has 0.9 from vector
				// 3 has 0.7 from keyword
				if len(res) != 3 {
					t.Errorf("Max: expected 3 results, got %d", len(res))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuseResults(vecRes, kwRes, tt.config)
			tt.check(got)
		})
	}
}

func TestNormalizeScores(t *testing.T) {
	// Private function, but we are in package retrieval
	res := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "1"}, Score: 10},
		{Chunk: &domain.CodeChunk{ID: "2"}, Score: 0},
	}
	norm := normalizeScores(res)
	if norm[0].Score != 1.0 {
		t.Errorf("Expected 1.0, got %f", norm[0].Score)
	}
	if norm[1].Score != 0.0 {
		t.Errorf("Expected 0.0, got %f", norm[1].Score)
	}
}
