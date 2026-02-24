package prompt

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

// BenchmarkPromptGenerate measures prompt generation throughput.
// Used for performance optimization validation.
func BenchmarkPromptGenerate(b *testing.B) {
	g, err := NewTemplateGenerator(
		ProfessionalPromptTemplate,
		WithMaxTokens(4096),
	)
	if err != nil {
		b.Fatalf("NewTemplateGenerator: %v", err)
	}

	// Simulate 20 search results (typical retrieval size)
	results := make([]*domain.SearchResult, 20)
	for i := range results {
		results[i] = &domain.SearchResult{
			Chunk: &domain.CodeChunk{
				FilePath:  "internal/api/server.go",
				StartLine: i * 10,
				EndLine:   (i+1)*10 + 5,
				Content:   "func handleRequest() { /* 80 chars of code */ }",
			},
			RelevanceScore: float32(1.0 - float64(i)*0.02),
		}
	}

	ctx := context.Background()
	query := "How does the API handle incoming requests?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.Generate(ctx, query, results)
	}
}
