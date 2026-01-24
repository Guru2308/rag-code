package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
)

// Indexer coordinates the indexing pipeline
type Indexer struct {
	parser   Parser
	chunker  Chunker
	embedder Embedder
	store    ChunkStore
	mu       sync.RWMutex
	jobs     map[string]*domain.IndexingJob
}

// Embedder generates embeddings for code chunks
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// ChunkStore persists code chunks
type ChunkStore interface {
	Store(ctx context.Context, chunks []*domain.CodeChunk) error
	Delete(ctx context.Context, filePath string) error
	Get(ctx context.Context, id string) (*domain.CodeChunk, error)
	Search(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error)
}

// Config holds indexer configuration
type Config struct {
	MaxChunkSize int
	ChunkOverlap int
	BatchSize    int
}

// NewIndexer creates a new indexer
func NewIndexer(parser Parser, chunker Chunker, embedder Embedder, store ChunkStore) *Indexer {
	return &Indexer{
		parser:   parser,
		chunker:  chunker,
		embedder: embedder,
		store:    store,
		jobs:     make(map[string]*domain.IndexingJob),
	}
}

// IndexFile indexes a single file
func (idx *Indexer) IndexFile(ctx context.Context, filePath string) error {
	logger.Info("Indexing file", "path", filePath)

	// Detect language
	language := LanguageDetector(filePath)
	if language == "unknown" {
		logger.Debug("Skipping unknown file type", "path", filePath)
		return nil
	}

	// Parse file into chunks
	chunks, err := idx.parser.Parse(ctx, filePath)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to parse file")
	}

	if len(chunks) == 0 {
		logger.Debug("No chunks extracted from file", "path", filePath)
		return nil
	}

	// Process chunks (split if needed)
	processedChunks, err := idx.chunker.Chunk(ctx, chunks, 0)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to chunk file")
	}

	// Generate embeddings
	texts := make([]string, len(processedChunks))
	for i, chunk := range processedChunks {
		texts[i] = chunk.Content
	}

	embeddings, err := idx.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeExternal, "failed to generate embeddings")
	}

	// Attach embeddings to chunks
	for i, emb := range embeddings {
		processedChunks[i].Embedding = emb
	}

	// Store chunks
	if err := idx.store.Store(ctx, processedChunks); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to store chunks")
	}

	logger.Info("File indexed successfully",
		"path", filePath,
		"chunks", len(processedChunks),
	)

	return nil
}

// Index handles both files and directories
func (idx *Indexer) Index(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "failed to stat path")
	}

	if info.IsDir() {
		return idx.IndexDirectory(ctx, path)
	}
	return idx.IndexFile(ctx, path)
}

// IndexDirectory indexes all files in a directory recursively
func (idx *Indexer) IndexDirectory(ctx context.Context, dirPath string) error {
	logger.Info("Indexing directory", "path", dirPath)

	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and common ignore patterns
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only index files with known languages
		if LanguageDetector(path) != "unknown" {
			if err := idx.IndexFile(ctx, path); err != nil {
				logger.Error("Failed to index file in directory", "path", path, "error", err)
				// Continue with other files instead of stopping the whole walk
			}
		}

		return nil
	})
}

// DeleteFile removes a file from the index
func (idx *Indexer) DeleteFile(ctx context.Context, filePath string) error {
	logger.Info("Deleting file from index", "path", filePath)
	return idx.store.Delete(ctx, filePath)
}

// GetJob returns the status of an indexing job
func (idx *Indexer) GetJob(jobID string) (*domain.IndexingJob, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	job, ok := idx.jobs[jobID]
	if !ok {
		return nil, errors.NotFoundError("job not found")
	}

	return job, nil
}

// TODO: Implement batch processing
// TODO: Add retry logic for failed chunks
// TODO: Implement incremental indexing (only changed files)
// TODO: Add metrics and monitoring
