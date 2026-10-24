package apply

import (
	"errors"
	"testing"
)

type interruptibleStep struct {
	id  string
	ran bool
}

func (s *interruptibleStep) ID() string { return s.id }

func (s *interruptibleStep) Run() error {
	s.ran = true
	return nil
}

// TestRunner_InterruptStopsAtTheBoundary proves the deterministic seam stops
// the execution exactly where it is asked to, and that the steps that already
// succeeded stay recorded as succeeded: they were applied, and the caller
// needs to know it.
func TestRunner_InterruptStopsAtTheBoundary(t *testing.T) {
	first := &interruptibleStep{id: "first"}
	second := &interruptibleStep{id: "second"}

	runner := Runner{
		Interrupt: func(stage Stage, completed int) error {
			if completed == 1 {
				return ErrInterrupted
			}
			return nil
		},
	}

	result := runner.Run(StageApply, []Step{first, second})

	if result.Success {
		t.Fatal("expected the stage to fail with the interruption")
	}
	if !errors.Is(result.Err, ErrInterrupted) {
		t.Fatalf("stage error = %v, want ErrInterrupted", result.Err)
	}
	if second.ran {
		t.Fatal("the second step must not run after the interruption")
	}
	if len(result.Steps) != 1 {
		t.Fatalf("recorded steps = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].Status != StepStatusSucceeded {
		t.Fatalf("first step status = %q, want succeeded", result.Steps[0].Status)
	}
}

// TestRollbackPolicy_KeepsInterruptedState proves an interruption is not
// treated as a failure: rolling back here would hide the interruption
// instead of letting the caller resolve it.
func TestRollbackPolicy_KeepsInterruptedState(t *testing.T) {
	policy := DefaultRollbackPolicy()

	if policy.ShouldRollback(StageApply, ErrInterrupted) {
		t.Fatal("an interrupted execution must not be rolled back")
	}
	if !policy.ShouldRollback(StageApply, errors.New("step failed")) {
		t.Fatal("a failed execution must still be rolled back")
	}
}
