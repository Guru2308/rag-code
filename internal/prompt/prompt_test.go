package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

func TestTemplateGenerator_Generate(t *testing.T) {
	g, err := NewTemplateGenerator("")
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	results := []*domain.SearchResult{
		{
			Chunk: &domain.CodeChunk{
				FilePath:  "test.go",
				StartLine: 1,
				EndLine:   5,
				Content:   "func test() {}",
			},
		},
	}

	query := "How to test?"
	prompt, err := g.Generate(context.Background(), query, results)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(prompt, "How to test?") {
		t.Errorf("Prompt should contain the query")
	}
	if !strings.Contains(prompt, "test.go") {
		t.Errorf("Prompt should contain the file path")
	}
	if !strings.Contains(prompt, "func test() {}") {
		t.Errorf("Prompt should contain the code content")
	}
}

func TestTemplateGenerator_InvalidTemplate(t *testing.T) {
	_, err := NewTemplateGenerator("{{invalid")
	if err == nil {
		t.Errorf("Expected error for invalid template")
	}
}
