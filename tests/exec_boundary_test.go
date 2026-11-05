package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// execBoundary is the closed set of production files allowed to start an
// external process without going through utils.SafeCommand, each with the
// reason it cannot. Anything else is a defect: the mandate is that external
// commands go through the wrapper, and a mandate nothing enforces drifts the
// moment a new call site lands.
var execBoundary = map[string]string{
	"internal/utils/safe_exec.go":    "the gateway itself, the only place allowed to build the validated command",
	"internal/engine/executor.go":    "product runner: it executes the command the user asked scouter to run",
	"internal/filter/astgrep.go":     "filter adapter: the binary and its flags come from the filter definition",
	"internal/engine/lsp/manager.go": "long-running daemon: its -listen=unix;<socket> spec carries the separator SafeCommand refuses by design",
	"internal/engine/lsp/client.go":  "language-server client: it spawns the server binary with protocol-specific arguments and owns its stdio",
	"internal/mcp/server.go":         "test hook: var execCommand = exec.Command, replaced by tests and never used to run anything else"}

// TestExecCallsStayInsideTheDeclaredBoundary pins AGENTS.md's rule that raw
// exec is forbidden outside the declared exceptions. Eight production call
// sites had escaped it — the git helpers, Chronos, predict, the rules engine —
// which is exactly what an unenforced mandate produces.
func TestExecCallsStayInsideTheDeclaredBoundary(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}

	found := map[string][]string{}
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		hits, err := execCallsIn(path)
		if err != nil {
			return err
		}
		if len(hits) > 0 {
			found[rel] = hits
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}

	var unlisted []string
	for file, hits := range found {
		if _, ok := execBoundary[file]; ok {
			continue
		}
		for _, hit := range hits {
			unlisted = append(unlisted, file+":"+hit)
		}
	}
	sort.Strings(unlisted)
	if len(unlisted) > 0 {
		t.Fatalf("raw exec outside the declared boundary:\n\t%s\nroute them through utils.SafeCommand, or add the file to execBoundary with the reason it cannot", strings.Join(unlisted, "\n\t"))
	}

	for file := range execBoundary {
		if _, ok := found[file]; !ok {
			t.Errorf("execBoundary lists %s but it starts no process: a stale exception hides the next escape", file)
		}
	}
}

// execCallsIn returns the line numbers where one file reaches os/exec's
// Command or CommandContext, as a call or as a bare reference. The reference
// case matters: `var execCommand = exec.Command` hands the constructor to
// anyone, and a scanner that only looked at call expressions would call that
// file clean. The import alias is resolved instead of matching the text
// "exec.", so an aliased import cannot hide a site either.
func execCallsIn(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	aliases := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || importPath != "os/exec" {
			continue
		}
		name := "exec"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		aliases[name] = true
	}
	if len(aliases) == 0 {
		return nil, nil
	}

	var lines []string
	seen := map[int]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || !aliases[pkg.Name] {
			return true
		}
		if selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext" {
			return true
		}
		line := fset.Position(selector.Pos()).Line
		if seen[line] {
			return true
		}
		seen[line] = true
		lines = append(lines, strconv.Itoa(line))
		return true
	})
	return lines, nil
}

// TestExecGuardSeesAnEscapingCallSite breaks the rule on purpose and proves the
// guard reports it. A guard that has never blocked anything is an untested
// claim.
func TestExecGuardSeesAnEscapingCallSite(t *testing.T) {
	dir := t.TempDir()
	source := `package probe

import osexec "os/exec"

func run() {
	osexec.CommandContext(nil, "git", "status")
}
`
	path := filepath.Join(dir, "probe.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write probe: %v", err)
	}

	hits, err := execCallsIn(path)
	if err != nil {
		t.Fatalf("scan probe: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("aliased exec call sites found = %v, want exactly one", hits)
	}
}
