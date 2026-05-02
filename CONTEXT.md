# Scouter Domain Context (WAVE 12.0)

This file defines the core technical lexicon and architectural pillars of Scouter. Adhere to these terms to ensure **Locality** and **Depth**.

## 🧠 Core Concepts

- **TruthEngine** (Module): The central orchestrator of Scouter. It provides a deep interface for all truth-seeking operations (Index, Impact, Search, Fix, Propagate). Callers (CLI, MCP) MUST delegate to the TruthEngine.
- **Global Call Graph** (Domain): The structural map of the codebase, maintained in SQLite, representing all symbolic relationships (Calls, Interfaces, Emits).
- **Absolute Signal** (Mandate): The principle of minimizing noise in communication (CLI output, context window) to optimize cognitive and token budget (Ki).
- **Oracle** (Seam): The capability to request high-level architectural proposals from an LLM. It is abstracted via the `Messenger` interface.

## 🏛️ Architectural Pillars

1. **Hexagonal Isolation**: Business logic (Engines) must be agnostic to the transport layer (MCP, CLI).
2. **Deterministic Analysis**: Prefer AST-based truth over heuristic grep-based search.
3. **Transactional Integrity**: Multi-file or multi-table changes must be atomic and reversible.
