package retrieval_test

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/mocks"
	"github.com/Guru2308/rag-code/internal/retrieval"
)

func TestRetriever_Retrieve(t *testing.T) {
	// Setup mocks
	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		GetFunc: func(ctx context.Context, id string) (*domain.CodeChunk, error) {
			if id == "doc1" {
				return &domain.CodeChunk{ID: "doc1", Content: "func main() {}"}, nil
			}
			return nil, nil // Should return error for not found but nil for simplicity in mock
		},
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return []*domain.SearchResult{
				{Chunk: &domain.CodeChunk{ID: "doc1"}, Score: 0.9},
			}, nil
		},
	}
	mockKeyword := &mocks.MockKeywordSearcher{
		SearchFunc: func(ctx context.Context, tokens []string, limit int) ([]string, error) {
			return []string{"doc1"}, nil
		},
	}
	mockScorer := &mocks.MockScorer{
		ScoreFunc: func(ctx context.Context, queryTokens []string, docID string) (float64, error) {
			if docID == "doc1" {
				return 0.8, nil
			}
			return 0, nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{
		Strategy:     retrieval.FusionRRF,
		VectorWeight: 0.7,
	}

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, mockKeyword, mockScorer, preprocessor, config)

	// Test Retrieve
	results, err := retriever.Retrieve(context.Background(), domain.SearchQuery{Query: "search"})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Retrieve() returned 0 results")
	}
}
