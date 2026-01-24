package domain

import "time"

// CodeChunk represents a semantically meaningful piece of code
type CodeChunk struct {
	ID           string            `json:"id"`
	FilePath     string            `json:"file_path"`
	Language     string            `json:"language"`
	Content      string            `json:"content"`
	ChunkType    ChunkType         `json:"chunk_type"` // function, class, import, etc.
	StartLine    int               `json:"start_line"`
	EndLine      int               `json:"end_line"`
	Metadata     map[string]string `json:"metadata"`
	Dependencies []string          `json:"dependencies"` // imported modules, called functions
	Embedding    []float32         `json:"embedding,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ChunkType represents the type of code chunk
type ChunkType string

const (
	ChunkTypeFunction ChunkType = "function"
	ChunkTypeClass    ChunkType = "class"
	ChunkTypeMethod   ChunkType = "method"
	ChunkTypeImport   ChunkType = "import"
	ChunkTypeComment  ChunkType = "comment"
	ChunkTypeOther    ChunkType = "other"
)

// SearchQuery represents a user's query
type SearchQuery struct {
	Query      string            `json:"query"`
	Language   string            `json:"language,omitempty"`
	FilePath   string            `json:"file_path,omitempty"`
	MaxResults int               `json:"max_results,omitempty"`
	Filters    map[string]string `json:"filters,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Chunk          *CodeChunk `json:"chunk"`
	Score          float32    `json:"-"`
	Source         string     `json:"source"` // "vector", "keyword", "hybrid"
	VectorScore    float32    `json:"vector_score,omitempty"`
	KeywordScore   float32    `json:"keyword_score,omitempty"`
	RelevanceScore float32    `json:"relevance_score"`
}

// RetrievalContext represents the final context for LLM
type RetrievalContext struct {
	Query       string          `json:"query"`
	Results     []*SearchResult `json:"results"`
	TotalTokens int             `json:"total_tokens"`
	Metadata    map[string]any  `json:"metadata"`
}

// IndexingJob represents a code indexing task
type IndexingJob struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Status     JobStatus `json:"status"`
	Progress   float32   `json:"progress"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// JobStatus represents the status of an indexing job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)
