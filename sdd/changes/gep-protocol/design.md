# Genome Evolution Protocol (v6.0) Technical Design

## 1. Technical Approach
Implement a multi-file mutation engine via a structured JSON patch prompt, wrapped in an atomic Ouroboros evaluation loop that validates compilation (`just build`) and tests (`go test`). If the evaluation phase detects panics or compilation failures, a unified rollback mechanism restores all affected files from `.bak` snapshots.

## 2. Architecture Decisions

### Decision 1: Multi-File Mutations via JSON Patch
* **Choice**: The Sampling prompt will strictly enforce a JSON array schema for file modifications: `[{"file": "path", "content": "..."}]`.
* **Alternatives**: Unified diffs or line-by-line replacement.
* **Rationale**: JSON is deterministically parseable by the orchestrator, avoiding regex/parsing hell of unified diffs. It perfectly supports modifying `handlers.go` and `server.go` simultaneously in a single generation step.

### Decision 2: Safe Evaluation (Ouroboros Loop)
* **Choice**: Post-mutation, the system synchronously executes `just build` and `go test ./...`. Any non-zero exit code or detected panic (via standard error stream analysis) immediately triggers a full state rollback.
* **Alternatives**: Asynchronous validation or partial compilation.
* **Rationale**: Pure empirical absolute. The Ouroboros loop guarantees that the application remains in a deployable state. A compilation panic or test failure means the mutation is genetically invalid and must be discarded.

### Decision 3: Atomic State Rollback
* **Choice**: Expand the existing atomic `.bak` mechanism. Before any JSON patch is applied, every target file is copied to `<filename>.bak`. If evaluation fails, a deterministic rollback script restores all `.bak` files concurrently.
* **Alternatives**: Git stash/checkout or memory buffers.
* **Rationale**: `.bak` files are native to the file system, simple, and bypass potential Git lock issues during rapid iterative loops.

### Decision 4: Prompt Engineering
* **Choice**: The system prompt will mandate zero-slop JSON output.
* **Prompt Structure**:
  ```text
  You are an expert genome mutator. You must output EXACTLY a valid JSON array of objects and NO OTHER TEXT. 
  Each object must have "file" (relative path) and "content" (complete new file content).
  Example:
  [
    {"file": "internal/mcp/handlers.go", "content": "..."},
    {"file": "internal/mcp/server.go", "content": "..."}
  ]
  Failure to comply will result in immediate termination.
  ```
* **Rationale**: LLMs naturally drift into markdown or chitchat. Strict bounding ensures the parser does not fail on artifacts.

## 3. Data Flow (Ouroboros Cycle)

```text
+-----------------------+
|  Sampling Prompt      |
|  (LLM Generation)     |
+-----------+-----------+
            | JSON Array: [{"file": "...", "content": "..."}]
            v
+-----------------------+
|  Atomic Snapshot      |
|  Create *.bak files   |
+-----------+-----------+
            |
            v
+-----------------------+
|  Apply Mutations      |
|  Overwrite sources    |
+-----------+-----------+
            |
            v
+-----------------------+         [Panic/Fail]
| Safe Evaluation       | ---------------------+
| (just build & go test)|                      |
+-----------+-----------+                      v
            | [Success]             +-----------------------+
            v                       | State Rollback        |
+-----------------------+           | Restore *.bak files   |
| Commit / Archive      |           +-----------------------+
| Remove *.bak files    |                      |
+-----------------------+                      v
                                    [Discard Mutation]
```

## 4. File Changes (Impact-Verified)

| File | Action | Rationale |
| :--- | :--- | :--- |
| `internal/engine/executor.go` | Modify | Update the execution loop to process multi-file JSON arrays and handle the Ouroboros safe evaluation logic. |
| `internal/engine/manifest.go` | Modify | Expand state rollback to iterate over an array of modified files, restoring their `.bak` counterparts on failure. |
| `internal/mcp/prompts.go` | Modify | Embed the new strict JSON prompt engineering template for the Sampling phase. |
