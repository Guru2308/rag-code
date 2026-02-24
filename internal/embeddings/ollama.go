package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
)

// OllamaEmbedder implements the Embedder interface using Ollama
type OllamaEmbedder struct {
	baseURL       string
	model         string
	client        *http.Client
	numWorkers    int         // parallel workers per EmbedBatch call
	sem           chan struct{} // limits total concurrent Ollama requests
}

// NewOllamaEmbedder creates a new Ollama embedder with default parallelism (4 workers)
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	return NewOllamaEmbedderWithConfig(baseURL, model, 4, 16)
}

// NewOllamaEmbedderWithWorkers creates a new Ollama embedder with configurable parallelism
func NewOllamaEmbedderWithWorkers(baseURL, model string, numWorkers int) *OllamaEmbedder {
	return NewOllamaEmbedderWithConfig(baseURL, model, numWorkers, numWorkers*2)
}

// NewOllamaEmbedderWithConfig creates an embedder with full concurrency control.
// numWorkers: workers per EmbedBatch call. maxConcurrent: global cap on concurrent Ollama requests.
func NewOllamaEmbedderWithConfig(baseURL, model string, numWorkers, maxConcurrent int) *OllamaEmbedder {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	if maxConcurrent <= 0 {
		maxConcurrent = numWorkers * 2
	}
	return &OllamaEmbedder{
		baseURL:    baseURL,
		model:      model,
		numWorkers: numWorkers,
		sem:        make(chan struct{}, maxConcurrent),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// maxEmbeddingChars is a conservative limit for all-minilm (512 token limit).
// Use 384 chars (~128-192 tokens) to guarantee success across Ollama versions.
const maxEmbeddingChars = 384

func truncateForEmbedding(text string) string {
	// Sanitize invalid UTF-8 first (Ollama may reject it)
	text = strings.ToValidUTF8(text, "\ufffd")
	if utf8.RuneCountInString(text) <= maxEmbeddingChars {
		return text
	}
	// Truncate at rune boundary to avoid invalid UTF-8
	runes := []rune(text)
	truncated := string(runes[:maxEmbeddingChars])
	logger.Debug("Truncated chunk for embedding", "original_runes", len(runes), "truncated_runes", maxEmbeddingChars)
	return truncated
}

// Embed generates an embedding for a single text
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Limit concurrent Ollama requests to avoid overwhelming the service
	select {
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	text = truncateForEmbedding(text)
	reqBody := embeddingRequest{
		Model:  e.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "failed to marshal request")
	}

	url := fmt.Sprintf("%s/api/embeddings", e.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to send request to Ollama")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.New(errors.ErrorTypeExternal, fmt.Sprintf("Ollama returned non-200 status: %d, body: %s", resp.StatusCode, string(body)))
	}

	var res embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "failed to decode response")
	}

	return res.Embedding, nil
}

// embeddingJob carries position + text for a worker
type embeddingJob struct {
	index int
	text  string
}

// embeddingResult carries position + result (or error) from a worker
type embeddingResult struct {
	index     int
	embedding []float32
	err       error
}

// EmbedBatch generates embeddings for multiple texts in parallel using a
// worker pool. Results are returned in the same order as the input texts.
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	logger.Debug("Generating batch embeddings", "count", len(texts), "workers", e.numWorkers)

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	jobs := make(chan embeddingJob, len(texts))
	results := make(chan embeddingResult, len(texts))

	// Start worker pool
	var wg sync.WaitGroup
	numWorkers := e.numWorkers
	if numWorkers > len(texts) {
		numWorkers = len(texts)
	}
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				emb, err := e.Embed(ctx, job.text)
				results <- embeddingResult{index: job.index, embedding: emb, err: err}
			}
		}()
	}

	// Send jobs (truncate oversized texts to avoid context length errors)
	for i, text := range texts {
		jobs <- embeddingJob{index: i, text: truncateForEmbedding(text)}
	}
	close(jobs)

	// Wait and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results preserving order
	ordered := make([][]float32, len(texts))
	for res := range results {
		if res.err != nil {
			return nil, fmt.Errorf("embedding worker failed on index %d: %w", res.index, res.err)
		}
		ordered[res.index] = res.embedding
	}

	return ordered, nil
}
