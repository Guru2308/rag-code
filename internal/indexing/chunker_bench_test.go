package indexing

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
)

// BenchmarkChunker_Chunk measures chunking throughput for large code.
// Used for performance optimization validation.
func BenchmarkChunker_Chunk(b *testing.B) {
	chunker := NewSemanticChunker(512, 50)

	// Simulate a large Go file (~10KB)
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("func Handler%d(ctx context.Context, req *Request) (*Response, error) {\n", i))
		sb.WriteString("    // Implementation with some logic\n")
		sb.WriteString("    return &Response{}, nil\n}\n\n")
	}
	content := sb.String()

	chunks := []*domain.CodeChunk{
		{
			FilePath:  "internal/api/handlers.go",
			StartLine: 1,
			EndLine:   500,
			Content:   content,
			Metadata:  map[string]string{"type": "file"},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chunker.Chunk(ctx, chunks, 512)
	}
}
