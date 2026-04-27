# 🛡️ TECHNICAL DESIGN: Atomic Self-Healing (v5.0)

## 1. Technical Approach
Implement an autonomous test-failure resolution loop via MCP. The system leverages `goTestFailureRegex` to extract failure context, applies LLM-generated fixes in-place with Git-backed rollbacks, and verifies using a localized test runner before committing the change.

## 2. Architecture Decisions

### Decision 1: RCA Engine Reusability
- **Choice:** Reuse `goTestFailureRegex` from `internal/filter/actions.go`.
- **Alternatives:** Implement a new AST-based error parser.
- **Rationale:** `goTestFailureRegex` is already validated and integrated within the project for test output extraction, providing absolute signal without redundant development.

### Decision 2: In-Place Sandboxing with Git Rollback
- **Choice:** Apply fixes directly to the working tree, backed by `git restore <file>` on failure.
- **Alternatives:** Copy the entire project to a temporary directory (`os.MkdirTemp`).
- **Rationale:** Go modules and LSP references are highly sensitive to directory paths. In-place modification ensures accurate verification while Git provides native, atomic rollback capabilities for the sandbox.

### Decision 3: MCP-Integrated Verification
- **Choice:** Embed the `go test` verification runner directly within the MCP `SelfHeal` handler.
- **Alternatives:** External bash scripts or delegating to the CLI engine.
- **Rationale:** Tight coupling in the MCP handler guarantees minimal latency and prevents external state mutation during the self-healing loop.

### Decision 4: Sampling Prompt Strictness
- **Choice:** Zero-slop, raw-code-only system prompt.
- **Alternatives:** Standard conversational prompts.
- **Rationale:** Prevents context window pollution and eliminates the need to parse markdown blocks before applying the patch.

## 3. Data Flow
```ascii
+-------------------+
|  Failed Go Test   |
+--------+----------+
         |
         v
+-------------------+
| RCA Engine        |  <-- Applies goTestFailureRegex
| (Extract Context) |
+--------+----------+
         |
         v
+-------------------+
| LLM Fix Generator |  <-- Uses Zero-Slop Sampling Prompt
+--------+----------+
         |
         v
+-------------------+
| Sandbox Execution |  <-- Applies fix in-place
+--------+----------+
         |
         v
+-------------------+  (Pass)  +-------------------+
| MCP Test Runner   | -------> | Preserve Fix      |
+--------+----------+          +-------------------+
         | (Fail)
         v
+-------------------+
| Git Rollback      |  <-- Executes `git restore <file>`
+-------------------+
```

## 4. File Changes (Impact-Verified)
| File | Action | Rationale |
|------|--------|-----------|
| `internal/filter/actions.go` | Modify | Expose `goTestFailureRegex` for MCP consumption if currently unexported. |
| `internal/mcp/handlers.go` | Modify | Implement the `scouter_self_heal` MCP endpoint and inline test runner logic. |
| `internal/mcp/prompts.go` | Add | Introduce the strict sampling prompt for the LLM fix generation. |
| `internal/utils/git.go` | Modify | Add atomic `RestoreFile` helper to facilitate the sandboxed rollback. |

## 5. Interfaces / Contracts

```go
// DTO for triggering the healing loop
type SelfHealRequest struct {
    FilePath string `json:"file_path"`
    TestName string `json:"test_name"`
    ErrorLog string `json:"error_log"`
}

// DTO for loop resolution
type SelfHealResponse struct {
    Success    bool   `json:"success"`
    AppliedFix string `json:"applied_fix"`
    Rollback   bool   `json:"rollback"`
    Message    string `json:"message"`
}
```

### Sampling Prompt (System Instruction)
```text
You are Scouter's Atomic Self-Healing engine.
A test has failed. Review the provided error log and source code.

MANDATES:
1. Fix the specific logic causing the failure.
2. DO NOT output Markdown code blocks (e.g., ```go).
3. DO NOT include explanations, comments, or apologies.
4. Return ONLY the raw code replacement for the target block.
5. Adhere to Go 1.24+ idioms.
```