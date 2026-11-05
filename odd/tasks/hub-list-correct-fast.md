# Feature: hub visible, correcto antes de rápido

## Specs

- **S1.** Publica al Hub como proyecto: descripción corta + tópico gentle-hub-project en GitHub. Te lista sin tocar código.
- **S2.** Reconcilia fix/apply-interrupt-error-and-durability: revisa si su contenido ya entró a main y borra la rama si sí.
- **S3.** Respeta .gitignore al indexar: todos los pares lo hacen y hoy indexas de más.
- **S4.** Cierra el recall no-Go (alias, map(fn), partial(fn)): es precisión, no velocidad.
- **S5.** Rinde al final: maxWorkers a NumCPU y packages.Load mínimo, con bench-baseline.sh como gate.

## Tasks

| ID | Spec | Ruta | Commit |
| --- | --- | --- | --- |
| T1 | S1 | Hub: repo description + topic gentle-hub-project, sin código | — |
| T2 | S2 | Rama fix/apply-interrupt: verificar merge en main, podar local+remoto | podada 2026-11-05 (c061798 en main, PR #80 MERGED) |
| T3 | S3 | Index respeta .gitignore, limpia vec_symbols huérfanas | 4560e7e fix/t3-gitignore-vec-orphans (ignore.go/ignore_test.go, indexer.go, structural.go, store.go/interfaces.go + vec_orphan_test.go; engine+store verdes) |
| T4 | S4 | Recall no-Go: link kind propio para map/partial sin inflar blast radius | 98a6e87 (reference en treesitter + blast-radius lo salta, truth.json sin FN, P/R 1.00; Go 34/34 intacto) |
| T5 | S5 | Perf: maxWorkers NumCPU + packages.Load mínimo, gate bench-baseline.sh | 638ebcd (NumCPU + Load floor: Types/TypesInfo intactos por recall Go 1.00; bench 12643→12434ms ruido, search 48→49ms, bytes idénticos; engine+store verdes + probe GOMAXPROCS=1) |

## Log

- **L1.** "Con scouter, en este orden: 1. Publica al Hub como proyecto: descripción corta + tópico gentle-hub-project en GitHub. Te lista sin tocar código. 2. Reconcilia fix/apply-interrupt-error-and-durability: revisa si su contenido ya entró a main y borra la rama si sí. 3. Respeta .gitignore al indexar: todos los pares lo hacen y hoy indexas de más. 4. Cierra el recall no-Go (alias, map(fn), partial(fn)): es precisión, no velocidad. 5. Rinde al final: maxWorkers a NumCPU y packages.Load mínimo, con bench-baseline.sh como gate. El 1 te pone visible hoy; del 3 en adelante te vuelve correcto antes de rápido."
- **L2.** 2026-11-05 — T2 ya reconciliada fuera de orden (sesión previa): c061798 y origen b0adb86 ambos ancestros de main, PR #80 MERGED, podadas local+remoto con autorización. T5 parcial fuera de orden: maxWorkers→NumCPU aplicado sin commit, bench 12784ms igual al baseline (cuello en packages.Load). Se respeta el orden desde ahora: T1, luego T3, T4, y T5 se re-gatea al final.
- **L3.** 2026-11-05 — T1 verificado (desc + topic en público). T3 verificado con ambas pruebas en rojo-sin-fix/verde-con-fix y suites verdes; reserva del verificador cerrada con test vec dedicado. Kernels a AGENTS.md (ignore contract, vec invariant, id rule, verificación sin -race) y 4 topics a Engram (hub-publish, index-gitignore, vec-orphans, perf-maxworkers). T4/T5 soltados a la otra sesión.
- **L4.** 2026-11-05 — Re-medición bench en rama T3 (`20b7f9d`, 463 ficheros): index 13255 ms / 16510976 B, search 48 ms / 863 B estables, rg 6 ms / 7547 B. El gap se mueve con el corpus; esta es la cifra gate para T5, no comparable 1:1 con main.
