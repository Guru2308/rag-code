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
