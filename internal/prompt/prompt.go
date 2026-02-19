package prompt

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/Guru2308/rag-code/internal/domain"
)

// Generator defines the interface for prompt generation
type Generator interface {
	Generate(ctx context.Context, query string, results []*domain.SearchResult) (string, error)
}

// TemplateGenerator implements Generator using Go templates
type TemplateGenerator struct {
	tmpl *template.Template
}

const DefaultPromptTemplate = `You are a helpful code assistant. Use the provided code context to answer the user's question.
If the context is insufficient, explain what's missing.

Code Context:
{{range .Results}}
--- {{.Chunk.FilePath}} (Lines {{.Chunk.StartLine}}-{{.Chunk.EndLine}}) ---
{{.Chunk.Content}}
{{end}}

Question: {{.Query}}
Answer:`

// NewTemplateGenerator creates a new template-based generator
func NewTemplateGenerator(tplString string) (*TemplateGenerator, error) {
	if tplString == "" {
		tplString = DefaultPromptTemplate
	}
	tmpl, err := template.New("prompt").Parse(tplString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	return &TemplateGenerator{tmpl: tmpl}, nil
}

// Generate creates a prompt from the query and search results
func (g *TemplateGenerator) Generate(ctx context.Context, query string, results []*domain.SearchResult) (string, error) {
	data := struct {
		Query   string
		Results []*domain.SearchResult
	}{
		Query:   query,
		Results: results,
	}

	var buf bytes.Buffer
	if err := g.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
