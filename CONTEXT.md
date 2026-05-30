# 🗺️ CONTEXT (Map) - Scouter

Guía rápida de navegación para agentes en el codebase de Scouter.

## 📂 Directorios Clave

- `cmd/scouter/`: Punto de entrada de la CLI.
- `internal/engine/`: Core de inteligencia estructural.
  - `treesitter.go`: Consultas multi-lenguaje (Go, TS, Py, Rs).
  - `parser.go`: Análisis profundo nativo de Go (AST + TypeInfo).
  - `lsp/`: Gestión de servidores de lenguaje y persistencia (Warp Speed).
  - `impact.go`: Análisis de blast radius y propagación.
- `internal/mcp/`: Implementación del servidor Model Context Protocol.
- `internal/store/`: Persistencia en SQLite y búsqueda vectorial/texto.
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
