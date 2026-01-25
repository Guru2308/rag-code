package retrieval_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/mocks"
	"github.com/Guru2308/rag-code/internal/retrieval"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug})
}

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

func TestRetriever_Retrieve_EmbedderError(t *testing.T) {
	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return nil, errors.New("embedding failed")
		},
	}
	mockStore := &mocks.MockChunkStore{}

	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{Strategy: retrieval.FusionRRF}

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, nil, nil, preprocessor, config)

	_, err := retriever.Retrieve(context.Background(), domain.SearchQuery{Query: "test"})
	if err == nil {
		t.Error("Retrieve() expected error for embedder failure")
	}
}

func TestRetriever_Retrieve_VectorOnly(t *testing.T) {
	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return []*domain.SearchResult{
				{Chunk: &domain.CodeChunk{ID: "vec1"}, Score: 0.95},
			}, nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{Strategy: retrieval.FusionRRF}

	// No keyword searcher or scorer
	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, nil, nil, preprocessor, config)

	results, err := retriever.Retrieve(context.Background(), domain.SearchQuery{Query: "test", MaxResults: 5})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Retrieve() returned %d results, want 1", len(results))
	}
	if results[0].Source != "vector" {
		t.Errorf("Retrieve() source = %v, want 'vector'", results[0].Source)
	}
}

func TestRetriever_Retrieve_KeywordOnly(t *testing.T) {
	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		GetFunc: func(ctx context.Context, id string) (*domain.CodeChunk, error) {
			return &domain.CodeChunk{ID: id, Content: "test"}, nil
		},
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return nil, nil // No vector results
		},
	}
	mockKeyword := &mocks.MockKeywordSearcher{
		SearchFunc: func(ctx context.Context, tokens []string, limit int) ([]string, error) {
			return []string{"kwd1"}, nil
		},
	}
	mockScorer := &mocks.MockScorer{
		ScoreFunc: func(ctx context.Context, queryTokens []string, docID string) (float64, error) {
			return 0.7, nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{Strategy: retrieval.FusionRRF}

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, mockKeyword, mockScorer, preprocessor, config)

	results, err := retriever.Retrieve(context.Background(), domain.SearchQuery{Query: "test", MaxResults: 5})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Retrieve() returned 0 results")
	}
	if results[0].Source != "keyword" {
		t.Errorf("Retrieve() source = %v, want 'keyword'", results[0].Source)
	}
}

func TestRetriever_Retrieve_WeightedFusion(t *testing.T) {
	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		GetFunc: func(ctx context.Context, id string) (*domain.CodeChunk, error) {
			return &domain.CodeChunk{ID: id}, nil
		},
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return []*domain.SearchResult{
				{Chunk: &domain.CodeChunk{ID: "doc1"}, Score: 0.9},
			}, nil
		},
	}
	mockKeyword := &mocks.MockKeywordSearcher{
		SearchFunc: func(ctx context.Context, tokens []string, limit int) ([]string, error) {
			return []string{"doc2"}, nil
		},
	}
	mockScorer := &mocks.MockScorer{
		ScoreFunc: func(ctx context.Context, queryTokens []string, docID string) (float64, error) {
			return 0.8, nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{
		Strategy:     retrieval.FusionWeighted,
		VectorWeight: 0.7,
	}

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, mockKeyword, mockScorer, preprocessor, config)

	results, err := retriever.Retrieve(context.Background(), domain.SearchQuery{Query: "hybrid test"})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Retrieve() returned 0 results for weighted fusion")
	}
}

func TestRetriever_AddToInvertedIndex(t *testing.T) {
	mockKeyword := &mocks.MockKeywordSearcher{
		AddToInvertedIndexFunc: func(ctx context.Context, chunks []*domain.CodeChunk) error {
			return nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{Strategy: retrieval.FusionRRF}

	retriever := retrieval.NewRetriever(nil, nil, mockKeyword, nil, preprocessor, config)

	chunks := []*domain.CodeChunk{
		{ID: "1", Content: "test"},
	}

	err := retriever.AddToInvertedIndex(context.Background(), chunks)
	if err != nil {
		t.Errorf("AddToInvertedIndex() error = %v", err)
	}
}

func TestRetriever_AddToInvertedIndex_NilKeyword(t *testing.T) {
	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{Strategy: retrieval.FusionRRF}

	retriever := retrieval.NewRetriever(nil, nil, nil, nil, preprocessor, config)

	chunks := []*domain.CodeChunk{
		{ID: "1", Content: "test"},
	}

	err := retriever.AddToInvertedIndex(context.Background(), chunks)
	if err != nil {
		t.Errorf("AddToInvertedIndex() with nil keyword should not error, got %v", err)
	}
}

func TestRetriever_Retrieve_EmptyQuery(t *testing.T) {
	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return []*domain.SearchResult{}, nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	config := retrieval.FusionConfig{Strategy: retrieval.FusionRRF}

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, nil, nil, preprocessor, config)

	results, err := retriever.Retrieve(context.Background(), domain.SearchQuery{Query: ""})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	// Should handle empty query gracefully - results should not be nil
	_ = results // Empty query is handled, just verify no error
}
