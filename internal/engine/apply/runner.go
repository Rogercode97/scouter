package apply

import (
	"errors"
	"time"
)

// Runner executes a list of steps for a given stage.
type Runner struct {
	FailurePolicy FailurePolicy
	OnProgress    ProgressFunc
	Interrupt     InterruptHook
}

func (r Runner) Run(stage Stage, steps []Step) StageResult {
	result := StageResult{Stage: stage, Success: true, Steps: make([]StepResult, 0, len(steps))}
	var errs []error
	completed := 0

	for _, step := range steps {
		r.emitProgress(ProgressEvent{StepID: step.ID(), Stage: stage, Status: StepStatusRunning})

		started := time.Now().UTC()
		err := step.Run()
		finished := time.Now().UTC()

		stepResult := StepResult{
			StepID:     step.ID(),
			StartedAt:  started,
			FinishedAt: finished,
		}

		if err != nil {
			stepResult.Status = StepStatusFailed
			stepResult.Err = err
			result.Steps = append(result.Steps, stepResult)

			r.emitProgress(ProgressEvent{StepID: step.ID(), Stage: stage, Status: StepStatusFailed, Err: err})

			errs = append(errs, err)
			result.Success = false

			if r.FailurePolicy == StopOnError {
				result.Err = err
				return result
			}

			continue
		}

		stepResult.Status = StepStatusSucceeded
		result.Steps = append(result.Steps, stepResult)
		r.emitProgress(ProgressEvent{StepID: step.ID(), Stage: stage, Status: StepStatusSucceeded})

		completed++
		if r.Interrupt != nil {
			if err := r.Interrupt(stage, completed); err != nil {
				result.Success = false
				// The interruption stops the stage, but a failure recorded before
				// it must survive: joining keeps both, so the caller learns that a
				// step failed and the rollback decision sees it. Reporting only the
				// interruption would present a partially applied execution as a
				// clean pause.
				result.Err = errors.Join(append(errs, err)...)
				return result
			}
		}
	}

	if len(errs) > 0 {
		result.Err = errors.Join(errs...)
	}

	return result
}

func (r Runner) emitProgress(event ProgressEvent) {
	if r.OnProgress != nil {
		r.OnProgress(event)
	}
}
