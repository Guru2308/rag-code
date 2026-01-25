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

// MockRedisIndex is a mock for RedisIndex used in BM25 testing
type MockRedisIndex struct {
	GetDocCountFunc       func(ctx context.Context) (int, error)
	GetAvgDocLengthFunc   func(ctx context.Context) (float64, error)
	GetDocLengthFunc      func(ctx context.Context, docID string) (int, error)
	GetTermFrequencyFunc  func(ctx context.Context, term, docID string) (int, error)
	GetDocFrequencyFunc   func(ctx context.Context, term string) (int, error)
}

func (m *MockRedisIndex) GetDocCount(ctx context.Context) (int, error) {
	if m.GetDocCountFunc != nil {
		return m.GetDocCountFunc(ctx)
	}
	return 0, nil
}

func (m *MockRedisIndex) GetAvgDocLength(ctx context.Context) (float64, error) {
	if m.GetAvgDocLengthFunc != nil {
		return m.GetAvgDocLengthFunc(ctx)
	}
	return 0, nil
}

func (m *MockRedisIndex) GetDocLength(ctx context.Context, docID string) (int, error) {
	if m.GetDocLengthFunc != nil {
		return m.GetDocLengthFunc(ctx, docID)
	}
	return 0, nil
}

func (m *MockRedisIndex) GetTermFrequency(ctx context.Context, term, docID string) (int, error) {
	if m.GetTermFrequencyFunc != nil {
		return m.GetTermFrequencyFunc(ctx, term, docID)
	}
	return 0, nil
}

func (m *MockRedisIndex) GetDocFrequency(ctx context.Context, term string) (int, error) {
	if m.GetDocFrequencyFunc != nil {
		return m.GetDocFrequencyFunc(ctx, term)
	}
	return 0, nil
}
