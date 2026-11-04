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
| T1 | S2, S3 | corpus de fixtures Python + `truth.json` etiquetado a mano | `885f5ba` |
| T2 | S1, S3 | arnés de comparación (TP/FP/FN) con ratchet de FP y FN conocidos | `885f5ba` |
| T3 | S1, S2 | aristas directas a nivel de índice (TestImpactPrecision_GoIndex, oráculo go/parser TestGoCallerOracle), opt-in SCOUTER_IMPACT_SELF=1, sin modo directo en CLI | `aa313e0`, `d613838`, `a83db8b` (PR 94 mergeada en 1ea9bc8) |
| T4 | S4 | `docs/benchmarks/impact-precision.md` con los números medidos | `00080ba` |
| T5 | S1, S2 | alias de import Python resueltos por la tabla de imports con el módulo viajando con el nombre (`resolvePythonCallee`, `pythonImportAliases`) + caso negativo `ext.build` vs `build` local + regla de conflicto sin resolver | `7043b8b`, `1197a5d` (PR #96 mergeada en `d42e67b`) |

## Log

- **L1.** "continuamos?" — sesión retomada desde el handoff de cola de dependencias.
- **L2.** "como hacer mas rapido scouter / busca info web similares a scouter" — pedido de rendimiento y de panorama de herramientas similares.
- **L3.** Bench medido en esta máquina: `index .` 12,548 ms para 265 archivos, ~64 % del tiempo antes del primer archivo (`packages.Load` con `NeedSyntax|NeedTypes|NeedTypesInfo`), `search` 48-49 ms contra 5 ms de ripgrep, 863 B contra 7,432 B.
- **L4.** "Primero validar precisión, después optimizar" — decisión del humano: la precisión va antes que el rendimiento. Abre este feature.
- **L5.** Medición Python (end-to-end por el binario, corpus de fixtures): `scale` TP=2 FP=0 FN=0; `boost` TP=0 con 3 FN documentados (llamada por alias de import, referencia pasada a `map`, referencia pasada a `partial`); `transform` ambiguo respondía en vez de rechazar. Agregado: precision 1.00, recall 0.40.
- **L6.** Defecto encontrado y corregido: `resolveSymbolFQN` deduplicaba por nombre renderizado, así que dos definiciones sin `PackagePath` (Python) colapsaban en un FQN y `impact` fusionaba dos call graphs. Ahora la identidad es (FQN, archivo) y el error nombra ambos archivos.
- **L7.** Decisión de alcance: la medición Go se separa. `impact` sobre Go responde con el blast radius **transitivo** a profundidad 5 y con nombres **FQN**, así que una verdad etiquetada de callers directos no es comparable contra la salida del CLI. Medir Go exige definir la métrica sobre callers directos (nivel de índice) o añadir un modo directo al comando; queda como tarea siguiente, no se inventa aquí.

- **L8.** 2026-11-04 — T3 reconciliado: se tomó la vía que L7 pedía definir (métrica sobre aristas directas a nivel de índice, sin modo directo en CLI). PR 94 (`aa313e0`, `d613838`, `a83db8b`) implementa TestImpactPrecision_GoIndex con oráculo go/parser y publica Go 1.00/1.00 en 34 aristas en `docs/benchmarks/impact-precision.md`; mergeada en 1ea9bc8 con checks en verde.

## Delivery

Un solo PR (#92) con tres commits lógicos, 606 líneas autoradas. Supera la guía de ~400 líneas por slice, y se acepta la excepción porque el instrumento, el defecto que encontró y su corrección comparten un solo criterio de verificación: si se separaran, la corrección viajaría sin la medición que la justifica y la medición sin el caso que la vuelve verde.

PR #92 cerró Python + ambigüedad; PR 94 (4 commits, mergeada en 1ea9bc8 con checks en verde) añade Go-directo.

PR #96 (4 commits, mergeada en `d42e67b` con 7/7 checks): cierra el recall no-Go en **0.60** con tres reglas — la tabla de imports resuelve las dos formas de alias, el módulo viaja con el nombre (símbolo Python registrado bajo el stem del archivo), y un alias ligado a dos módulos deja la llamada sin resolver en vez de adivinar. Tres hallazgos de CodeRabbit sobre el instrumento, los tres correctos y reproducidos antes de arreglarlos: closures anidadas (`Parent.func1.func1`, 113 filas en el árbol), oráculo por líneas (4 casos: desreferencia, comentario al final de línea, string, `func Generic[T any]`), y texto stale del doc. Quedan 2 FN (`map(fn)`, `partial(fn)`): referencias como valor, que necesitan un tipo de arista propio — decisión de diseño, no de parser.
