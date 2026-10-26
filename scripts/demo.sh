#!/usr/bin/env bash
# demo.sh — the 30-second Scouter demo from docs/demo.md, reproducible.
# Read-only against the repository for part 1 (index/search/impact);
# part 2 (propagate/diff/commit) runs on a throwaway module in /tmp.
# State (binary, index, ledger) lives in a temp dir; nothing touches
# the working tree or ~/.config/scouter.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

GO_BIN="$(command -v go)"
TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

BIN="$TMP_DIR/scouter"
"$GO_BIN" build -o "$BIN" ./cmd/scouter

export SCOUTER_CONFIG="$TMP_DIR/config.toml"
printf '[tracking]\ndb_path = "%s/scouter.db"\n' "$TMP_DIR" > "$SCOUTER_CONFIG"

section() { printf '\n=== %s ===\n' "$1"; }

section "1a. index this repository"
"$BIN" index . 2>/dev/null | tail -1

section "1b. structural answer (stable bytes)"
"$BIN" search IndexerPipeline
"$BIN" search IndexerPipeline 2>/dev/null | wc -c

section "1c. text baseline (grows with the corpus)"
if command -v rg >/dev/null 2>&1; then
    rg -n -- IndexerPipeline . | wc -c
else
    echo "rg not on PATH: baseline unavailable"
fi

section "2. blast radius before touching SafeCommand"
"$BIN" impact 'github.com/Rogercode97/scouter/internal/utils.SafeCommand' internal/utils/safe_exec.go

section "3. ledger loop on a throwaway module"
MOD="$TMP_DIR/mini"
mkdir -p "$MOD"
printf 'module demo\n\ngo 1.26\n' > "$MOD/go.mod"
cat > "$MOD/main.go" <<'EOF'
package main

import "fmt"

func Greet(name string) string {
	return "hello " + name
}

func main() {
	fmt.Println(Greet("scouter"))
}
EOF
(
    cd "$MOD"
    "$BIN" index . 2>/dev/null | tail -1
    "$BIN" propagate 'demo.Greet' 'rename:Welcome'
    "$BIN" diff | head -12
    "$BIN" commit
    CGO_ENABLED=0 "$GO_BIN" build ./... && "$GO_BIN" run .
)
