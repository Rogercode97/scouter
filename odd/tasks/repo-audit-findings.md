# Feature: cierra los hallazgos de la auditoría de main (42faa62)

## Specs

- **S1.** El invariante vectorial deja de depender de la memoria del llamador: "Every path-clearing transaction MUST call `ClearSemanticVectors` before `ClearSymbols`" (AGENTS.md:16) se cumple porque `ClearSymbols` limpia primero `vec_symbols`. Cobertura en la ruta del healer e index-vault, no solo en `SaveFileIndexBatch`.
- **S2.** Un release deja de mentir sobre su versión: el ldflag de `.goreleaser.yaml` apunta al símbolo que existe (`internal/cli.Version`), y un test falla si un `-X` del goreleaser nombra un símbolo inexistente. Evidencia actual: binario con el ldflag publicado imprime `scouter vdev`.
- **S3.** El mandato de execución se vuelve un invariante vigilado: "Never execute raw commands via `exec.Command`. MUST use `internal/utils/safe_exec.go` (`SafeCommand`)" (AGENTS.md:14) se cumple donde es cumplible (git/show/rules/astgrep) y un test de árbol falla en cuanto aparece un `exec.Command` nuevo fuera de las excepciones declaradas (runner del producto, daemon LSP, hook de test).
- **S4.** Ninguna escritura temporal usa una ruta predecible dentro del repositorio: ni `<archivo>.scouter.tmp` (que `os.Remove` puede borrar de un archivo real del usuario) ni un socket fijo compartido en `os.TempDir()`.
- **S5.** Un error de limpieza dentro de una transacción nunca se descarta en silencio, ni en `store` ni en el healer.
- **S6.** La documentación y los gates no afirmran lo contrario del código: el comentario `#nosec G118` duplicado desaparece, las exclusiones de gosec quedan explicadas una por una, y la frase de `-race` en AGENTS.md dice lo que realmente pasa.
- **S7.** Todo lo anterior verificado con `CGO_ENABLED=0 go build ./... && go vet ./... && go test ./...` en verde, y un commit por unidad con su test.

## Tasks

| ID | Spec | Ruta | Commit |
| --- | --- | --- | --- |
| T1 | S1 | `internal/store/store.go` + `internal/store/vec_orphan_test.go` | — |
| T2 | S2 | `.goreleaser.yaml` + guard de ldflags | — |
| T3 | S5 | `internal/store/store.go:881`, `internal/engine/healer.go:261` | — |
| T4 | S4 | `internal/engine/chronos.go`, `internal/engine/evolution_engine.go`, `internal/engine/lsp/manager.go` | — |
| T5 | S6 | `.github/workflows/security.yml`, `internal/engine/pipeline.go`, `AGENTS.md` | — |
| T6 | S3 | `internal/utils/safe_exec.go` + llamadores + guard de árbol | — |
| T7 | S6 | `internal/telemetry/tracker.go` (WAL) | — |

## Log

- **L1.** "audita en worktree"
- **L2.** "usa worktrunk" — la auditoría vive en un worktree creado con `wt switch --create`, no con `git worktree add`.
- **L3.** Elección "Corregir todo lo listado": "Los seis hallazgos, un commit por unidad con su test, empezando por los altos."
- **L4.** 2026-11-05 — Auditoría read-only sobre main `42faa62`: gates verdes (build/vet/test `exit=0`, 21 paquetes; govulncheck 0 vulnerabilidades alcanzables; sin secretos trackeados). Hallazgos y evidencia en Engram `scouter/audit/main-42faa62` (obs 7547).
