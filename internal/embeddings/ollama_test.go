package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Guru2308/rag-code/internal/logger"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug, Format: "text"})
}

func TestOllamaEmbedder_Embed(t *testing.T) {
	mockResponse := embeddingResponse{
		Embedding: []float32{0.1, 0.2, 0.3},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("Expected path /api/embeddings, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-model")
	ctx := context.Background()

	emb, err := embedder.Embed(ctx, "hello")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if !reflect.DeepEqual(emb, mockResponse.Embedding) {
		t.Errorf("Embed() = %v, want %v", emb, mockResponse.Embedding)
	}
}

func TestOllamaEmbedder_Embed_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-model")
	ctx := context.Background()

	_, err := embedder.Embed(ctx, "hello")
	if err == nil {
		t.Error("Embed() expected error for HTTP 503")
	}
}

func TestOllamaEmbedder_Embed_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-model")
	ctx := context.Background()

	_, err := embedder.Embed(ctx, "hello")
	if err == nil {
		t.Error("Embed() expected error for invalid JSON response")
	}
}

func TestOllamaEmbedder_EmbedBatch(t *testing.T) {
	mockResponse := embeddingResponse{
		Embedding: []float32{0.1, 0.2, 0.3},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-model")
	ctx := context.Background()

	embs, err := embedder.EmbedBatch(ctx, []string{"one", "two"})
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}

	if len(embs) != 2 {
		t.Errorf("EmbedBatch() count = %d, want 2", len(embs))
	}
}

func TestOllamaEmbedder_EmbedBatch_ErrorPropagation(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(embeddingResponse{Embedding: []float32{0.1}})
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-model")
	ctx := context.Background()

	_, err := embedder.EmbedBatch(ctx, []string{"one", "two", "three"})
	if err == nil {
		t.Error("EmbedBatch() expected error when individual Embed fails")
	}
}

func TestOllamaEmbedder_EmbedBatch_EmptyInput(t *testing.T) {
	embedder := NewOllamaEmbedder("http://localhost:11434", "test-model")
	ctx := context.Background()

	embs, err := embedder.EmbedBatch(ctx, []string{})
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}

	if len(embs) != 0 {
		t.Errorf("EmbedBatch() with empty input should return empty slice, got %d", len(embs))
	}
}

func TestNewOllamaEmbedder(t *testing.T) {
	embedder := NewOllamaEmbedder("http://localhost:11434", "all-minilm")
	if embedder.baseURL != "http://localhost:11434" {
		t.Errorf("NewOllamaEmbedder() baseURL = %v, want http://localhost:11434", embedder.baseURL)
	}
	if embedder.model != "all-minilm" {
		t.Errorf("NewOllamaEmbedder() model = %v, want all-minilm", embedder.model)
	}
	if embedder.client == nil {
		t.Error("NewOllamaEmbedder() client should not be nil")
	}
}
