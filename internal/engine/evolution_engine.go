package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/apply"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/pmezard/go-difflib/difflib"
)

type EvolutionEngine struct {
	ledger *Ledger
	ripple *RippleEngine
	store  store.Store
}

func NewEvolutionEngine(store store.Store, ledger *Ledger, ripple *RippleEngine) *EvolutionEngine {
	return &EvolutionEngine{
		store:  store,
		ledger: ledger,
		ripple: ripple,
	}
}

func (e *EvolutionEngine) ProposeEvolution(ctx context.Context, proposal string, force bool, messenger Messenger) (string, error) {
	// 1. Sampling: Request Genome Mutation via Messenger
	// The prompt should be provided by the caller or we can pass a generic one
	systemPrompt := "You are a genome evolution agent. Produce JSON mutations based on the proposal. Format: [{\"file\": \"...\", \"content\": \"...\"}]"

	txt, err := messenger.Ask(ctx, systemPrompt, proposal)
	if err != nil {
		return "", fmt.Errorf("sampling evolution failed: %w", err)
	}

	// 2. Parse JSON Mutations
	var mutations []struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}
	rawJSON := utils.ExtractJSON(txt)
	if err := json.Unmarshal([]byte(rawJSON), &mutations); err != nil {
		return "", fmt.Errorf("failed to parse mutation JSON: %w\nRaw: %s", err, txt)
	}

	// 3. Stage in Ledger
	stagedCount := 0
	for _, m := range mutations {
		if !force && strings.Contains(m.File, "internal/mcp/handlers.go") {
			return "", fmt.Errorf("SOVEREIGNTY VIOLATION: Mutation attempts to modify GEP core logic in '%s'. Use 'force:true' if this is an intended self-lobotomy.", m.File)
		}

		if err := e.StageMutation(ctx, m.File, m.Content); err != nil {
			return "", fmt.Errorf("failed to stage mutation for %s: %w", m.File, err)
		}
		stagedCount++
	}

	return fmt.Sprintf("✅ Evolution staged in Ledger for %d files. Use 'scouter_diff' to review and 'scouter_commit' to apply changes.", stagedCount), nil
}

func (e *EvolutionEngine) Propagate(ctx context.Context, symbol, transformation string, messenger Messenger) (string, error) {
	if messenger != nil {
		e.ripple.Transformer = NewMCPTransformer(e.store, func(ctx context.Context, file, sym, prompt string) (string, error) {
			return messenger.Ask(ctx, "You are a surgical refactoring agent.", prompt)
		})
	}
	ledger, err := e.ripple.Propagate(ctx, symbol, transformation, 5)
	if err != nil {
		if ledger != nil && len(ledger.StagedFiles()) > 0 {
			return fmt.Sprintf("❌ Validation failed: %v. Staged files: %v", err, ledger.StagedFiles()), err
		}
		return "", err
	}

	return fmt.Sprintf("✅ Transformation staged in Ledger for %d files: %v. Use 'scouter_commit' to apply or 'scouter_diff' to review.", len(ledger.AffectedFiles()), ledger.AffectedFiles()), nil
}

func (e *EvolutionEngine) CommitLedger(ctx context.Context) (string, error) {
	if e.ledger == nil {
		return "", fmt.Errorf("ledger not initialized")
	}

	files := e.ledger.StagedFiles()
	if len(files) == 0 {
		return "No changes staged in Ledger.", nil
	}

	plan := apply.StagePlan{
		Prepare: []apply.Step{},
		Apply:   []apply.Step{},
	}

	for _, patch := range e.ledger.GetStaged() {
		p := patch // copy
		plan.Prepare = append(plan.Prepare, &preparePatchStep{path: p.FilePath, patch: p})
		plan.Apply = append(plan.Apply, &applyPatchStep{path: p.FilePath, original: p.Original})
	}

	orch := apply.NewOrchestrator(apply.DefaultRollbackPolicy())
	res := orch.Execute(plan)
	if res.Err != nil {
		return "", fmt.Errorf("commit failed: %v", res.Err)
	}

	e.ledger.Rollback(ctx) // clear ledger after successful commit

	return fmt.Sprintf("✅ Committed changes to %d files: %v", len(files), files), nil
}

type preparePatchStep struct {
	path  string
	patch Patch
}

func (p *preparePatchStep) ID() string { return p.path + "_prepare" }
func (p *preparePatchStep) Run() error {
	if err := os.MkdirAll(filepath.Dir(p.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(p.path+".scouter.tmp", []byte(p.patch.NewContent), 0644)
}

type applyPatchStep struct {
	path     string
	original string
}

func (a *applyPatchStep) ID() string { return a.path + "_apply" }
func (a *applyPatchStep) Run() error {
	return os.Rename(a.path+".scouter.tmp", a.path)
}
func (a *applyPatchStep) Rollback() error {
	if a.original == "" {
		return os.Remove(a.path)
	}
	return os.WriteFile(a.path, []byte(a.original), 0644)
}

func (e *EvolutionEngine) RollbackLedger(ctx context.Context) (string, error) {
	if e.ledger == nil {
		return "", fmt.Errorf("ledger not initialized")
	}

	if err := e.ledger.Rollback(ctx); err != nil {
		return "", err
	}

	return "✅ Ledger rolled back. All staged changes cleared.", nil
}

func (e *EvolutionEngine) GetLedgerSummary(ctx context.Context) string {
	if e.ledger == nil {
		return "Ledger not initialized."
	}
	return e.ledger.Summary()
}

func (e *EvolutionEngine) GetLedgerDiff(ctx context.Context) (string, error) {
	if e.ledger == nil {
		return "", fmt.Errorf("ledger not initialized")
	}

	patches := e.ledger.GetStaged()
	if len(patches) == 0 {
		return "No changes staged in Ledger.", nil
	}

	var sb strings.Builder
	for _, p := range patches {
		diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(p.Original),
			B:        difflib.SplitLines(p.NewContent),
			FromFile: p.FilePath + " (Original)",
			ToFile:   p.FilePath + " (Staged)",
			Context:  3,
		})
		if err != nil {
			sb.WriteString(fmt.Sprintf("--- %s (Original)\n+++ %s (Staged)\n", p.FilePath, p.FilePath))
			if p.Diff != "" {
				sb.WriteString(p.Diff)
			} else {
				sb.WriteString(" (Diff not available, full content staged)\n")
			}
			sb.WriteString("\n")
			continue
		}

		if diff == "" {
			sb.WriteString(fmt.Sprintf("--- %s (Original)\n+++ %s (Staged)\n (No changes)\n\n", p.FilePath, p.FilePath))
		} else {
			sb.WriteString(diff)
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func (e *EvolutionEngine) StageMutation(ctx context.Context, filePath, newContent string) error {
	if e.ledger == nil {
		return fmt.Errorf("ledger not initialized")
	}

	cleanPath, err := utils.ValidatePath(filePath)
	if err != nil {
		return err
	}

	original, _ := os.ReadFile(cleanPath)

	patch := Patch{
		FilePath:   cleanPath,
		Original:   string(original),
		NewContent: newContent,
	}

	return e.ledger.Stage(cleanPath, patch)
}
