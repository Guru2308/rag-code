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
	embedder indexing.Embedder
	store    indexing.ChunkStore
}

// NewRetriever creates a new hybrid retriever
func NewRetriever(embedder indexing.Embedder, store indexing.ChunkStore) *Retriever {
	return &Retriever{
		embedder: embedder,
		store:    store,
	}
}

// Retrieve finds relevant code chunks for a query
func (r *Retriever) Retrieve(ctx context.Context, query domain.SearchQuery) ([]*domain.SearchResult, error) {
	logger.Debug("Retrieving code chunks", "query", query.Query)

	// 1. Generate embedding for the query
	queryVector, err := r.embedder.Embed(ctx, query.Query)
	if err != nil {
		return nil, err
	}

	// 2. Vector Search (Dense Retrieval)
	// We'll cast to the specific store implementation if needed, or update interface
	// For now, let's assume the store has a Search method or we add it to the interface
	results, err := r.vectorSearch(ctx, queryVector, query.MaxResults)
	if err != nil {
		return nil, err
	}

	// 3. Keyword Search (Sparse Retrieval) - TODO: Implement BM25
	// sparseResults, err := r.keywordSearch(ctx, query.Query)

	// 4. Combine and Rerank (Simple for now)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// vectorSearch performs a vector search.
// Note: We need to adapt the indexer interface or handle the search logic here if it's not in the interface.
func (r *Retriever) vectorSearch(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
	// For now, we'll implement a bridge or extend the interface
	// I'll add a Search method to the indexing.ChunkStore interface or handle it via a new interface
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
