package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCodeChunkJSON(t *testing.T) {
	chunk := CodeChunk{
		ID:        "123",
		FilePath:  "main.go",
		Language:  "go",
		Content:   "func main() {}",
		ChunkType: ChunkTypeFunction,
		StartLine: 1,
		EndLine:   10,
		Metadata:  map[string]string{"author": "me"},
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var parsed CodeChunk
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if parsed.ID != chunk.ID {
		t.Errorf("ID = %v, want %v", parsed.ID, chunk.ID)
	}
	if parsed.ChunkType != ChunkTypeFunction {
		t.Errorf("ChunkType = %v, want %v", parsed.ChunkType, ChunkTypeFunction)
	}
}

func TestSearchResultScore(t *testing.T) {
	res := SearchResult{
		Chunk:          &CodeChunk{ID: "1"},
		RelevanceScore: 0.9,
	}

	if res.RelevanceScore != 0.9 {
		t.Errorf("RelevanceScore = %v, want 0.9", res.RelevanceScore)
	}
}
