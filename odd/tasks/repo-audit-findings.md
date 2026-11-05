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
| T1 | S1 | `internal/store/store.go` + `internal/store/vec_orphan_test.go` | e18a8f1 (RED: 2 huérfanas → GREEN: 1) |
| T2 | S2 | `.goreleaser.yaml` + `tests/release_ldflags_test.go` | 7f6d991 (guard RED con `cli.version`; binario pasa de `vdev` a `v1.5.0`) |
| T3 | S5 | `internal/store/store.go` (ImportDelta), `internal/engine/healer.go`, `internal/store/import_delta_test.go` | 5d41c91 (RED vía FK de `file_index`) |
| T4 | S4 | `internal/engine/chronos.go` (+`chronos_scratch_test.go`), `internal/engine/lsp/manager.go`, `sys_unix.go`, `sys_windows.go` (+`manager_socket_test.go`) | 0e2b85f (ambos guards probados rompiéndolos a propósito) |
| T6 | S3 | `internal/utils/safe_exec.go` + 7 llamadores + `tests/exec_boundary_test.go` | cedbc9e (guard RED: 11 sitios, luego verde) |
| T5 | S6 | `AGENTS.md`, `.github/workflows/security.yml`, `internal/engine/pipeline.go` | 1160eeb |
| T7 | S6 | `internal/telemetry/tracker.go` + `tracker_wal_test.go` | 2390090 (RED: `journal_mode = delete`) |
| T8 | S3 | `internal/utils/safe_exec.go` (+`safe_exec_test.go`), `internal/engine/healer.go`, `AGENTS.md` | 2b0b8b5 (RED: 6 subcomandos aceptados que debían rechazarse) |

## Log

- **L1.** "audita en worktree"
- **L2.** "usa worktrunk" — la auditoría vive en un worktree creado con `wt switch --create`, no con `git worktree add`.
- **L3.** Elección "Corregir todo lo listado": "Los seis hallazgos, un commit por unidad con su test, empezando por los altos."
- **L4.** 2026-11-05 — Auditoría read-only sobre main `42faa62`: gates verdes (build/vet/test `exit=0`, 21 paquetes; govulncheck 0 vulnerabilidades alcanzables; sin secretos trackeados). Hallazgos y evidencia en Engram `scouter/audit/main-42faa62` (obs 7547).
- **L5.** 2026-11-05 — Corte de slice en 583 líneas tras `0e2b85f` (T1-T4): el branch `fix/repo-audit-findings` es el contenedor del slice; push y PR son decisión del humano, no se ejecutan. Segundo slice `cedbc9e..aa076cb` (T6, T5, T7 y el desacople del test de omniscience).
- **L6.** 2026-11-05 — Desvío declarado en T4: el temp de 2PC (`<archivo>.scouter.tmp`) no se cambió. No es descuido sino artefacto de recuperación: `ledger.go:544` borra ese nombre exacto en la recuperación y `applyPatchStep` renombra ese path. Volverlo único exige registrar el nombre en el journal, es decir un cambio de formato del journal: riesgo alto por sí mismo. Queda como unidad separada si se autoriza.
- **L7.** 2026-11-05 — Cierre verificado en el worktree: `CGO_ENABLED=0 go build ./...` exit=0, `go vet ./...` exit=0, `go test ./...` exit=0 con 21 paquetes `ok`. T6 destapó cuatro sitios que la auditoría no había visto (`impact.go`, `ripple.go` ×2, `lsp/client.go`) y dos huecos del propio gateway (flags ausentes en la política y `-run` en `go`), corregidos con su test. `TestOmniscienceCallHierarchy` se acoplaba a un número de línea y se desacopló en `aa076cb`.
- **L8.** 2026-11-05 — El verificador independiente dio 6/7 en `pass` y marcó S4 como fallo por el temp de 2PC ya declarado en L6, más un residual real: el gateway validaba flags pero dejaba libre el verbo (`go run` pasaba). T8 lo cierra con conjunto de subcomandos por binario y, de paso, con los dos bulk writes del healer que aún descartaban su error. Suite completa verde otra vez.
