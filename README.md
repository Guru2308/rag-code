# RAG System - Cursor-style Code Understanding

A production-grade RAG (Retrieval-Augmented Generation) system for understanding and querying codebases, built in Go.

## Features

- Intelligent Code Indexing: AST-based parsing for deep code understanding
- Hybrid Retrieval: Combines dense (vector) and sparse (keyword) search
- Hierarchical Context: File to function to line level selection
- Smart Reranking: Code-aware relevance scoring
- LLM Integration: Ollama compatable in dev environment
- Real-time Updates: File watcher for automatic re-indexing

## Quick Start

### Prerequisites

- Go 1.21+
- Docker (for Qdrant vector database)
- Ollama in local

### Installation

```bash
# Install dependencies
go mod download

# Start Qdrant vector database
docker run -p 6333:6333 qdrant/qdrant

# Configure environment
vim .env

# Run the server
go run cmd/rag-server/main.go
```

### Configuration

Edit `.env` file:

```env
# Ollama Configuration
OLLAMA_URL=http://localhost:11434
EMBEDDING_MODEL=all-minilm
LLM_MODEL=llama3.2:1b

# Vector Store Configuration
VECTOR_STORE_URL=http://localhost:6333
COLLECTION_NAME=code_chunks

# Indexing Configuration
TARGET_CODEBASE=/path/to/indexing

# Server Configuration
SERVER_PORT=8080

# Logging Configuration
LOG_LEVEL=info
LOG_FORMAT=json
```

### Usage

```bash
# Index a codebase
curl -X POST http://localhost:8080/api/index \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/your/codebase"}'

# Query the codebase
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "How does authentication work?"}'
```

## Architecture

```
User Query → Intent Detection → Embedding → Hybrid Retrieval
  → Hierarchical Filtering → Reranking → Prompt Assembly → LLM
```

Background indexing pipeline:
```
File Change → Parse → Chunk → Embed → Vector Store
```

## Project Structure

```
rag-code/
├── cmd/rag-server/           # Entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── logger/              # Structured logging
│   ├── errors/              # Error handling
│   ├── validator/           # Input validation
│   ├── domain/              # Domain models
│   ├── indexing/            # Code parsing and chunking
│   ├── embeddings/          # Embedding generation
│   ├── vectorstore/         # Vector DB operations
│   ├── retrieval/           # Query and search
│   ├── hierarchy/           # Context selection
│   ├── reranker/            # Result reranking
│   ├── prompt/              # Prompt assembly
│   ├── llm/                 # LLM integration
│   └── api/                 # HTTP API
└── docs/                    # Documentation
```

## Development

### Current Status

Phase 0 and Phase 1 complete. See implementation guide for next steps.

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o rag-server cmd/rag-server/main.go
```

## Documentation

- [Architecture](docs/architecture/ARCHITECTURE.md)
- [Implementation Plan](cursor_rag_implementation_plan.md)

## License

MIT
