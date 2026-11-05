package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var ldflagInjection = regexp.MustCompile(`-X\s+(\S+)=`)

// TestGoreleaserLdflagsNameExistingSymbols closes the gap that let every
// release report the development version. The linker ignores -X when the
// symbol it names does not exist, so the only moment a typo is observable is a
// release build: `goreleaser` injected internal/cli.version while the package
// declared Version, and the shipped binary printed "scouter vdev". The guard
// resolves each -X target against the source tree instead, and fails on the
// mismatch a compiler would refuse.
func TestGoreleaserLdflagsNameExistingSymbols(t *testing.T) {
	root := ".."
	raw, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yaml: %v", err)
	}

	var cfg struct {
		Builds []struct {
			Ldflags []string `yaml:"ldflags"`
		} `yaml:"builds"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yaml: %v", err)
	}

	module := modulePath(t, root)

	injections := 0
	for _, build := range cfg.Builds {
		for _, line := range build.Ldflags {
			for _, match := range ldflagInjection.FindAllStringSubmatch(line, -1) {
				injections++
				target := match[1]
				dot := strings.LastIndex(target, ".")
				if dot <= 0 || dot == len(target)-1 {
					t.Errorf("ldflag -X %q is not importpath.Symbol", target)
					continue
				}
				pkgPath, symbol := target[:dot], target[dot+1:]
				if !strings.HasPrefix(pkgPath, module) {
					t.Errorf("ldflag -X %q targets %q, outside module %q", target, pkgPath, module)
					continue
				}
				dir := filepath.Join(root, strings.TrimPrefix(pkgPath, module))
				if !declaresVar(t, dir, symbol) {
					t.Errorf("ldflag -X %q: package %s declares no package-level variable %q, so the linker drops the version silently", target, pkgPath, symbol)
				}
			}
		}
	}

	if injections == 0 {
		t.Fatal("no -X ldflag found in .goreleaser.yaml: the release would ship without a version, or this guard stopped seeing the file it protects")
	}
}

func modulePath(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	t.Fatal("go.mod declares no module path")
	return ""
}

// declaresVar reports whether the package in dir has a package-level var with
// this name. Test files are skipped: a symbol that only exists under _test.go is
// not linked into the released binary.
func declaresVar(t *testing.T, dir, symbol string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Errorf("read package dir %s: %v", dir, err)
		return false
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Errorf("parse %s: %v", name, err)
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, id := range value.Names {
					if id.Name == symbol {
						return true
					}
				}
			}
		}
	}
	return false
}
