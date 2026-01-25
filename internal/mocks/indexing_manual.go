package mocks

import (
	"context"

	"github.com/Guru2308/rag-code/internal/domain"
)

// MockParser implements indexing.Parser
type MockParser struct {
	ParseFunc func(ctx context.Context, filePath string) ([]*domain.CodeChunk, error)
}

func (m *MockParser) Parse(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
	if m.ParseFunc != nil {
		return m.ParseFunc(ctx, filePath)
	}
	return nil, nil
}

// MockChunker implements indexing.Chunker
type MockChunker struct {
	ChunkFunc func(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error)
}

func (m *MockChunker) Chunk(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error) {
	if m.ChunkFunc != nil {
		return m.ChunkFunc(ctx, chunks, maxSize)
	}
	return chunks, nil
}

// MockEmbedder implements indexing.Embedder
type MockEmbedder struct {
	EmbedFunc      func(ctx context.Context, text string) ([]float32, error)
	EmbedBatchFunc func(ctx context.Context, texts []string) ([][]float32, error)
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, text)
	}
	return []float32{}, nil
}

func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if m.EmbedBatchFunc != nil {
		return m.EmbedBatchFunc(ctx, texts)
	}
	results := make([][]float32, len(texts))
	return results, nil
}

// MockChunkStore implements indexing.ChunkStore
type MockChunkStore struct {
	StoreFunc  func(ctx context.Context, chunks []*domain.CodeChunk) error
	DeleteFunc func(ctx context.Context, filePath string) error
	GetFunc    func(ctx context.Context, id string) (*domain.CodeChunk, error)
	SearchFunc func(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error)
}

func (m *MockChunkStore) Store(ctx context.Context, chunks []*domain.CodeChunk) error {
	if m.StoreFunc != nil {
		return m.StoreFunc(ctx, chunks)
	}
	return nil
}

func (m *MockChunkStore) Delete(ctx context.Context, filePath string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, filePath)
	}
	return nil
}

func (m *MockChunkStore) Get(ctx context.Context, id string) (*domain.CodeChunk, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockChunkStore) Search(ctx context.Context, vector []float32, limit int) ([]*domain.SearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, vector, limit)
	}
	return nil, nil
}

// MockKeywordIndexer implements indexing.KeywordIndexer
type MockKeywordIndexer struct {
	AddToInvertedIndexFunc func(ctx context.Context, chunks []*domain.CodeChunk) error
}

func (m *MockKeywordIndexer) AddToInvertedIndex(ctx context.Context, chunks []*domain.CodeChunk) error {
	if m.AddToInvertedIndexFunc != nil {
		return m.AddToInvertedIndexFunc(ctx, chunks)
	}
	return nil
}
