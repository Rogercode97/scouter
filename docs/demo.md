# Demo: 30 segundos de Scouter sobre su propio repo

Lee esto como el guion de [`assets/demo.cast`](../assets/demo.cast): cada bloque
es un comando y su salida real, medida en este repositorio. Para repetirlo:

```bash
./scripts/demo.sh
```

El script construye el binario en un directorio temporal, indexa con una base
dedicada (`SCOUTER_CONFIG` apuntando a `/tmp`) y no toca tu estado: ni el
working tree ni `~/.config/scouter`. La misma secuencia existe como terminal
regrabable en [`assets/demo.cast`](../assets/demo.cast) (asciicast v2, 32 s):

```bash
asciinema play assets/demo.cast   # o: python3 scripts/demo-cast.py para regenerarlo
```

## Parte 1 — Antes / después: texto vs estructura (solo lectura)

```bash
$ scouter index .
✅ Indexed .

$ scouter search IndexerPipeline
🔍 Search results for "IndexerPipeline":
- [class] IndexerPipeline (/home/roger/Projects/scouter/internal/engine/indexer.go:51)
- [function] NewIndexerPipeline (/home/roger/Projects/scouter/internal/engine/indexer.go:55)
- [method] IndexerPipeline.Index (/home/roger/Projects/scouter/internal/engine/indexer.go:179)
- [method] IndexerPipeline.indexDirectory (/home/roger/Projects/scouter/internal/engine/indexer.go:257)
- [function] IndexerPipeline.indexDirectory.func1 (/home/roger/Projects/scouter/internal/engine/indexer.go:288)
- [function] IndexerPipeline.indexDirectory.func2 (/home/roger/Projects/scouter/internal/engine/indexer.go:319)
- [function] IndexerPipeline.indexDirectory.func2.func1 (/home/roger/Projects/scouter/internal/engine/indexer.go:320)
- [method] IndexerPipeline.indexFile (/home/roger/Projects/scouter/internal/engine/indexer.go:356)
```

Ocho símbolos, 863 bytes, idénticos en cada corrida. La baseline de texto, en
cambio, devuelve cada línea que nombra el query — incluidos este documento y el
propio script del benchmark — y crece con el corpus (2,572 B → 3,946 B):

```bash
$ rg -n -- IndexerPipeline . | wc -c
5905   # crece con el corpus: era 2,572 B en docs/benchmarks/results.md (runs A/B)
```

Los números defendibles están en [`benchmarks/results.md`](benchmarks/results.md):
**863 B vs 2,572 B por pregunta, 48 ms vs 6 ms por query, índice de 12.6 s /
14.9 MB para 252 archivos.** Scouter es más lento; su respuesta es un tercio
del texto, y ese texto es estructural.

## Parte 2 — Impacto antes de tocar

```bash
$ scouter impact 'github.com/Rogercode97/scouter/internal/utils.SafeCommand' internal/utils/safe_exec.go
Risk Level: Low
Blast Radius: 10 callers
Affected Callers:
  - github.com/Rogercode97/scouter/internal/adapters/rtk.Reader.Read (internal/adapters/rtk/reader.go)
  - github.com/Rogercode97/scouter/internal/mcp.Server.handlePredict (internal/mcp/handle_intelligence.go)
  - github.com/Rogercode97/scouter/internal/engine.StructuralRefactor (internal/engine/structural.go)
  - ... (tests y fuzz, 7 más)
```

El nombre corto resuelve al fully-qualified cuando no es ambiguo; si hay
varios símbolos con el mismo nombre, el comando lo dice y pide el FQN
(`scouter search <Nombre>` lo confirma).

## Caso real: el hardening de SafeCommand no rompió nada

`internal/utils/safe_exec.go` es el único punto por donde Scouter ejecuta
binarios externos (`SafeCommand`, allow-list cerrado). Al endurecerlo, `impact`
mostró exactamente **3 callers en producción** — `rtk.Reader.Read`,
`mcp handlePredict`, `engine.StructuralRefactor` — y ninguno pasa una shell ni
un builtin (`internal/engine/executor.go` construye su propio `sh -c` sin pasar
por `SafeCommand`). Conclusión: eliminar la rama de fallback tocaba solo
`safe_exec.go`. Medido con grep y con `impact`, registrado en
`odd/tasks/safecommand-hardening.md` (L6). Los otros 7 callers del blast radius
son tests y fuzz, que siguieron en verde.

## Parte 3 — Mutación por Ledger: stage → diff → commit

Sobre una copia del repo (el demo nunca muta tu checkout):

```bash
$ scouter propagate 'demo.Greet' 'rename:Welcome'
✅ Transformation staged in Ledger for 1 files: [.../main.go]

$ scouter diff
--- .../main.go (Original)
+++ .../main.go (Staged)
-  func Greet(name string) string {
+  func Welcome(name string) string {
-    fmt.Println(Greet("scouter"))
+    fmt.Println(Welcome("scouter"))

$ scouter commit && go build ./... && go run .
✅ Committed changes to 1 files
hello scouter
```

Reglas del Ledger que el demo ejerce: nada llega a disco sin pasar por stage,
`diff` muestra antes de aplicar, `commit` reemplaza cada archivo
atómicamente y `rollback` vacía el staging sin tocar lo ya aplicado.
`propagate` solo acepta la forma `rename:NewName`; cualquier otra cosa se
rechaza con el uso en lugar de mutar a ciegas.
