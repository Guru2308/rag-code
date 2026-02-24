package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug})
}

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

func TestTemplateByName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // substring that must appear in the template
	}{
		{"professional", "professional", "senior software engineer"},
		{"Professional", "Professional", "senior software engineer"},
		{"default", "default", "helpful code assistant"},
		{"empty", "", "senior software engineer"},
		{"unknown", "unknown", "senior software engineer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl := TemplateByName(tt.input)
			if !strings.Contains(tpl, tt.contains) {
				t.Errorf("TemplateByName(%q) should contain %q", tt.input, tt.contains)
			}
		})
	}
}

func TestTemplateGenerator_ProfessionalPrompt(t *testing.T) {
	g, err := NewTemplateGenerator(ProfessionalPromptTemplate)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	results := []*domain.SearchResult{
		{
			Chunk: &domain.CodeChunk{
				FilePath:  "internal/api/server.go",
				StartLine: 100,
				EndLine:   120,
				Content:   "func (s *Server) handleQuery(c *gin.Context) { ... }",
			},
		},
	}

	query := "How does the API handle queries?"
	prompt, err := g.Generate(context.Background(), query, results)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(prompt, "How does the API handle queries?") {
		t.Errorf("Prompt should contain the query")
	}
	if !strings.Contains(prompt, "internal/api/server.go") {
		t.Errorf("Prompt should contain the file path")
	}
	if !strings.Contains(prompt, "senior software engineer") {
		t.Errorf("Professional prompt should contain role description")
	}
}
