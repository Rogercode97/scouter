package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine/apply"
)

// interruptedState builds the state an interrupted commit leaves on disk: the
// journal is in the committing phase, one file was already replaced, the
// other was not.
func interruptedState(t *testing.T, dir string, applied, pending []string, contents map[string][2]string) *Ledger {
	t.Helper()

	ledgerPath := filepath.Join(dir, "ledger.json")
	l := NewLedger()
	l.SetLedgerPath(ledgerPath)

	for path, pair := range contents {
		original, updated := pair[0], pair[1]
		if err := os.WriteFile(path, []byte(original), 0644); err != nil {
			t.Fatalf("write original %q: %v", path, err)
		}
		if err := l.Stage(path, Patch{FilePath: path, NewContent: updated}); err != nil {
			t.Fatalf("stage %q: %v", path, err)
		}
	}

	// The commit replaced the files it managed to reach before it died.
	for _, path := range applied {
		if err := os.WriteFile(path, []byte(contents[path][1]), 0644); err != nil {
			t.Fatalf("write applied %q: %v", path, err)
		}
	}

	l.Phase = PhaseCommitting
	l.Applied = applied
	l.Pending = pending
	if err := l.Save(); err != nil {
		t.Fatalf("save journal: %v", err)
	}

	return l
}

// TestLedger_RollbackRestoresAfterInterruptedCommit pins the crash-safety
// property of the Staging Ledger: when a commit is interrupted after a file
// was already replaced, the rollback path must restore the original content
// of the files that were already written.
func TestLedger_RollbackRestoresAfterInterruptedCommit(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	original := "original content\n"
	updated := "updated content\n"

	l := interruptedState(t, tmpDir, []string{target}, nil, map[string][2]string{
		target: {original, updated},
	})

	if err := l.Rollback(context.Background()); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != original {
		t.Fatalf("target content = %q, want the original %q", string(data), original)
	}

	if _, err := os.Stat(l.ledgerPath); !os.IsNotExist(err) {
		t.Fatalf("journal must be removed after the rollback, stat error = %v", err)
	}
}

// TestLedger_InterruptedCommitIsReported proves the interrupted state is
// visible instead of silent.
func TestLedger_InterruptedCommitIsReported(t *testing.T) {
	tmpDir := t.TempDir()
	first := filepath.Join(tmpDir, "a.txt")
	second := filepath.Join(tmpDir, "b.txt")

	l := interruptedState(t, tmpDir, []string{first}, []string{second}, map[string][2]string{
		first:  {"a original\n", "a updated\n"},
		second: {"b original\n", "b updated\n"},
	})

	if !l.HasInterruptedCommit() {
		t.Fatal("expected the ledger to report an interrupted commit")
	}

	applied := l.InterruptedAppliedFiles()
	if len(applied) != 1 || applied[0] != first {
		t.Fatalf("interrupted applied files = %v, want [%s]", applied, first)
	}

	pending := l.InterruptedPendingFiles()
	if len(pending) != 1 || pending[0] != second {
		t.Fatalf("interrupted pending files = %v, want [%s]", pending, second)
	}
}

// TestLedger_CommitRefusesWhileInterrupted proves the commit path fails
// closed: it never applies a new plan on top of an unresolved tree.
func TestLedger_CommitRefusesWhileInterrupted(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")

	l := interruptedState(t, tmpDir, []string{target}, nil, map[string][2]string{
		target: {"original\n", "updated\n"},
	})

	err := l.CommitStaged(context.Background())
	if !errors.Is(err, ErrInterruptedCommit) {
		t.Fatalf("CommitStaged error = %v, want ErrInterruptedCommit", err)
	}
}

// TestLedger_RollForwardCompletesInterruptedCommit proves the other
// resolution: the files the commit did not reach are written.
func TestLedger_RollForwardCompletesInterruptedCommit(t *testing.T) {
	tmpDir := t.TempDir()
	first := filepath.Join(tmpDir, "a.txt")
	second := filepath.Join(tmpDir, "b.txt")

	l := interruptedState(t, tmpDir, []string{first}, []string{second}, map[string][2]string{
		first:  {"a original\n", "a updated\n"},
		second: {"b original\n", "b updated\n"},
	})

	if err := l.RollForward(context.Background()); err != nil {
		t.Fatalf("roll forward: %v", err)
	}

	firstData, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	if string(firstData) != "a updated\n" {
		t.Fatalf("first content = %q, want the applied content", string(firstData))
	}

	secondData, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if string(secondData) != "b updated\n" {
		t.Fatalf("second content = %q, want the rolled forward content", string(secondData))
	}

	if _, err := os.Stat(l.ledgerPath); !os.IsNotExist(err) {
		t.Fatalf("journal must be removed after the roll forward, stat error = %v", err)
	}
}

// TestLedger_RollbackRefusesWhenFileChangedAfterInterruption proves the
// rollback never destroys work the journal does not know about.
func TestLedger_RollbackRefusesWhenFileChangedAfterInterruption(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")

	l := interruptedState(t, tmpDir, []string{target}, nil, map[string][2]string{
		target: {"original\n", "updated\n"},
	})

	// Somebody edited the file after the interruption.
	edited := "edited by hand\n"
	if err := os.WriteFile(target, []byte(edited), 0644); err != nil {
		t.Fatalf("write edited: %v", err)
	}

	if err := l.Rollback(context.Background()); err == nil {
		t.Fatal("expected the rollback to refuse a file that changed after the interruption")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != edited {
		t.Fatalf("target content = %q, want the hand edit to be preserved", string(data))
	}
}

// TestLedger_InterruptedCommitRecoversToAllOldOrAllNew is the crash-consistency
// property: whatever the interruption point, after the recovery the tree is
// entirely in the old state or entirely in the new one, never in between.
func TestLedger_InterruptedCommitRecoversToAllOldOrAllNew(t *testing.T) {
	t.Run("rollback after one file was replaced", func(t *testing.T) {
		tmpDir := t.TempDir()
		first := filepath.Join(tmpDir, "a.txt")
		second := filepath.Join(tmpDir, "b.txt")

		for path, content := range map[string]string{first: "a original\n", second: "b original\n"} {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("write %q: %v", path, err)
			}
		}

		l := NewLedger()
		l.SetLedgerPath(filepath.Join(tmpDir, "ledger.json"))
		if err := l.Stage(first, Patch{FilePath: first, NewContent: "a updated\n"}); err != nil {
			t.Fatalf("stage first: %v", err)
		}
		if err := l.Stage(second, Patch{FilePath: second, NewContent: "b updated\n"}); err != nil {
			t.Fatalf("stage second: %v", err)
		}

		// Stop the commit after the first file was replaced.
		l.interruptHook = func(stage apply.Stage, completed int) error {
			if stage == apply.StageApply && completed == 1 {
				return apply.ErrInterrupted
			}
			return nil
		}

		err := l.CommitStaged(context.Background())
		if !errors.Is(err, apply.ErrInterrupted) {
			t.Fatalf("CommitStaged error = %v, want the interruption", err)
		}

		// The tree is half applied, and the journal says so.
		if !l.HasInterruptedCommit() {
			t.Fatal("expected the ledger to report an interrupted commit")
		}

		if err := l.Rollback(context.Background()); err != nil {
			t.Fatalf("rollback: %v", err)
		}

		for path, want := range map[string]string{first: "a original\n", second: "b original\n"} {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %q: %v", path, err)
			}
			if string(data) != want {
				t.Fatalf("%q content = %q, want the original %q", path, string(data), want)
			}
		}
	})

	t.Run("roll forward after every file was replaced", func(t *testing.T) {
		tmpDir := t.TempDir()
		first := filepath.Join(tmpDir, "a.txt")
		second := filepath.Join(tmpDir, "b.txt")

		for path, content := range map[string]string{first: "a original\n", second: "b original\n"} {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("write %q: %v", path, err)
			}
		}

		l := NewLedger()
		l.SetLedgerPath(filepath.Join(tmpDir, "ledger.json"))
		if err := l.Stage(first, Patch{FilePath: first, NewContent: "a updated\n"}); err != nil {
			t.Fatalf("stage first: %v", err)
		}
		if err := l.Stage(second, Patch{FilePath: second, NewContent: "b updated\n"}); err != nil {
			t.Fatalf("stage second: %v", err)
		}

		// Stop the commit after the last file was replaced, before the
		// journal was removed.
		l.interruptHook = func(stage apply.Stage, completed int) error {
			if stage == apply.StageApply && completed == 2 {
				return apply.ErrInterrupted
			}
			return nil
		}

		err := l.CommitStaged(context.Background())
		if !errors.Is(err, apply.ErrInterrupted) {
			t.Fatalf("CommitStaged error = %v, want the interruption", err)
		}

		if err := l.RollForward(context.Background()); err != nil {
			t.Fatalf("roll forward: %v", err)
		}

		for path, want := range map[string]string{first: "a updated\n", second: "b updated\n"} {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %q: %v", path, err)
			}
			if string(data) != want {
				t.Fatalf("%q content = %q, want the new content %q", path, string(data), want)
			}
		}
	})
}
