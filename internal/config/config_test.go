package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Cleanup env after test
	defer os.Clearenv()

	t.Run("success with defaults", func(t *testing.T) {
		os.Setenv("OLLAMA_URL", "http://test-ollama:11434")
		defer os.Unsetenv("OLLAMA_URL")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.OllamaURL != "http://test-ollama:11434" {
			t.Errorf("OllamaURL = %v, want %v", cfg.OllamaURL, "http://test-ollama:11434")
		}
		if cfg.EmbeddingModel != "all-minilm" {
			t.Errorf("EmbeddingModel = %v, want %v", cfg.EmbeddingModel, "all-minilm")
		}
		if cfg.MaxChunkSize != 256 {
			t.Errorf("MaxChunkSize = %v, want %v", cfg.MaxChunkSize, 256)
		}
	})

	t.Run("defaults when missing", func(t *testing.T) {
		os.Clearenv()
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.OllamaURL != "http://localhost:11434" {
			t.Errorf("OllamaURL = %v, want default", cfg.OllamaURL)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		envVars := map[string]string{
			"OLLAMA_URL":           "http://custom:11434",
			"EMBEDDING_MODEL":      "custom-model",
			"LLM_MODEL":            "custom-llm",
			"VECTOR_STORE_URL":     "http://custom-vec:6333",
			"COLLECTION_NAME":      "custom-coll",
			"SERVER_PORT":          "9090",
			"LOG_LEVEL":            "debug",
			"LOG_FORMAT":           "text",
			"REDIS_URL":            "custom-redis:6379",
			"REDIS_DB":             "1",
			"HYBRID_ENABLED":       "false",
			"HYBRID_VECTOR_WEIGHT": "0.5",
			"FUSION_STRATEGY":      "custom-fusion",
			"BM25_K1":              "1.5",
			"BM25_B":               "0.8",
		}

		for k, v := range envVars {
			os.Setenv(k, v)
			defer os.Unsetenv(k)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.OllamaURL != "http://custom:11434" {
			t.Errorf("OllamaURL = %v", cfg.OllamaURL)
		}
		if cfg.RedisDB != 1 {
			t.Errorf("RedisDB = %v", cfg.RedisDB)
		}
		if cfg.HybridEnabled != false {
			t.Errorf("HybridEnabled = %v", cfg.HybridEnabled)
		}
		if cfg.HybridVectorWeight != 0.5 {
			t.Errorf("HybridVectorWeight = %v", cfg.HybridVectorWeight)
		}
		if cfg.BM25K1 != 1.5 {
			t.Errorf("BM25K1 = %v", cfg.BM25K1)
		}
	})
}
