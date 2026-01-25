package llm

import (
	"context"
	"encoding/json"
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
