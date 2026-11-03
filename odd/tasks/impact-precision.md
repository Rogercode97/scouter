# Feature: impact precision (FP/FN per language)

Measure whether `scouter impact` answers correctly before making it faster.
Selected by the human over three performance options: optimize a result that was
never validated is the failure mode this feature exists to prevent.

## Specs

- **S1.** Medir falsos positivos y falsos negativos de `scouter impact` por lenguaje, antes de cualquier cambio de rendimiento. Origen: "Primero validar precisión, después optimizar".
- **S2.** El oráculo de verdad es independiente del índice: se etiqueta leyendo el fuente, nunca leyendo las filas que el propio indexador escribió.
- **S3.** La medición es determinista y reproducible sin red; el corpus de fixtures corre en CI y la corrida sobre un corpus real es opt-in.
- **S4.** Los números medidos se publican en `docs/benchmarks/`, junto al procedimiento y al entorno, como ya hace `docs/benchmarks/results.md`.

## Tasks

| ID | Spec | Ruta | Commit |
| --- | --- | --- | --- |
| T1 | S2, S3 | corpus de fixtures Python + `truth.json` etiquetado a mano | |
| T2 | S1, S3 | arnés de comparación (TP/FP/FN) con ratchet de FP conocidos | |
| T3 | S1, S2 | corpus Go real (este repo) con verdad etiquetada, opt-in | |
| T4 | S4 | `docs/benchmarks/impact-precision.md` con los números medidos | |

## Log

- **L1.** "continuamos?" — sesión retomada desde el handoff de cola de dependencias.
- **L2.** "como hacer mas rapido scouter / busca info web similares a scouter" — pedido de rendimiento y de panorama de herramientas similares.
- **L3.** Bench medido en esta máquina: `index .` 12,548 ms para 265 archivos, ~64 % del tiempo antes del primer archivo (`packages.Load` con `NeedSyntax|NeedTypes|NeedTypesInfo`), `search` 48-49 ms contra 5 ms de ripgrep, 863 B contra 7,432 B.
- **L4.** "Primero validar precisión, después optimizar" — decisión del humano: la precisión va antes que el rendimiento. Abre este feature.
- **L5.** Medición Python (end-to-end por el binario, corpus de fixtures): `scale` TP=2 FP=0 FN=0; `boost` TP=0 con 3 FN documentados (llamada por alias de import, referencia pasada a `map`, referencia pasada a `partial`); `transform` ambiguo respondía en vez de rechazar. Agregado: precision 1.00, recall 0.40.
- **L6.** Defecto encontrado y corregido: `resolveSymbolFQN` deduplicaba por nombre renderizado, así que dos definiciones sin `PackagePath` (Python) colapsaban en un FQN y `impact` fusionaba dos call graphs. Ahora la identidad es (FQN, archivo) y el error nombra ambos archivos.
- **L7.** Decisión de alcance: la medición Go se separa. `impact` sobre Go responde con el blast radius **transitivo** a profundidad 5 y con nombres **FQN**, así que una verdad etiquetada de callers directos no es comparable contra la salida del CLI. Medir Go exige definir la métrica sobre callers directos (nivel de índice) o añadir un modo directo al comando; queda como tarea siguiente, no se inventa aquí.
