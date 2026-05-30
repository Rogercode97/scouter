package engine

import (
	"context"
	"fmt"

	"github.com/Rogercode97/scouter/internal/types"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// SSACallGraph generates a high-precision call graph using SSA and CHA.
func SSACallGraph(ctx context.Context, dir string) ([]types.ASTCall, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cfg := &packages.Config{
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedTypesSizes | packages.NeedImports | packages.NeedDeps,
		Tests:   true,
		Dir:     dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}

	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contains errors")
	}

	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	cg := cha.CallGraph(prog)

	var calls []types.ASTCall
	callgraph.GraphVisitEdges(cg, func(e *callgraph.Edge) error {
		if e.Caller.Func == nil || e.Callee.Func == nil {
			return nil
		}

		callerName := e.Caller.Func.String()
		calleeName := e.Callee.Func.String()

		// Filter out standard library calls to reduce noise if needed, 
		// but for now, let's capture everything and let the consumer filter.
		
		var path string
		var line int
		if e.Site != nil {
			pos := prog.Fset.Position(e.Site.Pos())
			path = pos.Filename
			line = pos.Line
		} else if e.Caller.Func != nil {
			// If no call site (e.g. implicit calls), use caller's position
			pos := prog.Fset.Position(e.Caller.Func.Pos())
			path = pos.Filename
			line = pos.Line
		}

		calls = append(calls, types.ASTCall{
			CallerName: callerName,
			CalleeName: calleeName,
			LinkType:   "ssa-call",
			Path:       path,
			Line:       line,
		})
		return nil
	})

	return calls, nil
}

// AnalyzeInterfaceImplementations finds all concrete implementations of interfaces.
func AnalyzeInterfaceImplementations(prog *ssa.Program) []types.ASTCall {
	var links []types.ASTCall
	// TODO: Implement using SSA type information
	return links
}
