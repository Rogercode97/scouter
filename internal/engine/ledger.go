package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Rogercode97/scouter/internal/engine/apply"
)

// Patch represents a pending file modification.
type Patch struct {
	FilePath   string      `json:"file_path"`
	Original   string      `json:"original,omitempty"`
	NewContent string      `json:"new_content"`
	Diff       string      `json:"diff,omitempty"`
	Mode       os.FileMode `json:"mode,omitempty"`
	IsNew      bool        `json:"is_new,omitempty"`
}

// MissionStats tracks the resource consumption of the current operation.
type MissionStats struct {
	StartTime  time.Time `json:"start_time"`
	TotalKi    int64     `json:"total_ki"`   // Token usage (approximated)
	TurnCount  int       `json:"turn_count"` // Number of iterations
	FilesCount int       `json:"files_count"`
}

// Ledger holds staged changes and mission metrics.
type Ledger struct {
	mu         sync.RWMutex
	Staged     map[string]Patch `json:"staged"`
	Stats      MissionStats     `json:"stats"`
	KiLimit    int64            `json:"ki_limit"`
	TurnLimit  int              `json:"turn_limit"`
	Project    string           `json:"project"`
	ledgerPath string
}

func NewLedger() *Ledger {
	l := &Ledger{
		Staged: make(map[string]Patch),
		Stats: MissionStats{
			StartTime: time.Now(),
		},
		KiLimit:    100000, // Default 100k Ki
		TurnLimit:  10,     // Default 10 turns
		ledgerPath: ".scouter/staging/ledger.json",
	}

	return l
}

// SetLedgerPath updates the persistence path and attempts to load it.
func (l *Ledger) SetLedgerPath(path string) {
	l.mu.Lock()
	l.ledgerPath = path
	l.mu.Unlock()

	if _, err := os.Stat(path); err == nil {
		if err := l.Load(); err != nil {
			slog.Error("failed to load ledger", "path", path, "error", err)
		}
	}
}

// Save persists the current ledger state to disk.
func (l *Ledger) Save() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	dir := filepath.Dir(l.ledgerPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(l.ledgerPath, data, 0644)
}

// Load restores the ledger state from disk.
func (l *Ledger) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, l)
}

// SetBudget sets the limits for the current mission.
func (l *Ledger) SetBudget(kiLimit int64, turnLimit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.KiLimit = kiLimit
	l.TurnLimit = turnLimit
}

// Stage adds a patch to the ledger and saves to disk.
func (l *Ledger) Stage(path string, patch Patch) error {
	if patch.FilePath == "" {
		patch.FilePath = path
	}

	if info, err := os.Stat(path); err == nil {
		patch.IsNew = false
		if patch.Original == "" && !info.IsDir() {
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				if !errors.Is(rerr, os.ErrNotExist) && !os.IsNotExist(rerr) {
					return rerr
				}
			} else {
				patch.Original = string(data)
			}
		}
		if patch.Mode == 0 {
			patch.Mode = info.Mode()
		}
	} else if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
		patch.IsNew = true
	} else {
		return err
	}

	l.mu.Lock()

	if l.Stats.TurnCount > l.TurnLimit && l.TurnLimit > 0 {
		l.mu.Unlock()
		return fmt.Errorf("mission budget exceeded: max turns reached (%d)", l.TurnLimit)
	}

	kiIncurred := int64(len(patch.NewContent) / 4)
	if l.Stats.TotalKi+kiIncurred > l.KiLimit && l.KiLimit > 0 {
		l.mu.Unlock()
		return fmt.Errorf("mission budget exceeded: Ki limit reached (%d/%d)", l.Stats.TotalKi+kiIncurred, l.KiLimit)
	}

	l.Staged[path] = patch
	l.Stats.TotalKi += kiIncurred
	l.Stats.FilesCount = len(l.Staged)
	l.mu.Unlock()

	return l.Save()
}

// IncrementTurn advances the mission turn counter.
func (l *Ledger) IncrementTurn() {
	l.mu.Lock()
	l.Stats.TurnCount++
	l.mu.Unlock()
	if err := l.Save(); err != nil {
		slog.Error("failed to save ledger after turn increment", "error", err)
	}
}

// Unstage removes a patch from the ledger.
func (l *Ledger) Unstage(path string) {
	l.mu.Lock()
	delete(l.Staged, path)
	l.Stats.FilesCount = len(l.Staged)
	l.mu.Unlock()
	if err := l.Save(); err != nil {
		slog.Error("failed to save ledger after unstage", "error", err)
	}
}

// GetStaged returns all pending patches.
func (l *Ledger) GetStaged() []Patch {
	l.mu.RLock()
	defer l.mu.RUnlock()

	patches := make([]Patch, 0, len(l.Staged))
	for _, p := range l.Staged {
		patches = append(patches, p)
	}
	return patches
}

// StagedFiles returns the list of paths in the ledger.
func (l *Ledger) StagedFiles() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	files := make([]string, 0, len(l.Staged))
	for path := range l.Staged {
		files = append(files, path)
	}
	return files
}

func (l *Ledger) buildCommitPlan() apply.StagePlan {
	plan := apply.StagePlan{
		Prepare: make([]apply.Step, 0, len(l.Staged)),
		Apply:   make([]apply.Step, 0, len(l.Staged)),
	}

	paths := make([]string, 0, len(l.Staged))
	for path := range l.Staged {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		patch := l.Staged[path]
		plan.Prepare = append(plan.Prepare, &preparePatchStep{path: patch.FilePath, patch: patch})
		plan.Apply = append(plan.Apply, &applyPatchStep{
			path:     patch.FilePath,
			original: patch.Original,
			mode:     patch.Mode,
			isNew:    patch.IsNew,
		})
	}
	return plan
}

// CommitStaged applies all staged changes to the filesystem atomically using a two-phase commit.
func (l *Ledger) CommitStaged(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.Staged) == 0 {
		return nil
	}

	plan := l.buildCommitPlan()

	orch := apply.NewOrchestrator(apply.DefaultRollbackPolicy())
	res := orch.Execute(plan)
	if res.Err != nil {
		return fmt.Errorf("commit failed: %w", res.Err)
	}

	// Clear ledger and remove file after successful commit
	l.Staged = make(map[string]Patch)
	l.Stats.FilesCount = 0
	if err := os.Remove(l.ledgerPath); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove ledger file after commit", "path", l.ledgerPath, "error", err)
	}
	return nil
}

// Rollback clears all staged changes and removes the ledger file.
func (l *Ledger) Rollback(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Staged = make(map[string]Patch)
	l.Stats.FilesCount = 0
	if err := os.Remove(l.ledgerPath); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove ledger file after rollback", "path", l.ledgerPath, "error", err)
	}
	return nil
}

// AffectedFiles is an alias for StagedFiles.
func (l *Ledger) AffectedFiles() []string {
	return l.StagedFiles()
}

// Summary returns a human-readable mission report.
func (l *Ledger) Summary() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	duration := time.Since(l.Stats.StartTime)
	return fmt.Sprintf("🚀 Mission Summary:\n- Status: Staged\n- Files: %d\n- Budget: %d/%d Ki\n- Turns: %d/%d\n- Duration: %v",
		l.Stats.FilesCount, l.Stats.TotalKi, l.KiLimit, l.Stats.TurnCount, l.TurnLimit, duration)
}
