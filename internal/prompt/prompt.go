package prompt

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
)

// Generator defines the interface for prompt generation
type Generator interface {
	Generate(ctx context.Context, query string, results []*domain.SearchResult) (string, error)
}

// ---------------------------------------------------------------------------
// Prompt templates
// ---------------------------------------------------------------------------

// DefaultPromptTemplate is a simple generic prompt (legacy).
const DefaultPromptTemplate = `You are a helpful code assistant. Use the provided code context to answer the user's question.
If the context is insufficient, explain what's missing.

Code Context:
{{range .Results}}
--- {{.Chunk.FilePath}} (Lines {{.Chunk.StartLine}}-{{.Chunk.EndLine}}) ---
{{.Chunk.Content}}
{{end}}

Question: {{.Query}}
Answer:`

// ProfessionalPromptTemplate is a prompt for a professional code assistant that helps
// understand codebases and acts as a code reviewer. Supports both understanding
// ("How does X work?") and review ("Review this", "Suggest improvements").
const ProfessionalPromptTemplate = `You are a senior software engineer and professional code reviewer. Your role is to:
1. **Understand the codebase** — Explain architecture, data flow, design patterns, and how components interact.
2. **Review code** — Assess correctness, maintainability, security, performance, and adherence to best practices.

Guidelines:
- Base your answers strictly on the provided code context. Cite file paths and line numbers when referencing code.
- If the context is insufficient, state what's missing and suggest where to look.
- For code review: be constructive, specific, and actionable. Prioritize critical issues (security, correctness) over style.
- For understanding: explain the "why" and relationships, not just the "what".
- Use clear structure (bullets, sections) for complex answers.

Code Context (retrieved from the codebase):
{{range .Results}}
--- {{.Chunk.FilePath}} (Lines {{.Chunk.StartLine}}-{{.Chunk.EndLine}}) ---
{{.Chunk.Content}}
{{end}}

User question: {{.Query}}

Provide a professional, well-structured response:`

// ---------------------------------------------------------------------------
// TemplateGenerator
// ---------------------------------------------------------------------------

// TemplateGenerator implements Generator using Go templates with optional
// context-window management (token-budget truncation + metadata injection).
type TemplateGenerator struct {
	tmpl          *template.Template
	maxTokens     int    // 0 = unlimited
	charsPerToken int    // rough estimate (default: 4)
	model         string // injected as metadata if non-empty
}

// Option is a functional option for TemplateGenerator.
type Option func(*TemplateGenerator)

// WithMaxTokens sets a soft token budget for the context section.
// Results are trimmed (lowest-scoring first) until the context fits.
func WithMaxTokens(n int) Option {
	return func(g *TemplateGenerator) { g.maxTokens = n }
}

// WithCharsPerToken overrides the default rough chars-per-token estimate (4).
func WithCharsPerToken(n int) Option {
	return func(g *TemplateGenerator) {
		if n > 0 {
			g.charsPerToken = n
		}
	}
}

// WithModel attaches the model name to the generated prompt as a metadata comment.
func WithModel(model string) Option {
	return func(g *TemplateGenerator) { g.model = model }
}

// TemplateByName returns the template string for the given name.
// Supported names: "professional" (default), "default".
func TemplateByName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "default":
		return DefaultPromptTemplate
	case "professional", "":
		return ProfessionalPromptTemplate
	default:
		return ProfessionalPromptTemplate
	}
}

// NewTemplateGenerator creates a new template-based generator.
// Pass an empty tplString to use the DefaultPromptTemplate.
// For named templates, use TemplateByName("professional") or TemplateByName("default").
func NewTemplateGenerator(tplString string, opts ...Option) (*TemplateGenerator, error) {
	if tplString == "" {
		tplString = DefaultPromptTemplate
	}
	tmpl, err := template.New("prompt").Parse(tplString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	g := &TemplateGenerator{
		tmpl:          tmpl,
		charsPerToken: 4,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

// ---------------------------------------------------------------------------
// Generate
// ---------------------------------------------------------------------------

// Generate creates a prompt from the query and search results.
// It:
//  1. Sorts results by relevance (highest first)
//  2. Truncates the result list to fit within the token budget (if set)
//  3. Injects generation metadata as a leading comment in the prompt
func (g *TemplateGenerator) Generate(ctx context.Context, query string, results []*domain.SearchResult) (string, error) {
	// Sort by relevance so we always keep the best chunks when truncating
	sorted := g.sortByRelevance(results)

	// Fit within context window
	fitted := g.fitToWindow(sorted, query)

	logger.Debug("Prompt generation",
		"total_results", len(results),
		"fitted_results", len(fitted),
		"max_tokens", g.maxTokens,
	)

	data := struct {
		Query    string
		Results  []*domain.SearchResult
		Metadata map[string]string
	}{
		Query:    query,
		Results:  fitted,
		Metadata: g.buildMetadata(results, fitted),
	}

	var buf bytes.Buffer
	if err := g.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Prepend metadata header as a comment so it's always visible
	header := g.renderHeader(data.Metadata)
	return header + buf.String(), nil
}

// ---------------------------------------------------------------------------
// Context-window management helpers
// ---------------------------------------------------------------------------

// fitToWindow removes the lowest-relevance results until the total character
// count (approximated as chars / charsPerToken) fits within maxTokens.
// If maxTokens == 0, no truncation is applied.
func (g *TemplateGenerator) fitToWindow(results []*domain.SearchResult, query string) []*domain.SearchResult {
	if g.maxTokens <= 0 || len(results) == 0 {
		return results
	}

	budget := g.maxTokens * g.charsPerToken // chars budget
	// Reserve ~20 % for the query + template boilerplate
	budget = int(float64(budget) * 0.8)

	fitted := make([]*domain.SearchResult, 0, len(results))
	used := 0

	for _, r := range results {
		if r.Chunk == nil {
			continue
		}
		chunkChars := len(r.Chunk.Content)
		if used+chunkChars > budget && len(fitted) > 0 {
			// Would exceed budget; stop here
			logger.Debug("Context window budget reached",
				"used_chars", used,
				"budget_chars", budget,
				"dropped_results", len(results)-len(fitted),
			)
			break
		}
		fitted = append(fitted, r)
		used += chunkChars
	}

	return fitted
}

// sortByRelevance returns a copy of results sorted descending by RelevanceScore.
func (g *TemplateGenerator) sortByRelevance(results []*domain.SearchResult) []*domain.SearchResult {
	sorted := make([]*domain.SearchResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].RelevanceScore > sorted[j].RelevanceScore
	})
	return sorted
}

// buildMetadata returns a map of metadata to inject into the prompt header.
func (g *TemplateGenerator) buildMetadata(all, fitted []*domain.SearchResult) map[string]string {
	meta := map[string]string{
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"total_results":   fmt.Sprintf("%d", len(all)),
		"context_results": fmt.Sprintf("%d", len(fitted)),
	}
	if g.model != "" {
		meta["model"] = g.model
	}
	if g.maxTokens > 0 {
		meta["max_tokens"] = fmt.Sprintf("%d", g.maxTokens)
	}

	// Collect unique source files
	files := make(map[string]struct{})
	for _, r := range fitted {
		if r.Chunk != nil {
			files[r.Chunk.FilePath] = struct{}{}
		}
	}
	fileList := make([]string, 0, len(files))
	for f := range files {
		fileList = append(fileList, f)
	}
	sort.Strings(fileList)
	meta["source_files"] = strings.Join(fileList, ", ")

	return meta
}

// renderHeader returns a short metadata block that prefixes the prompt.
func (g *TemplateGenerator) renderHeader(meta map[string]string) string {
	if len(meta) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<!-- RAG context: ")
	parts := make([]string, 0, len(meta))
	keys := []string{"generated_at", "model", "context_results", "total_results", "max_tokens", "source_files"}
	added := map[string]bool{}
	for _, k := range keys {
		if v, ok := meta[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			added[k] = true
		}
	}
	for k, v := range meta {
		if !added[k] {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sb.WriteString(strings.Join(parts, " | "))
	sb.WriteString(" -->\n")
	return sb.String()
}
