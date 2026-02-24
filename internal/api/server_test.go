package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/llm"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/mocks"
	"github.com/Guru2308/rag-code/internal/prompt"
	"github.com/Guru2308/rag-code/internal/retrieval"
	"github.com/gin-gonic/gin"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug})
}

func TestServer_HandleStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := NewServer("8080", nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/status", nil)
	server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestServer_HandleQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup mocks for Retrieval
	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return []*domain.SearchResult{
				{Chunk: &domain.CodeChunk{ID: "1", Content: "code"}},
			}, nil
		},
		GetFunc: func(ctx context.Context, id string) (*domain.CodeChunk, error) {
			return &domain.CodeChunk{ID: "1", Content: "code"}, nil
		},
	}

	// We need KeywordSearcher and Scorer mocks for Retriever
	// But since we are testing API, we can use nil for those features if we disable hybrid or mock them if NewRetriever allows nil.
	// NewRetriever takes interfaces.
	mockKeyword := &mocks.MockKeywordSearcher{
		SearchFunc: func(ctx context.Context, tokens []string, limit int) ([]string, error) {
			return nil, nil // Return empty for keyword to rely on vector
		},
	}
	mockScorer := &mocks.MockScorer{} // methods return 0

	preprocessor := retrieval.NewQueryPreprocessor()
	fusionConfig := retrieval.DefaultFusionConfig()

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, mockKeyword, mockScorer, preprocessor, nil, nil, nil, fusionConfig)

	// Setup LLM mock via httptest
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": "LLM response"},
			"done":    true, // for stream if used
		})
	}))
	defer llmServer.Close()

	llmClient := llm.NewOllamaLLM(llmServer.URL, "model")

	prompter, _ := prompt.NewTemplateGenerator("")
	server := NewServer("8080", nil, retriever, llmClient, prompter)

	// Payload
	query := domain.SearchQuery{
		Query:      "how does this work",
		MaxResults: 1,
	}
	body, _ := json.Marshal(query)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/query", bytes.NewBuffer(body))
	server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["response"] != "LLM response" {
		t.Errorf("Expected LLM response, got %v", resp["response"])
	}
}

func TestServer_HandleQuery_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServer("8080", nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/query", bytes.NewBufferString("invalid json"))
	server.Router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestServer_HandleQuery_RetrievalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return nil, errors.New("embedding failed")
		},
	}
	mockStore := &mocks.MockChunkStore{}

	preprocessor := retrieval.NewQueryPreprocessor()
	fusionConfig := retrieval.DefaultFusionConfig()

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, nil, nil, preprocessor, nil, nil, nil, fusionConfig)
	server := NewServer("8080", nil, retriever, nil, nil)

	query := domain.SearchQuery{Query: "test"}
	body, _ := json.Marshal(query)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/query", bytes.NewBuffer(body))
	server.Router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("Expected 500 for retrieval error, got %d", w.Code)
	}
}

func TestServer_HandleQuery_LLMError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return []*domain.SearchResult{
				{Chunk: &domain.CodeChunk{ID: "1", Content: "code"}},
			}, nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	fusionConfig := retrieval.DefaultFusionConfig()

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, nil, nil, preprocessor, nil, nil, nil, fusionConfig)

	// LLM server that returns error
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer llmServer.Close()

	llmClient := llm.NewOllamaLLM(llmServer.URL, "model")
	prompter, _ := prompt.NewTemplateGenerator("")
	server := NewServer("8080", nil, retriever, llmClient, prompter)

	query := domain.SearchQuery{Query: "test"}
	body, _ := json.Marshal(query)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/query", bytes.NewBuffer(body))
	server.Router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("Expected 500 for LLM error, got %d", w.Code)
	}
}

func TestServer_HandleIndex_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServer("8080", nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/index", bytes.NewBufferString("invalid"))
	server.Router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestServer_HandleIndex_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockParser := &mocks.MockParser{}
	mockChunker := &mocks.MockChunker{}
	mockEmbedder := &mocks.MockEmbedder{}
	mockStore := &mocks.MockChunkStore{}

	indexer := indexing.NewIndexer(mockParser, mockChunker, mockEmbedder, mockStore, nil, nil, 1)
	server := NewServer("8080", indexer, nil, nil, nil)

	payload := map[string]string{"path": "/tmp/test"}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/index", bytes.NewBuffer(body))
	server.Router.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Errorf("Expected 202, got %d", w.Code)
	}
}

func TestServer_HandleQuery_DefaultMaxResults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockEmbedder := &mocks.MockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1}, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		SearchFunc: func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
			return []*domain.SearchResult{
				{Chunk: &domain.CodeChunk{ID: "1", Content: "code"}},
			}, nil
		},
	}

	preprocessor := retrieval.NewQueryPreprocessor()
	fusionConfig := retrieval.DefaultFusionConfig()

	retriever := retrieval.NewRetriever(mockEmbedder, mockStore, nil, nil, preprocessor, nil, nil, nil, fusionConfig)

	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": "response"},
			"done":    true,
		})
	}))
	defer llmServer.Close()

	llmClient := llm.NewOllamaLLM(llmServer.URL, "model")
	server := NewServer("8080", nil, retriever, llmClient, nil)

	// Query without MaxResults
	query := domain.SearchQuery{Query: "test"}
	body, _ := json.Marshal(query)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/query", bytes.NewBuffer(body))
	server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}
