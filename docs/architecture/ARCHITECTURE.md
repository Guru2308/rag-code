# RAG System Architecture

## Overview

This document describes the architecture of the RAG (Retrieval-Augmented Generation) system for code understanding and querying.

## Core Philosophy

**Retrieval does the heavy lifting. The LLM does the final reasoning.**

The system is designed to retrieve the most relevant code context through a sophisticated pipeline, then provide this grounded context to an LLM for generating accurate responses.

## System Components

### 1. Indexing Pipeline

```
File System → File Watcher → Parser → Chunker → Embedder → Vector Store
```

- **File Watcher**: Monitors codebase for changes
- **Parser**: Language-aware AST parsing to extract semantic units
- **Chunker**: Splits code at natural boundaries (functions, classes)
- **Embedder**: Generates embeddings for semantic search
- **Vector Store**: Stores embeddings with metadata

### 2. Retrieval Pipeline

```
Query → Preprocessor → Hybrid Search → Hierarchical Filter → Reranker → Context Builder
```

- **Preprocessor**: Cleans and expands query
- **Hybrid Search**: Combines dense (vector) and sparse (keyword) search
- **Hierarchical Filter**: File-level → Function-level selection
- **Reranker**: Code-aware relevance scoring
- **Context Builder**: Assembles final context with dependencies

### 3. Generation Pipeline

```
Context → Prompt Builder → LLM → Response Stream
```

- **Prompt Builder**: Constructs prompts with context and metadata
- **LLM**: Generates responses based on grounded context
- **Response Stream**: Streams results to user

## Data Flow

### Indexing Flow

1. File change detected by watcher
2. File parsed into AST
3. Code chunked at semantic boundaries
4. Each chunk embedded
5. Embedding + metadata stored in vector database

### Query Flow

1. User submits query
2. Query preprocessed and embedded
3. Hybrid search retrieves candidates
4. Hierarchical filtering narrows results
5. Reranker scores by relevance
6. Context builder assembles prompt context
7. LLM generates response
8. Response streamed to user

## Design Decisions

### Why Hierarchical Retrieval?

Starting broad (file-level) then narrowing (function-level) ensures we capture complete context while staying within token limits.

### Why Hybrid Search?

- **Vector search**: Captures semantic similarity
- **Keyword search**: Ensures exact matches (function names, APIs)
- **Combined**: Best of both approaches

### Why Code-Aware Chunking?

Generic chunking (e.g., every 100 lines) can split functions or classes mid-definition, losing semantic meaning. Our chunker respects code structure.

### Why Reranking?

Initial retrieval casts a wide net. Reranking applies code-specific heuristics:
- Recency (recently modified files)
- Popularity (frequently accessed code)
- Dependency relationships
- Call graph proximity

## Technology Choices

- **Language**: Go for performance and simplicity
- **Vector Store**: Qdrant for efficient vector search
- **Logging**: Standard library `log/slog` for structured logging
- **LLM**: OpenAI/Anthropic for generation

## Scalability Considerations

- Incremental indexing (only changed files)
- Batch embedding generation
- Vector database sharding
- Async processing pipeline

## Future Enhancements

- Multi-language support (currently Go-focused)
- Graph-based dependency tracking
- User feedback loop for relevance tuning
- Custom embedding fine-tuning
