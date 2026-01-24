package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Guru2308/rag-code/internal/api"
	"github.com/Guru2308/rag-code/internal/config"
	"github.com/Guru2308/rag-code/internal/embeddings"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/llm"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/retrieval"
	"github.com/Guru2308/rag-code/internal/vectorstore"
)

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

	// 4. Indexing Pipeline
	parser := indexing.NewGoParser()
	chunker := indexing.NewSemanticChunker(cfg.MaxChunkSize, cfg.ChunkOverlap)
	indexer := indexing.NewIndexer(parser, chunker, embedder, qStore)

	// Initialize Collection in Qdrant
	// all-minilm (sentence-transformers) typically has 384 dimensions
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()
	if err := qStore.InitCollection(initCtx, 384); err != nil {
		logger.Error("Failed to initialize Qdrant collection", "error", err)
		os.Exit(1)
	}

	// 5. Retrieval Engine
	retriever := retrieval.NewRetriever(embedder, qStore)

	// 6. API Server
	srv := api.NewServer(cfg.ServerPort, indexer, retriever, llmClient)

	logger.Info("All services initialized successfully")

	// Start server
	if err := srv.Start(); err != nil {
		logger.Error("API server failed", "error", err)
		os.Exit(1)
	}
}
