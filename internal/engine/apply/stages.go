package apply

import "errors"

// ErrInterrupted marks an execution that was stopped at a step boundary on
// purpose. The steps that already succeeded stay applied, so the caller owns
// the recovery decision instead of the package guessing it.
var ErrInterrupted = errors.New("apply: interrupted")

// InterruptHook is invoked after each successful step with the number of
// steps completed in that stage. Returning an error stops the execution at
// that boundary. It is the deterministic seam that makes an interrupted commit
// testable without killing a process.
type InterruptHook func(stage Stage, completedInStage int) error

type Stage string

const (
	StagePrepare  Stage = "prepare"
	StageApply    Stage = "apply"
	StageRollback Stage = "rollback"
)

type Step interface {
	ID() string
	Run() error
}

type RollbackStep interface {
	Step
	Rollback() error
}

// FailurePolicy controls how the runner behaves when a step fails.
type FailurePolicy int

const (
	// StopOnError stops execution at the first failed step (default).
	StopOnError FailurePolicy = iota
	// ContinueOnError continues executing remaining steps, collecting all errors.
	ContinueOnError
)

// ProgressEvent is emitted by the runner as each step starts and completes.
type ProgressEvent struct {
	StepID string
	Stage  Stage
	Status StepStatus
	Err    error
}

// ProgressFunc is a callback invoked for every step lifecycle event.
type ProgressFunc func(ProgressEvent)

type StagePlan struct {
	Prepare []Step
	Apply   []Step
}
