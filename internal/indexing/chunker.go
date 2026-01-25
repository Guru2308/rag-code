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
	step := c.calculateStep(maxSize)

	for start := 0; start < len(content); {
		end := start + maxSize
		if end > len(content) {
			end = len(content)
		}

		breakPoint := c.findBreakPoint(content, start, end, maxSize)
		subContent := content[start:breakPoint]

		subChunk := c.createSubChunk(chunk, subContent, start)
		chunks = append(chunks, subChunk)

		if breakPoint >= len(content) {
			break
		}

		start = c.nextStart(start, step)
	}

	return chunks
}

func (c *SemanticChunker) calculateStep(maxSize int) int {
	step := maxSize - c.overlap
	if step < 1 {
		step = maxSize / 2
	}
	if step < 1 {
		step = 1
	}
	return step
}

func (c *SemanticChunker) findBreakPoint(content string, start, end, maxSize int) int {
	if end >= len(content) {
		return end
	}

	// Try to find a good breaking point (newline) within the last 20% of the chunk
	searchRange := maxSize / 5
	if searchRange > 0 {
		if lastNewline := strings.LastIndex(content[start:end], "\n"); lastNewline != -1 && lastNewline > maxSize-searchRange {
			return start + lastNewline + 1
		}
	}
	return end
}

func (c *SemanticChunker) createSubChunk(original *domain.CodeChunk, subContent string, startOffset int) *domain.CodeChunk {
	// Simple line number estimation for the sub-chunk
	startLine := original.StartLine + strings.Count(original.Content[:startOffset], "\n")
	endLine := startLine + strings.Count(subContent, "\n")

	subChunk := &domain.CodeChunk{
		FilePath:  original.FilePath,
		Language:  original.Language,
		Content:   subContent,
		ChunkType: original.ChunkType,
		StartLine: startLine,
		EndLine:   endLine,
		Metadata:  original.Metadata,
	}
	subChunk.ID = c.generateChunkID(subChunk)
	subChunk.CreatedAt = time.Now()
	subChunk.UpdatedAt = time.Now()
	return subChunk
}

func (c *SemanticChunker) nextStart(currentStart, step int) int {
	nextStart := currentStart + step
	if nextStart <= currentStart {
		return currentStart + 1
	}
	return nextStart
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
