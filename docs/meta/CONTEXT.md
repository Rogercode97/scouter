# 🗺️ CONTEXT (Map) - Scouter

Guía rápida de navegación para agentes en el codebase de Scouter.

## 📂 Directorios Clave

- `internal/engine/`: Core de inteligencia estructural (Role-based services).
  - `indexer.go`: IndexerPipeline (Batch control, Disk walking).
  - `treesitter.go` & `parser.go`: AST nativo y consultas multi-lenguaje.
  - `semantic.go`: Motor Semántico local (`goformer` + BGE-small).
  - `impact.go` & `ssa.go`: Análisis de flujo de datos y propagación.
- `internal/mcp/`: Servidor MCP. Usa `display.Presenter` para aislar la lógica de UI/formateo.
- `internal/store/`: Persistencia en SQLite (`ncruces/go-sqlite3`) con Dual Pools para evitar deadlocks.
- `tests/`: Tests de integración de alto nivel (Ripple, Closure, Omniscience).

## 🛠️ Herramientas de Desarrollo

- `Makefile`: `make build` para generar el binario en `bin/scouter`.
- `justfile`: Tareas de automatización para desarrollo local.
- `rtk`: Proxy de optimización de tokens (usar para git, go test, etc.).

## 📜 Protocolos Activos

- **Sovereign SDD:** Seguir siempre el ciclo Explore -> Propose -> Spec -> Design -> Apply -> Verify.
- **Docs Alignment:** Cualquier cambio en el naming o arquitectura debe reflejarse en `SABIDURIA.md`.
- **Deep AST:** Las funciones anónimas deben ser indexadas jerárquicamente.

---
*Scouter: Seeing the invisible, analyzing the complex.*
