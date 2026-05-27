package engine

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/imports"

	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	)

	// HealerEngine manages the autonomous RCA -> Fix -> Verify loop using the Shinigami Protocol.
	type HealerEngine struct {
	store    TransactionalStore
	analyzer *AnalysisEngine
	impact   *ImpactEngine
	lspMgr   *lsp.Manager
	Ledger   *Ledger

	// Bridge to MCP sampling
	DoFixRequest func(ctx context.Context, prompt string) (string, error)
	
	Search *SearchEngine
}

func NewHealerEngine(s TransactionalStore, l *lsp.Manager, a *AnalysisEngine, i *ImpactEngine, search *SearchEngine) *HealerEngine {
	return &HealerEngine{
		store:    s,
		lspMgr:   l,
		analyzer: a,
		impact:   i,
		Search:   search,
		Ledger:   NewLedger(),
	}
}

// Fix attempts to repair a test failure using the Shinigami Protocol (Solver-Verifier).
func (e *HealerEngine) Fix(ctx context.Context, errorLog string) (*types.HealResult, error) {
	if errorLog == "" {
		return nil, fmt.Errorf("empty error log")
	}

	// 1. RCA: Extract File and Line
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return nil, fmt.Errorf("could not identify failing file:line in log")
	}

	// For Shinigami, we focus on the first frame for the fix, but use others for context
	primaryMatch := allMatches[0]
	failingFileRaw := primaryMatch[1]
	lineNum, _ := strconv.Atoi(primaryMatch[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return nil, err
	}

	// 2. Resolve Context
	itPointers, _, err := StreamSymbols(ctx, failingFile)
	if err != nil {
		return nil, err
	}

	var target *types.ASTPointer
	for p := range itPointers {
		if lineNum >= p.StartLine && lineNum <= p.EndLine {
			target = &p
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("could not resolve symbol at %s:%d", failingFile, lineNum)
	}

	originalCode, err := ReadFragment(ctx, failingFile, target.Range)
	if err != nil {
		return nil, err
	}

	// 3. Parallel Solvers (Shinigami Phase 1)
	// Enriched context for DeepRCA
	var contextBuilder strings.Builder
	contextBuilder.WriteString(fmt.Sprintf("Failing File: %s\nTarget: %s\nError:\n%s\n\n", failingFile, target.Name, errorLog))
	
	for _, match := range allMatches {
		f := match[1]
		l, _ := strconv.Atoi(match[2])
		
		if e.lspMgr != nil {
			client, err := e.lspMgr.GetClient(ctx, f)
			if err == nil {
				hover, _ := client.Hover(ctx, lsp.HoverParams{
					TextDocumentPositionParams: lsp.TextDocumentPositionParams{
						TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + f},
						Position:     lsp.Position{Line: l - 1, Character: 1},
					},
				})
				if hover != nil && hover.Contents.Value != "" {
					contextBuilder.WriteString(fmt.Sprintf("Context at %s:%d: %s\n", f, l, hover.Contents.Value))
				}
			}
		}
	}
	contextBuilder.WriteString("\nCode:\n" + originalCode)
	
	// Add Impact Analysis for TruthEngine parity
	risk, _ := e.impact.Analyze(ctx, target.Name, failingFile, 1)
	if risk != nil {
		contextBuilder.WriteString(fmt.Sprintf("\n\nCurrent Risk Score: %.2f (%s)", risk.Target.RiskScore, risk.RiskLevel))
	}

	prompt := contextBuilder.String()

	type candidate struct {
		id       int
		code     string
		score    float64
		fullCode string
		valid    bool
	}

	candidates := make(chan candidate, 3)
	var wg sync.WaitGroup
	var valMu sync.Mutex

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fullContent, _ := os.ReadFile(failingFile)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resRaw, err := e.DoFixRequest(ctx, prompt+"\n\nProvide solution variant #"+strconv.Itoa(id))
			if err == nil {
				candidateCode := utils.ExtractCodeBlock(resRaw)
				
				newContent := string(fullContent[:target.Range.Start]) + candidateCode + string(fullContent[target.Range.End:])
				
				processedContent, err := imports.Process(failingFile, []byte(newContent), nil)
				if err != nil {
					processedContent = []byte(newContent)
				}

				tempLedger := NewLedger()
				_ = tempLedger.Stage(failingFile, Patch{
					FilePath:   failingFile,
					Original:   string(fullContent),
					NewContent: string(processedContent),
				})
				
				validator := NewLSPValidator("")

				select {
				case <-ctx.Done():
					return
				default:
				}

				valMu.Lock()
				if ctx.Err() != nil {
					valMu.Unlock()
					return
				}
				valRes, _ := validator.Validate(ctx, tempLedger)
				valMu.Unlock()

				valid := valRes.Valid

				candidates <- candidate{
					id:       id,
					code:     candidateCode,
					fullCode: string(processedContent),
					valid:    valid,
				}

				if valid {
					cancel()
				}
			}
		}(i)
	}

	wg.Wait()
	close(candidates)

	// 4. Verification & Selection (Shinigami Phase 2)
	var bestCandidate *candidate
	
	for c := range candidates {
		if !c.valid {
			continue
		}

		curr := c
		curr.score = 1.0 
		
		// Penalty for overly long solutions (KISS principle)
		if len(curr.code) > len(originalCode)*2 {
			curr.score -= 0.3
		}

		// Deterministic tie-breaker: prefer lower ID if scores are equal
		if bestCandidate == nil || curr.score > bestCandidate.score || (curr.score == bestCandidate.score && curr.id < bestCandidate.id) {
			bestCandidate = &curr
		}
	}

	if bestCandidate == nil {
		return nil, fmt.Errorf("all candidates failed validation")
	}

	// 5. Stage in Ledger (Wave 12.0 Mandate)
	err = e.Ledger.Stage(failingFile, Patch{
		FilePath:   failingFile,
		Original:   string(fullContent),
		NewContent: bestCandidate.fullCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to stage fix: %w", err)
	}

	return &types.HealResult{
		Status:    "STAGED",
		FixedCode: bestCandidate.code,
		Metadata: map[string]string{
			"failingFile": failingFile,
			"method": "shinigami-parallel-sampling",
			"ledger_summary": e.Ledger.Summary(),
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

	return e.store.WithTransaction(ctx, func(ctx context.Context, tx store.Store) error {
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
				Name:           ptr.Name,
				Type:           ptr.Type,
				Signature:      ptr.Signature,
				Doc:            ptr.Doc,
				Path:           path,
				StartByte:      ptr.Range.Start,
				EndByte:        ptr.Range.End,
				StartLine:      ptr.StartLine,
				StartCol:       ptr.StartCol,
				EndLine:        ptr.EndLine,
				StructuralHash: ptr.StructuralHash,
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

// DiagnosticHUD representa la visión térmica/estructural del motor de diagnóstico.
type DiagnosticHUD struct {
	FailingSymbol      string
	LastCommit         string
	RiskLevel          string
	BlastRadius        int
	SimilarPatternPath string
}

// DiagnoseHUD ejecuta la visión de diagnóstico combinando Git, AST, Impacto y Bleve.
// Retorna la representación del HUD en formato ZON.
func (e *HealerEngine) DiagnoseHUD(ctx context.Context, errorLog string) (string, error) {
	if errorLog == "" {
		return "", fmt.Errorf("empty error log")
	}

	// 1. Parseo: Extraer archivo y línea
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return "", fmt.Errorf("could not identify failing file:line in log")
	}

	primaryMatch := allMatches[0]
	failingFileRaw := primaryMatch[1]
	lineNum, _ := strconv.Atoi(primaryMatch[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return "", err
	}

	// 2. Visión de Tiempo (Provenance)
	var lastCommit string
	// Intentamos obtener la procedencia usando el root del proyecto.
	repoRoot := "." // Idealmente utils.GetRepoName(ctx) o similar, pero usamos "." como fallback
	provenance, err := GetFileProvenance(ctx, repoRoot, failingFile)
	if err == nil && len(provenance) >= lineNum && lineNum > 0 {
		// La línea es 1-indexed, el array también lo hacemos 1-indexed en la lógica o verificamos
		// GetFileProvenance devuelve slice, así que el índice sería lineNum - 1
		idx := lineNum - 1
		if idx < len(provenance) {
			lastCommit = provenance[idx].Commit + " (" + provenance[idx].EngineeringEra + ")"
		}
	} else {
		lastCommit = "Unknown"
	}

	// 3. Visión de Rayos X (AST)
	itPointers, _, err := StreamSymbols(ctx, failingFile)
	if err != nil {
		return "", err
	}

	var symbol string
	for p := range itPointers {
		if lineNum >= p.StartLine && lineNum <= p.EndLine {
			symbol = p.Name
			break
		}
	}
	if symbol == "" {
		symbol = "Unknown"
	}

	// 4. Visión de Radar (Impacto)
	riskLevel := "Unknown"
	blastRadius := 0
	if symbol != "Unknown" {
		impact, err := e.impact.Analyze(ctx, symbol, failingFile, 1)
		if err == nil && impact != nil {
			riskLevel = impact.RiskLevel
			blastRadius = len(impact.Callers)
		}
	}

	// 5. Visión Térmica (Similitud)
	similarPath := "None"
	if symbol != "Unknown" && e.Search != nil {
		// Usamos HybridSearch para encontrar Logical Twins
		searchRes, err := e.Search.HybridSearch(ctx, symbol, 10, 0)
		if err == nil && searchRes != nil {
			for _, sym := range searchRes.Symbols {
				if sym.Path != failingFile {
					similarPath = sym.Path
					break
				}
			}
		}
	}

	hud := DiagnosticHUD{
		FailingSymbol:      symbol,
		LastCommit:         lastCommit,
		RiskLevel:          riskLevel,
		BlastRadius:        blastRadius,
		SimilarPatternPath: similarPath,
	}

	// Generamos el output en formato ZON
	zonStr, err := EncodeZON([]DiagnosticHUD{hud})
	if err != nil {
		return "", err
	}

	return zonStr, nil
}
