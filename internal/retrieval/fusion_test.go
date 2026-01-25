package retrieval

import (
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

func TestDefaultFusionConfig(t *testing.T) {
	config := DefaultFusionConfig()
	if config.Strategy != FusionRRF {
		t.Errorf("DefaultFusionConfig() strategy = %v, want %v", config.Strategy, FusionRRF)
	}
	if config.VectorWeight != 0.7 {
		t.Errorf("DefaultFusionConfig() vector weight = %v, want %v", config.VectorWeight, 0.7)
	}
	if config.RRFConstant != 60 {
		t.Errorf("DefaultFusionConfig() RRF constant = %v, want %v", config.RRFConstant, 60)
	}
}

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

func TestDeduplicateResults(t *testing.T) {
	results := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "1", Content: "code1"}, Score: 0.9},
		{Chunk: &domain.CodeChunk{ID: "2", Content: "code2"}, Score: 0.8},
		{Chunk: &domain.CodeChunk{ID: "1", Content: "code1"}, Score: 0.7}, // duplicate
	}

	dedup := DeduplicateResults(results)
	if len(dedup) != 2 {
		t.Errorf("DeduplicateResults() expected 2 results, got %d", len(dedup))
	}

	// Should keep the higher score
	if dedup[0].Chunk.ID != "1" || dedup[0].Score != 0.9 {
		t.Errorf("DeduplicateResults() expected ID 1 with score 0.9")
	}
}

func TestTruncateResults(t *testing.T) {
	results := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "1"}, Score: 0.9},
		{Chunk: &domain.CodeChunk{ID: "2"}, Score: 0.8},
		{Chunk: &domain.CodeChunk{ID: "3"}, Score: 0.7},
		{Chunk: &domain.CodeChunk{ID: "4"}, Score: 0.6},
	}

	truncated := TruncateResults(results, 2)
	if len(truncated) != 2 {
		t.Errorf("TruncateResults() expected 2 results, got %d", len(truncated))
	}
	if truncated[0].Chunk.ID != "1" {
		t.Errorf("TruncateResults() expected first ID to be 1, got %s", truncated[0].Chunk.ID)
	}
	if truncated[1].Chunk.ID != "2" {
		t.Errorf("TruncateResults() expected second ID to be 2, got %s", truncated[1].Chunk.ID)
	}
}

func TestTruncateResults_LimitLargerThanResults(t *testing.T) {
	results := []*domain.SearchResult{
		{Chunk: &domain.CodeChunk{ID: "1"}, Score: 0.9},
	}

	truncated := TruncateResults(results, 10)
	if len(truncated) != 1 {
		t.Errorf("TruncateResults() expected 1 result, got %d", len(truncated))
	}
}
