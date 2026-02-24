package indexing

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	fileHashes     map[string]string // path -> md5 hash for incremental indexing
	metrics        *IndexMetrics
	batchSize      int
	maxRetries     int
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
	MaxRetries   int
	NumWorkers   int
}

// DefaultConfig returns sensible defaults for the indexer
func DefaultConfig() Config {
	return Config{
		MaxChunkSize: 1500,
		ChunkOverlap: 150,
		BatchSize:    20,
		MaxRetries:   3,
		NumWorkers:   4,
	}
}

// IndexMetrics tracks key statistics for the indexing process
type IndexMetrics struct {
	mu            sync.Mutex
	FilesIndexed  int
	FilesSkipped  int
	FilesErrored  int
	ChunksCreated int
	ChunksRetried int
	TotalDuration time.Duration
	startTime     time.Time
}

func newIndexMetrics() *IndexMetrics {
	return &IndexMetrics{startTime: time.Now()}
}

func (m *IndexMetrics) recordFile(indexed bool, errored bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if errored {
		m.FilesErrored++
	} else if indexed {
		m.FilesIndexed++
	} else {
		m.FilesSkipped++
	}
}

func (m *IndexMetrics) recordChunks(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChunksCreated += n
}

func (m *IndexMetrics) recordRetry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChunksRetried++
}

func (m *IndexMetrics) finish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalDuration = time.Since(m.startTime)
}

// Log emits a single summary log line at INFO level
func (m *IndexMetrics) Log() {
	m.mu.Lock()
	defer m.mu.Unlock()
	logger.Info("Indexing metrics",
		"files_indexed", m.FilesIndexed,
		"files_skipped", m.FilesSkipped,
		"files_errored", m.FilesErrored,
		"chunks_created", m.ChunksCreated,
		"retries", m.ChunksRetried,
		"duration_ms", m.TotalDuration.Milliseconds(),
	)
}

// NewIndexer creates a new indexer with default configuration
func NewIndexer(parser Parser, chunker Chunker, embedder Embedder, store ChunkStore, keywordIndexer KeywordIndexer, g *graph.Graph, numWorkers int) *Indexer {
	cfg := DefaultConfig()
	if numWorkers > 0 {
		cfg.NumWorkers = numWorkers
	}
	return NewIndexerWithConfig(parser, chunker, embedder, store, keywordIndexer, g, cfg)
}

// NewIndexerWithConfig creates a new indexer with explicit configuration
func NewIndexerWithConfig(parser Parser, chunker Chunker, embedder Embedder, store ChunkStore, keywordIndexer KeywordIndexer, g *graph.Graph, cfg Config) *Indexer {
	numWorkers := cfg.NumWorkers
	if numWorkers <= 0 {
		numWorkers = 1
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
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
		fileHashes:     make(map[string]string),
		metrics:        newIndexMetrics(),
		batchSize:      batchSize,
		maxRetries:     maxRetries,
	}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// IndexFile indexes a single file, skipping it when the content has not
// changed since the last run (incremental indexing).
func (idx *Indexer) IndexFile(ctx context.Context, filePath string) error {
	logger.Info("Indexing file", "path", filePath)

	// Detect language
	language := LanguageDetector(filePath)
	if language == "unknown" {
		logger.Debug("Skipping unknown file type", "path", filePath)
		idx.metrics.recordFile(false, false)
		return nil
	}

	// ── Incremental indexing: skip unchanged files ──────────────────────────
	currentHash, err := hashFile(filePath)
	if err != nil {
		logger.Warn("Failed to hash file, will index anyway", "path", filePath, "error", err)
	} else {
		idx.mu.RLock()
		previousHash, seen := idx.fileHashes[filePath]
		idx.mu.RUnlock()
		if seen && previousHash == currentHash {
			logger.Debug("File unchanged, skipping", "path", filePath)
			idx.metrics.recordFile(false, false)
			return nil
		}
	}

	// Parse file into chunks
	chunks, err := idx.parser.Parse(ctx, filePath)
	if err != nil {
		idx.metrics.recordFile(false, true)
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to parse file")
	}

	if len(chunks) == 0 {
		logger.Debug("No chunks extracted from file", "path", filePath)
		idx.metrics.recordFile(false, false)
		return nil
	}

	// Process chunks (split/merge as needed)
	processedChunks, err := idx.chunker.Chunk(ctx, chunks, 0)
	if err != nil {
		idx.metrics.recordFile(false, true)
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to chunk file")
	}

	// ── Batch embedding generation ───────────────────────────────────────────
	if err := idx.embedChunksBatched(ctx, processedChunks); err != nil {
		idx.metrics.recordFile(false, true)
		return err
	}

	// Delete existing chunks for this file to prevent stale data
	if err := idx.store.Delete(ctx, filePath); err != nil {
		logger.Warn("Failed to delete old chunks", "path", filePath, "error", err)
	}

	// ── Store chunks in batches with retry ───────────────────────────────────
	if err := idx.storeChunksBatched(ctx, processedChunks); err != nil {
		idx.metrics.recordFile(false, true)
		return err
	}

	// Update keyword index
	if idx.keywordIndexer != nil {
		if err := idx.keywordIndexer.AddToInvertedIndex(ctx, processedChunks); err != nil {
			logger.Error("Failed to add to keyword index", "error", err, "path", filePath)
		}
	}

	// Update dependency graph
	if idx.graph != nil {
		builder := graph.NewBuilderWithGraph(idx.graph)
		builder.Build(ctx, processedChunks)
	}

	// Persist the hash so we can skip this file next time
	if currentHash != "" {
		idx.mu.Lock()
		idx.fileHashes[filePath] = currentHash
		idx.mu.Unlock()
	}

	idx.metrics.recordFile(true, false)
	idx.metrics.recordChunks(len(processedChunks))

	logger.Info("File indexed successfully",
		"path", filePath,
		"chunks", len(processedChunks),
	)
	return nil
}

// Index handles both files and directories
func (idx *Indexer) Index(ctx context.Context, path string) error {
	idx.metrics = newIndexMetrics() // reset metrics for each top-level run
	defer func() {
		idx.metrics.finish()
		idx.metrics.Log()
	}()

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

	var filesToIndex []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if LanguageDetector(path) != "unknown" {
			filesToIndex = append(filesToIndex, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	numWorkers := idx.numWorkers
	fileChan := make(chan string, len(filesToIndex))
	errChan := make(chan error, len(filesToIndex))

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

	for _, filePath := range filesToIndex {
		fileChan <- filePath
	}
	close(fileChan)
	wg.Wait()
	close(errChan)

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
	idx.mu.Lock()
	delete(idx.fileHashes, filePath)
	idx.mu.Unlock()
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

// Metrics returns the current indexing metrics snapshot
func (idx *Indexer) Metrics() IndexMetrics {
	idx.metrics.mu.Lock()
	defer idx.metrics.mu.Unlock()
	return *idx.metrics
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// embedChunksBatched generates embeddings in configurable batches to avoid
// overwhelming the embedding service and improve throughput.
func (idx *Indexer) embedChunksBatched(ctx context.Context, chunks []*domain.CodeChunk) error {
	for start := 0; start < len(chunks); start += idx.batchSize {
		end := start + idx.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[start:end]

		texts := make([]string, len(batch))
		for i, c := range batch {
			texts[i] = c.Content
		}

		embeddings, err := idx.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return errors.Wrap(err, errors.ErrorTypeExternal, "failed to generate batch embeddings")
		}

		for i, emb := range embeddings {
			batch[i].Embedding = emb
		}

		logger.Debug("Embedded batch", "start", start, "end", end, "total", len(chunks))
	}
	return nil
}

// storeChunksBatched stores chunks in batches and retries individual batches
// on transient failures using exponential back-off.
func (idx *Indexer) storeChunksBatched(ctx context.Context, chunks []*domain.CodeChunk) error {
	for start := 0; start < len(chunks); start += idx.batchSize {
		end := start + idx.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[start:end]

		if err := idx.storeWithRetry(ctx, batch); err != nil {
			return err
		}

		logger.Debug("Stored batch", "start", start, "end", end, "total", len(chunks))
	}
	return nil
}

// storeWithRetry attempts to store a batch of chunks, retrying on error with
// simple exponential back-off.
func (idx *Indexer) storeWithRetry(ctx context.Context, batch []*domain.CodeChunk) error {
	var lastErr error
	for attempt := 0; attempt < idx.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			logger.Warn("Retrying store", "attempt", attempt+1, "backoff_ms", backoff.Milliseconds(), "error", lastErr)
			idx.metrics.recordRetry()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		if err := idx.store.Store(ctx, batch); err != nil {
			lastErr = err
			continue
		}
		return nil // success
	}
	return errors.Wrap(lastErr, errors.ErrorTypeInternal, fmt.Sprintf("failed to store chunk batch after %d retries", idx.maxRetries))
}

// hashFile computes an MD5 hash of a file's content for change detection.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
