package vectorstore

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/mocks"
	"github.com/qdrant/go-client/qdrant"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug})
}

func TestQdrantStore_Store(t *testing.T) {
	mockClient := &mocks.MockQdrantClient{
		UpsertFunc: func(ctx context.Context, in *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
			if len(in.Points) != 1 {
				t.Errorf("Expected 1 point, got %d", len(in.Points))
			}
			return &qdrant.UpdateResult{}, nil
		},
	}

	store := &QdrantStore{
		client:     mockClient,
		collection: "test-collection",
	}

	chunks := []*domain.CodeChunk{
		{
			ID:        "uuid1",
			FilePath:  "test.go",
			Content:   "test content",
			Embedding: []float32{0.1, 0.2, 0.3},
		},
	}

	err := store.Store(context.Background(), chunks)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
}

func TestQdrantStore_Get(t *testing.T) {
	mockClient := &mocks.MockQdrantClient{
		GetFunc: func(ctx context.Context, in *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error) {
			return []*qdrant.RetrievedPoint{
				{
					Id: qdrant.NewID("uuid1"),
					Payload: map[string]*qdrant.Value{
						"file_path":  qdrant.NewValueString("test.go"),
						"content":    qdrant.NewValueString("content"),
						"start_line": qdrant.NewValueDouble(10),
						"end_line":   qdrant.NewValueDouble(20),
						"chunk_type": qdrant.NewValueString("function"),
						"language":   qdrant.NewValueString("go"),
					},
				},
			}, nil
		},
	}

	store := &QdrantStore{
		client:     mockClient,
		collection: "test",
	}

	chunk, err := store.Get(context.Background(), "uuid1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if chunk.ID != "uuid1" || chunk.FilePath != "test.go" || chunk.StartLine != 10 {
		t.Errorf("Mismatched chunk data: %+v", chunk)
	}
}

func TestQdrantStore_Search(t *testing.T) {
	mockClient := &mocks.MockQdrantClient{
		QueryFunc: func(ctx context.Context, in *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
			return []*qdrant.ScoredPoint{
				{
					Id:    qdrant.NewID("uuid1"),
					Score: 0.95,
					Payload: map[string]*qdrant.Value{
						"file_path": qdrant.NewValueString("test.go"),
						"content":   qdrant.NewValueString("content"),
					},
				},
			}, nil
		},
	}

	store := &QdrantStore{
		client:     mockClient,
		collection: "test",
	}

	results, err := store.Search(context.Background(), []float32{0.1}, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].Score != 0.95 {
		t.Errorf("Expected score 0.95, got %f", results[0].Score)
	}
}

func TestQdrantStore_Delete(t *testing.T) {
	mockClient := &mocks.MockQdrantClient{
		DeleteFunc: func(ctx context.Context, in *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
			return &qdrant.UpdateResult{}, nil
		},
	}

	store := &QdrantStore{
		client:     mockClient,
		collection: "test",
	}

	err := store.Delete(context.Background(), "test.go")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}
}

func TestQdrantStore_InitCollection(t *testing.T) {
	mockClient := &mocks.MockQdrantClient{
		CollectionExistsFunc: func(ctx context.Context, collectionName string) (bool, error) {
			return false, nil
		},
		CreateCollectionFunc: func(ctx context.Context, in *qdrant.CreateCollection) error {
			if in.CollectionName != "test" {
				t.Errorf("Expected collection name 'test', got %s", in.CollectionName)
			}
			return nil
		},
	}

	store := &QdrantStore{
		client:     mockClient,
		collection: "test",
	}

	err := store.InitCollection(context.Background(), 128)
	if err != nil {
		t.Errorf("InitCollection failed: %v", err)
	}
}
