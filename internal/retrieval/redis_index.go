package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RedisIndex implements a Redis-backed inverted index for BM25 search
type RedisIndex struct {
	client    *redis.Client
	keyPrefix string
}

// IndexedDocument represents a document in the inverted index
type IndexedDocument struct {
	ID      string
	Content string
	Length  int
	Tokens  map[string]int // token -> term frequency
}

// NewRedisIndex creates a new Redis-backed inverted index
func NewRedisIndex(client *redis.Client, keyPrefix string) *RedisIndex {
	return &RedisIndex{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// AddDocuments adds multiple documents to the inverted index
func (r *RedisIndex) AddDocuments(ctx context.Context, docs []*IndexedDocument) error {
	pipe := r.client.Pipeline()

	for _, doc := range docs {
		// Store document metadata
		pipe.Set(ctx, r.docLengthKey(doc.ID), doc.Length, 0)
		pipe.Set(ctx, r.docContentKey(doc.ID), doc.Content[:min(200, len(doc.Content))], 0)

		// Add to inverted index
		for token, freq := range doc.Tokens {
			// Add document to token's posting list
			pipe.SAdd(ctx, r.tokenIndexKey(token), doc.ID)

			// Store term frequency
			pipe.Set(ctx, r.termFreqKey(token, doc.ID), freq, 0)

			// Increment document frequency for this token
			pipe.Incr(ctx, r.docFreqKey(token))
		}
	}

	// Increment total document count
	pipe.IncrBy(ctx, r.statsKey("doc_count"), int64(len(docs)))

	// Update average document length
	totalLength := 0
	for _, doc := range docs {
		totalLength += doc.Length
	}
	if len(docs) > 0 {
		// Get current stats
		docCount, err := r.GetDocCount(ctx)
		if err != nil {
			docCount = 0
		}

		currentAvg, err := r.GetAvgDocLength(ctx)
		if err != nil {
			currentAvg = 0
		}

		// Calculate new average
		newCount := docCount + len(docs)
		newAvg := (currentAvg*float64(docCount) + float64(totalLength)) / float64(newCount)
		pipe.Set(ctx, r.statsKey("avg_doc_length"), newAvg, 0)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// AddToInvertedIndex adds chunks to the inverted index (adapting domain.CodeChunk)
func (r *RedisIndex) AddToInvertedIndex(ctx context.Context, chunks []*domain.CodeChunk) error {
	indexedDocs := make([]*IndexedDocument, len(chunks))
	preprocessor := NewQueryPreprocessor() // Use default preprocessor

	for i, chunk := range chunks {
		processed := preprocessor.Preprocess(chunk.Content)

		tf := make(map[string]int)
		for _, token := range processed.Tokens {
			tf[token]++
		}

		indexedDocs[i] = &IndexedDocument{
			ID:      chunk.ID,
			Content: chunk.Content,
			Length:  len(processed.Tokens),
			Tokens:  tf,
		}
	}

	return r.AddDocuments(ctx, indexedDocs)
}

// RemoveDocument removes a document from the inverted index
func (r *RedisIndex) RemoveDocument(ctx context.Context, docID string) error {
	// Get all tokens for this document (we need to scan)
	// This is expensive - in production, we'd store a doc->tokens mapping

	pipe := r.client.Pipeline()

	// Remove document metadata
	pipe.Del(ctx, r.docLengthKey(docID))
	pipe.Del(ctx, r.docContentKey(docID))

	// Decrement doc count
	pipe.Decr(ctx, r.statsKey("doc_count"))

	_, err := pipe.Exec(ctx)
	return err
}

// Search returns document IDs that contain any of the given tokens
func (r *RedisIndex) Search(ctx context.Context, tokens []string, limit int) ([]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	// Get all document IDs that contain at least one token
	// Use SUNION to combine all posting lists
	keys := make([]string, len(tokens))
	for i, token := range tokens {
		keys[i] = r.tokenIndexKey(token)
	}

	docIDs, err := r.client.SUnion(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	// Limit results
	if limit > 0 && len(docIDs) > limit {
		docIDs = docIDs[:limit]
	}

	return docIDs, nil
}

// GetTermFrequency returns the term frequency for a token in a document
func (r *RedisIndex) GetTermFrequency(ctx context.Context, token, docID string) (int, error) {
	val, err := r.client.Get(ctx, r.termFreqKey(token, docID)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}

// GetDocFrequency returns the document frequency for a token
func (r *RedisIndex) GetDocFrequency(ctx context.Context, token string) (int, error) {
	val, err := r.client.Get(ctx, r.docFreqKey(token)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}

// GetDocCount returns the total number of documents in the index
func (r *RedisIndex) GetDocCount(ctx context.Context) (int, error) {
	val, err := r.client.Get(ctx, r.statsKey("doc_count")).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}

// GetAvgDocLength returns the average document length
func (r *RedisIndex) GetAvgDocLength(ctx context.Context) (float64, error) {
	val, err := r.client.Get(ctx, r.statsKey("avg_doc_length")).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(val, 64)
}

// GetDocLength returns the length of a specific document
func (r *RedisIndex) GetDocLength(ctx context.Context, docID string) (int, error) {
	val, err := r.client.Get(ctx, r.docLengthKey(docID)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}

// GetDocumentsByIDs retrieves document content previews
func (r *RedisIndex) GetDocumentsByIDs(ctx context.Context, docIDs []string) (map[string]string, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}

	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(docIDs))

	for i, docID := range docIDs {
		cmds[i] = pipe.Get(ctx, r.docContentKey(docID))
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	docs := make(map[string]string)
	for i, cmd := range cmds {
		if val, err := cmd.Result(); err == nil {
			docs[docIDs[i]] = val
		}
	}

	return docs, nil
}

// Clear removes all data from the index
func (r *RedisIndex) Clear(ctx context.Context) error {
	// Scan and delete all keys with our prefix
	iter := r.client.Scan(ctx, 0, r.keyPrefix+"*", 0).Iterator()
	pipe := r.client.Pipeline()

	count := 0
	for iter.Next(ctx) {
		pipe.Del(ctx, iter.Val())
		count++

		// Execute in batches
		if count%1000 == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
			pipe = r.client.Pipeline()
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Key generation helpers
func (r *RedisIndex) tokenIndexKey(token string) string {
	return fmt.Sprintf("%sindex:token:%s", r.keyPrefix, token)
}

func (r *RedisIndex) termFreqKey(token, docID string) string {
	return fmt.Sprintf("%stf:%s:%s", r.keyPrefix, token, docID)
}

func (r *RedisIndex) docFreqKey(token string) string {
	return fmt.Sprintf("%sstats:token:%s:df", r.keyPrefix, token)
}

func (r *RedisIndex) docLengthKey(docID string) string {
	return fmt.Sprintf("%sdoc:%s:length", r.keyPrefix, docID)
}

func (r *RedisIndex) docContentKey(docID string) string {
	return fmt.Sprintf("%sdoc:%s:content", r.keyPrefix, docID)
}

func (r *RedisIndex) statsKey(name string) string {
	return fmt.Sprintf("%sstats:%s", r.keyPrefix, name)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Export exports the entire index to JSON (for backup/debugging)
func (r *RedisIndex) Export(ctx context.Context) ([]byte, error) {
	data := make(map[string]interface{})

	// Get all keys
	iter := r.client.Scan(ctx, 0, r.keyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		val, err := r.client.Get(ctx, key).Result()
		if err == nil {
			data[key] = val
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return json.Marshal(data)
}
