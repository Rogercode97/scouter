package scoutercmd

import (
	"context"
	"fmt"
	"sort"
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
	type definition struct {
		fqn  string
		path string
	}
	seen := map[definition]bool{}
	var definitions []definition
	for _, s := range syms {
		if s.Name != name {
			continue
		}
		fqn := s.Name
		if s.PackagePath != "" {
			fqn = s.PackagePath + "." + s.Name
		}
		// Identity is the definition, not the rendered name: two symbols in two
		// files can share a fully-qualified name when the language attaches no
		// package path to them, and deduplicating on the name alone would merge
		// two call graphs into one confident answer.
		key := definition{fqn: fqn, path: s.Path}
		if seen[key] {
			continue
		}
		seen[key] = true
		definitions = append(definitions, key)
	}
	switch len(definitions) {
	case 1:
		return definitions[0].fqn, nil
	case 0:
		return "", fmt.Errorf("no symbol named %q: try `scouter search %s` to find its fully-qualified name", name, name)
	default:
		labels := make([]string, 0, len(definitions))
		sharesFQN := false
		for _, d := range definitions {
			if d.fqn == definitions[0].fqn {
				sharesFQN = true
				break
			}
		}
		for _, d := range definitions {
			if sharesFQN {
				labels = append(labels, fmt.Sprintf("%s (%s)", d.fqn, d.path))
				continue
			}
			labels = append(labels, d.fqn)
		}
		sort.Strings(labels)
		return "", fmt.Errorf("%q is ambiguous between %d symbols (%s): rerun with the fully-qualified name", name, len(definitions), strings.Join(labels, ", "))
	}
}
