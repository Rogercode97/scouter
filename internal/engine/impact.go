package engine

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

var hunkRegex = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
var engramIDRegex = regexp.MustCompile(`\[\d+\] #\d+`)

type ImpactEngine struct {
	store      GraphStore
	LSPManager *lsp.Manager
	memory     memory.MemoryProvider
}

func NewImpactEngine(s GraphStore, lm *lsp.Manager, mem memory.MemoryProvider) *ImpactEngine {
	return &ImpactEngine{
		store:      s,
		LSPManager: lm,
		memory:     mem,
	}
}

// Analyze performs a deep impact analysis for a given symbol.
func (e *ImpactEngine) Analyze(ctx context.Context, symbol string, path string, maxDepth int) (*types.ImpactResult, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	// 1. Recursive Traversal (via Store)
	callers, err := e.store.GetCallersRecursive(ctx, symbol, path, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("impact analysis failed: %w", err)
	}

	// 2. Fetch Target Metadata
	// Note: We search for the specific symbol in the specific path to get its metadata.
	targetSymbols, err := e.store.GetSymbolsByNameInFile(ctx, symbol, path)
	var centrality float64
	var isExported bool
	var signature string
	if err == nil && len(targetSymbols) > 0 {
		sym := targetSymbols[0]
		isExported = strings.Contains("ABCDEFGHIJKLMNOPQRSTUVWXYZ", string(sym.Name[0]))
		signature = sym.Signature
		centrality = sym.PageRank
	}

	symbolID := utils.SymbolSignatureHash(symbol, path, signature)

	res := &types.ImpactResult{
		Target: types.ImpactEntity{
			Symbol: symbol,
			File:   path,
			Metrics: types.RiskMetrics{
				Centrality:         centrality,
				PublicExport:       isExported,
				HistoricalBugfixes: e.getHistoricalRisk(ctx, symbol, path, symbolID),
			},
		},
		Callers: []types.ImpactEntity{},
	}

	// 3. Build Graph and Calculate Blast Radius
	mermaid := "graph TD\n"
	blastRadius := 0
	edges := make(map[string]bool)

	for _, c := range callers {
		r := types.ImpactEntity{
			Symbol:   c.CallerName,
			File:     c.Path,
			Distance: c.Line, // We stored distance in Line field in GetCallersRecursive
			LinkType: c.LinkType,
		}
		res.Callers = append(res.Callers, r)
		blastRadius++

		// Build Mermaid Edge
		parent := symbol
		if r.Distance > 1 {
			for _, prev := range res.Callers {
				if prev.Distance == r.Distance-1 {
					parent = prev.Symbol
					break
				}
			}
		}
		
		edgeSymbol := "-->"
		if r.LinkType == "implements" {
			edgeSymbol = "-.->"
		}
		
		edge := fmt.Sprintf("    %s[\"%s\"] %s %s[\"%s\"]", r.Symbol, r.Symbol, edgeSymbol, parent, parent)
		if !edges[edge] {
			mermaid += edge + "\n"
			edges[edge] = true
		}
	}
	res.Mermaid = mermaid

	// 4. Risk Score
	cScore := math.Min(1.0, math.Log1p(centrality)/math.Log1p(100.0))
	bScore := math.Min(1.0, math.Log1p(float64(blastRadius))/math.Log1p(500.0))
	hScore := math.Min(0.2, float64(res.Target.Metrics.HistoricalBugfixes)*0.05)
	eScore := 0.0
	if isExported { eScore = 0.2 }

	res.Target.RiskScore = (cScore * 0.4) + (bScore * 0.4) + hScore + eScore
	res.Target.RiskScore = math.Min(1.0, res.Target.RiskScore)

	switch {
	case res.Target.RiskScore >= 0.8: res.RiskLevel = "Critical"
	case res.Target.RiskScore >= 0.5: res.RiskLevel = "High"
	case res.Target.RiskScore >= 0.2: res.RiskLevel = "Medium"
	default: res.RiskLevel = "Low"
	}

	return res, nil
}

func (e *ImpactEngine) getHistoricalRisk(ctx context.Context, symbol string, path string, symbolID string) int {
	topicKey := "scouter/risk/" + symbolID
	relPath := path
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil {
			relPath = rel
		}
	}
	queries := []string{symbol, relPath, topicKey}
	uniqueIDs := make(map[string]bool)
	for _, q := range queries {
		if e.memory == nil {
			continue
		}
		insights, err := e.memory.SearchInsights(ctx, q, 10)
		if err == nil {
			for _, insight := range insights {
				// We only care about bugfixes for historical risk
				if strings.EqualFold(insight.Type, "bugfix") {
					uniqueIDs[insight.ID] = true
				}
			}
		}
	}
	return len(uniqueIDs)
}

// PredictTests identifies tests affected by changes described in the diff string.
func (e *ImpactEngine) PredictTests(ctx context.Context, diff string) ([]types.TestTarget, error) {
	if diff == "" {
		return nil, nil
	}

	ranges, err := parseDiff(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	var allSymbols []store.Symbol
	for _, r := range ranges {
		absPath, err := filepath.Abs(r.Path)
		if err != nil {
			absPath = r.Path
		}

		symbols, err := e.store.GetSymbolsByRange(ctx, absPath, r.StartLine, r.EndLine)
		if err != nil {
			continue
		}
		allSymbols = append(allSymbols, symbols...)
	}

	return findTestsForSymbols(ctx, e.store, allSymbols)
}

func (e *ImpactEngine) GetDeterministicCallers(ctx context.Context, symbolName string) ([]store.Call, error) {
	if e.LSPManager == nil {
		return nil, fmt.Errorf("lsp manager not configured")
	}

	results, err := e.store.SearchSymbols(ctx, symbolName, "", 0, 0)
	if err != nil || len(results) == 0 {
		return nil, nil
	}

	sym := results[0]
	client, err := e.LSPManager.GetClient(ctx, sym.Path)
	if err != nil {
		return nil, fmt.Errorf("lsp client failed: %w", err)
	}

	items, err := client.PrepareCallHierarchy(ctx, lsp.CallHierarchyPrepareParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + sym.Path},
			Position: lsp.Position{
				Line:      sym.StartLine - 1,
				Character: sym.StartCol,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("prepare call hierarchy failed: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}

	calls, err := client.IncomingCalls(ctx, lsp.CallHierarchyIncomingCallsParams{
		Item: items[0],
	})
	if err != nil {
		return nil, fmt.Errorf("incoming calls failed: %w", err)
	}

	var callers []store.Call
	for _, call := range calls {
		path := strings.TrimPrefix(call.From.URI, "file://")
		callers = append(callers, store.Call{
			Path:       path,
			CallerName: call.From.Name,
		})
	}
	return callers, nil
}

// IdentifyCriticalContext finds high-risk symbols affected by the current diff.
func (e *ImpactEngine) IdentifyCriticalContext(ctx context.Context, diff string) ([]types.ImpactEntity, error) {
	if diff == "" {
		return nil, nil
	}

	ranges, err := parseDiff(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	var critical []types.ImpactEntity
	seen := make(map[string]bool)

	// Fetch git root for proper path resolution
	gitRoot := ""
	if rootOut, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output(); err == nil {
		gitRoot = strings.TrimSpace(string(rootOut))
	}

	impactChecks := 0

	for _, r := range ranges {
		absPath := r.Path
		if gitRoot != "" && !filepath.IsAbs(r.Path) {
			absPath = filepath.Join(gitRoot, r.Path)
		} else if !filepath.IsAbs(absPath) {
			absPath, _ = filepath.Abs(r.Path)
		}

		symbols, err := e.store.GetSymbolsByRange(ctx, absPath, r.StartLine, r.EndLine)
		if err != nil {
			continue
		}

		for _, sym := range symbols {
			key := sym.Name + ":" + sym.Path
			if seen[key] {
				continue
			}
			seen[key] = true

			// Prevent N+1 IO Thrashing: Cap the deep impact analysis to 5 symbols per diff
			if impactChecks >= 5 {
				break
			}
			impactChecks++

			impact, err := e.Analyze(ctx, sym.Name, sym.Path, 3)
			if err != nil {
				continue
			}

			if impact.Target.RiskScore > 0.6 {
				critical = append(critical, impact.Target)
			}
		}
	}

	return critical, nil
}

type diffRange struct {
	Path      string
	StartLine int
	EndLine   int
}

func parseDiff(diff string) ([]diffRange, error) {
	var ranges []diffRange
	var currentFile string
	scanner := bufio.NewScanner(strings.NewReader(diff))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			matches := hunkRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				start, _ := strconv.Atoi(matches[1])
				count := 1
				if len(matches) == 3 && matches[2] != "" {
					count, _ = strconv.Atoi(matches[2])
				}

				ranges = append(ranges, diffRange{
					Path:      currentFile,
					StartLine: start,
					EndLine:   start + count - 1,
				})
			}
		}
	}

	return ranges, nil
}

func findTestsForSymbols(ctx context.Context, db GraphStore, symbols []store.Symbol) ([]types.TestTarget, error) {
	uniqueTests := make(map[string]types.TestTarget)
	for _, sym := range symbols {
		affectedTests, err := db.GetAffectedTestsRecursive(ctx, sym.Name, sym.Path)
		if err != nil {
			return nil, err
		}
		for _, t := range affectedTests {
			key := t.Path + ":" + t.Name
			uniqueTests[key] = types.TestTarget{
				Name: t.Name,
				File: t.Path,
			}
		}
	}

	result := make([]types.TestTarget, 0, len(uniqueTests))
	for _, t := range uniqueTests {
		result = append(result, t)
	}
	return result, nil
}
