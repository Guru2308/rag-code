# Cursor-style RAG Implementation Plan

This document is an implementation-first plan to build a Cursor-style, code-aware RAG system.
It is meant to be used directly inside Cursor or other IDEs while starting development.

---

## 1. Goal

Build a system that:
- Understands a large codebase
- Retrieves the right code for a query
- Grounds LLM responses to reduce hallucinations
- Supports explain, edit, and refactor workflows

Core principle:
Retrieval and ranking do the hard thinking. LLM does the final reasoning.

---

## 2. High-Level Architecture

User Query
  → Intent and Scope Detection
  → Query Embedding
  → Hybrid Retrieval (Dense and Sparse)
  → Hierarchical Filtering
  → Reranker and Heuristics
  → Context Selection
  → Prompt Assembly
  → LLM

Background (always running):
File Change → Parse → Chunk → Embed → Local Vector Store

---

## 3. Phase-wise Implementation Plan

### Phase 1: Repository Indexing

- Parse code using Tree-sitter or native parsers
- Extract functions, classes, structs, interfaces
- Ignore vendor, node_modules, generated files

### Phase 2: Embeddings and Vector Storage

- Generate code embeddings
- Use local ANN index (FAISS or HNSW)
- Re-embed only changed files

### Phase 3: Query-Time Retrieval

- Detect intent: explain, edit, refactor
- Perform dense vector search
- Perform sparse symbol and path search
- Merge candidate results

### Phase 4: Hierarchical Selection

- Apply Repo → Module → File → Symbol → Block filtering
- Remove irrelevant chunks early

### Phase 5: Reranker and Heuristics

- Vector similarity
- Symbol name match
- File path relevance
- Call graph proximity
- Recency bias
- Chunk type weighting
- Deduplication

### Phase 6: Prompt Assembly

- Combine system instructions, user query, and code context
- Respect token limits
- Enforce edit or explain constraints

### Phase 7: LLM Integration

- Send grounded prompt to cloud LLM
- Return explanation, code, or diff

---

## 4. Background Indexing Pipeline

Trigger on file open, save, or delete.

Change → Parse → Chunk → Embed → Update Vector Store

---

## 5. Milestones

Week 1: Parsing, chunking, embeddings
Week 2: Hybrid retrieval, hierarchy filtering
Week 3: Reranking, prompt assembly
Week 4: LLM integration, testing

---

## 6. Definition of Done

- Accurate code retrieval
- Minimal hallucination
- Fast response time
- Clear explain vs edit behavior

---

## 7. Summary

This is a retrieval-first, structure-aware RAG system for codebases.
