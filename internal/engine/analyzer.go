package engine

import (
	"context"
	"fmt"
	"go/types"
	"sort"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
	"golang.org/x/tools/go/packages"
)

type AnalysisEngine struct {
	store       store.Repository
	ProjectRoot string
}

func NewAnalysisEngine(store store.Repository) *AnalysisEngine {
	return &AnalysisEngine{
		store:       store,
		ProjectRoot: ".",
	}
}

func (a *AnalysisEngine) BuildTypeUniverse() (map[string]*types.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedSyntax | packages.NeedDeps | packages.NeedImports | packages.NeedName,
		Dir:  a.ProjectRoot,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}

	universe := make(map[string]*types.Package)
	for _, pkg := range pkgs {
		if pkg.Types != nil {
			universe[pkg.PkgPath] = pkg.Types
		}
	}
	return universe, nil
}

func (a *AnalysisEngine) saveImplementation(ctx context.Context, tx store.Repository, typeSym, ifaceSym store.Symbol, V types.Type, iface *types.Interface) {
	// Task 4: Use fully qualified names for 'implements' and 'satisfies'
	callerFQ := typeSym.PackagePath + "." + typeSym.Name
	calleeFQ := ifaceSym.PackagePath + "." + ifaceSym.Name

	_ = tx.SaveCall(ctx, store.Call{
		CallerName: callerFQ,
		CalleeName: calleeFQ,
		Path:       typeSym.Path,
		CalleePath: ifaceSym.Path,
		LinkType:   "implements",
	})

	// Satisfies links for methods
	mset := types.NewMethodSet(V)
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		sel := mset.Lookup(m.Pkg(), m.Name())
		if sel != nil {
			// Method FQ names: pkg.Type.Method
			_ = tx.SaveCall(ctx, store.Call{
				CallerName: callerFQ + "." + m.Name(),
				CalleeName: calleeFQ + "." + m.Name(),
				Path:       typeSym.Path,
				CalleePath: ifaceSym.Path,
				LinkType:   "satisfies",
			})
		}
	}
}

func (a *AnalysisEngine) ResolveInterfaces(ctx context.Context) error {
	universe, err := a.BuildTypeUniverse()
	if err != nil {
		return fmt.Errorf("failed to load type universe: %w", err)
	}

	var interfaces []store.Symbol
	var types_ []store.Symbol

	for sym, err := range a.store.GetAllSymbols(ctx) {
		if err != nil {
			return err
		}
		if sym.Type == "interface" {
			interfaces = append(interfaces, sym)
		} else if sym.Type == "struct" || sym.Type == "type" || sym.Type == "class" {
			types_ = append(types_, sym)
		}
	}

	return a.store.WithTransaction(ctx, func(txCtx context.Context, tx store.Repository) error {
		for _, ifaceSym := range interfaces {
			pkg, ok := universe[ifaceSym.PackagePath]
			if !ok {
				continue
			}

			obj := pkg.Scope().Lookup(ifaceSym.Name)
			if obj == nil {
				continue
			}

			// We need the underlying interface type
			var iface *types.Interface
			if named, ok := obj.Type().(*types.Named); ok {
				if i, ok := named.Underlying().(*types.Interface); ok {
					iface = i
				}
			} else if i, ok := obj.Type().Underlying().(*types.Interface); ok {
				iface = i
			}

			if iface == nil {
				continue
			}

			for _, typeSym := range types_ {
				tPkg, ok := universe[typeSym.PackagePath]
				if !ok {
					continue
				}

				tObj := tPkg.Scope().Lookup(typeSym.Name)
				if tObj == nil {
					continue
				}

				V := tObj.Type()

				// Check if value implements
				if types.Implements(V, iface) {
					a.saveImplementation(txCtx, tx, typeSym, ifaceSym, V, iface)
				} else if types.Implements(types.NewPointer(V), iface) {
					// Check if pointer implements
					a.saveImplementation(txCtx, tx, typeSym, ifaceSym, types.NewPointer(V), iface)
				}
			}
		}
		return nil
	})
}

func (a *AnalysisEngine) ResolveCentrality(ctx context.Context) error {
	indegree := make(map[string]int)
	
	for call, err := range a.store.GetAllCalls(ctx) {
		if err != nil {
			return err
		}
		key := call.CalleeName + ":" + call.CalleePath
		indegree[key]++
	}

	return a.store.WithTransaction(ctx, func(txCtx context.Context, tx store.Repository) error {
		for key, count := range indegree {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				if err := tx.UpdateSymbolCentrality(txCtx, parts[0], parts[1], count); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *AnalysisEngine) GetCriticalSymbols(ctx context.Context, limit int) ([]store.CriticalSymbol, error) {
	if limit <= 0 {
		limit = 20
	}

	centrality := make(map[string]int)
	for call, err := range a.store.GetAllCalls(ctx) {
		if err != nil {
			return nil, err
		}
		key1 := call.CalleeName + ":" + call.CalleePath
		centrality[key1]++
		if call.CalleePath == "" {
			key2 := call.CalleeName + ":"
			centrality[key2]++
		}
	}

	fragility := make(map[string]int)
	for tr, err := range a.store.GetAllFailedTests(ctx) {
		if err != nil {
			return nil, err
		}
		fragility[tr.TargetSymbol]++
	}

	var results []store.CriticalSymbol
	for sym, err := range a.store.GetAllSymbols(ctx) {
		if err != nil {
			return nil, err
		}
		
		cKey1 := sym.Name + ":" + sym.Path
		cKey2 := sym.Name + ":"
		
		cent := centrality[cKey1]
		if cent == 0 {
			cent = centrality[cKey2]
		}
		frag := fragility[sym.Name]
		
		if cent > 0 || frag > 0 || len(results) < limit {
			results = append(results, store.CriticalSymbol{
				Symbol:     sym,
				Centrality: cent,
				Fragility:  frag,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Centrality == results[j].Centrality {
			return results[i].Fragility > results[j].Fragility
		}
		return results[i].Centrality > results[j].Centrality
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
