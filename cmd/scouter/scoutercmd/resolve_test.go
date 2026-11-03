package scoutercmd

import (
	"context"
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/store"
)

type stubSearcher struct {
	syms []store.Symbol
	err  error
}

func (s stubSearcher) SearchSymbols(ctx context.Context, q, t, pathPrefix string, limit, offset int) ([]store.Symbol, error) {
	return s.syms, s.err
}

func TestResolveSymbolFQN_QualifiedPassthrough(t *testing.T) {
	got, err := resolveSymbolFQN(context.Background(), stubSearcher{}, "github.com/x/y.SafeCommand")
	if err != nil {
		t.Fatalf("qualified name should pass through: %v", err)
	}
	if got != "github.com/x/y.SafeCommand" {
		t.Errorf("got %q", got)
	}
}

func TestResolveSymbolFQN_UnambiguousShortName(t *testing.T) {
	db := stubSearcher{syms: []store.Symbol{
		{Name: "SafeCommand", PackagePath: "github.com/Rogercode97/scouter/internal/utils", Path: "/tmp/x.go"},
		{Name: "SafeCommanders", PackagePath: "other/pkg", Path: "/tmp/y.go"},
	}}
	got, err := resolveSymbolFQN(context.Background(), db, "SafeCommand")
	if err != nil {
		t.Fatalf("unambiguous short name should resolve: %v", err)
	}
	want := "github.com/Rogercode97/scouter/internal/utils.SafeCommand"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSymbolFQN_NoMatchIsError(t *testing.T) {
	_, err := resolveSymbolFQN(context.Background(), stubSearcher{}, "Nope")
	if err == nil || !strings.Contains(err.Error(), "no symbol named") {
		t.Errorf("expected a no-match error, got %v", err)
	}
}

func TestResolveSymbolFQN_AmbiguousIsError(t *testing.T) {
	db := stubSearcher{syms: []store.Symbol{
		{Name: "Greet", PackagePath: "a", Path: "/tmp/a.go"},
		{Name: "Greet", PackagePath: "b", Path: "/tmp/b.go"},
	}}
	_, err := resolveSymbolFQN(context.Background(), db, "Greet")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected an ambiguity error, got %v", err)
	}
}

// A language whose symbols carry no package path (Python, for one) must still
// refuse a short name that names two different definitions in two different
// files. Collapsing them onto one fully-qualified name silently merges two
// call graphs and reports callers from both.
func TestResolveSymbolFQN_AmbiguousWithoutPackagePathIsError(t *testing.T) {
	db := stubSearcher{syms: []store.Symbol{
		{Name: "transform", Path: "/tmp/core.py"},
		{Name: "transform", Path: "/tmp/decoys.py"},
	}}
	_, err := resolveSymbolFQN(context.Background(), db, "transform")
	if err == nil {
		t.Fatal("two definitions in two files resolved to one name")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected an ambiguity error, got %v", err)
	}
	if !strings.Contains(err.Error(), "core.py") || !strings.Contains(err.Error(), "decoys.py") {
		t.Fatalf("ambiguity error should name both files, got %v", err)
	}
}
