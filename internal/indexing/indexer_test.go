package indexing_test

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/indexing"
	"github.com/Guru2308/rag-code/internal/mocks"
)

func TestIndexer_IndexFile(t *testing.T) {
	// Setup mocks
	mockParser := &mocks.MockParser{
		ParseFunc: func(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
			return []*domain.CodeChunk{{Content: "func main() {}"}}, nil
		},
	}
	mockChunker := &mocks.MockChunker{
		ChunkFunc: func(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error) {
			return chunks, nil
		},
	}
	mockEmbedder := &mocks.MockEmbedder{
		EmbedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			return make([][]float32, len(texts)), nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		StoreFunc: func(ctx context.Context, chunks []*domain.CodeChunk) error {
			return nil
		},
	}
	mockKeyword := &mocks.MockKeywordIndexer{
		AddToInvertedIndexFunc: func(ctx context.Context, chunks []*domain.CodeChunk) error {
			return nil
		},
	}

	indexer := indexing.NewIndexer(mockParser, mockChunker, mockEmbedder, mockStore, mockKeyword)

	// Test IndexFile
	err := indexer.IndexFile(context.Background(), "main.go")
	if err != nil {
		t.Errorf("IndexFile() error = %v", err)
	}
}
