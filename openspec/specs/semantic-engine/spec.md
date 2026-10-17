---
id: "SEMANTIC_ENGINE"
title: "Semantic Engine: Pure Go Vector Embeddings"
status: "proposed"
created_at: "2026-06-03"
---

# Semantic Engine Specification

## Overview
Scouter requires a "Semantic Engine" capable of understanding user queries by meaning (semantic search) rather than exact text matching. To maintain Scouter's identity as a lightweight, standalone, single-binary application without CGO or Python dependencies, we must avoid external embedding APIs or C-compiled extensions.

## Architecture Decisions

### 1. The Encoder (`goformer`)
We will integrate `github.com/MichaelAyles/goformer`.
- **Why:** It enables inference of transformer models (like `BGE-small-en-v1.5`) natively in Go by directly reading HuggingFace `.safetensors` and `tokenizer.json` files.
- **Benefits:** No CGO, no ONNX, no Python. Fast embedding generation localized entirely within the Go process memory.

### 2. Vector Database (`sqlite-vec` via WebAssembly)
We will transition our SQLite driver to `github.com/ncruces/go-sqlite3` and use the WebAssembly bindings for `sqlite-vec` (`github.com/asg017/sqlite-vec-go-bindings/ncruces`).
- **Why:** The current driver (`modernc.org/sqlite`) does not support dynamic `.so`/`.dylib` C extensions. `sqlite-vec` is a C extension.
- **How it works:** `ncruces/go-sqlite3` compiles SQLite and the extension to WASM, which is then executed safely within Go without needing CGO. This preserves the CGO-free nature of Scouter while providing state-of-the-art vector similarity search (KNN).

## Implementation Plan (sdd-apply)

1.  **Dependency Migration (Storage Layer):**
    - Replace `modernc.org/sqlite` with `github.com/ncruces/go-sqlite3` across all `internal/store` packages.
    - Add `github.com/asg017/sqlite-vec-go-bindings/ncruces`.
    - Validate that existing AST structural indices continue to work perfectly.

2.  **Vector Schema Setup:**
    - Create a virtual table using `sqlite-vec`: `CREATE VIRTUAL TABLE vec_ast USING vec0(embedding float[384]);`
    - Create relational mapping between AST symbols and their vector ID.

3.  **Encoder Integration (goformer):**
    - Import `github.com/MichaelAyles/goformer`.
    - Create `internal/engine/semantic.go` with a worker pool for generating embeddings asynchronously as the TruthEngine parses files.
    - Embed or download a lightweight Transformer model (`BGE-small-en-v1.5` is ~133MB, we can require downloading it to `~/.scouter/models/` at startup or runtime).

4.  **Query Interface Update:**
    - Update the CLI and search API to detect natural language queries vs structural queries.
    - Route semantic queries through `goformer` to generate a vector array, then query `vec0` using `MATCH` and `ORDER BY distance`.

## Risks and Mitigations
- **Model Size:** Embedding a ~100MB model inside the binary inflates the binary size.
  *Mitigation:* Download the model on-demand during the first `scouter index --semantic` run.
- **WASM Overhead:** WASM SQLite might introduce a slight overhead compared to the current transpiled `modernc` C code.
  *Mitigation:* Run benchmarks. Standard `ncruces` benchmarks show it is highly competitive and often faster.
