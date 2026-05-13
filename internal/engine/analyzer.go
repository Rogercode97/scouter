package engine

import (
	"context"
	"fmt"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/dominikbraun/graph"
	"golang.org/x/tools/go/packages"
)

func (a *AnalysisEngine) ResolvePageRank(ctx context.Context) error {
	// 1. Create a weighted directed graph
	g := graph.New(graph.StringHash, graph.Directed(), graph.Weighted())

	// 2. Add all symbols as vertices
	for sym, err := range a.store.GetAllSymbols(ctx) {
		if err != nil {
			return err
		}
		key := sym.Name + ":" + sym.Path
		_ = g.AddVertex(key)
	}

	// 3. Add edges from calls
	for call, err := range a.store.GetAllCalls(ctx) {
		if err != nil {
			return err
		}
		
		callerKey := call.CallerName + ":" + call.Path
		calleeKey := call.CalleeName + ":" + call.CalleePath
		
		// Weight based on link type
		weight := 1
		switch call.LinkType {
		case "implements":
			weight = 10
		case "satisfies":
			weight = 5
		case "embeds":
			weight = 3
		case "dynamic":
			weight = 2
		}

		_ = g.AddEdge(callerKey, calleeKey, graph.EdgeWeight(weight))
	}

	// 4. Calculate PageRank (Simplified Iterative Implementation)
	// Note: dominikbraun/graph doesn't have native PageRank yet in v0.23,
	// so we implement it using the graph structure.
	
	adjacency, _ := g.AdjacencyMap()
	nodes := make([]string, 0, len(adjacency))
	for node := range adjacency {
		nodes = append(nodes, node)
	}

	ranks := make(map[string]float64)
	initialRank := 1.0 / float64(len(nodes))
	for _, node := range nodes {
		ranks[node] = initialRank
	}

	damping := 0.85
	iterations := 10
	
	// Inverse adjacency for easier rank propagation
	incoming := make(map[string][]string)
	outdegree := make(map[string]int)
	for src, targets := range adjacency {
		outdegree[src] = len(targets)
		for dest := range targets {
			incoming[dest] = append(incoming[dest], src)
		}
	}

	for i := 0; i < iterations; i++ {
		newRanks := make(map[string]float64)
		for _, node := range nodes {
			rankSum := 0.0
			for _, neighbor := range incoming[node] {
				rankSum += ranks[neighbor] / float64(outdegree[neighbor])
			}
			newRanks[node] = (1.0-damping)/float64(len(nodes)) + damping*rankSum
		}
		ranks = newRanks
	}

	// 5. Update store
	return a.store.WithTransaction(ctx, func(txCtx context.Context, tx store.Repository) error {
		for node, score := range ranks {
			parts := strings.SplitN(node, ":", 2)
			if len(parts) == 2 {
				if err := tx.UpdateSymbolPageRank(txCtx, parts[0], parts[1], score); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

type AnalysisEngine struct {
	store       store.Repository
	ProjectRoot string
}

func NewAnalysisEngine(store store.Repository) *AnalysisEngine {
	root, _ := filepath.Abs(".")
	return &AnalysisEngine{
		store:       store,
		ProjectRoot: root,
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

			// Phase 1: Hierarchical Link Enrichment (Embedding)
			for i := 0; i < iface.NumEmbeddeds(); i++ {
				emb := iface.EmbeddedType(i)
				if named, ok := emb.(*types.Named); ok {
					embObj := named.Obj()
					embPkg := embObj.Pkg()
					if embPkg != nil {
						embFQ := embPkg.Path() + "." + embObj.Name()
						ifaceFQ := ifaceSym.PackagePath + "." + ifaceSym.Name

						// Link interfaces (embeds)
						_ = tx.SaveCall(ctx, store.Call{
							CallerName: ifaceFQ,
							CalleeName: embFQ,
							Path:       ifaceSym.Path,
							LinkType:   "embeds",
						})

						// Link methods for deep propagation
						if embIface, ok := named.Underlying().(*types.Interface); ok {
							for j := 0; j < embIface.NumMethods(); j++ {
								m := embIface.Method(j)
								_ = tx.SaveCall(ctx, store.Call{
									CallerName: ifaceFQ + "." + m.Name(),
									CalleeName: embPkg.Path() + "." + embObj.Name() + "." + m.Name(),
									Path:       ifaceSym.Path,
									LinkType:   "embeds",
								})
							}
						}
					}
				}
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
