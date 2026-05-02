package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// HealerEngine manages the autonomous RCA -> Fix -> Verify loop.
type HealerEngine struct {
	store    store.Repository
	analyzer *AnalysisEngine
	impact   *ImpactEngine
	lspMgr   *lsp.Manager

	// Bridge to MCP sampling
	DoFixRequest func(ctx context.Context, prompt string) (string, error)
}

type HealResult struct {
	Status     string            `json:"status"` // SUCCESS, FAILED, SUCCESS_WITH_WARNING
	FixedCode  string            `json:"fixed_code"`
	TestOutput string            `json:"test_output"`
	ImpactDiff *ImpactDiff       `json:"impact_diff,omitempty"`
	Metadata   map[string]string `json:"metadata"`
}

type ImpactDiff struct {
	PreRiskScore   float64 `json:"pre_risk_score"`
	PostRiskScore  float64 `json:"post_risk_score"`
	PreCentrality  float64 `json:"pre_centrality"`
	PostCentrality float64 `json:"post_centrality"`
	Warning        string  `json:"warning,omitempty"`
}

func NewHealerEngine(s store.Repository, l *lsp.Manager, a *AnalysisEngine, i *ImpactEngine) *HealerEngine {
	return &HealerEngine{
		store:    s,
		lspMgr:   l,
		analyzer: a,
		impact:   i,
	}
}

// Fix attempts to repair a test failure.
func (e *HealerEngine) Fix(ctx context.Context, errorLog string) (*HealResult, error) {
	if errorLog == "" {
		return nil, fmt.Errorf("empty error log")
	}

	// 1. RCA: Extract File and Line from log (Multi-frame support)
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return nil, fmt.Errorf("could not identify failing file:line in log")
	}

	var enrichedContext strings.Builder
	var primaryFile string
	var primarySymbol string
	var primaryTarget *types.ASTPointer
	var preRisk float64
	var preCentrality float64

	for i, matches := range allMatches {
		failingFileRaw := matches[1]
		lineNum, _ := strconv.Atoi(matches[2])

		failingFile, err := utils.ValidatePath(failingFileRaw)
		if err != nil {
			continue
		}

		// 2. JIT Resolve Symbol
		itPointers, _, err := StreamSymbols(ctx, failingFile)
		if err != nil {
			continue
		}

		var target *types.ASTPointer
		for p := range itPointers {
			if lineNum >= p.StartLine && lineNum <= p.EndLine {
				target = &p
				break
			}
		}

		if target == nil {
			continue
		}

		if primaryTarget == nil {
			primaryFile = failingFile
			primarySymbol = target.Name
			primaryTarget = target

			// Capture pre-fix metrics (Task 4/5)
			if e.impact != nil {
				impact, _ := e.impact.Analyze(ctx, target.Name, failingFile, 1)
				if impact != nil {
					preRisk = impact.Target.RiskScore
					preCentrality = impact.Target.Metrics.Centrality
				}
			}
		}

		// LSP Enrichment (Task 3)
		hoverContent := ""
		if e.lspMgr != nil {
			client, err := e.lspMgr.GetClient(ctx, failingFile)
			if err == nil {
				hover, err := client.Hover(ctx, lsp.HoverParams{
					TextDocumentPositionParams: lsp.TextDocumentPositionParams{
						TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + failingFile},
						Position: lsp.Position{
							Line:      lineNum - 1,
							Character: 0,
						},
					},
				})
				if err == nil && hover != nil {
					hoverContent = hover.Contents.Value
				}
			}
		}

		enrichedContext.WriteString(fmt.Sprintf("\n--- Frame %d: %s:%d [%s] ---\n", i, failingFile, lineNum, target.Name))
		if hoverContent != "" {
			enrichedContext.WriteString(fmt.Sprintf("LSP Context: %s\n", hoverContent))
		}

		// Impact Summary for prompt (Task 4)
		if i == 0 && e.impact != nil {
			impact, err := e.impact.Analyze(ctx, target.Name, failingFile, 3)
			if err == nil {
				enrichedContext.WriteString(fmt.Sprintf("Current Risk Score: %.4f (%s)\n", impact.Target.RiskScore, impact.RiskLevel))
				if len(impact.Callers) > 0 {
					enrichedContext.WriteString("Direct Callers (Blast Radius):\n")
					for _, c := range impact.Callers {
						if c.Distance <= 1 {
							enrichedContext.WriteString(fmt.Sprintf(" - %s (%s)\n", c.Symbol, c.File))
						}
					}
				}
			}
		}
	}

	if primaryTarget == nil {
		return nil, fmt.Errorf("could not resolve primary symbol")
	}

	code, err := ReadFragment(ctx, primaryFile, primaryTarget.Range)
	if err != nil {
		return nil, fmt.Errorf("failed to read source context: %w", err)
	}

	// 3. Request Fix via Sampling
	prompt := fmt.Sprintf("Failing File: %s\nTarget Symbol: %s\nError Log:\n%s\n\nEnriched Context:\n%s\n\nCurrent Code:\n%s",
		primaryFile, primarySymbol, errorLog, enrichedContext.String(), code)
	newCodeRaw, err := e.DoFixRequest(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("sampling fix failed: %w", err)
	}
	newCode := utils.ExtractCodeBlock(newCodeRaw)

	// 4. Atomic Backup & Apply
	input, err := os.ReadFile(primaryFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	backupFile := primaryFile + ".bak"
	if err := os.WriteFile(backupFile, input, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}
	defer os.Remove(backupFile)

	updatedContent := string(input[:primaryTarget.Range.Start]) + newCode + string(input[primaryTarget.Range.End:])
	if err := os.WriteFile(primaryFile, []byte(updatedContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to apply fix: %w", err)
	}

	// 5. Verify (Task 5)
	pkgDir := filepath.Dir(primaryFile)
	root, _ := utils.GetRepoRoot()
	relPkgDir, _ := filepath.Rel(root, pkgDir)
	if relPkgDir == "" || relPkgDir == "." {
		relPkgDir = "./"
	} else {
		relPkgDir = "./" + relPkgDir
	}

	cmd := exec.CommandContext(ctx, "go", "test", "-v", relPkgDir)
	testOut, testErr := cmd.CombinedOutput()

	status := "SUCCESS"
	if testErr != nil {
		status = "FAILED"
		_ = os.WriteFile(primaryFile, input, 0644) // Restore
		return &HealResult{
			Status:     status,
			FixedCode:  newCode,
			TestOutput: string(testOut),
			Metadata:   map[string]string{"failingFile": primaryFile},
		}, nil
	}

	// Post-Fix Integrity Check (Task 5)
	var diff *ImpactDiff
	if e.impact != nil && e.analyzer != nil {
		// Re-index and Resolve
		_ = e.Index(ctx, primaryFile)
		_ = e.analyzer.ResolveCentrality(ctx)

		postImpact, _ := e.impact.Analyze(ctx, primarySymbol, primaryFile, 1)
		if postImpact != nil {
			diff = &ImpactDiff{
				PreRiskScore:   preRisk,
				PostRiskScore:  postImpact.Target.RiskScore,
				PreCentrality:  preCentrality,
				PostCentrality: postImpact.Target.Metrics.Centrality,
			}
			// Warning if centrality spikes > 20%
			if preCentrality > 0 && (postImpact.Target.Metrics.Centrality/preCentrality) > 1.2 {
				diff.Warning = "Centrality increased by more than 20%. The fix might be introducing high coupling."
				status = "SUCCESS_WITH_WARNING"
			}
		}
	}

	return &HealResult{
		Status:     status,
		FixedCode:  newCode,
		TestOutput: string(testOut),
		ImpactDiff: diff,
		Metadata: map[string]string{
			"failingFile": primaryFile,
		},
	}, nil
}

// Index parses and persists a file. (Internal helper for HealerEngine)
func (e *HealerEngine) Index(ctx context.Context, path string) error {
	itPointers, itCalls, err := StreamSymbols(ctx, path)
	if err != nil {
		return err
	}

	hash, _ := utils.CalculateHash(path)
	fi, _ := os.Stat(path)
	mtime := int64(0)
	if fi != nil {
		mtime = fi.ModTime().UnixNano()
	}

	return e.store.WithTransaction(ctx, func(ctx context.Context, tx store.Repository) error {
		tx.SaveFileIndex(ctx, &store.FileIndex{
			Path:    path,
			Mtime:   mtime,
			Hash:    hash,
			ASTJSON: "{}",
			Project: utils.GetRepoName(ctx),
		})
		tx.ClearSymbols(ctx, path)
		tx.ClearCalls(ctx, path)

		for ptr := range itPointers {
			_ = tx.SaveSymbol(ctx, &store.Symbol{
				Name:      ptr.Name,
				Type:      ptr.Type,
				Signature: ptr.Signature,
				Doc:       ptr.Doc,
				Path:      path,
				StartByte: ptr.Range.Start,
				EndByte:   ptr.Range.End,
				StartLine: ptr.StartLine,
				StartCol:  ptr.StartCol,
				EndLine:   ptr.EndLine,
			})
		}
		for c := range itCalls {
			_ = tx.SaveCall(ctx, store.Call{
				CallerName: c.CallerName,
				CalleeName: c.CalleeName,
				CalleePath: c.CalleePath,
				LinkType:   c.LinkType,
				Path:       path,
				Line:       c.Line,
			})
		}
		return nil
	})
}
