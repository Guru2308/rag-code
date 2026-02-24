package indexing

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
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

// Chunk processes already-parsed code chunks.
// It first merges related chunks (e.g. docstring + function), then splits
// anything that is still too large while preserving overlap and metadata.
func (c *SemanticChunker) Chunk(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error) {
	if maxSize == 0 {
		maxSize = c.maxSize
	}

	// Step 1: merge comment/docstring preceding a function/class
	merged := c.MergeRelatedChunks(chunks)

	result := make([]*domain.CodeChunk, 0, len(merged))
	now := time.Now()

	for _, chunk := range merged {
		// Propagate timestamps if not already set
		if chunk.CreatedAt.IsZero() {
			chunk.CreatedAt = now
		}
		chunk.UpdatedAt = now

		// Ensure metadata is always initialised so sub-chunks can inherit it
		if chunk.Metadata == nil {
			chunk.Metadata = make(map[string]string)
		}

		// Preserve key identifiers in metadata before any split
		c.preserveMetadata(chunk)

		// Generate ID after metadata is finalised
		chunk.ID = c.generateChunkID(chunk)

		if len(chunk.Content) <= maxSize {
			result = append(result, chunk)
			continue
		}

		// Split large chunks with context awareness + overlap
		split := c.splitLargeChunk(chunk, maxSize)
		logger.Debug("Split large chunk", "file", chunk.FilePath, "original_size", len(chunk.Content), "sub_chunks", len(split))
		result = append(result, split...)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Merging
// ---------------------------------------------------------------------------

// MergeRelatedChunks combines logically related chunks that should be kept
// together, specifically a leading comment/docstring and the following
// function or class declaration.
func (c *SemanticChunker) MergeRelatedChunks(chunks []*domain.CodeChunk) []*domain.CodeChunk {
	if len(chunks) == 0 {
		return chunks
	}

	result := make([]*domain.CodeChunk, 0, len(chunks))
	i := 0
	for i < len(chunks) {
		cur := chunks[i]

		// If this is a comment and the next chunk is a function/class/method,
		// merge them so the doc comment travels with its declaration.
		if cur.ChunkType == domain.ChunkTypeComment && i+1 < len(chunks) {
			next := chunks[i+1]
			if next.ChunkType == domain.ChunkTypeFunction ||
				next.ChunkType == domain.ChunkTypeClass ||
				next.ChunkType == domain.ChunkTypeMethod {

				merged := c.mergeTwo(cur, next)
				result = append(result, merged)
				i += 2
				continue
			}
		}

		result = append(result, cur)
		i++
	}
	return result
}

// mergeTwo combines two consecutive chunks into one, keeping the metadata
// of the more specific (non-comment) chunk as the primary.
func (c *SemanticChunker) mergeTwo(comment, code *domain.CodeChunk) *domain.CodeChunk {
	combinedContent := comment.Content + "\n" + code.Content

	meta := make(map[string]string)
	for k, v := range comment.Metadata {
		meta[k] = v
	}
	for k, v := range code.Metadata {
		meta[k] = v // code metadata wins on collision
	}
	meta["merged_from_comment"] = "true"

	merged := &domain.CodeChunk{
		FilePath:     code.FilePath,
		Language:     code.Language,
		Content:      combinedContent,
		ChunkType:    code.ChunkType,
		StartLine:    comment.StartLine,
		EndLine:      code.EndLine,
		Metadata:     meta,
		Dependencies: append(comment.Dependencies, code.Dependencies...),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	merged.ID = c.generateChunkID(merged)
	return merged
}

// ---------------------------------------------------------------------------
// Metadata preservation
// ---------------------------------------------------------------------------

// preserveMetadata ensures key semantic identifiers are stored in the
// chunk's Metadata map so sub-chunks inherit them after splitting.
func (c *SemanticChunker) preserveMetadata(chunk *domain.CodeChunk) {
	if _, ok := chunk.Metadata["chunk_type"]; !ok {
		chunk.Metadata["chunk_type"] = string(chunk.ChunkType)
	}
	if _, ok := chunk.Metadata["language"]; !ok {
		chunk.Metadata["language"] = chunk.Language
	}
	if _, ok := chunk.Metadata["file_path"]; !ok {
		chunk.Metadata["file_path"] = chunk.FilePath
	}
	// Persist function / class name if the parser already set it
	if name, ok := chunk.Metadata["name"]; ok && name != "" {
		chunk.Metadata["symbol_name"] = name
	}
}

// ---------------------------------------------------------------------------
// Splitting
// ---------------------------------------------------------------------------

// splitLargeChunk splits a chunk that exceeds the maximum size.
// It uses brace-depth tracking to seek natural break points and then adds
// an overlapping prefix from the preceding sub-chunk for continuity.
func (c *SemanticChunker) splitLargeChunk(chunk *domain.CodeChunk, maxSize int) []*domain.CodeChunk {
	content := chunk.Content
	chunks := make([]*domain.CodeChunk, 0)
	step := c.calculateStep(maxSize)

	var prevOverlap string // overlap carried forward from previous sub-chunk

	for start := 0; start < len(content); {
		end := start + maxSize
		if end > len(content) {
			end = len(content)
		}

		// Context-aware break: prefer to break at a brace/block boundary
		breakPoint := c.findContextAwareBreakPoint(content, start, end, maxSize)
		subContent := content[start:breakPoint]

		// Prepend overlap from the previous sub-chunk for continuity
		var fullContent string
		if prevOverlap != "" {
			fullContent = prevOverlap + subContent
		} else {
			fullContent = subContent
		}

		subChunk := c.createSubChunk(chunk, fullContent, start)
		chunks = append(chunks, subChunk)

		if breakPoint >= len(content) {
			break
		}

		// Carry forward the last `overlap` characters as context prefix.
		// Align to rune boundaries to avoid invalid UTF-8 from mid-rune slicing.
		if c.overlap > 0 && len(subContent) > 0 {
			overlapStart := len(subContent) - c.overlap
			if overlapStart < 0 {
				overlapStart = 0
			}
			for overlapStart > 0 && !utf8.RuneStart(subContent[overlapStart]) {
				overlapStart--
			}
			prevOverlap = "// ...context...\n" + subContent[overlapStart:]
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

// findContextAwareBreakPoint tries to break at a natural boundary:
//  1. A position where the brace depth returns to zero (block end)
//  2. A blank line (paragraph boundary)
//  3. A newline in the last 20% of the window
//  4. The hard end of the window
func (c *SemanticChunker) findContextAwareBreakPoint(content string, start, end, maxSize int) int {
	if end >= len(content) {
		return end
	}

	window := content[start:end]

	// Strategy 1: find the last position where brace depth returns to 0
	if bp := c.findBlockBoundary(window, start, end); bp > start {
		return bp
	}

	// Strategy 2: blank line (empty line between two newlines)
	if idx := strings.LastIndex(window, "\n\n"); idx != -1 && idx > 0 {
		return start + idx + 2
	}

	// Strategy 3: any newline in last 20%
	searchRange := maxSize / 5
	if searchRange > 0 {
		if lastNewline := strings.LastIndex(window, "\n"); lastNewline != -1 && lastNewline > maxSize-searchRange {
			return start + lastNewline + 1
		}
	}

	return end
}

// findBlockBoundary scans the window and returns the position just after
// the last closing brace where depth returns to 0.
func (c *SemanticChunker) findBlockBoundary(window string, start, end int) int {
	depth := 0
	lastZero := -1
	for i, ch := range window {
		switch ch {
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
			if depth <= 0 {
				depth = 0
				lastZero = i + 1 // position after closing brace
			}
		}
	}

	if lastZero > 0 && lastZero < len(window) {
		return start + lastZero
	}
	return -1
}

func (c *SemanticChunker) createSubChunk(original *domain.CodeChunk, subContent string, startOffset int) *domain.CodeChunk {
	startLine := original.StartLine + strings.Count(original.Content[:startOffset], "\n")
	endLine := startLine + strings.Count(subContent, "\n")

	// Deep-copy metadata so sub-chunks don't share the same map reference
	meta := make(map[string]string, len(original.Metadata)+2)
	for k, v := range original.Metadata {
		meta[k] = v
	}
	meta["start_line"] = fmt.Sprintf("%d", startLine)
	meta["end_line"] = fmt.Sprintf("%d", endLine)

	subChunk := &domain.CodeChunk{
		FilePath:     original.FilePath,
		Language:     original.Language,
		Content:      subContent,
		ChunkType:    original.ChunkType,
		StartLine:    startLine,
		EndLine:      endLine,
		Metadata:     meta,
		Dependencies: original.Dependencies,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	subChunk.ID = c.generateChunkID(subChunk)
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
