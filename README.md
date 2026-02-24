# RAG System for Codebases

A production-grade RAG system combining dense vector search and sparse keyword retrieval for deep code understanding.

## Features

- **Hybrid Retrieval**: Combines Qdrant (Vector) and Redis (Keyword/BM25) with RRF fusion.
- **Deep Indexing**: AST-based parsing for chunking of Go code.
- **Hierarchical Context**: Understanding from file to function level.
- **LLM Integration**: Works seamlessly with local Ollama models.
- **Automated Docs**: Swagger/OpenAPI documentation auto-generated.

## Infrastructure Setup

The system uses a hybrid deployment model for the best local development experience:
- **Infrastructure**: Qdrant and Redis run in Docker containers (via Docker Compose).
- **Application**: The Go server runs natively on your host for full filesystem access.

### Prerequisites
- Go 1.22+
- Docker & Docker Compose
- [Ollama](https://ollama.ai) running locally

### Quick Start

1. **Start Infrastructure** (Qdrant & Redis)
   ```bash
   docker-compose up -d
   ```

2. **Build Server**
   ```bash
   go build -o rag-server cmd/rag-server/main.go
   ```

3. **Run Server**
   ```bash
   ./rag-server
   ```
   The API will be available at `http://localhost:8080`.

## API Usage

### Indexing a Codebase
You can index any folder on your local machine.

```bash
curl -X POST http://localhost:8080/api/index \
  -H "Content-Type: application/json" \
  -d '{"path": "/Users/guru/projects/my-app"}'
```

### Querying
Ask natural language questions about your code.

```bash
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How is authentication handled?",
    "max_results": 5
  }'
```

## API Documentation

Swagger UI is available at:
`http://localhost:8080/swagger/index.html`

Regenerate docs:
```bash
swag init -g cmd/rag-server/main.go
```

## Configuration

Configuration is managed via `.env` file.

```env
# Ollama
OLLAMA_URL=http://localhost:11434
EMBEDDING_MODEL=all-minilm
LLM_MODEL=llama3.2:1b

# Databases
VECTOR_STORE_URL=http://localhost:6333
REDIS_URL=localhost:6379

# Hybrid Search Tuning
HYBRID_ENABLED=true
HYBRID_VECTOR_WEIGHT=0.7
FUSION_STRATEGY=rrf
```

## Project Structure

```
├── cmd/rag-server/      # Application entrypoint
├── internal/
│   ├── api/             # HTTP handlers & middleware
│   ├── indexing/        # AST parsing & chunking logic
│   ├── retrieval/       # Hybrid search & ranking engine
│   ├── vectorstore/     # Qdrant integration
│   └── domain/          # Core data models
├── docs/                # Generated Swagger docs
└── docker-compose.yml   # Infrastructure orchestration
```
