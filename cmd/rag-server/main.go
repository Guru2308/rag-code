package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Guru2308/rag-code/docs"
	"github.com/Guru2308/rag-code/internal/api"
	"github.com/Guru2308/rag-code/internal/config"
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

// @title           RAG Code API
// @version         1.0
// @description     A basic RAG system for codebases.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	loggerCfg := logger.Config{
		Level:  logger.Level(cfg.LogLevel),
		Format: cfg.LogFormat,
	}
	if err := logger.Init(loggerCfg); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info("RAG system starting",
		"ollama_url", cfg.OllamaURL,
		"embedding_model", cfg.EmbeddingModel,
		"llm_model", cfg.LLMModel,
		"vector_store", cfg.VectorStoreURL,
		"port", cfg.ServerPort,
	)

	// Root context — cancelled on SIGINT/SIGTERM for clean shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize services
	logger.Info("Initializing services")

	// 1. Ollama Embedding Service
	embedder := embeddings.NewOllamaEmbedder(cfg.OllamaURL, cfg.EmbeddingModel)

	// 2. Ollama LLM Service
	llmClient := llm.NewOllamaLLM(cfg.OllamaURL, cfg.LLMModel)

	// 3. Qdrant Vector Store
	qStore, err := vectorstore.NewQdrantStore(cfg.VectorStoreURL, cfg.CollectionName)
	if err != nil {
		logger.Error("Failed to initialize Qdrant store", "error", err)
		os.Exit(1)
	}

	// 4. Redis Inverted Index (for BM25)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	redisIndex := retrieval.NewRedisIndex(redisClient, "rag:")

	// 5. Hybrid Retrieval Components
	preprocessor := retrieval.NewQueryPreprocessor()
	bm25Scorer := retrieval.NewBM25Scorer(cfg.BM25K1, cfg.BM25B, redisIndex)

	fusionConfig := retrieval.FusionConfig{
		Strategy:     retrieval.FusionRRF, // Defaulting to RRF for now
		VectorWeight: cfg.HybridVectorWeight,
		RRFConstant:  60,
	}

	// 5a. Phase 4: Dependency Graph and Expander
	depGraph := graph.NewGraph()
	expander := retrieval.NewContextExpander(depGraph, qStore)

	// 5b. Phase 5 & 6: Reranker and Hierarchy
	reRanker := reranker.NewHeuristicReranker()
	hierFilter := hierarchy.NewHierarchicalFilter(3)

	// 6. Retrieval Engine
	retr := retrieval.NewRetriever(embedder, qStore, redisIndex, bm25Scorer, preprocessor, expander, reRanker, hierFilter, fusionConfig)

	// 7. Indexing Pipeline
	parser := indexing.NewMultiParser()
	chunker := indexing.NewSemanticChunker(cfg.MaxChunkSize, cfg.ChunkOverlap)
	indexer := indexing.NewIndexer(parser, chunker, embedder, qStore, retr, depGraph, cfg.NumWorkers)

	// Initialize Collection in Qdrant
	// all-minilm has 384 dimensions
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()
	if err := qStore.InitCollection(initCtx, 384); err != nil {
		logger.Error("Failed to initialize Qdrant collection", "error", err)
		os.Exit(1)
	}

	// 7a. Prompt Generator
	prompter, err := prompt.NewTemplateGenerator("")
	if err != nil {
		logger.Error("Failed to initialize prompt generator", "error", err)
		os.Exit(1)
	}

	// 7b. File Watcher — auto re-index on file changes
	watchPath := os.Getenv("WATCH_PATH")
	if watchPath == "" {
		watchPath = "."
	}

	watcher, err := indexing.NewWatcher(func(watchCtx context.Context, path string, event indexing.FileEvent) error {
		switch event {
		case indexing.FileEventDelete:
			logger.Info("File deleted — removing from index", "path", path)
			return indexer.DeleteFile(watchCtx, path)
		default: // create or modify
			logger.Info("File changed — re-indexing", "path", path, "event", event)
			return indexer.IndexFile(watchCtx, path)
		}
	}, 500*time.Millisecond)
	if err != nil {
		logger.Error("Failed to create file watcher", "error", err)
		os.Exit(1)
	}

	if err := watcher.AddPath(watchPath); err != nil {
		logger.Warn("Failed to add watch path — auto-indexing disabled", "path", watchPath, "error", err)
	} else {
		logger.Info("File watcher started", "path", watchPath, "debounce", "500ms")
		go func() {
			if err := watcher.Start(ctx); err != nil {
				logger.Error("File watcher stopped with error", "error", err)
			}
		}()
	}

	// 8. API Server
	srv := api.NewServer(cfg.ServerPort, indexer, retr, llmClient, prompter)

	logger.Info("All services initialized successfully")

	// Start server (blocks until error or context cancelled)
	if err := srv.Start(); err != nil {
		logger.Error("API server failed", "error", err)
		os.Exit(1)
	}
}
