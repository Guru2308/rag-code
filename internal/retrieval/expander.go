package retrieval

import (
	"context"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/graph"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/logger"
)

// ContextExpander expands search results by including related code chunks
type ContextExpander struct {
	graph *graph.Graph
	store indexing.ChunkStore
}

// NewContextExpander creates a new context expander
func NewContextExpander(g *graph.Graph, store indexing.ChunkStore) *ContextExpander {
	return &ContextExpander{
		graph: g,
		store: store,
	}
}

// ExpandConfig configures context expansion behavior
type ExpandConfig struct {
	IncludeCalledFunctions bool // Include functions called by retrieved chunks
	IncludeImports         bool // Include imported modules
	MaxDepth               int  // Maximum depth for recursive expansion
	MaxChunks              int  // Maximum number of chunks to return
}

// DefaultExpandConfig returns sensible defaults for expansion
func DefaultExpandConfig() ExpandConfig {
	return ExpandConfig{
		IncludeCalledFunctions: true,
		IncludeImports:         false, // Imports are usually too broad
		MaxDepth:               1,     // Only direct dependencies
		MaxChunks:              50,    // Limit total context size
	}
}

// Expand takes initial search results and expands them with related chunks
func (e *ContextExpander) Expand(
	ctx context.Context,
	results []*domain.SearchResult,
	config ExpandConfig,
) ([]*domain.SearchResult, error) {
	if e.graph == nil {
		logger.Warn("No graph available for context expansion")
		return results, nil
	}

	expanded := make([]*domain.SearchResult, 0, len(results)*2)
	seen := make(map[string]bool)

	// Add all original results
	for _, result := range results {
		if result.Chunk == nil || result.Chunk.ID == "" {
			continue
		}
		expanded = append(expanded, result)
		seen[result.Chunk.ID] = true
	}

	// Expand each result
	for _, result := range results {
		if result.Chunk == nil || result.Chunk.ID == "" {
			continue
		}

		// Check limit before expanding more
		if len(expanded) >= config.MaxChunks {
			break
		}

		relatedChunks := e.getRelatedChunks(ctx, result.Chunk.ID, config, seen, 0, len(expanded))
		for _, chunk := range relatedChunks {
			if len(expanded) >= config.MaxChunks {
				break
			}
			expanded = append(expanded, chunk)
		}
	}

	logger.Info("Expanded context",
		"original", len(results),
		"expanded", len(expanded),
	)

	return expanded, nil
}

// getRelatedChunks recursively retrieves related chunks
func (e *ContextExpander) getRelatedChunks(
	ctx context.Context,
	chunkID string,
	config ExpandConfig,
	seen map[string]bool,
	depth int,
	currentCount int,
) []*domain.SearchResult {
	if depth >= config.MaxDepth {
		return nil
	}

	// Stop if we're approaching the limit
	if currentCount >= config.MaxChunks {
		return nil
	}

	related := make([]*domain.SearchResult, 0)

	// Get related nodes from graph
	if config.IncludeCalledFunctions {
		calledNodes := e.graph.GetRelated(chunkID, graph.RelationCall)
		logger.Debug("Checking related nodes",
			"chunk_id", chunkID,
			"relation", "call",
			"count", len(calledNodes))

		for _, node := range calledNodes {
			if seen[node.ID] {
				continue
			}

			// Check limit before adding more
			if currentCount+len(related) >= config.MaxChunks {
				break
			}

			chunk, err := e.store.Get(ctx, node.ID)
			if err != nil {
				logger.Debug("Failed to retrieve related chunk", "id", node.ID, "error", err)
				continue
			}

			logger.Info("Found related chunk",
				"source_id", chunkID,
				"target_id", node.ID,
				"target_name", node.Name,
				"type", "call")

			seen[node.ID] = true
			related = append(related, &domain.SearchResult{
				Chunk:          chunk,
				Score:          0.5, // Lower score for expanded context
				Source:         "expansion",
				RelevanceScore: 0.5,
			})

			// Recursively expand
			if depth+1 < config.MaxDepth && currentCount+len(related) < config.MaxChunks {
				nested := e.getRelatedChunks(ctx, node.ID, config, seen, depth+1, currentCount+len(related))
				for _, n := range nested {
					if currentCount+len(related) >= config.MaxChunks {
						break
					}
					related = append(related, n)
				}
			}
		}
	}

	if config.IncludeImports && currentCount+len(related) < config.MaxChunks {
		importedNodes := e.graph.GetRelated(chunkID, graph.RelationImport)
		for _, node := range importedNodes {
			if seen[node.ID] {
				continue
			}

			// Check limit before adding more
			if currentCount+len(related) >= config.MaxChunks {
				break
			}

			chunk, err := e.store.Get(ctx, node.ID)
			if err != nil {
				logger.Debug("Failed to retrieve import chunk", "id", node.ID, "error", err)
				continue
			}

			seen[node.ID] = true
			related = append(related, &domain.SearchResult{
				Chunk:          chunk,
				Score:          0.3, // Even lower score for imports
				Source:         "expansion",
				RelevanceScore: 0.3,
			})
		}
	}

	return related
}
