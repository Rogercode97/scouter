package scoutercmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
)

// symbolSearcher is the minimal store surface the FQN resolver needs.
// store.Store satisfies it; tests use a stub.
type symbolSearcher interface {
	SearchSymbols(ctx context.Context, q, t, pathPrefix string, limit, offset int) ([]store.Symbol, error)
}

// resolveSymbolFQN maps what the user typed to the fully-qualified name the
// call graph stores. Qualified input passes through untouched. A short name
// resolves only when it names exactly one symbol; zero or several matches
// are an error, never a silent empty blast radius.
func resolveSymbolFQN(ctx context.Context, db symbolSearcher, name string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}
	syms, err := db.SearchSymbols(ctx, name, "", "", 50, 0)
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	var fqns []string
	for _, s := range syms {
		if s.Name != name {
			continue
		}
		fqn := s.Name
		if s.PackagePath != "" {
			fqn = s.PackagePath + "." + s.Name
		}
		if !seen[fqn] {
			seen[fqn] = true
			fqns = append(fqns, fqn)
		}
	}
	switch len(fqns) {
	case 1:
		return fqns[0], nil
	case 0:
		return "", fmt.Errorf("no symbol named %q: try `scouter search %s` to find its fully-qualified name", name, name)
	default:
		return "", fmt.Errorf("%q is ambiguous between %d symbols (%s): rerun with the fully-qualified name", name, len(fqns), strings.Join(fqns, ", "))
	}
}
