package engine

import (
	"context"
	"sort"
	"strings"

	"github.com/Rogercode97/scouter/internal/store"
)

type AnalysisEngine struct {
	store store.Repository
}

func NewAnalysisEngine(store store.Repository) *AnalysisEngine {
	return &AnalysisEngine{
		store: store,
	}
}

func (a *AnalysisEngine) ResolveInterfaces(ctx context.Context) error {
	type methodInfo struct {
		name string
		sig  string
	}

	interfaces := make(map[string][]methodInfo)
	structs := make(map[string][]methodInfo)
	structPaths := make(map[string]string)

	for sym, err := range a.store.GetAllSymbols(ctx) {
		if err != nil {
			return err
		}
		if sym.Type == "method_spec" {
			parts := strings.Split(sym.Name, ":")
			if len(parts) == 2 {
				interfaces[parts[0]] = append(interfaces[parts[0]], methodInfo{name: parts[1], sig: sym.Signature})
			}
		} else if sym.Type == "method" {
			parts := strings.Split(sym.Name, ".")
			if len(parts) == 2 {
				structs[parts[0]] = append(structs[parts[0]], methodInfo{name: parts[1], sig: sym.Signature})
				structPaths[parts[0]] = sym.Path
			}
		}
	}

	return a.store.WithTransaction(ctx, func(txCtx context.Context, tx store.Repository) error {
		for iface, requiredMethods := range interfaces {
			for strct, actualMethods := range structs {
				if strct == iface {
					continue
				}

				matches := 0
				for _, req := range requiredMethods {
					for _, act := range actualMethods {
						if req.name == act.name && req.sig == act.sig {
							matches++
							break
						}
					}
				}

				if matches == len(requiredMethods) && len(requiredMethods) > 0 {
					_ = tx.SaveCall(txCtx, store.Call{
						CallerName: strct,
						CalleeName: iface,
						Path:       structPaths[strct],
						LinkType:   "implements",
					})
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
