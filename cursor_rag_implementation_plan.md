# Cursor-style RAG System — Go Implementation Plan

This document describes a professional, production-grade Go project structure
and an implementation plan for building a Cursor-style, code-aware RAG system.

---

## 1. System Goal

Build a system that:
- Understands large codebases
- Retrieves the right code for a query
- Grounds LLM responses to reduce hallucinations
- Supports explain, edit, and refactor workflows

Core principle:
Retrieval and ranking do the hard thinking. The LLM does the final reasoning.

---

## 2. High-Level Architecture

User Query  
→ Intent and Scope Detection  
→ Query Embedding  
→ Hybrid Retrieval (Dense + Sparse)  
→ Hierarchical Filtering  
→ Reranker + Heuristics  
→ Context Selection  
→ Prompt Assembly  
→ LLM  

Background:
File Change → Parse → Chunk → Embed → Local Vector Store

---

## 3. Professional Go Project Structure

```
rag-system/
├── cmd/
│   └── rag-server/
│       └── main.go
├── internal/
│   ├── app/
│   ├── config/
│   ├── indexing/
│   │   ├── watcher.go
│   │   ├── parser.go
│   │   ├── chunker.go
│   │   └── indexer.go
│   ├── embeddings/
│   ├── vectorstore/
│   ├── retrieval/
│   ├── hierarchy/
│   ├── reranker/
│   ├── prompt/
│   ├── llm/
│   ├── api/
│   └── domain/
├── scripts/
├── docs/
│   └── architecture/
├── go.mod
└── README.md
```

---

## 4. Phase-wise Plan

Phase 1: Indexing foundation  
Phase 2: Embeddings and vector store  
Phase 3: Hybrid retrieval  
Phase 4: Hierarchical selection  
Phase 5: Reranker and heuristics  
Phase 6: Prompt assembly  
Phase 7: LLM integration  

---

## 5. Summary

This is a Go-based, retrieval-first, structure-aware RAG system.