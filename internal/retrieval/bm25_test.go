package retrieval

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/mocks"
)

func TestNewBM25Scorer(t *testing.T) {
	mockRedis := &mocks.MockRedisIndex{}
	scorer := NewBM25Scorer(1.2, 0.75, mockRedis)

	if scorer == nil {
		t.Error("NewBM25Scorer() returned nil")
	}
	if scorer.k1 != 1.2 {
		t.Errorf("NewBM25Scorer() k1 = %v, want 1.2", scorer.k1)
	}
	if scorer.b != 0.75 {
		t.Errorf("NewBM25Scorer() b = %v, want 0.75", scorer.b)
	}
}

func TestBM25Scorer_Score(t *testing.T) {
	mockRedis := &mocks.MockRedisIndex{
		GetDocCountFunc: func(ctx context.Context) (int, error) {
			return 100, nil
		},
		GetAvgDocLengthFunc: func(ctx context.Context) (float64, error) {
			return 50.0, nil
		},
		GetDocLengthFunc: func(ctx context.Context, docID string) (int, error) {
			return 45, nil
		},
		GetTermFrequencyFunc: func(ctx context.Context, term, docID string) (int, error) {
			if term == "test" {
				return 3, nil
			}
			return 0, nil
		},
		GetDocFrequencyFunc: func(ctx context.Context, term string) (int, error) {
			if term == "test" {
				return 10, nil
			}
			return 0, nil
		},
	}

	scorer := NewBM25Scorer(1.2, 0.75, mockRedis)
	score, err := scorer.Score(context.Background(), []string{"test"}, "doc1")

	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}
	if score <= 0 {
		t.Errorf("Score() = %v, want positive score", score)
	}
}

func TestBM25Scorer_Score_ZeroDocCount(t *testing.T) {
	mockRedis := &mocks.MockRedisIndex{
		GetDocCountFunc: func(ctx context.Context) (int, error) {
			return 0, nil
		},
	}

	scorer := NewBM25Scorer(1.2, 0.75, mockRedis)
	score, err := scorer.Score(context.Background(), []string{"test"}, "doc1")

	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}
	if score != 0 {
		t.Errorf("Score() with zero doc count = %v, want 0", score)
	}
}

func TestBM25Scorer_Score_TermNotFound(t *testing.T) {
	mockRedis := &mocks.MockRedisIndex{
		GetDocCountFunc: func(ctx context.Context) (int, error) {
			return 100, nil
		},
		GetAvgDocLengthFunc: func(ctx context.Context) (float64, error) {
			return 50.0, nil
		},
		GetDocLengthFunc: func(ctx context.Context, docID string) (int, error) {
			return 45, nil
		},
		GetTermFrequencyFunc: func(ctx context.Context, term, docID string) (int, error) {
			return 0, nil // Term not found
		},
		GetDocFrequencyFunc: func(ctx context.Context, term string) (int, error) {
			return 0, nil
		},
	}

	scorer := NewBM25Scorer(1.2, 0.75, mockRedis)
	score, err := scorer.Score(context.Background(), []string{"missing"}, "doc1")

	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}
	if score != 0 {
		t.Errorf("Score() for missing term = %v, want 0", score)
	}
}

func TestBM25Scorer_ScoreBatch(t *testing.T) {
	mockRedis := &mocks.MockRedisIndex{
		GetDocCountFunc: func(ctx context.Context) (int, error) {
			return 100, nil
		},
		GetAvgDocLengthFunc: func(ctx context.Context) (float64, error) {
			return 50.0, nil
		},
		GetDocLengthFunc: func(ctx context.Context, docID string) (int, error) {
			return 45, nil
		},
		GetTermFrequencyFunc: func(ctx context.Context, term, docID string) (int, error) {
			return 2, nil
		},
		GetDocFrequencyFunc: func(ctx context.Context, term string) (int, error) {
			return 10, nil
		},
	}

	scorer := NewBM25Scorer(1.2, 0.75, mockRedis)
	scores, err := scorer.ScoreBatch(context.Background(), []string{"test"}, []string{"doc1", "doc2", "doc3"})

	if err != nil {
		t.Fatalf("ScoreBatch() error = %v", err)
	}
	if len(scores) != 3 {
		t.Errorf("ScoreBatch() returned %d scores, want 3", len(scores))
	}
}

func TestBM25Scorer_Explain(t *testing.T) {
	mockRedis := &mocks.MockRedisIndex{
		GetDocCountFunc: func(ctx context.Context) (int, error) {
			return 100, nil
		},
		GetAvgDocLengthFunc: func(ctx context.Context) (float64, error) {
			return 50.0, nil
		},
		GetDocLengthFunc: func(ctx context.Context, docID string) (int, error) {
			return 45, nil
		},
		GetTermFrequencyFunc: func(ctx context.Context, term, docID string) (int, error) {
			if term == "test" {
				return 3, nil
			}
			return 0, nil
		},
		GetDocFrequencyFunc: func(ctx context.Context, term string) (int, error) {
			if term == "test" {
				return 10, nil
			}
			return 0, nil
		},
	}

	scorer := NewBM25Scorer(1.2, 0.75, mockRedis)
	explanation, err := scorer.Explain(context.Background(), []string{"test"}, "doc1")

	if err != nil {
		t.Fatalf("Explain() error = %v", err)
	}
	if explanation == "" {
		t.Error("Explain() returned empty explanation")
	}
	// Check that explanation contains expected sections
	if len(explanation) < 100 {
		t.Errorf("Explain() returned unexpectedly short explanation: %s", explanation)
	}
}

func TestFormatFloat(t *testing.T) {
	result := formatFloat(1.23456789)
	if result != "1.235" {
		t.Errorf("formatFloat(1.23456789) = %s, want 1.235", result)
	}
}

func TestFormatInt(t *testing.T) {
	result := formatInt(12345)
	if result != "12345" {
		t.Errorf("formatInt(12345) = %s, want 12345", result)
	}
}
