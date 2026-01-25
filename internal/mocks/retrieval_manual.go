package mocks

import (
	"context"

	"github.com/Guru2308/rag-code/internal/domain"
)

// MockKeywordSearcher implements retrieval.KeywordSearcher
type MockKeywordSearcher struct {
	SearchFunc             func(ctx context.Context, tokens []string, limit int) ([]string, error)
	AddToInvertedIndexFunc func(ctx context.Context, chunks []*domain.CodeChunk) error
}

func (m *MockKeywordSearcher) Search(ctx context.Context, tokens []string, limit int) ([]string, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, tokens, limit)
	}
	return nil, nil
}

func (m *MockKeywordSearcher) AddToInvertedIndex(ctx context.Context, chunks []*domain.CodeChunk) error {
	if m.AddToInvertedIndexFunc != nil {
		return m.AddToInvertedIndexFunc(ctx, chunks)
	}
	return nil
}

// MockScorer implements retrieval.Scorer
type MockScorer struct {
	ScoreFunc func(ctx context.Context, queryTokens []string, docID string) (float64, error)
}

func (m *MockScorer) Score(ctx context.Context, queryTokens []string, docID string) (float64, error) {
	if m.ScoreFunc != nil {
		return m.ScoreFunc(ctx, queryTokens, docID)
	}
	return 0, nil
}
