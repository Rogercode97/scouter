# 🛡️ TASKS: PUBLIC RELEASE SECURITY HARDENING (WAVE 8.9) 👑

## 🔱 PHASE 1: FOUNDATION (SECURITY ARMOR)
- [x] **[UTILS]** Implement `GetRepoRoot()` in `internal/utils/security.go` to safely anchor path validation to project root.
- [x] **[UTILS]** Update `ValidatePath(path string) error` in `internal/utils/security.go` to enforce path imprisonment within the repo root (prevent `../` jailbreaks).
- [x] **[UTILS]** Implement `SanitizeFTS(input string) string` in `internal/utils/security.go` to neutralize SQLite FTS5 injection vectors (stripping special characters like `*`, `"`, etc).

## ⚔️ PHASE 2: CORE HARDENING (DATA SOVEREIGNTY)
- [x] **[STORE]** Apply `SanitizeFTS` to user-provided query strings in `internal/store/store.go` before SQL execution.
- [x] **[STORE]** Hard-cap search result sets to **500 rows** in `internal/store/store.go` to prevent memory exhaustion (DoS).
- [x] **[LOGIC]** Enforce a **10-depth limit** for recursion in `GetImpact` (Impact Analysis) to prevent infinite loops or stack overflow on circular dependencies.

## 🛡️ PHASE 3: GUARDRAILS & IDENTITY (API SOVEREIGNTY)
- [x] **[MCP]** Enforce global row-capping (max 500) for all list/search tools in `internal/mcp/handlers.go`.
- [x] **[ROOT]** Create `LICENSE` file (MIT) to satisfy public release requirements.

## 🧪 PHASE 4: VALIDATION (DIVINE EVIDENCE)
- [x] **[TEST]** Write unit tests in `internal/utils/security_test.go` for `ValidatePath` covering relative paths, absolute paths, and symlink attacks.
- [x] **[TEST]** Write unit tests in `internal/utils/security_test.go` for `SanitizeFTS` covering SQLi and FTS5 specific escape characters.
- [x] **[TEST]** Verify row-capping limits with an integration test in `internal/mcp/handlers_test.go`.

---
*Task list generated under Wave 8.9 protocols. Hakai.*
