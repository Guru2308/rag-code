package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/mocks"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug})
}

func TestIndexer_IndexFile_Success(t *testing.T) {
	mockParser := &mocks.MockParser{
		ParseFunc: func(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
			return []*domain.CodeChunk{
				{ID: "1", Content: "func main() {}", FilePath: filePath, Language: "go"},
			}, nil
		},
	}
	mockChunker := &mocks.MockChunker{
		ChunkFunc: func(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error) {
			return chunks, nil
		},
	}
	mockEmbedder := &mocks.MockEmbedder{
		EmbedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range texts {
				result[i] = []float32{0.1, 0.2}
			}
			return result, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		StoreFunc: func(ctx context.Context, chunks []*domain.CodeChunk) error {
			return nil
		},
	}
	mockKeyword := &mocks.MockKeywordSearcher{
		AddToInvertedIndexFunc: func(ctx context.Context, chunks []*domain.CodeChunk) error {
			return nil
		},
	}

	indexer := NewIndexer(mockParser, mockChunker, mockEmbedder, mockStore, mockKeyword, nil, 1)

	// Create a temp file
	tmpDir, _ := os.MkdirTemp("", "indexer_test")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\nfunc main() {}"), 0644)

	err := indexer.IndexFile(context.Background(), testFile)
	if err != nil {
		t.Errorf("IndexFile() error = %v", err)
	}
}

func TestIndexer_IndexFile_UnknownLanguage(t *testing.T) {
	indexer := NewIndexer(nil, nil, nil, nil, nil, nil, 1)

	tmpDir, _ := os.MkdirTemp("", "indexer_test")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.unknown")
	os.WriteFile(testFile, []byte("unknown content"), 0644)

	err := indexer.IndexFile(context.Background(), testFile)
	if err != nil {
		t.Errorf("IndexFile() with unknown language should not error, got %v", err)
	}
}

func TestIndexer_IndexFile_NoChunks(t *testing.T) {
	mockParser := &mocks.MockParser{
		ParseFunc: func(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
			return []*domain.CodeChunk{}, nil // No chunks
		},
	}

	indexer := NewIndexer(mockParser, nil, nil, nil, nil, nil, 1)

	tmpDir, _ := os.MkdirTemp("", "indexer_test")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "empty.go")
	os.WriteFile(testFile, []byte("package main"), 0644)

	err := indexer.IndexFile(context.Background(), testFile)
	if err != nil {
		t.Errorf("IndexFile() with no chunks should not error, got %v", err)
	}
}

func TestIndexer_IndexDirectory(t *testing.T) {
	mockParser := &mocks.MockParser{
		ParseFunc: func(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
			return []*domain.CodeChunk{
				{ID: "1", Content: "test", FilePath: filePath},
			}, nil
		},
	}
	mockChunker := &mocks.MockChunker{
		ChunkFunc: func(ctx context.Context, chunks []*domain.CodeChunk, maxSize int) ([]*domain.CodeChunk, error) {
			return chunks, nil
		},
	}
	mockEmbedder := &mocks.MockEmbedder{
		EmbedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range texts {
				result[i] = []float32{0.1}
			}
			return result, nil
		},
	}
	mockStore := &mocks.MockChunkStore{
		StoreFunc: func(ctx context.Context, chunks []*domain.CodeChunk) error {
			return nil
		},
	}

	indexer := NewIndexer(mockParser, mockChunker, mockEmbedder, mockStore, nil, nil, 1)

	tmpDir, _ := os.MkdirTemp("", "indexer_test")
	defer os.RemoveAll(tmpDir)

	// Create nested structure
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file2.go"), []byte("package sub"), 0644)

	err := indexer.IndexDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Errorf("IndexDirectory() error = %v", err)
	}
}

func TestIndexer_IndexDirectory_SkipHidden(t *testing.T) {
	indexer := NewIndexer(nil, nil, nil, nil, nil, nil, 1)

	tmpDir, _ := os.MkdirTemp("", "indexer_test")
	defer os.RemoveAll(tmpDir)

	// Create hidden directory
	os.Mkdir(filepath.Join(tmpDir, ".hidden"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".hidden", "file.go"), []byte("package hidden"), 0644)

	err := indexer.IndexDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Errorf("IndexDirectory() error = %v", err)
	}
}

func TestIndexer_Index_File(t *testing.T) {
	mockParser := &mocks.MockParser{
		ParseFunc: func(ctx context.Context, filePath string) ([]*domain.CodeChunk, error) {
			return []*domain.CodeChunk{}, nil
		},
	}

	indexer := NewIndexer(mockParser, nil, nil, nil, nil, nil, 1)

	tmpDir, _ := os.MkdirTemp("", "indexer_test")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main"), 0644)

	err := indexer.Index(context.Background(), testFile)
	if err != nil {
		t.Errorf("Index() on file error = %v", err)
	}
}

func TestIndexer_Index_Directory(t *testing.T) {
	indexer := NewIndexer(nil, nil, nil, nil, nil, nil, 1)

	tmpDir, _ := os.MkdirTemp("", "indexer_test")
	defer os.RemoveAll(tmpDir)

	err := indexer.Index(context.Background(), tmpDir)
	if err != nil {
		t.Errorf("Index() on directory error = %v", err)
	}
}

func TestIndexer_DeleteFile(t *testing.T) {
	mockStore := &mocks.MockChunkStore{
		DeleteFunc: func(ctx context.Context, filePath string) error {
			return nil
		},
	}

	indexer := NewIndexer(nil, nil, nil, mockStore, nil, nil, 1)

	err := indexer.DeleteFile(context.Background(), "/path/to/file.go")
	if err != nil {
		t.Errorf("DeleteFile() error = %v", err)
	}
}

func TestIndexer_GetJob_NotFound(t *testing.T) {
	indexer := NewIndexer(nil, nil, nil, nil, nil, nil, 1)

	_, err := indexer.GetJob("nonexistent")
	if err == nil {
		t.Error("GetJob() expected error for nonexistent job")
	}
}
