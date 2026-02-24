package retrieval

import (
	"math"
	"sort"

	"github.com/Guru2308/rag-code/internal/domain"
)

// FusionStrategy defines how to combine multiple search results
type FusionStrategy int

const (
	// FusionRRF uses Reciprocal Rank Fusion
	FusionRRF FusionStrategy = iota
	// FusionWeighted uses weighted linear combination
	FusionWeighted
	// FusionMax takes the maximum score from either source
	FusionMax
)

// FusionConfig holds configuration for result fusion
type FusionConfig struct {
	Strategy     FusionStrategy
	VectorWeight float64 // weight for vector search (0.0 to 1.0)
	RRFConstant  int     // k parameter for RRF (typically 60)
}

// DefaultFusionConfig returns sensible defaults
func DefaultFusionConfig() FusionConfig {
	return FusionConfig{
		Strategy:     FusionRRF,
		VectorWeight: 0.7,
		RRFConstant:  60,
	}
}

// FuseResults combines results from multiple retrieval sources
func FuseResults(vectorResults, keywordResults []*domain.SearchResult, config FusionConfig) []*domain.SearchResult {
	switch config.Strategy {
	case FusionRRF:
		return reciprocalRankFusion(vectorResults, keywordResults, config.RRFConstant)
	case FusionWeighted:
		return weightedCombination(vectorResults, keywordResults, config.VectorWeight)
	case FusionMax:
		return maxScoreFusion(vectorResults, keywordResults)
	default:
		return reciprocalRankFusion(vectorResults, keywordResults, config.RRFConstant)
	}
}

// reciprocalRankFusion implements RRF: score = sum(1 / (k + rank_i))
// This is parameter-free and robust for combining heterogeneous rankers
func reciprocalRankFusion(vectorResults, keywordResults []*domain.SearchResult, k int) []*domain.SearchResult {
	// Build a map of chunk ID to combined RRF score
	scores := make(map[string]*fusionScore)

	// Process vector results
	for rank, result := range vectorResults {
		chunkID := result.Chunk.ID
		if _, exists := scores[chunkID]; !exists {
			scores[chunkID] = &fusionScore{
				chunk:        result.Chunk,
				vectorScore:  result.Score,
				keywordScore: 0,
			}
		}
		scores[chunkID].rrfScore += 1.0 / float32(k+rank+1)
		scores[chunkID].vectorRank = rank + 1
	}

	// Process keyword results
	for rank, result := range keywordResults {
		chunkID := result.Chunk.ID
		if _, exists := scores[chunkID]; !exists {
			scores[chunkID] = &fusionScore{
				chunk:        result.Chunk,
				vectorScore:  0,
				keywordScore: result.Score,
			}
		} else {
			scores[chunkID].keywordScore = result.Score
		}
		scores[chunkID].rrfScore += 1.0 / float32(k+rank+1)
		scores[chunkID].keywordRank = rank + 1
	}

	// Convert to results and sort by RRF score
	results := make([]*domain.SearchResult, 0, len(scores))
	for _, fs := range scores {
		results = append(results, &domain.SearchResult{
			Chunk:        fs.chunk,
			Score:        fs.rrfScore,
			Source:       "hybrid",
			VectorScore:  fs.vectorScore,
			KeywordScore: fs.keywordScore,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// weightedCombination combines scores using a weighted average
// finalScore = α * vectorScore + (1-α) * keywordScore
func weightedCombination(vectorResults, keywordResults []*domain.SearchResult, alpha float64) []*domain.SearchResult {
	// First normalize scores to [0, 1] range
	normalizedVector := normalizeScores(vectorResults)
	normalizedKeyword := normalizeScores(keywordResults)

	// Build a map of chunk ID to combined score
	scores := make(map[string]*fusionScore)

	// Process normalized vector results
	for _, result := range normalizedVector {
		chunkID := result.Chunk.ID
		scores[chunkID] = &fusionScore{
			chunk:       result.Chunk,
			vectorScore: result.Score,
		}
	}

	// Process normalized keyword results
	for _, result := range normalizedKeyword {
		chunkID := result.Chunk.ID
		if fs, exists := scores[chunkID]; exists {
			fs.keywordScore = result.Score
		} else {
			scores[chunkID] = &fusionScore{
				chunk:        result.Chunk,
				keywordScore: result.Score,
			}
		}
	}

	// Calculate weighted combination
	results := make([]*domain.SearchResult, 0, len(scores))
	for _, fs := range scores {
		combinedScore := float32(alpha)*fs.vectorScore + float32(1-alpha)*fs.keywordScore
		results = append(results, &domain.SearchResult{
			Chunk:        fs.chunk,
			Score:        combinedScore,
			Source:       "hybrid",
			VectorScore:  fs.vectorScore,
			KeywordScore: fs.keywordScore,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// maxScoreFusion takes the maximum score from either source
func maxScoreFusion(vectorResults, keywordResults []*domain.SearchResult) []*domain.SearchResult {
	scores := make(map[string]*fusionScore)

	// Process vector results
	for _, result := range vectorResults {
		chunkID := result.Chunk.ID
		scores[chunkID] = &fusionScore{
			chunk:       result.Chunk,
			vectorScore: result.Score,
		}
	}

	// Process keyword results
	for _, result := range keywordResults {
		chunkID := result.Chunk.ID
		if fs, exists := scores[chunkID]; exists {
			fs.keywordScore = result.Score
		} else {
			scores[chunkID] = &fusionScore{
				chunk:        result.Chunk,
				keywordScore: result.Score,
			}
		}
	}

	// Take max score
	results := make([]*domain.SearchResult, 0, len(scores))
	for _, fs := range scores {
		maxScore := fs.vectorScore
		if fs.keywordScore > maxScore {
			maxScore = fs.keywordScore
		}
		results = append(results, &domain.SearchResult{
			Chunk:        fs.chunk,
			Score:        maxScore,
			Source:       "hybrid",
			VectorScore:  fs.vectorScore,
			KeywordScore: fs.keywordScore,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// normalizeScores normalizes scores to [0, 1] range using min-max normalization
func normalizeScores(results []*domain.SearchResult) []*domain.SearchResult {
	if len(results) == 0 {
		return results
	}

	// Find min and max scores
	minScore := results[0].Score
	maxScore := results[0].Score

	for _, result := range results {
		if result.Score < minScore {
			minScore = result.Score
		}
		if result.Score > maxScore {
			maxScore = result.Score
		}
	}

	// Normalize
	scoreRange := maxScore - minScore
	if scoreRange == 0 {
		// All scores are the same, return as is
		return results
	}

	normalized := make([]*domain.SearchResult, len(results))
	for i, result := range results {
		normalizedScore := (result.Score - minScore) / scoreRange
		normalized[i] = &domain.SearchResult{
			Chunk:  result.Chunk,
			Score:  normalizedScore,
			Source: result.Source,
		}
	}

	return normalized
}

// DeduplicateResults removes duplicate chunks based on ID
func DeduplicateResults(results []*domain.SearchResult) []*domain.SearchResult {
	seen := make(map[string]bool)
	deduplicated := make([]*domain.SearchResult, 0, len(results))

	for _, result := range results {
		if !seen[result.Chunk.ID] {
			deduplicated = append(deduplicated, result)
			seen[result.Chunk.ID] = true
		}
	}

	return deduplicated
}

// TruncateResults limits results to a maximum count
func TruncateResults(results []*domain.SearchResult, maxResults int) []*domain.SearchResult {
	if maxResults <= 0 || len(results) <= maxResults {
		return results
	}
	return results[:maxResults]
}

// fusionScore holds intermediate scoring data for fusion
type fusionScore struct {
	chunk        *domain.CodeChunk
	vectorScore  float32
	keywordScore float32
	rrfScore     float32
	vectorRank   int
	keywordRank  int
}

// CalculateRelevance computes a relevance score based on various factors
func CalculateRelevance(result *domain.SearchResult, query string) float32 {
	// This is a placeholder for more sophisticated relevance calculation
	// Could include factors like:
	// - Exact match bonus
	// - Code type match (function vs class)
	// - Recency
	// - File path relevance

	relevance := result.Score

	// Add exact match bonus (case-insensitive)
	// This is simplified - real implementation would be more sophisticated
	chunkContentLower := result.Chunk.Content
	queryLower := query
	if len(chunkContentLower) > 0 && len(queryLower) > 0 {
		// Simple substring check
		// In production, use proper text matching
		_ = math.Max(0, 0) // placeholder to use math
	}

	return relevance
}
