package engine

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/imports"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
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
	Diagnostic *DiagnosticEngine
}

func NewHealerEngine(s TransactionalStore, l *lsp.Manager, a *AnalysisEngine, i *ImpactEngine, search *SearchEngine, mem memory.MemoryProvider) *HealerEngine {
	he := &HealerEngine{
		store:    s,
		lspMgr:   l,
		analyzer: a,
		impact:   i,
		Search:   search,
		memory:   mem,
		Ledger:   NewLedger(),
	}
	he.Diagnostic = NewDiagnosticEngine(s, a, i, he, l, search)
	return he
}

// Fix attempts to repair a test failure using the Shinigami Protocol (Solver-Verifier).
func (e *HealerEngine) Fix(ctx context.Context, errorLog string) (*types.HealResult, error) {
	if errorLog == "" {
		return nil, fmt.Errorf("empty error log")
	}

	// 1. RCA: Extract File and Line
	failingFile, lineNum, allMatches, err := e.Diagnostic.extractRCA(errorLog)
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
	prompt := e.Diagnostic.buildHealerContext(ctx, target, failingFile, errorLog, allMatches, originalCode)

	// 4. Parallel Solvers (Shinigami Phase 1 & 2)
	diag := &DiagnosticContext{
		FailingFile:  failingFile,
		ErrorLog:     errorLog,
		OriginalCode: originalCode,
		Prompt:       prompt,
		Target:       target,
	}

	pipeline := NewShinigamiPipeline(
		&LLMSolver{DoFixRequest: e.DoFixRequest},
		&LSPVerifier{},
	)

	bestCandidate, err := pipeline.Run(ctx, diag)
	if err != nil {
		return nil, err
	}

	// 5. Stage in Ledger (Wave 12.0 Mandate)
	fullContent, _ := os.ReadFile(failingFile)
	err = e.Ledger.Stage(failingFile, Patch{
		FilePath:   failingFile,
		Original:   string(fullContent),
		NewContent: bestCandidate.FullCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to stage fix: %w", err)
	}

	e.recordInoculation(ctx, target, failingFile, errorLog)

	return &types.HealResult{
		Status:    "STAGED",
		FixedCode: bestCandidate.Code,
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

type LLMSolver struct {
	DoFixRequest func(ctx context.Context, prompt string) (string, error)
}

func (s *LLMSolver) Generate(ctx context.Context, diag *DiagnosticContext) (<-chan FixCandidate, error) {
	candidates := make(chan FixCandidate, 3)

	go func() {
		defer close(candidates)
		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			id := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				resRaw, err := resile.Do(ctx, func(c context.Context) (string, error) {
					return s.DoFixRequest(c, diag.Prompt+"\n\nProvide solution variant #"+strconv.Itoa(id))
				}, resile.WithRetry(3))

				if err != nil {
					return
				}

				candidateCode := utils.ExtractCodeBlock(resRaw)
				
				select {
				case <-ctx.Done():
				case candidates <- FixCandidate{
					ID:   id,
					Code: candidateCode,
				}:
				}
			}()
		}
		wg.Wait()
	}()

	return candidates, nil
}

type LSPVerifier struct{}

func (v *LSPVerifier) Evaluate(ctx context.Context, diag *DiagnosticContext, c *FixCandidate) (bool, error) {
	fullContent, err := os.ReadFile(diag.FailingFile)
	if err != nil {
		return false, err
	}

	newContent := string(fullContent[:diag.Target.Range.Start]) + c.Code + string(fullContent[diag.Target.Range.End:])

	processedContent, err := imports.Process(diag.FailingFile, []byte(newContent), nil)
	if err != nil {
		processedContent = []byte(newContent)
	}

	c.FullCode = string(processedContent)

	tempLedger := NewLedger()
	_ = tempLedger.Stage(diag.FailingFile, Patch{
		FilePath:   diag.FailingFile,
		Original:   string(fullContent),
		NewContent: c.FullCode,
	})

	validator := NewLSPValidator("")
	valRes, err := validator.Validate(ctx, tempLedger)
	if err != nil {
		return false, err
	}

	c.Valid = valRes.Valid
	return c.Valid, nil
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
