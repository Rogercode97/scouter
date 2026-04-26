# 🏛️ TASKS: Impact Analysis Engine (Omniscience Edition) 👑

## 🔱 PHASE 1: FOUNDATION (SCHEMA & TYPES)
- [x] **DB Migration**: Add `indegree` (INTEGER) and `link_type` (TEXT) columns to `calls` table in `internal/telemetry/schema.go`.
- [x] **Type Definition**: Implement `ImpactResult`, `ImpactEntity`, and `RiskMetrics` structs in `internal/types/types.go`.
- [x] **Store Interface**: Add `GetImpact` method signature to `Store` interface in `internal/store/store.go`.

## 🧠 PHASE 2: CORE LOGIC (RECURSIVE ENGINE)
- [x] **CTE Implementation**: Write Recursive CTE query for `GetImpact` in `internal/store/store.go` to traverse callers up the tree.
- [x] **Risk Scoring**: Implement dynamic scoring logic (0.0-1.0) based on centrality and blast radius in `internal/store/store.go`.
- [x] **Parser Update**: Enhance `internal/engine/parser.go` to extract `CalleePath` and update `indegree` counts during indexing.

## 🔌 PHASE 3: WIRING (CLI & TOOLING)
- [x] **Tool Registration**: Define and register `scouter_impact` tool in `cmd/scouter/main.go`.
- [x] **Mermaid Generation**: Implement Mermaid.js flowchart exporter for the impact graph.
- [x] **Integration**: Connect `scouter_impact` tool to the `Store.GetImpact` implementation.

## 🧪 PHASE 4: VERIFICATION (ZERO SLOP)
- [x] **Unit Tests**: Test recursive traversal logic with mock graphs in `internal/store/store_test.go`.
- [x] **Risk Tests**: Validate risk calculation edge cases (cycles, deep chains).
- [x] **Integration Test**: Verify end-to-end `scouter_impact` output against a sample project.
