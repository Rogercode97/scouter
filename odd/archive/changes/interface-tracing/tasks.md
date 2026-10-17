# 📋 TASKS: Interface Tracing (LSP-Enhanced) 🛡️

Change: `interface-tracing`
Status: **STRIKE READY**
Wave: 8.9 (Pure Impact)

## 🏗️ Phase 1: Foundation (LSP & Types)

- [ ] **LSP Client Extension** (`internal/engine/lsp/client.go`)
    - **Action**: Implement `Implementation(ctx context.Context, params ImplementationParams) ([]Location, error)` in `LSPClient` interface and its implementation.
    - **Aceptación**: El cliente LSP puede solicitar implementaciones de un símbolo a un servidor LSP (e.g., gopls).
- [ ] **LSP Types Update** (`internal/engine/lsp/types.go`)
    - **Action**: Add `ImplementationParams` struct following LSP 3.17 specification.
    - **Aceptación**: Estructura compatible con la petición `textDocument/implementation`.

## 🧠 Phase 2: Core Implementation (Enricher & Store)

- [ ] **Store Enhancement** (`internal/store/store.go`)
    - **Action**: 
        - Ensure `SaveCall` (or equivalent method in `Store`) accepts and persists `LinkType` (Static vs Interface).
        - Add `GetSymbolsByType(ctx context.Context, kind string) ([]Symbol, error)` to filter interfaces.
    - **Aceptación**: La DB puede distinguir y recuperar interfaces para su procesamiento posterior.
- [ ] **Interface Enricher Component** (`internal/engine/enricher.go`)
    - **Action**: Create `Enricher` struct that iterates over indexed interface symbols and queries LSP for their implementations, then saves the links in `Store`.
    - **Aceptación**: `Enricher.Process()` genera nuevas entradas de llamadas basadas en resoluciones dinámicas de interfaces.

## 🔌 Phase 3: Wiring & Integration (CLI)

- [ ] **CLI Flag Integration** (`internal/cli/flags.go` & `cmd/scouter/main.go`)
    - **Action**: Add `--enrich` boolean flag to the `index` command.
    - **Aceptación**: `scouter index --enrich` ejecuta el pipeline de enriquecimiento post-indexación.
- [ ] **Pipeline Orchestration** (`internal/engine/pipeline.go`)
    - **Action**: Inject the `Enricher` into the indexing pipeline, triggered if the `--enrich` flag is present.
    - **Aceptación**: El flujo de datos pasa del indexador base al enriquecedor de interfaces.

## ✅ Phase 4: Validation (Proof of Impact)

- [ ] **Integration Test: Interface Tracing** (`tests/integration_interface_test.go`)
    - **Action**: Write a test using a Go fixture with an interface and two implementations. Run indexer with `--enrich` and verify that `scouter_callers` returns both implementations for the interface method.
    - **Aceptación**: `go test ./tests/...` pasa con 100% de éxito y evidencia empírica de traza de interfaces.

---
*Execute with precision. Hakai.*
