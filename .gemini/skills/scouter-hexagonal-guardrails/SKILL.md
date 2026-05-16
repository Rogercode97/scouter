---
name: scouter-hexagonal-guardrails
description: "WHEN: 'refactor', 'new feature', 'architectural change'. Mandates for maintaining Scouter's Hexagonal and Screaming Architecture integrity."
version: "1.0.0"
tags: [architecture, hexagonal, screaming]
last_updated: 2026-05-16
---

# Scouter Hexagonal Guardrails (Wave 14.5)

**Goal**: Protect the integrity of Scouter's Hexagonal architecture, ensuring the domain remains sovereign and decoupled from infrastructure and delivery mechanisms.

> **LIVE LIBRARY**: See `docs/ARCHITECTURE.md` for the core architectural pillars and the Evolutionary Loop.

## 🔱 MANDATES
- **Domain Sovereignty**: ALL business logic and analytical intelligence MUST reside in `internal/engine/`.
- **Infrastructure Isolation**: `internal/store/` (Persistence) and `internal/mcp/` (Delivery) MUST remain thin adapters.
- **Dependency Rule**: Dependencies MUST point inward toward the domain. The domain (`internal/engine/`) MUST NOT depend on adapters (`internal/mcp/`, `internal/cli/`).
- **No Direct DB Access**: Adapters MUST NOT query the database directly. They MUST use the `TruthEngine` or its specialized components.

## 🏗️ DIRECTORY OWNERSHIP
| Directory | Responsibility | Constraint |
|---|---|---|
| `internal/engine/` | Core Domain (The "Truth"). | NO dependencies on MCP/CLI. |
| `internal/mcp/` | Delivery Adapter (JSON-RPC). | Strictly for handling requests and delegating to Engine. |
| `internal/store/` | Persistence Adapter (SQLite). | Strictly for storage and retrieval logic. |

## 🔄 PROTOCOL
1. **Analyze Intent**: Determine if the change is a core analytical capability or a delivery/persistence detail.
2. **Place Logic**: 
    - Business logic? -> `internal/engine/`
    - New tool/resource? -> `internal/mcp/`
    - New query/schema? -> `internal/store/`
3. **Verify Boundaries**: Run `scouter impact` to ensure no illegal dependencies were introduced.

## 🚩 RED FLAGS
- Logic leaking from `internal/engine/` into `internal/mcp/`.
- The `engine` package importing `mcp` or `cli` packages.
- Direct database calls from the MCP tool handlers.

## 🧠 COMMON RATIONALIZATIONS
| Rationalization | Reality |
|---|---|
| "It's just a small validation in the handler." | Small leaks lead to a big mess. Validation is part of the Domain's contract. |
| "I need this database field directly." | Add a method to the `Store` and expose it via the `Engine`. Respect the boundaries. |

## 📜 SUCCESS HEURISTIC
The architecture remains "Screaming": the directory structure clearly reveals the system's intent, and the domain remains pure and testable in isolation.

<!-- MCP:START -->
## MCP Availability And Fallback
Preferred MCP Servers: `scouter`.
<!-- MCP:END -->
