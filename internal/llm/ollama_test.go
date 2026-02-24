package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaLLM_Generate(t *testing.T) {
	mockResponse := ChatResponse{
		Message: ChatMessage{
			Content: "generated response",
		},
		Done: true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected path /api/chat, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	l := NewOllamaLLM(server.URL, "test-model")
	ctx := context.Background()

	resp, err := l.Generate(ctx, []ChatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if resp != "generated response" {
		t.Errorf("Generate() = %v, want %v", resp, "generated response")
	}
}

func TestOllamaLLM_Generate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	l := NewOllamaLLM(server.URL, "test-model")
	ctx := context.Background()

	_, err := l.Generate(ctx, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("Generate() expected error for HTTP 500")
	}
}

func TestOllamaLLM_Generate_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	l := NewOllamaLLM(server.URL, "test-model")
	ctx := context.Background()

	_, err := l.Generate(ctx, []ChatMessage{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("Generate() expected error for invalid JSON response")
	}
}

func TestOllamaLLM_StreamGenerate(t *testing.T) {
	responses := []ChatResponse{
		{Message: ChatMessage{Content: "part1"}, Done: false},
		{Message: ChatMessage{Content: "part2"}, Done: true},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := json.NewEncoder(w)
		for _, res := range responses {
			enc.Encode(res)
		}
	}))
	defer server.Close()

	l := NewOllamaLLM(server.URL, "test-model")
	ctx := context.Background()

	var collected string
	err := l.StreamGenerate(ctx, []ChatMessage{{Role: "user", Content: "hi"}}, func(part string) error {
		collected += part
		return nil
	})

	if err != nil {
		t.Fatalf("StreamGenerate() error = %v", err)
	}

	if collected != "part1part2" {
		t.Errorf("StreamGenerate() collected = %v, want part1part2", collected)
	}
}

func TestOllamaLLM_StreamGenerate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	l := NewOllamaLLM(server.URL, "test-model")
	ctx := context.Background()

	err := l.StreamGenerate(ctx, []ChatMessage{{Role: "user", Content: "hi"}}, func(part string) error {
		return nil
	})

	if err == nil {
		t.Error("StreamGenerate() expected error for HTTP 502")
	}
}

func TestOllamaLLM_StreamGenerate_CallbackError(t *testing.T) {
	responses := []ChatResponse{
		{Message: ChatMessage{Content: "part1"}, Done: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := json.NewEncoder(w)
		for _, res := range responses {
			enc.Encode(res)
		}
	}))
	defer server.Close()

	l := NewOllamaLLM(server.URL, "test-model")
	ctx := context.Background()

	expectedErr := errors.New("callback error")
	err := l.StreamGenerate(ctx, []ChatMessage{{Role: "user", Content: "hi"}}, func(part string) error {
		return expectedErr
	})

	if err == nil {
		t.Error("StreamGenerate() expected callback error to be propagated")
	}
}

func TestOllamaLLM_StreamGenerate_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid\njson\nstream"))
	}))
	defer server.Close()

	l := NewOllamaLLM(server.URL, "test-model")
	ctx := context.Background()

	err := l.StreamGenerate(ctx, []ChatMessage{{Role: "user", Content: "hi"}}, func(part string) error {
		return nil
	})

	if err == nil {
		t.Error("StreamGenerate() expected error for invalid JSON stream")
	}
}

func TestNewOllamaLLM(t *testing.T) {
	llm := NewOllamaLLM("http://localhost:11434", "llama3")
	if llm.baseURL != "http://localhost:11434" {
		t.Errorf("NewOllamaLLM() baseURL = %v, want http://localhost:11434", llm.baseURL)
	}
	if llm.model != "llama3" {
		t.Errorf("NewOllamaLLM() model = %v, want llama3", llm.model)
	}
	if llm.client == nil {
		t.Error("NewOllamaLLM() client should not be nil")
	}
}
