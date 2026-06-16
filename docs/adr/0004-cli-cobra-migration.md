# ADR 0004: CLI Routing Migration to Cobra

## Status
Accepted

## Context
The Scouter CLI routing was historically managed by a monolithic "Fat Controller" (`internal/cli/cli.go`). This single file contained a 700+ line `switch` statement that parsed arguments, loaded configurations, and executed more than 20 subcommands. As the CLI surface area expanded, this structure violated separation of concerns, increased cyclomatic complexity, and made unit testing difficult.

## Decision
We migrated the entire CLI routing infrastructure to the `github.com/spf13/cobra` framework.
1. The monolithic switch was atomized into 20+ isolated files under a new package: `cmd/scouter/scoutercmd/`.
2. `cmd/scouter/main.go` was reduced to a simple bootstrapper that invokes `scoutercmd.Execute`.
3. To maintain 100% backward compatibility with legacy dynamic execution flows (where unknown commands are passed down to an underlying pipeline engine), a **Proxy Fallback** was implemented within `scoutercmd.Execute`.

## Gotchas Invariant Table
| Qué ocurrió (Diagnóstico) | Regla de Prevención (Invariante) |
|---|---|
| Al migrar a Cobra, comandos no registrados explícitamente fallaban con `unknown command`, rompiendo el enrutamiento dinámico (pipeline proxy) hacia el Engine. | **Proxy Fallback**: Si `rootCmd.Find()` retorna un error cuyo prefijo es `unknown command`, el error se descarta y el control se delega a `runProxy(ctx, args)`. **NUNCA** registrar un "catch-all" en Cobra si interfiere con esta regla. |
| El linter de seguridad o la compilación fallaban al mover el código debido a dependencias cruzadas entre el CLI antiguo y el nuevo. | **Clean Boundaries**: `internal/cli/cli.go` fue despojado de lógica de enrutamiento pero conservó estructuras base puras (`Flags`, `ParseFlags`) para evitar referencias circulares con `internal/engine`. |

## Consequences
- **Positive**: High modularity. Adding a new command is now as simple as adding a new `cmd/scouter/scoutercmd/newcmd.go` file.
- **Negative**: Slight overhead in parsing Cobra commands before falling back to the pipeline, though negligible in practice.
