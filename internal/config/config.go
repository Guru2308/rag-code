package config

import (
	"fmt"
	"os"

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
		MaxChunkSize:   256,
		ChunkOverlap:   50,
		ServerPort:     getEnvOrDefault("SERVER_PORT", "8080"),
		LogLevel:       getEnvOrDefault("LOG_LEVEL", "info"),
		LogFormat:      getEnvOrDefault("LOG_FORMAT", "json"),

		RedisURL:      getEnvOrDefault("REDIS_URL", "localhost:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		HybridEnabled:      getEnvAsBool("HYBRID_ENABLED", true),
		HybridVectorWeight: getEnvAsFloat("HYBRID_VECTOR_WEIGHT", 0.7),
		FusionStrategy:     getEnvOrDefault("FUSION_STRATEGY", "rrf"),
		BM25K1:             getEnvAsFloat("BM25_K1", 1.2),
		BM25B:              getEnvAsFloat("BM25_B", 0.75),
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
