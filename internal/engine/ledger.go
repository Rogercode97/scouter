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

// CommitPhase is the state of the mission recorded in the journal.
type CommitPhase string

const (
	// PhaseStaged means the changes are staged and no commit was attempted.
	PhaseStaged CommitPhase = "staged"
	// PhaseCommitting means a commit started and did not reach its end. Only
	// the journal can describe the resulting tree.
	PhaseCommitting CommitPhase = "committing"
)

// ErrInterruptedCommit is returned when a commit is attempted while the
// journal still records an interrupted commit. The operator resolves it first,
// with Rollback or RollForward.
var ErrInterruptedCommit = errors.New("ledger: an interrupted commit must be resolved before committing again")

// Ledger holds staged changes and mission metrics.
type Ledger struct {
	mu         sync.RWMutex
	Staged     map[string]Patch `json:"staged"`
	Stats      MissionStats     `json:"stats"`
	KiLimit    int64            `json:"ki_limit"`
	TurnLimit  int              `json:"turn_limit"`
	Project    string           `json:"project"`
	Phase      CommitPhase      `json:"phase,omitempty"`
	Applied    []string         `json:"applied,omitempty"`
	Pending    []string         `json:"pending,omitempty"`
	ledgerPath string

	// interruptHook is the deterministic seam that lets a test stop a commit
	// at a step boundary without killing a process.
	interruptHook apply.InterruptHook
}

func NewLedger() *Ledger {
	l := &Ledger{
		Staged: make(map[string]Patch),
		Stats: MissionStats{
			StartTime: time.Now(),
		},
		KiLimit:    100000, // Default 100k Ki
		TurnLimit:  10,     // Default 10 turns
		Phase:      PhaseStaged,
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

	return l.saveUnlocked()
}

// saveUnlocked writes the journal without taking the lock. The commit path
// already holds the write lock, and sync.RWMutex is not reentrant.
//
// The journal is written atomically and durably, because it is the only
// record of a commit in progress: a torn journal would lose the recovery
// information exactly when it is needed.
func (l *Ledger) saveUnlocked() error {
	dir := filepath.Dir(l.ledgerPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}

	tmp := l.ledgerPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, l.ledgerPath); err != nil {
		return err
	}

	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}

	return nil
}

// Load restores the ledger state from disk.
func (l *Ledger) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, l); err != nil {
		return err
	}

	// A journal written before the phase existed means a staged mission.
	if l.Phase == "" {
		l.Phase = PhaseStaged
	}

	return nil
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

// CommitStaged applies all staged changes to the filesystem using a two-phase
// commit.
//
// Each file is replaced atomically, but the batch is not atomic across
// files: the files are applied one at a time, in sorted path order. The
// journal therefore records the commit before the filesystem is touched, and
// keeps the list of files already replaced, so an interrupted commit can be
// resolved afterwards with Rollback or RollForward.
func (l *Ledger) CommitStaged(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.Staged) == 0 {
		return nil
	}

	// An unresolved interrupted commit owns the tree: committing again would
	// apply a plan on top of a state nobody has resolved.
	if l.Phase == PhaseCommitting {
		return ErrInterruptedCommit
	}

	plan := l.buildCommitPlan()
	stepPaths := l.applyStepIndex()

	l.Phase = PhaseCommitting
	l.Applied = nil
	l.Pending = l.sortedStagedPaths()
	if err := l.saveUnlocked(); err != nil {
		l.Phase = PhaseStaged
		l.Pending = nil
		return fmt.Errorf("commit failed: cannot record the journal state: %w", err)
	}

	opts := []apply.OrchestratorOption{
		apply.WithProgressFunc(l.recordProgress(stepPaths)),
	}
	if l.interruptHook != nil {
		opts = append(opts, apply.WithInterruptHook(l.interruptHook))
	}

	orch := apply.NewOrchestrator(apply.DefaultRollbackPolicy(), opts...)
	res := orch.Execute(plan)
	if res.Err != nil {
		if errors.Is(res.Err, apply.ErrInterrupted) {
			// The tree may be half applied and only the journal describes it.
			_ = l.saveUnlocked()
			return fmt.Errorf("commit interrupted: %w", res.Err)
		}

		// The batch failed and the orchestrator rolled it back in process.
		l.Phase = PhaseStaged
		l.Applied = nil
		l.Pending = nil
		_ = l.saveUnlocked()
		return fmt.Errorf("commit failed: %w", res.Err)
	}

	// Clear ledger and remove file after successful commit
	l.Staged = make(map[string]Patch)
	l.Stats.FilesCount = 0
	l.Phase = PhaseStaged
	l.Applied = nil
	l.Pending = nil
	if err := os.Remove(l.ledgerPath); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove ledger file after commit", "path", l.ledgerPath, "error", err)
	}
	return nil
}

// Rollback clears the staged changes.
//
// When the journal records an interrupted commit, the files that commit
// already replaced are restored first, so the tree returns to the state it
// had before the commit. A file whose content changed after the interruption
// is never overwritten: the rollback fails instead of destroying work that
// the journal does not know about.
func (l *Ledger) Rollback(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Phase == PhaseCommitting {
		if err := l.restoreApplied(); err != nil {
			return err
		}
	}

	l.Staged = make(map[string]Patch)
	l.Stats.FilesCount = 0
	l.Phase = PhaseStaged
	l.Applied = nil
	l.Pending = nil
	if err := os.Remove(l.ledgerPath); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove ledger file after rollback", "path", l.ledgerPath, "error", err)
	}
	return nil
}

// RollForward completes an interrupted commit by applying the files it had
// not replaced yet. It is the explicit counterpart of Rollback: the operator
// decides which side of the interruption the tree ends on.
func (l *Ledger) RollForward(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.Phase != PhaseCommitting {
		return fmt.Errorf("ledger: no interrupted commit to roll forward")
	}

	pending := make([]string, 0, len(l.Pending))
	for _, path := range l.Pending {
		if _, ok := l.Staged[path]; ok {
			pending = append(pending, path)
		}
	}
	sort.Strings(pending)

	if len(pending) > 0 {
		plan := apply.StagePlan{
			Prepare: make([]apply.Step, 0, len(pending)),
			Apply:   make([]apply.Step, 0, len(pending)),
		}
		for _, path := range pending {
			patch := l.Staged[path]
			plan.Prepare = append(plan.Prepare, &preparePatchStep{path: patch.FilePath, patch: patch})
			plan.Apply = append(plan.Apply, &applyPatchStep{
				path:     patch.FilePath,
				original: patch.Original,
				mode:     patch.Mode,
				isNew:    patch.IsNew,
			})
		}

		orch := apply.NewOrchestrator(apply.DefaultRollbackPolicy())
		if res := orch.Execute(plan); res.Err != nil {
			// The journal stays: the commit is still interrupted and the
			// operator can roll it back instead.
			_ = l.saveUnlocked()
			return fmt.Errorf("roll forward failed: %w", res.Err)
		}
	}

	l.Staged = make(map[string]Patch)
	l.Stats.FilesCount = 0
	l.Phase = PhaseStaged
	l.Applied = nil
	l.Pending = nil
	if err := os.Remove(l.ledgerPath); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove ledger file after roll forward", "path", l.ledgerPath, "error", err)
	}
	return nil
}

// HasInterruptedCommit reports whether the journal records a commit that
// started and did not finish.
func (l *Ledger) HasInterruptedCommit() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.Phase == PhaseCommitting
}

// InterruptedAppliedFiles returns the files the interrupted commit had
// already replaced.
func (l *Ledger) InterruptedAppliedFiles() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return append([]string(nil), l.Applied...)
}

// InterruptedPendingFiles returns the files the interrupted commit had not
// replaced yet.
func (l *Ledger) InterruptedPendingFiles() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return append([]string(nil), l.Pending...)
}

// restoreApplied reverts the files an interrupted commit already replaced.
//
// The journal is the record of what the commit did, but it is written after
// each replacement, so a crash between the replacement and the journal write
// leaves a file that the journal still calls pending. A pending file whose
// content is exactly the staged content was therefore applied, and it is
// reverted as well.
func (l *Ledger) restoreApplied() error {
	applied := append([]string(nil), l.Applied...)
	for _, path := range l.Pending {
		patch, ok := l.Staged[path]
		if !ok {
			continue
		}
		current, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if string(current) == patch.NewContent {
			applied = append(applied, path)
		}
	}

	for _, path := range applied {
		patch, ok := l.Staged[path]
		if !ok {
			continue
		}

		current, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot roll back %q: %w", path, err)
		}
		if string(current) != patch.NewContent {
			return fmt.Errorf("cannot roll back %q: the file changed after the interrupted commit", path)
		}

		if patch.IsNew {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("cannot roll back %q: %w", path, err)
			}
			continue
		}

		mode := patch.Mode
		if mode == 0 {
			mode = 0644
		}
		if err := os.WriteFile(path, []byte(patch.Original), mode); err != nil {
			return fmt.Errorf("cannot roll back %q: %w", path, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("cannot roll back %q: %w", path, err)
		}
	}

	// The files that were never applied keep their temporary file.
	for _, path := range l.Pending {
		_ = os.Remove(path + ".scouter.tmp")
	}

	return nil
}

// recordProgress tracks which files the commit already replaced, so the
// journal describes an interrupted commit precisely.
func (l *Ledger) recordProgress(stepPaths map[string]string) apply.ProgressFunc {
	return func(ev apply.ProgressEvent) {
		if ev.Stage != apply.StageApply || ev.Status != apply.StepStatusSucceeded {
			return
		}

		path, ok := stepPaths[ev.StepID]
		if !ok {
			return
		}

		l.Applied = append(l.Applied, path)
		l.Pending = removePath(l.Pending, path)

		// The journal has to survive the crash that may follow, so the applied
		// set is persisted as it grows.
		_ = l.saveUnlocked()
	}
}

// applyStepIndex maps an apply step identifier back to the file it writes.
func (l *Ledger) applyStepIndex() map[string]string {
	idx := make(map[string]string, len(l.Staged))
	for path := range l.Staged {
		idx[applyStepID(path)] = path
	}
	return idx
}

// sortedStagedPaths returns the staged paths in the order the commit applies
// them.
func (l *Ledger) sortedStagedPaths() []string {
	paths := make([]string, 0, len(l.Staged))
	for path := range l.Staged {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func removePath(list []string, v string) []string {
	out := make([]string, 0, len(list))
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
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
