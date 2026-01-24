package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
)

// OllamaEmbedder implements the Embedder interface using Ollama
type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaEmbedder creates a new Ollama embedder
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
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

// Embed generates an embedding for a single text
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
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

// EmbedBatch generates embeddings for multiple texts
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	logger.Debug("Generating batch embeddings", "count", len(texts))

	results := make([][]float32, len(texts))
	// Ollama currently doesn't have a native batch embeddings endpoint in one call for multiple prompts
	// in the same way OpenAI does, so we iterate.
	// TODO: Parallelize with worker pool if needed
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}

	return results, nil
}
