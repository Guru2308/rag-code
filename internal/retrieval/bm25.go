package retrieval

import (
	"context"
	"math"
	"strconv"
)

// Red isIndexInterface defines methods needed by BM25 scorer
type RedisIndexInterface interface {
	GetDocCount(ctx context.Context) (int, error)
	GetAvgDocLength(ctx context.Context) (float64, error)
	GetDocLength(ctx context.Context, docID string) (int, error)
	GetTermFrequency(ctx context.Context, term, docID string) (int, error)
	GetDocFrequency(ctx context.Context, term string) (int, error)
}

// BM25Scorer implements the BM25 ranking algorithm
type BM25Scorer struct {
	k1       float64 // term frequency saturation parameter (typically 1.2)
	b        float64 // length normalization parameter (typically 0.75)
	redisIdx RedisIndexInterface
}

// NewBM25Scorer creates a new BM25 scorer
func NewBM25Scorer(k1, b float64, redisIdx RedisIndexInterface) *BM25Scorer {
	return &BM25Scorer{
		k1:       k1,
		b:        b,
		redisIdx: redisIdx,
	}
}

// Score calculates BM25 score for a document given query tokens
func (bm *BM25Scorer) Score(ctx context.Context, queryTokens []string, docID string) (float64, error) {
	// Get collection statistics
	docCount, err := bm.redisIdx.GetDocCount(ctx)
	if err != nil || docCount == 0 {
		return 0, err
	}

	avgDocLength, err := bm.redisIdx.GetAvgDocLength(ctx)
	if err != nil {
		return 0, err
	}

	docLength, err := bm.redisIdx.GetDocLength(ctx, docID)
	if err != nil {
		return 0, err
	}

	// Calculate BM25 score
	score := 0.0

	for _, token := range queryTokens {
		// Get term frequency in this document
		tf, err := bm.redisIdx.GetTermFrequency(ctx, token, docID)
		if err != nil || tf == 0 {
			continue
		}

		// Get document frequency (number of documents containing this term)
		df, err := bm.redisIdx.GetDocFrequency(ctx, token)
		if err != nil || df == 0 {
			continue
		}

		// Calculate IDF: log((N - df + 0.5) / (df + 0.5))
		idf := math.Log((float64(docCount)-float64(df)+0.5)/(float64(df)+0.5) + 1.0)

		// Calculate normalized term frequency
		// TF component: (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * (docLen / avgDocLen)))
		denominator := float64(tf) + bm.k1*(1-bm.b+bm.b*(float64(docLength)/avgDocLength))
		tfComponent := (float64(tf) * (bm.k1 + 1)) / denominator

		// BM25 score for this term
		score += idf * tfComponent
	}

	return score, nil
}

// ScoreBatch calculates BM25 scores for multiple documents
func (bm *BM25Scorer) ScoreBatch(ctx context.Context, queryTokens []string, docIDs []string) (map[string]float64, error) {
	scores := make(map[string]float64)

	for _, docID := range docIDs {
		score, err := bm.Score(ctx, queryTokens, docID)
		if err != nil {
			continue
		}
		scores[docID] = score
	}

	return scores, nil
}

// Explain returns a detailed explanation of the BM25 score calculation
func (bm *BM25Scorer) Explain(ctx context.Context, queryTokens []string, docID string) (string, error) {
	docCount, _ := bm.redisIdx.GetDocCount(ctx)
	avgDocLength, _ := bm.redisIdx.GetAvgDocLength(ctx)
	docLength, _ := bm.redisIdx.GetDocLength(ctx, docID)

	explanation := "BM25 Score Breakdown:\n"
	explanation += "--------------------\n"
	explanation += "Parameters:\n"
	explanation += "  k1 (term freq saturation): " + formatFloat(bm.k1) + "\n"
	explanation += "  b (length normalization): " + formatFloat(bm.b) + "\n"
	explanation += "\n"
	explanation += "Collection Stats:\n"
	explanation += "  Total documents: " + formatInt(docCount) + "\n"
	explanation += "  Avg document length: " + formatFloat(avgDocLength) + "\n"
	explanation += "  This document length: " + formatInt(docLength) + "\n"
	explanation += "\n"
	explanation += "Term Scores:\n"

	totalScore := 0.0

	for _, token := range queryTokens {
		tf, _ := bm.redisIdx.GetTermFrequency(ctx, token, docID)
		df, _ := bm.redisIdx.GetDocFrequency(ctx, token)

		if tf == 0 || df == 0 {
			explanation += "  '" + token + "': not found in document\n"
			continue
		}

		idf := math.Log((float64(docCount)-float64(df)+0.5)/(float64(df)+0.5) + 1.0)
		denominator := float64(tf) + bm.k1*(1-bm.b+bm.b*(float64(docLength)/avgDocLength))
		tfComponent := (float64(tf) * (bm.k1 + 1)) / denominator
		termScore := idf * tfComponent
		totalScore += termScore

		explanation += "  '" + token + "':\n"
		explanation += "    TF: " + formatInt(tf) + "\n"
		explanation += "    DF: " + formatInt(df) + "\n"
		explanation += "    IDF: " + formatFloat(idf) + "\n"
		explanation += "    TF component: " + formatFloat(tfComponent) + "\n"
		explanation += "    Term score: " + formatFloat(termScore) + "\n"
	}

	explanation += "\n"
	explanation += "Total BM25 Score: " + formatFloat(totalScore) + "\n"

	return explanation, nil
}

func formatFloat(f float64) string {
	rounded := math.Round(f*1000) / 1000 // 3 decimal places
	return strconv.FormatFloat(rounded, 'f', 3, 64)
}

func formatInt(i int) string {
	return strconv.Itoa(i)
}
