//go:build integration

package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Guru2308/rag-code/internal/api"
	"github.com/Guru2308/rag-code/internal/config"
	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/embeddings"
	"github.com/Guru2308/rag-code/internal/graph"
	"github.com/Guru2308/rag-code/internal/hierarchy"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/llm"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/prompt"
	"github.com/Guru2308/rag-code/internal/reranker"
	"github.com/Guru2308/rag-code/internal/retrieval"
	"github.com/Guru2308/rag-code/internal/vectorstore"
	"github.com/redis/go-redis/v9"
)

// Integration tests require: docker-compose up (Qdrant + Redis) and Ollama running.
// Run with: go test -tags=integration ./test/...

func init() {
	logger.Init(logger.Config{Level: logger.LevelInfo})
}

func TestIntegration_IndexAndQuery(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Config load failed (missing .env?): %v", err)
	}

	// Quick connectivity check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	qStore, err := vectorstore.NewQdrantStore(cfg.VectorStoreURL, "rag-integration-test")
	if err != nil {
		t.Skipf("Qdrant unavailable: %v", err)
	}
	if err := qStore.InitCollection(ctx, 384); err != nil {
		t.Skipf("Qdrant init failed: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis unavailable: %v", err)
	}

	// Build pipeline
	embedder := embeddings.NewOllamaEmbedder(cfg.OllamaURL, cfg.EmbeddingModel)
	llmClient := llm.NewOllamaLLM(cfg.OllamaURL, cfg.LLMModel)
	redisIndex := retrieval.NewRedisIndex(redisClient, "rag:integration:")
	bm25Scorer := retrieval.NewBM25Scorer(cfg.BM25K1, cfg.BM25B, redisIndex)
	preprocessor := retrieval.NewQueryPreprocessor()
	depGraph := graph.NewGraph()
	expander := retrieval.NewContextExpander(depGraph, qStore)
	reRanker := reranker.NewHeuristicReranker()
	hierFilter := hierarchy.NewHierarchicalFilter(3)
	fusionConfig := retrieval.DefaultFusionConfig()

	retriever := retrieval.NewRetriever(
		embedder, qStore, redisIndex, bm25Scorer,
		preprocessor, expander, reRanker, hierFilter, fusionConfig,
	)

	parser := indexing.NewMultiParser()
	chunker := indexing.NewSemanticChunker(cfg.MaxChunkSize, cfg.ChunkOverlap)
	indexer := indexing.NewIndexer(parser, chunker, embedder, qStore, retriever, depGraph, 2)

	prompter, err := prompt.NewTemplateGenerator("", prompt.WithMaxTokens(4096))
	if err != nil {
		t.Fatalf("Prompt generator: %v", err)
	}

	server := api.NewServer("0", indexer, retriever, llmClient, prompter)

	// Index this codebase (project root when running: go test -tags=integration ./test/)
	indexPath := "."
	if wd, err := os.Getwd(); err == nil {
		// If cwd is test/, use parent
		if filepath.Base(wd) == "test" {
			indexPath = filepath.Join(wd, "..")
		} else {
			indexPath = wd
		}
	}

	indexReq := httptest.NewRequest("POST", "/api/index", strings.NewReader(`{"path":"`+indexPath+`"}`))
	indexReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Router.ServeHTTP(w, indexReq)

	if w.Code != http.StatusAccepted {
		t.Fatalf("Index returned %d: %s", w.Code, w.Body.String())
	}

	// Wait for indexing to complete (background goroutine)
	time.Sleep(15 * time.Second)

	// Query
	query := domain.SearchQuery{Query: "How does the retriever work?", MaxResults: 3}
	body, _ := json.Marshal(query)
	queryReq := httptest.NewRequest("POST", "/api/query", strings.NewReader(string(body)))
	queryReq.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.Router.ServeHTTP(w2, queryReq)

	if w2.Code != http.StatusOK {
		t.Fatalf("Query returned %d: %s", w2.Code, w2.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Parse response: %v", err)
	}
	if _, ok := resp["response"]; !ok {
		t.Error("Response missing 'response' field")
	}
	if results, ok := resp["results"].([]interface{}); ok && len(results) == 0 {
		t.Log("No results returned - indexing may not have completed")
	}
}
