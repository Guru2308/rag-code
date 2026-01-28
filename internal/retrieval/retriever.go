package retrieval

import (
	"context"
	"sort"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/logger"
)

// KeywordSearcher defines interface for keyword-based search
type KeywordSearcher interface {
	Search(ctx context.Context, tokens []string, limit int) ([]string, error)
	AddToInvertedIndex(ctx context.Context, chunks []*domain.CodeChunk) error
}

// Scorer defines interface for scoring documents
type Scorer interface {
	Score(ctx context.Context, queryTokens []string, docID string) (float64, error)
}

// Retriever handles the retrieval of relevant code chunks
type Retriever struct {
	embedder     indexing.Embedder
	store        indexing.ChunkStore
	keyword      KeywordSearcher
	scorer       Scorer
	preprocessor *QueryPreprocessor
	expander     *ContextExpander
	config       FusionConfig
}

// NewRetriever creates a new hybrid retriever
func NewRetriever(
	embedder indexing.Embedder,
	store indexing.ChunkStore,
	keyword KeywordSearcher,
	scorer Scorer,
	preprocessor *QueryPreprocessor,
	expander *ContextExpander,
	config FusionConfig,
) *Retriever {
	return &Retriever{
		embedder:     embedder,
		store:        store,
		keyword:      keyword,
		scorer:       scorer,
		preprocessor: preprocessor,
		expander:     expander,
		config:       config,
	}
}

// Retrieve finds relevant code chunks for a query using hybrid search
func (r *Retriever) Retrieve(ctx context.Context, query domain.SearchQuery) ([]*domain.SearchResult, error) {
	logger.Info("Retrieving code chunks", "query", query.Query, "max_results", query.MaxResults)

	processed := r.preprocessor.Preprocess(query.Query)
	if len(processed.Filtered) == 0 {
		logger.Warn("Empty query after preprocessing", "query", query.Query)
	}

	vectorResults, err := r.executeVectorSearch(ctx, query)
	if err != nil {
		return nil, err
	}

	keywordResults := r.executeKeywordSearch(ctx, processed.Filtered, query.MaxResults)

	combined := r.combineResults(vectorResults, keywordResults)

	// Finalize initial results
	finalResults := r.finalizeResults(combined, query.MaxResults, query.Query)

	// Apply Phase 4: Context Expansion
	if r.expander != nil {
		// Enabled by default unless explicitly disabled in filters
		enabled := true
		if val, ok := query.Filters["expand_context"]; ok && val == "false" {
			enabled = false
		}

		if enabled {
			// Using DefaultExpandConfig for now
			expanded, err := r.expander.Expand(ctx, finalResults, DefaultExpandConfig())
			if err != nil {
				logger.Error("Context expansion failed", "error", err)
			} else {
				finalResults = expanded
			}
		}
	}

	return finalResults, nil
}

func (r *Retriever) executeVectorSearch(ctx context.Context, query domain.SearchQuery) ([]*domain.SearchResult, error) {
	queryVector, err := r.embedder.Embed(ctx, query.Query)
	if err != nil {
		return nil, err
	}

	vectorResults, err := r.vectorSearch(ctx, queryVector, query.MaxResults*2)
	if err != nil {
		logger.Error("Vector search failed", "error", err)
		return nil, nil
	}

	for _, res := range vectorResults {
		res.Source = "vector"
	}
	return vectorResults, nil
}

func (r *Retriever) executeKeywordSearch(ctx context.Context, tokens []string, limit int) []*domain.SearchResult {
	if r.keyword == nil || r.scorer == nil {
		return nil
	}

	docIDs, err := r.keyword.Search(ctx, tokens, limit*2)
	if err != nil {
		logger.Error("Keyword search failed", "error", err)
		return nil
	}

	results := make([]*domain.SearchResult, 0, len(docIDs))
	for _, id := range docIDs {
		chunk, err := r.store.Get(ctx, id)
		if err != nil {
			continue
		}
		score, err := r.scorer.Score(ctx, tokens, id)
		if err != nil {
			continue
		}
		results = append(results, &domain.SearchResult{
			Chunk:        chunk,
			Score:        float32(score),
			Source:       "keyword",
			KeywordScore: float32(score),
		})
	}
	return results
}

func (r *Retriever) combineResults(vectorResults, keywordResults []*domain.SearchResult) []*domain.SearchResult {
	if len(vectorResults) > 0 && len(keywordResults) > 0 {
		return FuseResults(vectorResults, keywordResults, r.config)
	} else if len(vectorResults) > 0 {
		return vectorResults
	}
	return keywordResults
}

func (r *Retriever) finalizeResults(results []*domain.SearchResult, limit int, query string) []*domain.SearchResult {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit <= 0 {
		limit = 10
	}
	if len(results) > limit {
		results = results[:limit]
	}

	for _, res := range results {
		res.RelevanceScore = CalculateRelevance(res, query)
	}
	return results
}

// AddToInvertedIndex adds chunks to the keyword index
func (r *Retriever) AddToInvertedIndex(ctx context.Context, chunks []*domain.CodeChunk) error {
	if r.keyword == nil {
		return nil
	}
	return r.keyword.AddToInvertedIndex(ctx, chunks)
}

// vectorSearch performs a vector search.
func (r *Retriever) vectorSearch(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
	searchable, ok := r.store.(SearchableStore)
	if !ok {
		logger.Warn("Store does not support direct vector search")
		return nil, nil
	}

	return searchable.Search(ctx, vector, limit)
}

// SearchableStore defines the interface for stores that support vector search
type SearchableStore interface {
	Search(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error)
}
