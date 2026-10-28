package apply

import (
	"errors"
	"testing"
)

// failingStep is a step that records that it ran and returns a fixed error.
type failingStep struct {
	id  string
	err error
	ran bool
}

func (s *failingStep) ID() string { return s.id }

func (s *failingStep) Run() error {
	s.ran = true
	return s.err
}

// TestRunner_InterruptKeepsTheEarlierFailure covers the case where a stage both
// fails and is interrupted.
//
// With ContinueOnError a step can fail, a later step can succeed, and only then
// does the interruption hook fire. The interruption must stop the stage without
// erasing the failure that came before it: the caller has to learn that a step
// failed, and the rollback decision has to see it. Reporting only the
// interruption turns a partially applied execution into a clean-looking pause.
func TestRunner_InterruptKeepsTheEarlierFailure(t *testing.T) {
	failure := errors.New("first step failed")
	first := &failingStep{id: "first", err: failure}
	second := &interruptibleStep{id: "second"}

	runner := Runner{
		FailurePolicy: ContinueOnError,
		Interrupt: func(stage Stage, completed int) error {
			if completed == 1 {
				return ErrInterrupted
			}
			return nil
		},
	}

	result := runner.Run(StageApply, []Step{first, second})

	if !second.ran {
		t.Fatal("ContinueOnError must still run the second step")
	}
	if !errors.Is(result.Err, ErrInterrupted) {
		t.Fatalf("stage error = %v, want it to carry ErrInterrupted", result.Err)
	}
	if !errors.Is(result.Err, failure) {
		t.Fatalf("stage error = %v, want it to carry the earlier failure %v", result.Err, failure)
	}
	if !DefaultRollbackPolicy().ShouldRollback(StageApply, result.Err) {
		t.Fatal("a stage that both failed and was interrupted must be rolled back, " +
			"otherwise the failure never reaches the rollback decision")
	}
}

// TestRollbackPolicy_PureInterruptionStaysUnrolled keeps the other side of the
// rule: an interruption that carries no failure is still not a reason to roll
// back, whether it arrives bare or wrapped by errors.Join.
func TestRollbackPolicy_PureInterruptionStaysUnrolled(t *testing.T) {
	policy := DefaultRollbackPolicy()

	cases := map[string]error{
		"bare interruption":         ErrInterrupted,
		"joined interruption":       errors.Join(ErrInterrupted),
		"joined interruption twice": errors.Join(ErrInterrupted, ErrInterrupted),
	}

	for name, err := range cases {
		if policy.ShouldRollback(StageApply, err) {
			t.Fatalf("%s: an interruption with no failure must not be rolled back", name)
		}
	}

	if !policy.ShouldRollback(StageApply, errors.Join(ErrInterrupted, errors.New("real failure"))) {
		t.Fatal("an interruption that also carries a failure must be rolled back")
	}
}
