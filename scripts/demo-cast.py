#!/usr/bin/env python3
"""Generate assets/demo.cast (asciicast v2) from the real outputs in docs/demo.md."""
import json

OUT = "assets/demo.cast"
W, H = 100, 28

events = []
t = 0.0

def emit(delay, text):
    global t
    t += delay
    events.append([round(t, 2), "o", text])

P = "$ "
NL = "\r\n"

emit(2.0, P + "scouter index ." + NL)
emit(2.4, "✅ Indexed ." + NL)
emit(2.0, P + "scouter search IndexerPipeline" + NL)
emit(1.6, "🔍 Search results for \"IndexerPipeline\":" + NL)
for line in [
    "- [class] IndexerPipeline (internal/engine/indexer.go:51)",
    "- [function] NewIndexerPipeline (internal/engine/indexer.go:55)",
    "- [method] IndexerPipeline.Index (internal/engine/indexer.go:179)",
    "- [method] IndexerPipeline.indexDirectory (internal/engine/indexer.go:257)",
    "- [method] IndexerPipeline.indexFile (internal/engine/indexer.go:356)",
    "- ... 8 symbols, 863 bytes, identical on every run",
]:
    emit(2.4, line + NL)
emit(2.0, P + "rg -n -- IndexerPipeline . | wc -c" + NL)
emit(1.6, "5905   # the text baseline grows with the corpus" + NL)
emit(2.4, P + "scouter impact '.../utils.SafeCommand' internal/utils/safe_exec.go" + NL)
emit(2.4, "Risk Level: Low" + NL + "Blast Radius: 10 callers" + NL)
for line in [
    "  - .../adapters/rtk.Reader.Read (rtk/reader.go)",
    "  - .../mcp.Server.handlePredict (handle_intelligence.go)",
    "  - .../engine.StructuralRefactor (structural.go)",
    "  - ... (tests and fuzz)",
]:
    emit(2.4, line + NL)
emit(2.4, P + "scouter propagate 'demo.Greet' 'rename:Welcome'" + NL)
emit(1.6, "✅ Transformation staged in Ledger for 1 files" + NL)
emit(2.0, P + "scouter diff" + NL)
emit(2.0, "-  func Greet(name string) string {" + NL + "+  func Welcome(name string) string {" + NL)
emit(2.4, "-    fmt.Println(Greet(\"scouter\"))" + NL + "+    fmt.Println(Welcome(\"scouter\"))" + NL)
emit(2.0, P + "scouter commit && go run ." + NL)
emit(1.6, "✅ Committed changes to 1 files" + NL + "hello scouter" + NL)
emit(3.0, "")

header = {
    "version": 2,
    "width": W,
    "height": H,
    "timestamp": 1761432000,
    "env": {"SHELL": "/bin/bash", "TERM": "xterm-256color"},
    "title": "Scouter 30-second demo: index, search vs rg, impact, ledger commit",
}
TARGET = 32.0
if t > 0:
    k = TARGET / t
    events = [[round(ts * k, 2), typ, data] for ts, typ, data in events]
    t = TARGET

with open(OUT, "w") as f:
    f.write(json.dumps(header) + "\n")
    for ev in events:
        f.write(json.dumps(ev) + "\n")
print(f"wrote {OUT}: {len(events)} events, {t:.1f}s")
