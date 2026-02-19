package hierarchy

import (
	"context"
	"sort"

	"github.com/Guru2308/rag-code/internal/domain"
)

// Processor defines the interface for hierarchical processing of results
type Processor interface {
	Process(ctx context.Context, results []*domain.SearchResult) ([]*domain.SearchResult, error)
}

// HierarchicalFilter organizes results based on codebase structure
type HierarchicalFilter struct {
	MaxResultsPerFile int
}

// NewHierarchicalFilter creates a new hierarchical filter
func NewHierarchicalFilter(maxPerFile int) *HierarchicalFilter {
	if maxPerFile <= 0 {
		maxPerFile = 3
	}
	return &HierarchicalFilter{
		MaxResultsPerFile: maxPerFile,
	}
}

// Process groups results by file and filters them to ensure diversity
func (f *HierarchicalFilter) Process(ctx context.Context, results []*domain.SearchResult) ([]*domain.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Group by file
	byFile := make(map[string][]*domain.SearchResult)
	for _, res := range results {
		file := res.Chunk.FilePath
		byFile[file] = append(byFile[file], res)
	}

	filtered := make([]*domain.SearchResult, 0, len(results))

	// For each file, sort by score and take top N
	for _, fileResults := range byFile {
		sort.Slice(fileResults, func(i, j int) bool {
			return fileResults[i].RelevanceScore > fileResults[j].RelevanceScore
		})

		count := len(fileResults)
		if count > f.MaxResultsPerFile {
			count = f.MaxResultsPerFile
		}
		filtered = append(filtered, fileResults[:count]...)
	}

	// Final sort of all filtered results
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].RelevanceScore > filtered[j].RelevanceScore
	})

	return filtered, nil
}
