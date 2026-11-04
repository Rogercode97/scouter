# Feature: impact transitivity (depth-5 quality)

Measure whether the transitive blast radius `scouter impact` prints adds callers that would never be affected. Direct edges are already gated (PR 94: Go 1.00/1.00 on 34 edges); the CLI answer is transitive and still has no figure.

## Specs

- **S1.** Medir la calidad transitiva que `impact` imprime, no solo las aristas directas. Origen: "Diseñar transitividad".
- **S2.** El oráculo sigue independiente del índice: se etiqueta leyendo el fuente, nunca las filas indexadas.
- **S3.** La medición es determinista y opt-in como `TestImpactPrecision_GoIndex` (`SCOUTER_IMPACT_SELF=1`), y publica en `docs/benchmarks/` junto al procedimiento.

## Tasks

| ID | Spec | Ruta |
| --- | --- | --- |
| T1 | S1, S2 | corpus mínimo con cadena A<-B<-C y rama lateral que no afecta: verdad de alcance transitivo esperado vs impreso a depth 5 |
| T2 | S1, S3 | arnés que corre `impact` por el binario (CLI, `cmd/scouter/scoutercmd/impact.go:41` usa depth 5) y compara conjuntos transitivos |
| T3 | S3 | `docs/benchmarks/impact-transitivity.md` con sobre-expansión (callers-of-callers no afectados) |

## Log

- **L1.** "Diseñar transitividad (recomendado)" — pedido mientras la PR 95 está en CI. No toca `odd/tasks/impact-precision.md` (PR 95) ni los 4 archivos de la PR 94.
- **L2.** Evidencia: `internal/engine/impact.go:65-69` defaultea depth 3 y capa en 10; el CLI fija 5. `docs/benchmarks/impact-precision.md` (What this does not measure) excluye explícitamente la expansión transitiva.
