package retrieval

import (
	"context"
	"testing"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*RedisIndex, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	idx := NewRedisIndex(client, "test:")
	return idx, mr
}

func TestNewRedisIndex(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	idx := NewRedisIndex(client, "test:")

	if idx == nil {
		t.Error("NewRedisIndex() returned nil")
	}
	if idx.keyPrefix != "test:" {
		t.Errorf("NewRedisIndex() keyPrefix = %v, want 'test:'", idx.keyPrefix)
	}
}

func TestRedisIndex_AddDocuments(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{
			ID:      "doc1",
			Content: "hello world",
			Length:  2,
			Tokens:  map[string]int{"hello": 1, "world": 1},
		},
	}

	err := idx.AddDocuments(context.Background(), docs)
	if err != nil {
		t.Fatalf("AddDocuments() error = %v", err)
	}

	// Verify document was added
	count, err := idx.GetDocCount(context.Background())
	if err != nil {
		t.Fatalf("GetDocCount() error = %v", err)
	}
	if count != 1 {
		t.Errorf("GetDocCount() = %d, want 1", count)
	}
}

func TestRedisIndex_AddDocuments_MultipleDocuments(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{
			ID:      "doc1",
			Content: "hello world",
			Length:  2,
			Tokens:  map[string]int{"hello": 1, "world": 1},
		},
		{
			ID:      "doc2",
			Content: "hello universe",
			Length:  2,
			Tokens:  map[string]int{"hello": 1, "universe": 1},
		},
	}

	err := idx.AddDocuments(context.Background(), docs)
	if err != nil {
		t.Fatalf("AddDocuments() error = %v", err)
	}

	count, err := idx.GetDocCount(context.Background())
	if err != nil {
		t.Fatalf("GetDocCount() error = %v", err)
	}
	if count != 2 {
		t.Errorf("GetDocCount() = %d, want 2", count)
	}
}

func TestRedisIndex_AddToInvertedIndex(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	chunks := []*domain.CodeChunk{
		{
			ID:      "chunk1",
			Content: "func main() {}",
		},
	}

	err := idx.AddToInvertedIndex(context.Background(), chunks)
	if err != nil {
		t.Fatalf("AddToInvertedIndex() error = %v", err)
	}

	count, err := idx.GetDocCount(context.Background())
	if err != nil {
		t.Fatalf("GetDocCount() error = %v", err)
	}
	if count != 1 {
		t.Errorf("GetDocCount() = %d, want 1", count)
	}
}

func TestRedisIndex_RemoveDocument(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	// Add a document first
	docs := []*IndexedDocument{
		{
			ID:      "doc1",
			Content: "test",
			Length:  1,
			Tokens:  map[string]int{"test": 1},
		},
	}
	idx.AddDocuments(context.Background(), docs)

	// Remove it
	err := idx.RemoveDocument(context.Background(), "doc1")
	if err != nil {
		t.Fatalf("RemoveDocument() error = %v", err)
	}

	// Verify doc count decreased
	count, err := idx.GetDocCount(context.Background())
	if err != nil {
		t.Fatalf("GetDocCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("GetDocCount() after removal = %d, want 0", count)
	}
}

func TestRedisIndex_Search(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{
			ID:      "doc1",
			Content: "hello world",
			Length:  2,
			Tokens:  map[string]int{"hello": 1, "world": 1},
		},
		{
			ID:      "doc2",
			Content: "hello universe",
			Length:  2,
			Tokens:  map[string]int{"hello": 1, "universe": 1},
		},
	}
	idx.AddDocuments(context.Background(), docs)

	results, err := idx.Search(context.Background(), []string{"hello"}, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Search() returned %d documents, want 2", len(results))
	}
}

func TestRedisIndex_Search_EmptyTokens(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	results, err := idx.Search(context.Background(), []string{}, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if results != nil {
		t.Errorf("Search() with empty tokens should return nil, got %v", results)
	}
}

func TestRedisIndex_Search_WithLimit(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{ID: "doc1", Content: "test", Length: 1, Tokens: map[string]int{"test": 1}},
		{ID: "doc2", Content: "test", Length: 1, Tokens: map[string]int{"test": 1}},
		{ID: "doc3", Content: "test", Length: 1, Tokens: map[string]int{"test": 1}},
	}
	idx.AddDocuments(context.Background(), docs)

	results, err := idx.Search(context.Background(), []string{"test"}, 2)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) > 2 {
		t.Errorf("Search() with limit 2 returned %d documents, want at most 2", len(results))
	}
}

func TestRedisIndex_GetTermFrequency(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{
			ID:      "doc1",
			Content: "hello hello world",
			Length:  3,
			Tokens:  map[string]int{"hello": 2, "world": 1},
		},
	}
	idx.AddDocuments(context.Background(), docs)

	freq, err := idx.GetTermFrequency(context.Background(), "hello", "doc1")
	if err != nil {
		t.Fatalf("GetTermFrequency() error = %v", err)
	}

	if freq != 2 {
		t.Errorf("GetTermFrequency() = %d, want 2", freq)
	}
}

func TestRedisIndex_GetTermFrequency_NotFound(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	freq, err := idx.GetTermFrequency(context.Background(), "missing", "doc1")
	if err != nil {
		t.Fatalf("GetTermFrequency() error = %v", err)
	}

	if freq != 0 {
		t.Errorf("GetTermFrequency() for missing term = %d, want 0", freq)
	}
}

func TestRedisIndex_GetDocFrequency(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{ID: "doc1", Content: "test", Length: 1, Tokens: map[string]int{"test": 1}},
		{ID: "doc2", Content: "test", Length: 1, Tokens: map[string]int{"test": 1}},
	}
	idx.AddDocuments(context.Background(), docs)

	df, err := idx.GetDocFrequency(context.Background(), "test")
	if err != nil {
		t.Fatalf("GetDocFrequency() error = %v", err)
	}

	if df != 2 {
		t.Errorf("GetDocFrequency() = %d, want 2", df)
	}
}

func TestRedisIndex_GetAvgDocLength(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{ID: "doc1", Content: "a b", Length: 2, Tokens: map[string]int{"a": 1, "b": 1}},
		{ID: "doc2", Content: "a b c d", Length: 4, Tokens: map[string]int{"a": 1, "b": 1, "c": 1, "d": 1}},
	}
	idx.AddDocuments(context.Background(), docs)

	avgLen, err := idx.GetAvgDocLength(context.Background())
	if err != nil {
		t.Fatalf("GetAvgDocLength() error = %v", err)
	}

	expectedAvg := 3.0
	if avgLen != expectedAvg {
		t.Errorf("GetAvgDocLength() = %v, want %v", avgLen, expectedAvg)
	}
}

func TestRedisIndex_GetDocLength(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{ID: "doc1", Content: "test content", Length: 5, Tokens: map[string]int{"test": 1}},
	}
	idx.AddDocuments(context.Background(), docs)

	length, err := idx.GetDocLength(context.Background(), "doc1")
	if err != nil {
		t.Fatalf("GetDocLength() error = %v", err)
	}

	if length != 5 {
		t.Errorf("GetDocLength() = %d, want 5", length)
	}
}

func TestRedisIndex_GetDocumentsByIDs(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{ID: "doc1", Content: "content1", Length: 1, Tokens: map[string]int{"test": 1}},
		{ID: "doc2", Content: "content2", Length: 1, Tokens: map[string]int{"test": 1}},
	}
	idx.AddDocuments(context.Background(), docs)

	retrieved, err := idx.GetDocumentsByIDs(context.Background(), []string{"doc1", "doc2"})
	if err != nil {
		t.Fatalf("GetDocumentsByIDs() error = %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("GetDocumentsByIDs() returned %d documents, want 2", len(retrieved))
	}
}

func TestRedisIndex_GetDocumentsByIDs_EmptyInput(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	retrieved, err := idx.GetDocumentsByIDs(context.Background(), []string{})
	if err != nil {
		t.Fatalf("GetDocumentsByIDs() error = %v", err)
	}

	if retrieved != nil {
		t.Errorf("GetDocumentsByIDs() with empty input should return nil, got %v", retrieved)
	}
}

func TestRedisIndex_Clear(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{ID: "doc1", Content: "test", Length: 1, Tokens: map[string]int{"test": 1}},
	}
	idx.AddDocuments(context.Background(), docs)

	err := idx.Clear(context.Background())
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	count, err := idx.GetDocCount(context.Background())
	if err != nil {
		t.Fatalf("GetDocCount() error = %v", err)
	}

	if count != 0 {
		t.Errorf("GetDocCount() after Clear() = %d, want 0", count)
	}
}

func TestRedisIndex_Export(t *testing.T) {
	idx, mr := setupTestRedis(t)
	defer mr.Close()

	docs := []*IndexedDocument{
		{ID: "doc1", Content: "test", Length: 1, Tokens: map[string]int{"test": 1}},
	}
	idx.AddDocuments(context.Background(), docs)

	data, err := idx.Export(context.Background())
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Export() returned empty data")
	}
}

func TestRedisIndex_KeyGenerationHelpers(t *testing.T) {
	idx := &RedisIndex{keyPrefix: "test:"}

	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "tokenIndexKey",
			fn:       func() string { return idx.tokenIndexKey("hello") },
			expected: "test:index:token:hello",
		},
		{
			name:     "termFreqKey",
			fn:       func() string { return idx.termFreqKey("hello", "doc1") },
			expected: "test:tf:hello:doc1",
		},
		{
			name:     "docFreqKey",
			fn:       func() string { return idx.docFreqKey("hello") },
			expected: "test:stats:token:hello:df",
		},
		{
			name:     "docLengthKey",
			fn:       func() string { return idx.docLengthKey("doc1") },
			expected: "test:doc:doc1:length",
		},
		{
			name:     "docContentKey",
			fn:       func() string { return idx.docContentKey("doc1") },
			expected: "test:doc:doc1:content",
		},
		{
			name:     "statsKey",
			fn:       func() string { return idx.statsKey("doc_count") },
			expected: "test:stats:doc_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn()
			if result != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.want)
		}
	}
}
