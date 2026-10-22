#!/usr/bin/env bash
# bench-baseline.sh — measure one repeatable query through Scouter and through ripgrep.
#
# Procedure (fixed):
#   1. Build the scouter binary from the current checkout into a temp dir.
#   2. Index the repository (the corpus) with `scouter index .`, timing it and
#      recording the resulting SQLite index size.
#   3. Run `scouter search "<query>"`, timing it and recording stdout bytes/tokens.
#   4. Run `rg -n -- "<query>" .` over the same corpus, timing it and recording
#      stdout bytes/tokens (when ripgrep is available).
# Tokens are bytes/4, the same approximation internal/engine/ledger.go uses.
set -euo pipefail

QUERY="IndexerPipeline"

DIRNAME_BIN="$(command -v dirname)"
RM_BIN="$(command -v rm)"
CAT_BIN="$(command -v cat)"
DATE_BIN="$(command -v date)"
WC_BIN="$(command -v wc)"
TR_BIN="$(command -v tr)"
HEAD_BIN="$(command -v head)"
MKTEMP_BIN="$(command -v mktemp)"

REPO_ROOT="$(cd "$("$DIRNAME_BIN" "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

GO_BIN="$(command -v go)"
GIT_BIN="$(command -v git)"

RG_BIN=""
if command -v rg >/dev/null 2>&1; then
    RG_BIN="$(command -v rg)"
fi

TMP_DIR="$("$MKTEMP_BIN" -d)"
cleanup() {
    "$RM_BIN" -rf "$TMP_DIR"
}
trap cleanup EXIT

BIN="$TMP_DIR/scouter"
CONFIG="$TMP_DIR/config.toml"
DB="$TMP_DIR/scouter.db"

"$GO_BIN" build -o "$BIN" ./cmd/scouter

printf '[tracking]\ndb_path = "%s"\n' "$DB" > "$CONFIG"

COMMIT_SHA="$("$GIT_BIN" rev-parse HEAD)"
GO_VERSION="$("$GO_BIN" version)"
if command -v nproc >/dev/null 2>&1; then
    NPROC_BIN="$(command -v nproc)"
    CPU_COUNT="$("$NPROC_BIN")"
elif command -v getconf >/dev/null 2>&1; then
    GETCONF_BIN="$(command -v getconf)"
    CPU_COUNT="$("$GETCONF_BIN" _NPROCESSORS_ONLN)"
else
    CPU_COUNT="unknown"
fi
FILE_COUNT="$("$GIT_BIN" ls-files | "$WC_BIN" -l | "$TR_BIN" -d '[:space:]')"

RG_VERSION="unavailable (rg not on PATH)"
if [[ -n "$RG_BIN" ]]; then
    RG_VERSION="$("$RG_BIN" --version | "$HEAD_BIN" -n 1)"
fi

echo "Machine profile:"
echo "  commit: $COMMIT_SHA"
echo "  go: $GO_VERSION"
echo "  cpus: $CPU_COUNT"
echo "  corpus files (git-tracked): $FILE_COUNT"
echo "  ripgrep: $RG_VERSION"
echo
echo "Query: $QUERY"
echo

# --- Index cost (Scouter) ---
INDEX_STDOUT="$TMP_DIR/index.stdout"
INDEX_STDERR="$TMP_DIR/index.stderr"
INDEX_START_NS="$("$DATE_BIN" +%s%N)"
if ! SCOUTER_CONFIG="$CONFIG" "$BIN" index . >"$INDEX_STDOUT" 2>"$INDEX_STDERR"; then
    "$CAT_BIN" "$INDEX_STDERR" >&2
    exit 1
fi
INDEX_END_NS="$("$DATE_BIN" +%s%N)"
INDEX_MS="$(( (INDEX_END_NS - INDEX_START_NS) / 1000000 ))"
INDEX_BYTES="$("$WC_BIN" -c < "$DB" | "$TR_BIN" -d '[:space:]')"

echo "Index cost (Scouter):"
echo "  index time (ms): $INDEX_MS"
echo "  index size (bytes): $INDEX_BYTES"
echo

# --- Scouter side ---
SCOUT_STDOUT="$TMP_DIR/scouter.stdout"
SCOUT_STDERR="$TMP_DIR/scouter.stderr"
SCOUT_START_NS="$("$DATE_BIN" +%s%N)"
if ! SCOUTER_CONFIG="$CONFIG" "$BIN" search "$QUERY" >"$SCOUT_STDOUT" 2>"$SCOUT_STDERR"; then
    "$CAT_BIN" "$SCOUT_STDERR" >&2
    exit 1
fi
SCOUT_END_NS="$("$DATE_BIN" +%s%N)"
SCOUT_MS="$(( (SCOUT_END_NS - SCOUT_START_NS) / 1000000 ))"
SCOUT_BYTES="$("$WC_BIN" -c < "$SCOUT_STDOUT" | "$TR_BIN" -d '[:space:]')"
SCOUT_TOKENS="$(( SCOUT_BYTES / 4 ))"

echo "Scouter side:"
echo "  latency (ms): $SCOUT_MS"
echo "  output (bytes): $SCOUT_BYTES"
echo "  output (tokens, bytes/4): $SCOUT_TOKENS"
echo

# --- Conventional baseline (ripgrep) ---
if [[ -z "$RG_BIN" ]]; then
    echo "Conventional baseline (ripgrep): unavailable (rg not on PATH)"
    echo "  latency (ms): n/a"
    echo "  output (bytes): n/a"
    echo "  output (tokens, bytes/4): n/a"
else
    RG_STDOUT="$TMP_DIR/rg.stdout"
    RG_STDERR="$TMP_DIR/rg.stderr"
    RG_START_NS="$("$DATE_BIN" +%s%N)"
    RG_RC=0
    if ! "$RG_BIN" -n -- "$QUERY" . >"$RG_STDOUT" 2>"$RG_STDERR"; then
        RG_RC=$?
    fi
    RG_END_NS="$("$DATE_BIN" +%s%N)"
    if [[ "$RG_RC" -ne 0 && "$RG_RC" -ne 1 ]]; then
        "$CAT_BIN" "$RG_STDERR" >&2
        exit "$RG_RC"
    fi
    RG_MS="$(( (RG_END_NS - RG_START_NS) / 1000000 ))"
    RG_BYTES="$("$WC_BIN" -c < "$RG_STDOUT" | "$TR_BIN" -d '[:space:]')"
    RG_TOKENS="$(( RG_BYTES / 4 ))"

    echo "Conventional baseline (ripgrep):"
    echo "  latency (ms): $RG_MS"
    echo "  output (bytes): $RG_BYTES"
    echo "  output (tokens, bytes/4): $RG_TOKENS"
fi
