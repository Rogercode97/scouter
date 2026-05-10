# 📐 Scouter Architecture

Scouter is a **Sovereign Structural Intelligence Engine** designed to bridge the gap between static code analysis and autonomous AI reasoning. It provides high-fidelity AST mapping, impact analysis, and atomic refactoring tools through a modular, hex-inspired architecture.

## 🔱 Sovereign Anchors

| Pattern | Rationale |
| :--- | :--- |
| **Hexagonal Isolation** | The core domain (TruthEngine) is decoupled from MCP handlers and LSP infrastructure via clear Ports & Adapters. |
| **Screaming Architecture** | The directory structure (`internal/engine`, `internal/mcp`, `internal/store`) clearly reveals its purpose: Analyze, Communicate, Persist. |
| **Validation is Finality** | No destructive mutation is applied without passing through the **Staging Ledger** and the **Impact Engine**. |
| **Deep Modules** | The `TruthEngine` provides a high-leverage interface that hides the complexity of Tree-sitter, SQLite, and LSP coordination. |

## 🏗️ The Three Pillars

### 1. The Truth Engine (`internal/engine`)
The heart of Scouter. It coordinates specialized "organs" to achieve structural omniscience:
- **Search Engine**: AST-aware discovery using Tree-sitter patterns.
- **Impact Engine**: Calculates the "blast radius" of changes using the Global Call Graph.
- **Ripple Engine**: Propagates changes across interfaces and implementations.
- **Healer Engine**: Autonomous RCA (Root Cause Analysis) and TDD-driven fixing.

### 2. The Persistence Vault (`internal/store`)
A high-performance SQLite-backed repository that stores:
- **Symbol Index**: A comprehensive map of every AST node and its metadata.
- **Call Graph**: A bidirectional map of call sites and dependencies.
- **Engram Link**: Integration with long-term memory to preserve architectural wisdom.

### 3. The Sovereign Adapter (`internal/mcp`)
The primary interface for AI agents. It implements the **Model Context Protocol (MCP)**, providing:
- **Tools**: Direct access to Scouter's analytical and mutation powers.
- **Resources**: Real-time access to ADRs, the Staging Ledger, and the Roadmap.
- **Instructions**: Sovereign mandates inyected into the agent's system prompt.

## 🌊 Core Workflow: The Evolutionary Loop

1. **Scout**: Map the territory using `index` and `search`.
2. **Analyze**: Calculate risk and reach using `impact` and `critical_code`.
3. **Transform**: Stage atomic mutations in the **Staging Ledger** via `ripple_refactor` or `evolve`.
4. **Verify**: Prove integrity via `predict` (testing) and `self_heal`.
5. **Seal**: Commit changes to disk and persist the discovery in **Engram**.

---
*True engineering is the art of building systems that remain sovereign under the pressure of change. Hakai.*
