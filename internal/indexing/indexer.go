package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/graph"
	"github.com/Guru2308/rag-code/internal/logger"
)

// Indexer coordinates the indexing pipeline
type Indexer struct {
	parser         Parser
	chunker        Chunker
	embedder       Embedder
	store          ChunkStore
	keywordIndexer KeywordIndexer
	graph          *graph.Graph
	mu             sync.RWMutex
	jobs           map[string]*domain.IndexingJob
	numWorkers     int
}

// KeywordIndexer defines the interface for adding chunks to the keyword index
type KeywordIndexer interface {
	AddToInvertedIndex(ctx context.Context, chunks []*domain.CodeChunk) error
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
func NewIndexer(parser Parser, chunker Chunker, embedder Embedder, store ChunkStore, keywordIndexer KeywordIndexer, g *graph.Graph, numWorkers int) *Indexer {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	return &Indexer{
		parser:         parser,
		chunker:        chunker,
		embedder:       embedder,
		store:          store,
		keywordIndexer: keywordIndexer,
		graph:          g,
		jobs:           make(map[string]*domain.IndexingJob),
		numWorkers:     numWorkers,
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

	// Delete existing chunks for this file to prevent stale data
	if err := idx.store.Delete(ctx, filePath); err != nil {
		logger.Warn("Failed to delete old chunks", "path", filePath, "error", err)
		// Continue anyway to ensure new chunks are indexed
	}

	// Store chunks
	if err := idx.store.Store(ctx, processedChunks); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to store chunks")
	}

	// Add to keyword index if available
	if idx.keywordIndexer != nil {
		if err := idx.keywordIndexer.AddToInvertedIndex(ctx, processedChunks); err != nil {
			logger.Error("Failed to add to keyword index", "error", err, "path", filePath)
		}
	}

	// Update Dependency Graph
	if idx.graph != nil {
		builder := graph.NewBuilderWithGraph(idx.graph)
		builder.Build(ctx, processedChunks)
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

// IndexDirectory indexes all files in a directory recursively with concurrent processing
func (idx *Indexer) IndexDirectory(ctx context.Context, dirPath string) error {
	logger.Info("Indexing directory", "path", dirPath)

	// Collect all files to index
	var filesToIndex []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
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
			filesToIndex = append(filesToIndex, path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Process files concurrently using worker pool
	numWorkers := idx.numWorkers
	fileChan := make(chan string, len(filesToIndex))
	errChan := make(chan error, len(filesToIndex))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				if err := idx.IndexFile(ctx, filePath); err != nil {
					logger.Error("Failed to index file in directory", "path", filePath, "error", err)
					errChan <- err
				}
			}
		}()
	}

	// Send files to workers
	for _, filePath := range filesToIndex {
		fileChan <- filePath
	}
	close(fileChan)

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Collect any errors (non-blocking)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		logger.Warn("Some files failed to index", "failed_count", len(errs), "total", len(filesToIndex))
	}

	logger.Info("Directory indexing complete", "total_files", len(filesToIndex), "failed", len(errs))
	return nil
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
