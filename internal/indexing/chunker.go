package indexing

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/Guru2308/rag-code/internal/domain"
)

// Chunker splits code into semantically meaningful pieces
type Chunker interface {
	Chunk(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error)
}

// SemanticChunker chunks code with awareness of semantic boundaries
type SemanticChunker struct {
	maxSize int
	overlap int
}

// NewSemanticChunker creates a new semantic chunker
func NewSemanticChunker(maxSize, overlap int) *SemanticChunker {
	return &SemanticChunker{
		maxSize: maxSize,
		overlap: overlap,
	}
}

// Chunk processes already-parsed code chunks
// If a chunk exceeds maxSize, it attempts to split it intelligently
func (c *SemanticChunker) Chunk(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error) {
	if maxSize == 0 {
		maxSize = c.maxSize
	}

	result := make([]*domain.CodeChunk, 0)
	now := time.Now()

	for _, chunk := range chunks {
		// Generate unique ID
		chunk.ID = c.generateChunkID(chunk)
		chunk.CreatedAt = now
		chunk.UpdatedAt = now

		// If chunk is within size limit, keep as-is
		if len(chunk.Content) <= maxSize {
			result = append(result, chunk)
			continue
		}

		// Split large chunks
		split := c.splitLargeChunk(chunk, maxSize)
		result = append(result, split...)
	}

	return result, nil
}

// splitLargeChunk splits a chunk that exceeds the maximum size using character-based windows
func (c *SemanticChunker) splitLargeChunk(chunk *domain.CodeChunk, maxSize int) []*domain.CodeChunk {
	content := chunk.Content
	chunks := make([]*domain.CodeChunk, 0)

	// Determine step size to maintain overlap
	step := maxSize - c.overlap
	if step < 1 {
		step = maxSize / 2
	}
	if step < 1 {
		step = 1
	}

	for start := 0; start < len(content); {
		end := start + maxSize
		if end > len(content) {
			end = len(content)
		}

		// Try to find a good breaking point (newline) within the last 20% of the chunk
		breakPoint := end
		if end < len(content) {
			searchRange := maxSize / 5
			if searchRange > 0 {
				if lastNewline := strings.LastIndex(content[start:end], "\n"); lastNewline != -1 && lastNewline > maxSize-searchRange {
					breakPoint = start + lastNewline + 1
				}
			}
		}

		subContent := content[start:breakPoint]

		// Simple line number estimation for the sub-chunk
		startLine := chunk.StartLine + strings.Count(content[:start], "\n")
		endLine := startLine + strings.Count(subContent, "\n")

		subChunk := &domain.CodeChunk{
			FilePath:  chunk.FilePath,
			Language:  chunk.Language,
			Content:   subContent,
			ChunkType: chunk.ChunkType,
			StartLine: startLine,
			EndLine:   endLine,
			Metadata:  chunk.Metadata,
		}
		subChunk.ID = c.generateChunkID(subChunk)
		subChunk.CreatedAt = time.Now()
		subChunk.UpdatedAt = time.Now()

		chunks = append(chunks, subChunk)

		if breakPoint >= len(content) {
			break
		}

		// Move start forward by step, but ensure we don't get stuck
		nextStart := start + step
		if nextStart <= start {
			nextStart = start + 1
		}
		start = nextStart
	}

	return chunks
}

// generateChunkID creates a unique identifier for a chunk
func (c *SemanticChunker) generateChunkID(chunk *domain.CodeChunk) string {
	data := fmt.Sprintf("%s:%d:%d:%s",
		chunk.FilePath,
		chunk.StartLine,
		chunk.EndLine,
		chunk.Content,
	)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:16])
}

// MergeRelatedChunks combines chunks that should be together
// For example, a function and its documentation comment
func (c *SemanticChunker) MergeRelatedChunks(chunks []*domain.CodeChunk) []*domain.CodeChunk {
	// TODO: Implement chunk merging logic
	// - Combine imports
	// - Merge function + docstring
	// - Group related methods
	return chunks
}

// TODO: Implement context-aware splitting (respect code blocks, braces)
// TODO: Add overlapping context for better continuity
// TODO: Preserve important metadata when splitting
