package vectorstore

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/qdrant/go-client/qdrant"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug})
}

func TestNewQdrantStore(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		collection string
		wantErr    bool
	}{
		{
			name:       "valid URL with port 6333",
			url:        "http://localhost:6333",
			collection: "test",
			wantErr:    false,
		},
		{
			name:       "valid URL with port 6334",
			url:        "localhost:6334",
			collection: "test",
			wantErr:    false,
		},
		{
			name:       "URL with invalid port",
			url:        "localhost:abc",
			collection: "test",
			wantErr:    false, // uses default port 6334
		},
		{
			name:       "URL without port",
			url:        "localhost",
			collection: "test",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewQdrantStore(tt.url, tt.collection)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewQdrantStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && store == nil {
				t.Error("NewQdrantStore() returned nil store")
			}
			if !tt.wantErr && store.collection != tt.collection {
				t.Errorf("NewQdrantStore() collection = %v, want %v", store.collection, tt.collection)
			}
		})
	}
}

func TestQdrantStore_mapPointToChunk(t *testing.T) {
	store := &QdrantStore{collection: "test"}

	// Create a test payload
	payload := map[string]*qdrant.Value{
		"file_path":  qdrant.NewValueString("test.go"),
		"language":   qdrant.NewValueString("go"),
		"content":    qdrant.NewValueString("func main() {}"),
		"chunk_type": qdrant.NewValueString("function"),
		"start_line": qdrant.NewValueDouble(1),
		"end_line":   qdrant.NewValueDouble(3),
	}

	point := &qdrant.RetrievedPoint{
		Id:      qdrant.NewID("test-id-123"),
		Payload: payload,
	}

	chunk := store.mapPointToChunk(point)

	if chunk.ID != "test-id-123" {
		t.Errorf("mapPointToChunk() ID = %v, want %v", chunk.ID, "test-id-123")
	}
	if chunk.FilePath != "test.go" {
		t.Errorf("mapPointToChunk() FilePath = %v, want %v", chunk.FilePath, "test.go")
	}
	if chunk.Language != "go" {
		t.Errorf("mapPointToChunk() Language = %v, want %v", chunk.Language, "go")
	}
	if chunk.Content != "func main() {}" {
		t.Errorf("mapPointToChunk() Content = %v, want %v", chunk.Content, "func main() {}")
	}
	if chunk.ChunkType != domain.ChunkTypeFunction {
		t.Errorf("mapPointToChunk() ChunkType = %v, want %v", chunk.ChunkType, domain.ChunkTypeFunction)
	}
	if chunk.StartLine != 1 {
		t.Errorf("mapPointToChunk() StartLine = %v, want %v", chunk.StartLine, 1)
	}
	if chunk.EndLine != 3 {
		t.Errorf("mapPointToChunk() EndLine = %v, want %v", chunk.EndLine, 3)
	}
}

func TestQdrantStore_mapPointToChunk_WithDependencies(t *testing.T) {
	store := &QdrantStore{collection: "test"}

	deps := []*qdrant.Value{
		qdrant.NewValueString("fmt"),
		qdrant.NewValueString("os"),
	}

	payload := map[string]*qdrant.Value{
		"file_path":    qdrant.NewValueString("test.go"),
		"language":     qdrant.NewValueString("go"),
		"content":      qdrant.NewValueString("import \"fmt\""),
		"chunk_type":   qdrant.NewValueString("import"),
		"start_line":   qdrant.NewValueDouble(1),
		"end_line":     qdrant.NewValueDouble(1),
		"dependencies": qdrant.NewValueList(&qdrant.ListValue{Values: deps}),
	}

	point := &qdrant.RetrievedPoint{
		Id:      qdrant.NewID("test-id-456"),
		Payload: payload,
	}

	chunk := store.mapPointToChunk(point)

	if len(chunk.Dependencies) != 2 {
		t.Errorf("mapPointToChunk() dependencies length = %v, want %v", len(chunk.Dependencies), 2)
	}
	if chunk.Dependencies[0] != "fmt" {
		t.Errorf("mapPointToChunk() dependencies[0] = %v, want %v", chunk.Dependencies[0], "fmt")
	}
	if chunk.Dependencies[1] != "os" {
		t.Errorf("mapPointToChunk() dependencies[1] = %v, want %v", chunk.Dependencies[1], "os")
	}
}

func TestQdrantStore_mapPointToChunk_WithMetadata(t *testing.T) {
	store := &QdrantStore{collection: "test"}

	metadata := &qdrant.Struct{
		Fields: map[string]*qdrant.Value{
			"name":       qdrant.NewValueString("main"),
			"complexity": qdrant.NewValueString("low"),
		},
	}

	payload := map[string]*qdrant.Value{
		"file_path":  qdrant.NewValueString("test.go"),
		"language":   qdrant.NewValueString("go"),
		"content":    qdrant.NewValueString("func main() {}"),
		"chunk_type": qdrant.NewValueString("function"),
		"start_line": qdrant.NewValueDouble(1),
		"end_line":   qdrant.NewValueDouble(3),
		"metadata":   qdrant.NewValueStruct(metadata),
	}

	point := &qdrant.RetrievedPoint{
		Id:      qdrant.NewID("test-id-789"),
		Payload: payload,
	}

	chunk := store.mapPointToChunk(point)

	if len(chunk.Metadata) != 2 {
		t.Errorf("mapPointToChunk() metadata length = %v, want %v", len(chunk.Metadata), 2)
	}
	if chunk.Metadata["name"] != "main" {
		t.Errorf("mapPointToChunk() metadata[name] = %v, want %v", chunk.Metadata["name"], "main")
	}
	if chunk.Metadata["complexity"] != "low" {
		t.Errorf("mapPointToChunk() metadata[complexity] = %v, want %v", chunk.Metadata["complexity"], "low")
	}
}

func TestQdrantStore_mapScoredPointToChunk(t *testing.T) {
	store := &QdrantStore{collection: "test"}

	payload := map[string]*qdrant.Value{
		"file_path":  qdrant.NewValueString("test.go"),
		"language":   qdrant.NewValueString("go"),
		"content":    qdrant.NewValueString("func main() {}"),
		"chunk_type": qdrant.NewValueString("function"),
		"start_line": qdrant.NewValueDouble(1),
		"end_line":   qdrant.NewValueDouble(3),
	}

	point := &qdrant.ScoredPoint{
		Id:      qdrant.NewID("test-id-scored"),
		Payload: payload,
		Score:   0.95,
	}

	chunk := store.mapScoredPointToChunk(point)

	if chunk.ID != "test-id-scored" {
		t.Errorf("mapScoredPointToChunk() ID = %v, want %v", chunk.ID, "test-id-scored")
	}
	if chunk.FilePath != "test.go" {
		t.Errorf("mapScoredPointToChunk() FilePath = %v, want %v", chunk.FilePath, "test.go")
	}
}

func TestQdrantStore_mapScoredPointToChunk_WithDependenciesAndMetadata(t *testing.T) {
	store := &QdrantStore{collection: "test"}

	deps := []*qdrant.Value{
		qdrant.NewValueString("fmt"),
	}

	metadata := &qdrant.Struct{
		Fields: map[string]*qdrant.Value{
			"name": qdrant.NewValueString("helper"),
		},
	}

	payload := map[string]*qdrant.Value{
		"file_path":    qdrant.NewValueString("helper.go"),
		"language":     qdrant.NewValueString("go"),
		"content":      qdrant.NewValueString("func helper() {}"),
		"chunk_type":   qdrant.NewValueString("function"),
		"start_line":   qdrant.NewValueDouble(5),
		"end_line":     qdrant.NewValueDouble(10),
		"dependencies": qdrant.NewValueList(&qdrant.ListValue{Values: deps}),
		"metadata":     qdrant.NewValueStruct(metadata),
	}

	point := &qdrant.ScoredPoint{
		Id:      qdrant.NewID("test-scored-complex"),
		Payload: payload,
		Score:   0.85,
	}

	chunk := store.mapScoredPointToChunk(point)

	if len(chunk.Dependencies) != 1 {
		t.Errorf("mapScoredPointToChunk() dependencies length = %v, want %v", len(chunk.Dependencies), 1)
	}
	if chunk.Dependencies[0] != "fmt" {
		t.Errorf("mapScoredPointToChunk() dependencies[0] = %v, want %v", chunk.Dependencies[0], "fmt")
	}
	if len(chunk.Metadata) != 1 {
		t.Errorf("mapScoredPointToChunk() metadata length = %v, want %v", len(chunk.Metadata), 1)
	}
	if chunk.Metadata["name"] != "helper" {
		t.Errorf("mapScoredPointToChunk() metadata[name] = %v, want %v", chunk.Metadata["name"], "helper")
	}
}

func TestImplementsChunkStore(t *testing.T) {
	var _ interface {
		Store(ctx context.Context, chunks []*domain.CodeChunk) error
	} = (*QdrantStore)(nil)
}

func TestQdrantStore_Store(t *testing.T) {
	mockClient := &MockQdrantClient{}
	store := &QdrantStore{
		client:     mockClient,
		collection: "test-collection",
	}

	chunks := []*domain.CodeChunk{
		{
			ID:        "1",
			FilePath:  "test.go",
			Content:   "content",
			Embedding: []float32{0.1, 0.2},
		},
	}

	t.Run("success", func(t *testing.T) {
		mockClient.UpsertFunc = func(ctx context.Context, in *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
			if in.CollectionName != "test-collection" {
				t.Errorf("expected collection name test-collection, got %s", in.CollectionName)
			}
			if len(in.Points) != 1 {
				t.Errorf("expected 1 point, got %d", len(in.Points))
			}
			return &qdrant.UpdateResult{Status: qdrant.UpdateStatus_Completed}, nil
		}

		err := store.Store(context.Background(), chunks)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("success with complex chunks", func(t *testing.T) {
		complexChunks := []*domain.CodeChunk{
			{
				ID:           "2",
				FilePath:     "complex.go",
				Content:      "complex content",
				Embedding:    []float32{0.3, 0.4},
				Dependencies: []string{"fmt", "os"},
				Metadata:     map[string]string{"author": "me"},
			},
		}

		mockClient.UpsertFunc = func(ctx context.Context, in *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
			if len(in.Points) != 1 {
				t.Errorf("expected 1 point, got %d", len(in.Points))
			}
			// Verify payload contains dependencies and metadata
			payload := in.Points[0].Payload
			if _, ok := payload["dependencies"]; !ok {
				t.Error("expected dependencies in payload")
			}
			if _, ok := payload["metadata"]; !ok {
				t.Error("expected metadata in payload")
			}
			return &qdrant.UpdateResult{Status: qdrant.UpdateStatus_Completed}, nil
		}

		err := store.Store(context.Background(), complexChunks)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mockClient.UpsertFunc = func(ctx context.Context, in *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
			return nil, context.DeadlineExceeded
		}

		err := store.Store(context.Background(), chunks)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestQdrantStore_Delete(t *testing.T) {
	mockClient := &MockQdrantClient{}
	store := &QdrantStore{
		client:     mockClient,
		collection: "test-collection",
	}

	t.Run("success", func(t *testing.T) {
		mockClient.DeleteFunc = func(ctx context.Context, in *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
			if in.CollectionName != "test-collection" {
				t.Errorf("expected collection name test-collection, got %s", in.CollectionName)
			}
			if in.Points == nil {
				t.Error("expected points filter, got nil")
			}
			return &qdrant.UpdateResult{}, nil
		}

		err := store.Delete(context.Background(), "test.go")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mockClient.DeleteFunc = func(ctx context.Context, in *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
			return nil, context.DeadlineExceeded
		}

		err := store.Delete(context.Background(), "test.go")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestQdrantStore_Get(t *testing.T) {
	mockClient := &MockQdrantClient{}
	store := &QdrantStore{
		client:     mockClient,
		collection: "test-collection",
	}

	t.Run("success", func(t *testing.T) {
		mockClient.GetFunc = func(ctx context.Context, in *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error) {
			return []*qdrant.RetrievedPoint{
				{
					Id: qdrant.NewID("test-id"),
					Payload: map[string]*qdrant.Value{
						"file_path":  qdrant.NewValueString("test.go"),
						"language":   qdrant.NewValueString("go"),
						"content":    qdrant.NewValueString("code"),
						"chunk_type": qdrant.NewValueString("code"),
						"start_line": qdrant.NewValueDouble(1),
						"end_line":   qdrant.NewValueDouble(10),
					},
				},
			}, nil
		}

		chunk, err := store.Get(context.Background(), "test-id")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if chunk == nil {
			t.Error("expected chunk, got nil")
		}
	})

	t.Run("not found", func(t *testing.T) {
		mockClient.GetFunc = func(ctx context.Context, in *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error) {
			return []*qdrant.RetrievedPoint{}, nil
		}

		chunk, err := store.Get(context.Background(), "test-id")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if chunk != nil {
			t.Error("expected nil chunk, got chunk")
		}
	})

	t.Run("error", func(t *testing.T) {
		mockClient.GetFunc = func(ctx context.Context, in *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error) {
			return nil, context.DeadlineExceeded
		}

		_, err := store.Get(context.Background(), "test-id")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestQdrantStore_Search(t *testing.T) {
	mockClient := &MockQdrantClient{}
	store := &QdrantStore{
		client:     mockClient,
		collection: "test-collection",
	}

	t.Run("success", func(t *testing.T) {
		mockClient.QueryFunc = func(ctx context.Context, in *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
			return []*qdrant.ScoredPoint{
				{
					Id:    qdrant.NewID("test-id"),
					Score: 0.9,
					Payload: map[string]*qdrant.Value{
						"file_path":  qdrant.NewValueString("test.go"),
						"language":   qdrant.NewValueString("go"),
						"content":    qdrant.NewValueString("code"),
						"chunk_type": qdrant.NewValueString("code"),
						"start_line": qdrant.NewValueDouble(1),
						"end_line":   qdrant.NewValueDouble(10),
					},
				},
			}, nil
		}

		results, err := store.Search(context.Background(), []float32{0.1, 0.2}, 10)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("error", func(t *testing.T) {
		mockClient.QueryFunc = func(ctx context.Context, in *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
			return nil, context.DeadlineExceeded
		}

		_, err := store.Search(context.Background(), []float32{0.1}, 10)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestQdrantStore_InitCollection(t *testing.T) {
	mockClient := &MockQdrantClient{}
	store := &QdrantStore{
		client:     mockClient,
		collection: "test-collection",
	}

	t.Run("exists", func(t *testing.T) {
		mockClient.CollectionExistsFunc = func(ctx context.Context, collectionName string) (bool, error) {
			return true, nil
		}

		err := store.InitCollection(context.Background(), 768)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("create", func(t *testing.T) {
		mockClient.CollectionExistsFunc = func(ctx context.Context, collectionName string) (bool, error) {
			return false, nil
		}
		mockClient.CreateCollectionFunc = func(ctx context.Context, in *qdrant.CreateCollection) error {
			if in.CollectionName != "test-collection" {
				t.Errorf("expected collection name test-collection, got %s", in.CollectionName)
			}
			return nil
		}

		err := store.InitCollection(context.Background(), 768)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("error checking existence", func(t *testing.T) {
		mockClient.CollectionExistsFunc = func(ctx context.Context, collectionName string) (bool, error) {
			return false, context.DeadlineExceeded
		}

		err := store.InitCollection(context.Background(), 768)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("error creating", func(t *testing.T) {
		mockClient.CollectionExistsFunc = func(ctx context.Context, collectionName string) (bool, error) {
			return false, nil
		}
		mockClient.CreateCollectionFunc = func(ctx context.Context, in *qdrant.CreateCollection) error {
			return context.DeadlineExceeded
		}

		err := store.InitCollection(context.Background(), 768)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
