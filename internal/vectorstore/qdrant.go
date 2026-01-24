package vectorstore

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/qdrant/go-client/qdrant"
)

// QdrantStore implements the ChunkStore interface using Qdrant
type QdrantStore struct {
	client     *qdrant.Client
	collection string
}

// NewQdrantStore creates a new Qdrant vector store client
func NewQdrantStore(url string, collection string) (*QdrantStore, error) {
	// Parse host and port from URL if provided (expecting http://host:port or host:port)
	host := "localhost"
	port := 6334

	cleanURL := strings.TrimPrefix(url, "http://")
	cleanURL = strings.TrimPrefix(cleanURL, "https://")

	if h, p, err := net.SplitHostPort(cleanURL); err == nil {
		host = h
		if pi, err := strconv.Atoi(p); err == nil {
			// If user provided 6333 (HTTP), we should try 6334 (gRPC) if possible or just use as is
			// But the go-client is gRPC based.
			if pi == 6333 {
				port = 6334
			} else {
				port = pi
			}
		}
	} else {
		host = cleanURL
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to create Qdrant client")
	}

	return &QdrantStore{
		client:     client,
		collection: collection,
	}, nil
}

// Store persists code chunks in Qdrant
func (s *QdrantStore) Store(ctx context.Context, chunks []*domain.CodeChunk) error {
	points := make([]*qdrant.PointStruct, len(chunks))

	for i, chunk := range chunks {
		payloadMap := map[string]any{
			"file_path":  chunk.FilePath,
			"language":   chunk.Language,
			"chunk_type": string(chunk.ChunkType),
			"start_line": float64(chunk.StartLine),
			"end_line":   float64(chunk.EndLine),
			"content":    chunk.Content,
		}

		// Store dependencies
		if len(chunk.Dependencies) > 0 {
			payloadMap["dependencies"] = chunk.Dependencies
		}

		// Store metadata
		if len(chunk.Metadata) > 0 {
			metadataMap := make(map[string]any)
			for k, v := range chunk.Metadata {
				metadataMap[k] = v
			}
			payloadMap["metadata"] = metadataMap
		}

		payload := qdrant.NewValueMap(payloadMap)

		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewID(chunk.ID),
			Vectors: qdrant.NewVectors(chunk.Embedding...),
			Payload: payload,
		}
	}

	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points:         points,
	})
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeExternal, "failed to upsert points to Qdrant")
	}

	logger.Debug("Stored chunks in Qdrant", "count", len(chunks))
	return nil
}

// Delete removes a file's chunks from the index
func (s *QdrantStore) Delete(ctx context.Context, filePath string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Points:         qdrant.NewPointsSelectorFilter(&qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("file_path", filePath)}}),
	})
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeExternal, "failed to delete points from Qdrant")
	}

	logger.Info("Deleted file chunks from Qdrant", "path", filePath)
	return nil
}

// Get retrieves a single chunk by ID
func (s *QdrantStore) Get(ctx context.Context, id string) (*domain.CodeChunk, error) {
	resp, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection,
		Ids:            []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to get point from Qdrant")
	}

	if len(resp) == 0 {
		return nil, errors.NotFoundError("chunk not found")
	}

	point := resp[0]
	return s.mapPointToChunk(point), nil
}

// Search performs a vector search in Qdrant
func (s *QdrantStore) Search(ctx context.Context, queryVector []float32, limit int) ([]*domain.SearchResult, error) {
	resp, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(queryVector...),
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeExternal, "failed to search Qdrant")
	}

	results := make([]*domain.SearchResult, len(resp))
	for i, point := range resp {
		chunk := s.mapScoredPointToChunk(point)
		results[i] = &domain.SearchResult{
			Chunk: chunk,
			Score: float32(point.Score),
		}
	}

	return results, nil
}

// mapPointToChunk converts a Qdrant RetrievedPoint to a CodeChunk
func (s *QdrantStore) mapPointToChunk(point *qdrant.RetrievedPoint) *domain.CodeChunk {
	chunk := &domain.CodeChunk{
		ID:        point.Id.GetUuid(),
		FilePath:  point.Payload["file_path"].GetStringValue(),
		Language:  point.Payload["language"].GetStringValue(),
		Content:   point.Payload["content"].GetStringValue(),
		ChunkType: domain.ChunkType(point.Payload["chunk_type"].GetStringValue()),
		StartLine: int(point.Payload["start_line"].GetDoubleValue()),
		EndLine:   int(point.Payload["end_line"].GetDoubleValue()),
	}

	// Retrieve dependencies
	if depsValue, ok := point.Payload["dependencies"]; ok {
		if listValue := depsValue.GetListValue(); listValue != nil {
			deps := make([]string, len(listValue.Values))
			for i, v := range listValue.Values {
				deps[i] = v.GetStringValue()
			}
			chunk.Dependencies = deps
		}
	}

	// Retrieve metadata
	if metaValue, ok := point.Payload["metadata"]; ok {
		if structValue := metaValue.GetStructValue(); structValue != nil {
			meta := make(map[string]string)
			for k, v := range structValue.Fields {
				meta[k] = v.GetStringValue()
			}
			chunk.Metadata = meta
		}
	}

	return chunk
}

// mapScoredPointToChunk converts a Qdrant ScoredPoint to a CodeChunk
func (s *QdrantStore) mapScoredPointToChunk(point *qdrant.ScoredPoint) *domain.CodeChunk {
	chunk := &domain.CodeChunk{
		ID:        point.Id.GetUuid(),
		FilePath:  point.Payload["file_path"].GetStringValue(),
		Language:  point.Payload["language"].GetStringValue(),
		Content:   point.Payload["content"].GetStringValue(),
		ChunkType: domain.ChunkType(point.Payload["chunk_type"].GetStringValue()),
		StartLine: int(point.Payload["start_line"].GetDoubleValue()),
		EndLine:   int(point.Payload["end_line"].GetDoubleValue()),
	}

	// Retrieve dependencies
	if depsValue, ok := point.Payload["dependencies"]; ok {
		if listValue := depsValue.GetListValue(); listValue != nil {
			deps := make([]string, len(listValue.Values))
			for i, v := range listValue.Values {
				deps[i] = v.GetStringValue()
			}
			chunk.Dependencies = deps
		}
	}

	// Retrieve metadata
	if metaValue, ok := point.Payload["metadata"]; ok {
		if structValue := metaValue.GetStructValue(); structValue != nil {
			meta := make(map[string]string)
			for k, v := range structValue.Fields {
				meta[k] = v.GetStringValue()
			}
			chunk.Metadata = meta
		}
	}

	return chunk
}

// InitCollection ensures the collection exists with correct dimensions
func (s *QdrantStore) InitCollection(ctx context.Context, vectorSize int) error {
	exists, err := s.client.CollectionExists(ctx, s.collection)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeExternal, "failed to check collection existence")
	}

	if exists {
		return nil
	}

	logger.Info("Creating Qdrant collection", "name", s.collection, "size", vectorSize)
	err = s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: s.collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(vectorSize),
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeExternal, "failed to create collection")
	}

	return nil
}
