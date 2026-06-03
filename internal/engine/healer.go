package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/imports"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/cinar/resile"
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
	memory memory.MemoryProvider
}

func NewHealerEngine(s TransactionalStore, l *lsp.Manager, a *AnalysisEngine, i *ImpactEngine, search *SearchEngine, mem memory.MemoryProvider) *HealerEngine {
	return &HealerEngine{
		store:    s,
		lspMgr:   l,
		analyzer: a,
		impact:   i,
		Search:   search,
		memory:   mem,
		Ledger:   NewLedger(),
	}
}

// Fix attempts to repair a test failure using the Shinigami Protocol (Solver-Verifier).
func (e *HealerEngine) Fix(ctx context.Context, errorLog string) (*types.HealResult, error) {
	if errorLog == "" {
		return nil, fmt.Errorf("empty error log")
	}

	// 1. RCA: Extract File and Line
	failingFile, lineNum, allMatches, err := extractRCA(errorLog)
	if err != nil {
		return nil, err
	}

	// 2. Resolve Context
	itPointers, _, _, err := StreamSymbols(ctx, failingFile)
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

	// 3. Build Prompt Context
	prompt := e.buildHealerContext(ctx, target, failingFile, errorLog, allMatches, originalCode)

	// 4. Parallel Solvers (Shinigami Phase 1 & 2)
	bestCandidate, err := e.sampleParallelFixes(ctx, prompt, failingFile, originalCode, target)
	if err != nil {
		return nil, err
	}

	// 5. Stage in Ledger (Wave 12.0 Mandate)
	fullContent, _ := os.ReadFile(failingFile)
	err = e.Ledger.Stage(failingFile, Patch{
		FilePath:   failingFile,
		Original:   string(fullContent),
		NewContent: bestCandidate.fullCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to stage fix: %w", err)
	}

	e.recordInoculation(ctx, target, failingFile, errorLog)

	return &types.HealResult{
		Status:    "STAGED",
		FixedCode: bestCandidate.code,
		Metadata: map[string]string{
			"failingFile":    failingFile,
			"method":         "shinigami-parallel-sampling",
			"ledger_summary": e.Ledger.Summary(),
		},
	}, nil
}

func (e *HealerEngine) recordInoculation(ctx context.Context, target *types.ASTPointer, failingFile, errorLog string) {
	if e.memory == nil {
		return
	}

	// Extract a brief reason for RCA
	lines := strings.Split(errorLog, "\n")
	reason := "Autonomous repair"
	if len(lines) > 0 && len(lines[0]) > 0 {
		reason = lines[0]
	}

	content := fmt.Sprintf("**What**: Autonomous repair of failing test\n**Why**: %s\n**Where**: %s\n**Learned**: Successful Shinigami intervention", reason, failingFile)

	symbolID := utils.SymbolSignatureHash(target.Name, failingFile, target.Signature)
	topic := "scouter/risk/" + symbolID

	mem := memory.DistilledMemory{
		Title:   fmt.Sprintf("Healer Fix: %s", target.Name),
		Type:    "bugfix",
		Content: content,
		Topic:   topic,
	}

	_ = e.memory.SaveObservation(ctx, utils.GetRepoName(ctx), mem)
}

func extractRCA(errorLog string) (string, int, [][]string, error) {
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return "", 0, nil, fmt.Errorf("could not identify failing file:line in log")
	}

	primaryMatch := allMatches[0]
	failingFileRaw := primaryMatch[1]
	lineNum, _ := strconv.Atoi(primaryMatch[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return "", 0, nil, err
	}
	return failingFile, lineNum, allMatches, nil
}

func (e *HealerEngine) buildHealerContext(ctx context.Context, target *types.ASTPointer, failingFile, errorLog string, allMatches [][]string, originalCode string) string {
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

	risk, _ := e.impact.Analyze(ctx, target.Name, failingFile, 1)
	if risk != nil {
		contextBuilder.WriteString(fmt.Sprintf("\n\nCurrent Risk Score: %.2f (%s)", risk.Target.RiskScore, risk.RiskLevel))
	}

	return contextBuilder.String()
}

type fixCandidate struct {
	id       int
	code     string
	score    float64
	fullCode string
	valid    bool
}

func (e *HealerEngine) sampleParallelFixes(ctx context.Context, prompt string, failingFile string, originalCode string, target *types.ASTPointer) (*fixCandidate, error) {
	candidates := make(chan fixCandidate, 3)
	var valMu sync.Mutex

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)

	fullContent, _ := os.ReadFile(failingFile)

	for i := 0; i < 3; i++ {
		id := i
		eg.Go(func() error {
			resRaw, err := resile.Do(egCtx, func(c context.Context) (string, error) {
				return e.DoFixRequest(c, prompt+"\n\nProvide solution variant #"+strconv.Itoa(id))
			}, resile.WithRetry(3))
			if err != nil {
				return err
			}
			
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
			case <-egCtx.Done():
				return egCtx.Err()
			default:
			}

			valMu.Lock()
			if egCtx.Err() != nil {
				valMu.Unlock()
				return egCtx.Err()
			}
			valRes, _ := validator.Validate(egCtx, tempLedger)
			valMu.Unlock()

			valid := valRes.Valid

			candidates <- fixCandidate{
				id:       id,
				code:     candidateCode,
				fullCode: string(processedContent),
				valid:    valid,
			}

			if valid {
				cancel()
			}
			return nil
		})
	}

	err := eg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}
	close(candidates)

	var bestCandidate *fixCandidate

	for c := range candidates {
		if !c.valid {
			continue
		}

		curr := c
		curr.score = 1.0

		if len(curr.code) > len(originalCode)*2 {
			curr.score -= 0.3
		}

		if bestCandidate == nil || curr.score > bestCandidate.score || (curr.score == bestCandidate.score && curr.id < bestCandidate.id) {
			bestCandidate = &curr
		}
	}

	if bestCandidate == nil {
		return nil, fmt.Errorf("all candidates failed validation")
	}

	return bestCandidate, nil
}

// Index parses and persists a file. (Internal helper for HealerEngine)
func (e *HealerEngine) Index(ctx context.Context, path string) error {
	itPointers, itCalls, _, err := StreamSymbols(ctx, path)
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
			Mtime:   int(mtime),
			Hash:    hash,
			AstJson: "{}",
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
	RiskScore          float64
	BlastRadius        int
	SimilarPatternPath string
}

// DiagnoseHUD ejecuta la visión de diagnóstico combinando Git, AST, Impacto y Bleve.
// Retorna la representación del HUD en formato ZON.
func (e *HealerEngine) DiagnoseHUD(ctx context.Context, errorLog string) (*DiagnosticHUD, error) {
	if errorLog == "" {
		return nil, fmt.Errorf("empty error log")
	}

	// 1. Parseo: Extraer archivo y línea
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return nil, fmt.Errorf("could not identify failing file:line in log")
	}

	primaryMatch := allMatches[0]
	failingFileRaw := primaryMatch[1]
	lineNum, _ := strconv.Atoi(primaryMatch[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return nil, err
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
	itPointers, _, _, err := StreamSymbols(ctx, failingFile)
	if err != nil {
		return nil, err
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
	riskScore := 0.0
	blastRadius := 0
	if symbol != "Unknown" {
		impact, err := e.impact.Analyze(ctx, symbol, failingFile, 1)
		if err == nil && impact != nil {
			riskLevel = impact.RiskLevel
			riskScore = impact.Target.RiskScore
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
		RiskScore:          riskScore,
		BlastRadius:        blastRadius,
		SimilarPatternPath: similarPath,
	}

	return &hud, nil
}
