# The Shinigami Protocol (Solver-Verifier)

The Shinigami Protocol is Scouter's autonomous healing engine. It uses a Solver-Verifier model to ensure that fixes are not only correct but also structurally sound.

## 1. Root Cause Analysis (RCA)
The engine parses the error log to identify the failing file and line number. It then uses the AST to resolve the exact symbol (function, method, struct) that is failing.

## 2. Parallel Solvers
The engine spawns multiple parallel solvers (LLM instances) to generate potential fixes. Each solver is provided with:
- The failing code fragment.
- The error log.
- Enriched context from the LSP (Hover, Definition, References).

## 3. The Staging Ledger
Proposed fixes are NOT written directly to disk. They are staged in the `Ledger`, an in-memory overlay filesystem. This allows the engine to test multiple fixes without corrupting the workspace.

## 4. Verification
The engine runs the test suite against the staged changes in the Ledger.
- If the tests pass, the fix is considered verified.
- If the tests fail, the engine feeds the new error log back into the solvers for another iteration.

## 5. Impact Analysis
Before a verified fix is committed, the engine calculates its blast radius using the Call Graph. If the impact is too large or affects critical code paths, the fix may be rejected or flagged for manual review.

## 6. Commit
Once a fix is verified and its impact is deemed acceptable, the Ledger commits the changes to disk.