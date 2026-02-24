package config

import (
	"fmt"
	"os"
	"runtime"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// Ollama Configuration
	OllamaURL      string
	EmbeddingModel string
	LLMModel       string

	// Vector Store Configuration
	VectorStoreURL string
	CollectionName string

	// Indexing Configuration
	TargetCodebase string
	MaxChunkSize   int
	ChunkOverlap   int

	// Server Configuration
	ServerPort string
	LogLevel   string
	LogFormat  string

	// Redis Configuration
	RedisURL      string
	RedisPassword string
	RedisDB       int

	// Hybrid Retrieval Configuration
	HybridEnabled      bool
	HybridVectorWeight float64
	FusionStrategy     string
	BM25K1             float64
	BM25B              float64

	// Concurrency
	NumWorkers              int // file-level parallelism (default: 2*CPU)
	EmbeddingWorkers        int // workers per EmbedBatch call (default: 8)
	MaxConcurrentEmbeddings int // global cap on concurrent Ollama requests (default: 16)

	// Prompt
	PromptTemplate string // "professional" (default) or "default" — which prompt template to use

	// MMR (Maximal Marginal Relevance) — diversity in retrieval
	UseMMR     bool    // enable MMR reranking (default: true)
	MMRLambda  float64 // relevance vs diversity trade-off 0–1 (default: 0.7)
}

// Load reads configuration from environment variables and .env file
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		OllamaURL:      getEnvOrDefault("OLLAMA_URL", "http://localhost:11434"),
		EmbeddingModel: getEnvOrDefault("EMBEDDING_MODEL", "all-minilm"),
		LLMModel:       getEnvOrDefault("LLM_MODEL", "llama3.2:1b"),
		VectorStoreURL: getEnvOrDefault("VECTOR_STORE_URL", "http://localhost:6333"),
		CollectionName: getEnvOrDefault("COLLECTION_NAME", "code_chunks"),
		TargetCodebase: os.Getenv("TARGET_CODEBASE"),
		MaxChunkSize:   512, // all-minilm supports 512 tokens
		ChunkOverlap:   50,
		ServerPort:     getEnvOrDefault("SERVER_PORT", "8080"),
		LogLevel:       getEnvOrDefault("LOG_LEVEL", "debug"),
		LogFormat:      getEnvOrDefault("LOG_FORMAT", "json"),

		RedisURL:      getEnvOrDefault("REDIS_URL", "localhost:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		HybridEnabled:      getEnvAsBool("HYBRID_ENABLED", true),
		HybridVectorWeight: getEnvAsFloat("HYBRID_VECTOR_WEIGHT", 0.7),
		FusionStrategy:     getEnvOrDefault("FUSION_STRATEGY", "rrf"),
		BM25K1:             getEnvAsFloat("BM25_K1", 1.2),
		BM25B:              getEnvAsFloat("BM25_B", 0.75),

		NumWorkers:              getEnvAsInt("NUM_WORKERS", max(2*runtime.NumCPU(), 4)),
		EmbeddingWorkers:        getEnvAsInt("EMBEDDING_WORKERS", 8),
		MaxConcurrentEmbeddings: getEnvAsInt("MAX_CONCURRENT_EMBEDDINGS", 16),

		PromptTemplate: getEnvOrDefault("PROMPT_TEMPLATE", "professional"),

		UseMMR:    getEnvAsBool("USE_MMR", true),
		MMRLambda: getEnvAsFloat("MMR_LAMBDA", 0.7),
	}

	// Validate required fields
	if cfg.OllamaURL == "" {
		return nil, fmt.Errorf("OLLAMA_URL must be set")
	}

	return cfg, nil
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		fmt.Sscanf(value, "%d", &i)
		return i
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		var f float64
		fmt.Sscanf(value, "%f", &f)
		return f
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true"
	}
	return defaultValue
}
