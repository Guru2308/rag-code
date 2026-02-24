package mocks

import (
	"context"

	"github.com/qdrant/go-client/qdrant"
)

// MockQdrantClient is a manual mock for the QdrantClient interface
type MockQdrantClient struct {
	UpsertFunc           func(ctx context.Context, in *qdrant.UpsertPoints) (*qdrant.UpdateResult, error)
	DeleteFunc           func(ctx context.Context, in *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
	GetFunc              func(ctx context.Context, in *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error)
	QueryFunc            func(ctx context.Context, in *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
	CollectionExistsFunc func(ctx context.Context, collectionName string) (bool, error)
	CreateCollectionFunc func(ctx context.Context, in *qdrant.CreateCollection) error
}

func (m *MockQdrantClient) Upsert(ctx context.Context, in *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
	if m.UpsertFunc != nil {
		return m.UpsertFunc(ctx, in)
	}
	return &qdrant.UpdateResult{}, nil
}

func (m *MockQdrantClient) Delete(ctx context.Context, in *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, in)
	}
	return &qdrant.UpdateResult{}, nil
}

func (m *MockQdrantClient) Get(ctx context.Context, in *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, in)
	}
	return nil, nil
}

func (m *MockQdrantClient) Query(ctx context.Context, in *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, in)
	}
	return nil, nil
}

func (m *MockQdrantClient) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	if m.CollectionExistsFunc != nil {
		return m.CollectionExistsFunc(ctx, collectionName)
	}
	return true, nil
}

func (m *MockQdrantClient) CreateCollection(ctx context.Context, in *qdrant.CreateCollection) error {
	if m.CreateCollectionFunc != nil {
		return m.CreateCollectionFunc(ctx, in)
	}
	return nil
}
