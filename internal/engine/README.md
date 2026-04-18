<p align="center">
  <img src="https://img.shields.io/github/v/release/edouard-claude/scouter?style=flat-square" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/edouard-claude/scouter/ci.yaml?branch=master&style=flat-square&label=CI" alt="CI">
  <img src="https://img.shields.io/github/license/edouard-claude/scouter?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go">
</p>

# scouter

**CLI proxy that cuts LLM token waste from shell output.**

AI coding agents burn tokens on verbose shell output that adds zero signal. A passing `go test` produces hundreds of lines the LLM will never use. `git log` dumps full commit metadata when a one-liner per commit suffices.

scouter sits between your AI tool and the shell, filtering output through declarative YAML pipelines before it reaches the context window.

```
  scouter — Token Savings Report
  ══════════════════════════════

  Commands filtered     128
  Tokens saved          2.3M
  Avg savings           99.8%
  Efficiency            Elite
  Total time            725.9s

  ███████████████████░ 100%

  14-day trend  ▁█▇

  Top commands by tokens saved

  Command                    Runs  Saved   Savings  Impact
  ─────────────────────────  ────  ──────  ───────  ────────────
  go test ./...              8     806.2K  99.8%    ████████████
  go test ./pkg/...          3     482.9K  99.8%    ███████░░░░░
  go test ./... -count=1     3     482.0K  99.8%    ███████░░░░░
```

> Measured on a real Claude Code session — 128 commands, 2.3M tokens saved.

## Quick Start

```bash
# Homebrew (macOS/Linux)
brew install edouard-claude/tap/scouter

# Or with Go
go install github.com/Rogercode97/scouter/internal/engine/cmd/scouter@latest

# Then hook into Claude Code
scouter init
# That's it. Every shell command Claude runs now goes through scouter.
```

## How It Works

**Before** — Claude Code sees this (689 tokens):
```
$ go test ./...
ok  	github.com/Rogercode97/scouter/internal/cli	3.728s	coverage: 14.4% of statements
ok  	github.com/Rogercode97/scouter/internal/config	2.359s	coverage: 65.0% of statements
ok  	github.com/Rogercode97/scouter/internal/display	1.221s	coverage: 72.6% of statements
ok  	github.com/Rogercode97/scouter/internal/engine	1.816s	coverage: 47.9% of statements
ok  	github.com/Rogercode97/scouter/internal/filter	4.306s	coverage: 72.3% of statements
ok  	github.com/Rogercode97/scouter/internal/initcmd	2.981s	coverage: 59.1% of statements
ok  	github.com/Rogercode97/scouter/internal/tee	0.614s	coverage: 70.6% of statements
ok  	github.com/Rogercode97/scouter/internal/tracking	5.355s	coverage: 75.0% of statements
ok  	github.com/Rogercode97/scouter/internal/utils	5.515s	coverage: 100.0% of statements
```

**After** — scouter returns this (16 tokens):
```
10 passed, 0 failed
```

That's **97.7% fewer tokens**. The LLM gets the same signal — all tests pass — without the noise.

```
┌─────────────┐     ┌─────────────────┐     ┌──────────────┐     ┌────────────┐
│ Claude Code │────>│ scouter intercept  │────>│ run command  │────>│   filter   │
│  runs git   │     │  match filter   │     │  capture I/O │     │  pipeline  │
└─────────────┘     └─────────────────┘     └──────────────┘     └─────┬──────┘
                                                                       │
                    ┌─────────────────┐     ┌──────────────┐           │
                    │   Claude Code   │<────│ track savings│<──────────┘
                    │  sees filtered  │     │  in SQLite   │
                    └─────────────────┘     └──────────────┘
```

No filter match? The command passes through unchanged — zero overhead.

### Savings by Command

| Command | Before | After | Savings |
|---------|-------:|------:|--------:|
| `cargo test` | 591 tokens | 5 tokens | **99.2%** |
| `go test ./...` | 689 tokens | 16 tokens | **97.7%** |
| `git log` | 371 tokens | 53 tokens | **85.7%** |
| `git status` | 112 tokens | 16 tokens | **85.7%** |
| `git diff` | 355 tokens | 66 tokens | **81.4%** |

## Installation

### Homebrew (recommended)

```bash
brew install edouard-claude/tap/scouter
```

### From GitHub Releases

Download the latest binary for your platform from [Releases](https://github.com/Rogercode97/scouter/internal/engine/releases).

```bash
# macOS (Apple Silicon)
curl -Lo scouter.tar.gz https://github.com/Rogercode97/scouter/internal/engine/releases/latest/download/scouter_$(curl -s https://api.github.com/repos/edouard-claude/scouter/releases/latest | grep tag_name | cut -d'"' -f4 | tr -d v)_darwin_arm64.tar.gz
tar xzf scouter.tar.gz && mv scouter /usr/local/bin/
```

### From source

```bash
go install github.com/Rogercode97/scouter/internal/engine/cmd/scouter@latest
```

Or build locally:

```bash
git clone https://github.com/Rogercode97/scouter/internal/engine.git
cd scouter && make install
```

Requires Go 1.24+ and `jq` (for the hook script).

## Integration

### Claude Code

```bash
scouter init
```

This installs a `PreToolUse` hook that transparently rewrites supported commands. Claude Code never sees the substitution — it receives compressed output as if the original command produced it.

Supported commands: `git`, `go`, `cargo`, `npm`, `npx`, `yarn`, `pnpm`, `docker`, `kubectl`, `make`, `pip`, `pytest`, `jest`, `tsc`, `eslint`, `rustc`.

```bash
scouter init --uninstall   # remove the hook
```

### OpenCode

Install the [opencode-scouter](https://github.com/VincentHardouin/opencode-scouter) plugin by adding it to your OpenCode config (`~/.config/opencode/opencode.json`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": ["opencode-scouter@latest"]
}
```

The plugin uses the `tool.execute.before` hook to automatically prefix all commands with `scouter`. Commands not supported by scouter pass through unchanged.

### Cursor

Cursor supports hooks since v1.7 via `~/.cursor/hooks.json`:

```json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      { "command": "~/.claude/hooks/scouter-rewrite.sh" }
    ]
  }
}
```

### Aider / Windsurf / Other Tools

Use shell aliases:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias git="scouter git"
alias go="scouter go"
alias cargo="scouter cargo"
```

Or instruct the LLM via system prompt to prefix commands with `scouter`.

### Standalone

scouter works without any AI tool:

```bash
scouter git log -10
scouter go test ./...
scouter gain             # token savings report
```

## Usage

```bash
scouter <command> [args]       # filter a command
scouter gain                   # full dashboard (summary + sparkline + top commands)
scouter gain --daily           # daily breakdown
scouter gain --weekly          # weekly breakdown
scouter gain --monthly         # monthly breakdown
scouter gain --top 10          # top N commands by tokens saved
scouter gain --history 20      # last 20 commands
scouter gain --json            # machine-readable output
scouter gain --csv             # CSV export
scouter -v <command>           # verbose mode (show filter details)
scouter proxy <command>        # force passthrough (no filtering)
scouter config                 # show config
scouter init                   # install Claude Code hook
scouter init --uninstall       # remove hook
```

## Filters

Filters are declarative YAML files. The binary is the engine, filters are data — the two evolve independently.

```yaml
name: "git-log"
version: 1
description: "Condense git log to hash + message"

match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format", "--pretty", "--oneline"]

inject:
  args: ["--pretty=format:%h %s (%ar) <%an>", "--no-merges"]
  defaults:
    "-n": "10"

pipeline:
  - action: "keep_lines"
    pattern: "\\S"
  - action: "truncate_lines"
    max: 80
  - action: "format_template"
    template: "{{.count}} commits:\n{{.lines}}"

on_error: "passthrough"
```

### Built-in Filters

| Filter | What it does |
|--------|-------------|
| `git-status` | Categorized status with file counts |
| `git-diff` | Stat summary, truncated to 30 files |
| `git-log` | One-line per commit: hash + message + author + date |
| `go-test` | Pass/fail summary with failure details |
| `cargo-test` | Pass/fail summary with failure details |

### 16 Pipeline Actions

| Action | Description |
|--------|-------------|
| `keep_lines` | Keep lines matching regex |
| `remove_lines` | Remove lines matching regex |
| `truncate_lines` | Truncate lines to max length |
| `strip_ansi` | Remove ANSI escape codes |
| `head` / `tail` | Keep first/last N lines |
| `group_by` | Group lines by regex capture |
| `dedup` | Deduplicate with optional normalization |
| `json_extract` | Extract fields from JSON |
| `json_schema` | Infer schema from JSON |
| `ndjson_stream` | Process newline-delimited JSON |
| `regex_extract` | Extract regex captures |
| `state_machine` | Multi-state line processing |
| `aggregate` | Count pattern matches |
| `format_template` | Go template formatting |
| `compact_path` | Shorten file paths |

### Custom Filters

```bash
scouter init                                    # creates ~/.config/scouter/filters/
vim ~/.config/scouter/filters/my-tool.yaml      # add your filter
```

User filters take priority over built-in ones.

## Configuration

Optional TOML config at `~/.config/scouter/config.toml`:

```toml
[tracking]
db_path = "~/.local/share/scouter/tracking.db"

[display]
color = true
emoji = true

[filters]
dir = "~/.config/scouter/filters"

[tee]
enabled = true
mode = "failures"    # "failures" | "always" | "never"
max_files = 20
max_file_size = 1048576
```

## Design

- **Startup < 10ms** — scouter intercepts every shell command; latency is critical
- **Graceful degradation** — if a filter fails, fall back to raw output
- **Exit code preservation** — always propagate the underlying tool's exit code
- **Lazy regex compilation** — `sync.Once` per pattern, reused across invocations
- **Zero CGO** — pure Go SQLite driver, static binaries, trivial cross-compilation
- **Goroutine concurrency** — stdout/stderr captured in parallel without thread pools

## Why Go over Rust?

| | **rtk** (Rust) | **scouter** (Go) |
|---|---|---|
| Filters | Compiled into the binary | Declarative YAML — no code needed |
| Concurrency | 2 OS threads | Goroutines |
| SQLite | Requires CGO + C compiler | Pure Go driver — static binary |
| Cross-compilation | Per-target C toolchain | `GOOS=linux GOARCH=arm64 go build` |
| Contributing a filter | Write Rust, wait for release | Write YAML, drop in a folder |

## Development

```bash
make build        # static binary (CGO_ENABLED=0)
make test         # all tests with coverage
make test-race    # race detector
make lint         # go vet + golangci-lint
make install      # install to $GOPATH/bin
```

## Documentation

Full documentation is available on the **[Wiki](https://github.com/Rogercode97/scouter/internal/engine/wiki)**:

- [Installation](https://github.com/Rogercode97/scouter/internal/engine/wiki/Installation) — Homebrew, Go, binaries (macOS/Linux/Windows), from source
- [Integration](https://github.com/Rogercode97/scouter/internal/engine/wiki/Integration) — Claude Code, Cursor, Aider, standalone
- [Gain Dashboard](https://github.com/Rogercode97/scouter/internal/engine/wiki/Gain-Dashboard) — Token savings reports and analytics
- [Filters](https://github.com/Rogercode97/scouter/internal/engine/wiki/Filters) — Built-in filters, custom filters
- [Filter DSL Reference](https://github.com/Rogercode97/scouter/internal/engine/wiki/Filter-DSL-Reference) — All 16 pipeline actions
- [Configuration](https://github.com/Rogercode97/scouter/internal/engine/wiki/Configuration) — TOML config, environment variables
- [Architecture](https://github.com/Rogercode97/scouter/internal/engine/wiki/Architecture) — Design decisions, internals
- [Contributing](https://github.com/Rogercode97/scouter/internal/engine/wiki/Contributing) — Dev setup, adding filters, conventions

## Credits

Inspired by [rtk](https://github.com/rtk-ai/rtk) by the rtk-ai team. rtk proved that filtering shell output before it reaches the LLM context is a powerful idea. scouter rebuilds it in Go with a focus on extensibility — declarative YAML filters that anyone can write without touching the codebase.

## License

MIT
