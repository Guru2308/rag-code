# RAG System Implementation Tasks

## Phase 0: Project Setup
- [x] Initialize Go module and project structure
- [x] Set up configuration management (Ollama)
- [x] Create README and documentation structure
- [x] Set up logging infrastructure
- [x] Set up error handling utilities
- [x] Set up validation utilities

## Phase 1: Indexing Foundation
- [x] Implement file watcher for codebase changes
- [x] Build language-aware code parser (AST parsing)
- [x] Implement intelligent code chunker
- [x] Create indexer to store parsed code

## Phase 2: Embeddings and Vector Store
- [x] Set up Ollama embedding service
- [x] Implement vector store (using Qdrant or similar)
- [x] Create embedding pipeline for code chunks
- [x] Build batch processing for large codebases

## Phase 3: Hybrid Retrieval
- [x] Implement dense vector search
- [x] Add sparse/BM25 keyword search
- [x] Create hybrid retrieval combiner
- [x] Build query preprocessing

## Phase 4: Hierarchical Selection
- [x] Implement file-level filtering
- [x] Add function/class-level selection
- [x] Create dependency graph builder
- [x] Build context expansion logic

## Phase 5: Reranker and Heuristics
- [x] Implement reranking model
- [x] Add recency and relevance heuristics
- [x] Create score fusion logic
- [x] Build context deduplication

## Phase 6: Prompt Assembly
- [x] Design prompt templates
- [x] Implement context window management
- [x] Create prompt builder with truncation
- [x] Add metadata injection

## Phase 7: LLM Integration
- [x] Set up Ollama LLM client
- [x] Implement streaming responses
- [x] Add error handling and retries
- [x] Build API server with endpoints

## Phase 8: Testing and Refinement
- [x] Write unit tests for each component
- [x] Create integration tests
- [x] Build example queries and test cases
- [x] Performance optimization

## Phase 9: Chunker and Indexer Improvements
- [x] Implement chunk merging logic to avoid small fragments
- [x] Implement context-aware splitting (respect code blocks, braces)
- [x] Add overlapping context for better continuity
- [x] Preserve important metadata (function/class names) when splitting
- [x] Implement batch processing for vector store insertions
- [x] Add retry logic for failed indexing operations
- [x] Implement incremental indexing (file hash tracking)
- [x] Add metrics and monitoring for indexing process

## Phase 10: Retrieval and Prompt Enhancements
- [x] Improve graph-based context expansion (parent/child relations)
- [x] Add sophisticated reranking heuristics (recency, file priority)
- [x] Implement context window management and truncation in prompt assembly
- [x] Implement a proper prompt for a professional code assistant that helps to understand the codebase and a professional code reviewer
- [x] Add support for other languages (Add-ons)
- [x] Add multi language project support (Add-ons)
- [x] MMR Implementation (Add-ons)
