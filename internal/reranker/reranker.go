package reranker

import (
	"context"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
)

// Reranker defines the interface for reranking search results
type Reranker interface {
	Rerank(ctx context.Context, query string, results []*domain.SearchResult) ([]*domain.SearchResult, error)
}

// ---------------------------------------------------------------------------
// Heuristic Reranker
// ---------------------------------------------------------------------------

// HeuristicReranker implements Reranker using several heuristics:
//   - Code type bonus (functions > methods > classes > imports > comments)
//   - Exact / partial match bonus in content and file path
//   - Recency bonus (recently modified files score higher)
//   - File priority bonus (configurable high-signal path patterns)
type HeuristicReranker struct {
	weights         map[domain.ChunkType]float32
	priorityPaths   []string      // path substrings that deserve a boost (e.g. "cmd/", "api/")
	recencyHalfLife time.Duration // how quickly recency bonus decays
}

// HeuristicConfig allows customisation of the reranker
type HeuristicConfig struct {
	// PriorityPaths are filename substrings that get a recency / priority boost.
	// Example: []string{"cmd/", "api/", "main"}
	PriorityPaths   []string
	RecencyHalfLife time.Duration // default: 30 days
}

// DefaultHeuristicConfig returns sensible defaults
func DefaultHeuristicConfig() HeuristicConfig {
	return HeuristicConfig{
		PriorityPaths:   []string{"cmd/", "api/", "main", "handler", "server"},
		RecencyHalfLife: 30 * 24 * time.Hour,
	}
}

// NewHeuristicReranker creates a new heuristic reranker with default config
func NewHeuristicReranker() *HeuristicReranker {
	return NewHeuristicRerankerWithConfig(DefaultHeuristicConfig())
}

// NewHeuristicRerankerWithConfig creates a reranker with a custom config
func NewHeuristicRerankerWithConfig(cfg HeuristicConfig) *HeuristicReranker {
	halfLife := cfg.RecencyHalfLife
	if halfLife <= 0 {
		halfLife = 30 * 24 * time.Hour
	}
	return &HeuristicReranker{
		weights: map[domain.ChunkType]float32{
			domain.ChunkTypeFunction: 1.2,
			domain.ChunkTypeClass:    1.1,
			domain.ChunkTypeMethod:   1.15,
			domain.ChunkTypeImport:   0.8,
			domain.ChunkTypeComment:  0.5,
			domain.ChunkTypeOther:    1.0,
		},
		priorityPaths:   cfg.PriorityPaths,
		recencyHalfLife: halfLife,
	}
}

// Rerank applies multi-factor heuristics to improve result ordering
func (r *HeuristicReranker) Rerank(ctx context.Context, query string, results []*domain.SearchResult) ([]*domain.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	queryLower := strings.ToLower(query)
	queryTokens := strings.Fields(queryLower)

	for _, res := range results {
		if res.Chunk == nil {
			continue
		}
		score := res.Score

		// 1. Code-type bonus ────────────────────────────────────────────────
		if weight, ok := r.weights[res.Chunk.ChunkType]; ok {
			score *= weight
		}

		// 2. Exact content match ────────────────────────────────────────────
		contentLower := strings.ToLower(res.Chunk.Content)
		if strings.Contains(contentLower, queryLower) {
			score *= 1.5
		}

		// 3. Partial token matches in content ───────────────────────────────
		matchedTokens := 0
		for _, token := range queryTokens {
			if len(token) >= 3 && strings.Contains(contentLower, token) {
				matchedTokens++
			}
		}
		if len(queryTokens) > 0 && matchedTokens > 0 {
			tokenRatio := float32(matchedTokens) / float32(len(queryTokens))
			score *= 1.0 + 0.3*tokenRatio // up to +30% for full token coverage
		}

		// 4. Keyword match in file path ─────────────────────────────────────
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

		// 5. File priority boost ────────────────────────────────────────────
		for _, pattern := range r.priorityPaths {
			if strings.Contains(pathLower, strings.ToLower(pattern)) {
				score *= 1.15
				break
			}
		}

		// 6. Recency bonus ──────────────────────────────────────────────────
		score *= r.recencyBonus(res.Chunk.FilePath)

		res.RelevanceScore = score
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results, nil
}

// recencyBonus returns a multiplier in [1.0, 1.3] based on how recently
// the file was modified relative to r.recencyHalfLife.
// Files modified within the last half-life get a 30% boost that decays
// exponentially. Files that can't be stat-ed get no bonus.
func (r *HeuristicReranker) recencyBonus(filePath string) float32 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 1.0
	}
	age := time.Since(info.ModTime())
	if age < 0 {
		age = 0
	}
	// Exponential decay: bonus = 0.3 * exp(-age / halfLife)
	ratio := float64(age) / float64(r.recencyHalfLife)
	bonus := 0.3 * math.Exp(-ratio)
	logger.Debug("Recency bonus", "file", filePath, "age_days", int(age.Hours()/24), "bonus", bonus)
	return float32(1.0 + bonus)
}

// ---------------------------------------------------------------------------
// MMR – Maximal Marginal Relevance
// ---------------------------------------------------------------------------

// MMRReranker wraps another Reranker and then applies MMR to improve
// diversity: it iteratively selects the result that is both relevant to the
// query and dissimilar to the already-selected results.
//
// MMR score = λ · relevance − (1−λ) · max_similarity(candidate, selected)
type MMRReranker struct {
	inner  Reranker
	lambda float32 // 0 = pure diversity, 1 = pure relevance; default 0.7
}

// NewMMRReranker creates an MMR reranker that wraps an inner reranker.
// lambda controls the relevance / diversity trade-off (0–1, default 0.7).
func NewMMRReranker(inner Reranker, lambda float32) *MMRReranker {
	if lambda < 0 || lambda > 1 {
		lambda = 0.7
	}
	return &MMRReranker{inner: inner, lambda: lambda}
}

// Rerank first delegates to the inner reranker then applies MMR selection.
func (m *MMRReranker) Rerank(ctx context.Context, query string, results []*domain.SearchResult) ([]*domain.SearchResult, error) {
	// Step 1: run the inner reranker to get relevance scores
	ranked, err := m.inner.Rerank(ctx, query, results)
	if err != nil {
		return nil, err
	}
	if len(ranked) == 0 {
		return ranked, nil
	}

	// Step 2: MMR selection
	selected := make([]*domain.SearchResult, 0, len(ranked))
	candidates := make([]*domain.SearchResult, len(ranked))
	copy(candidates, ranked)

	for len(candidates) > 0 {
		bestIdx := -1
		var bestMMR float32 = -1e9

		for i, c := range candidates {
			relevance := c.RelevanceScore
			maxSim := m.maxSimilarity(c, selected)
			mmrScore := m.lambda*relevance - (1-m.lambda)*maxSim

			if bestIdx == -1 || mmrScore > bestMMR {
				bestMMR = mmrScore
				bestIdx = i
			}
		}

		selected = append(selected, candidates[bestIdx])
		candidates = append(candidates[:bestIdx], candidates[bestIdx+1:]...)
	}

	logger.Debug("MMR reranking complete", "input", len(results), "output", len(selected))
	return selected, nil
}

// maxSimilarity returns the maximum cosine similarity between a candidate
// and the already-selected results. Falls back to 0 when embeddings are absent.
func (m *MMRReranker) maxSimilarity(candidate *domain.SearchResult, selected []*domain.SearchResult) float32 {
	if candidate.Chunk == nil || len(candidate.Chunk.Embedding) == 0 {
		return 0
	}
	var max float32
	for _, sel := range selected {
		if sel.Chunk == nil || len(sel.Chunk.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(candidate.Chunk.Embedding, sel.Chunk.Embedding)
		if sim > max {
			max = sim
		}
	}
	return max
}

// cosineSimilarity computes cosine similarity between two float32 vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
