package retrieval

import (
	"context"
	"sort"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/logger"
)

// Retriever handles the retrieval of relevant code chunks
type Retriever struct {
	embedder     indexing.Embedder
	store        indexing.ChunkStore
	redisIdx     *RedisIndex
	bm25         *BM25Scorer
	preprocessor *QueryPreprocessor
	config       FusionConfig
}

// NewRetriever creates a new hybrid retriever
func NewRetriever(
	embedder indexing.Embedder,
	store indexing.ChunkStore,
	redisIdx *RedisIndex,
	bm25 *BM25Scorer,
	preprocessor *QueryPreprocessor,
	config FusionConfig,
) *Retriever {
	return &Retriever{
		embedder:     embedder,
		store:        store,
		redisIdx:     redisIdx,
		bm25:         bm25,
		preprocessor: preprocessor,
		config:       config,
	}
}

// Retrieve finds relevant code chunks for a query using hybrid search
func (r *Retriever) Retrieve(ctx context.Context, query domain.SearchQuery) ([]*domain.SearchResult, error) {
	logger.Info("Retrieving code chunks", "query", query.Query, "max_results", query.MaxResults)

	// 1. Preprocess the query
	processed := r.preprocessor.Preprocess(query.Query)
	if len(processed.Filtered) == 0 {
		logger.Warn("Empty query after preprocessing", "query", query.Query)
		// Return empty results or maybe fallback to original tokens if filtered is empty
	}

	// 2. Dense Retrieval (Vector Search)
	queryVector, err := r.embedder.Embed(ctx, query.Query)
	if err != nil {
		return nil, err
	}

	vectorResults, err := r.vectorSearch(ctx, queryVector, query.MaxResults*2) // Get more for better fusion
	if err != nil {
		logger.Error("Vector search failed", "error", err)
		// Don't fail the whole request if vector search fails, we still have keyword search
	} else {
		for _, res := range vectorResults {
			res.Source = "vector"
		}
	}

	// 3. Sparse Retrieval (Keyword Search via BM25)
	var keywordResults []*domain.SearchResult
	if r.redisIdx != nil && r.bm25 != nil {
		docIDs, err := r.redisIdx.Search(ctx, processed.Filtered, query.MaxResults*2)
		if err != nil {
			logger.Error("Keyword search failed", "error", err)
		} else {
			// Retrieve full chunks for the docIDs and score them
			// For now, we reuse the store to get chunks by ID if possible,
			// or we need a way to get chunks from the store by IDs.
			// Let's assume we can fetch them.
			keywordResults = make([]*domain.SearchResult, 0, len(docIDs))
			for _, id := range docIDs {
				chunk, err := r.store.Get(ctx, id)
				if err != nil {
					continue
				}
				score, err := r.bm25.Score(ctx, processed.Filtered, id)
				if err != nil {
					continue
				}
				keywordResults = append(keywordResults, &domain.SearchResult{
					Chunk:        chunk,
					Score:        float32(score),
					Source:       "keyword",
					KeywordScore: float32(score),
				})
			}
		}
	}

	// 4. Hybrid Fusion
	var combined []*domain.SearchResult
	if len(vectorResults) > 0 && len(keywordResults) > 0 {
		combined = FuseResults(vectorResults, keywordResults, r.config)
	} else if len(vectorResults) > 0 {
		combined = vectorResults
	} else {
		combined = keywordResults
	}

	// 5. Final Sorting and Truncation
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Score > combined[j].Score
	})

	limit := query.MaxResults
	if limit <= 0 {
		limit = 10
	}
	if len(combined) > limit {
		combined = combined[:limit]
	}

	// 6. Calculate Relevance Metadata
	for _, res := range combined {
		res.RelevanceScore = CalculateRelevance(res, query.Query)
	}

	return combined, nil
}

// AddToInvertedIndex adds chunks to the keyword index
func (r *Retriever) AddToInvertedIndex(ctx context.Context, chunks []*domain.CodeChunk) error {
	if r.redisIdx == nil {
		return nil
	}

	indexedDocs := make([]*IndexedDocument, len(chunks))
	for i, chunk := range chunks {
		processed := r.preprocessor.Preprocess(chunk.Content)

		tf := make(map[string]int)
		for _, token := range processed.Tokens {
			tf[token]++
		}

		indexedDocs[i] = &IndexedDocument{
			ID:      chunk.ID,
			Content: chunk.Content,
			Length:  len(processed.Tokens),
			Tokens:  tf,
		}
	}

	return r.redisIdx.AddDocuments(ctx, indexedDocs)
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
