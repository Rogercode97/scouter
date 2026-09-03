package apply

import "errors"

// OrchestratorOption configures the orchestrator.
type OrchestratorOption func(*Orchestrator)

// WithFailurePolicy sets the failure policy for the apply stage runner.
func WithFailurePolicy(policy FailurePolicy) OrchestratorOption {
	return func(o *Orchestrator) {
		o.runner.FailurePolicy = policy
	}
}

// WithProgressFunc sets a callback that receives progress events during execution.
func WithProgressFunc(fn ProgressFunc) OrchestratorOption {
	return func(o *Orchestrator) {
		o.runner.OnProgress = fn
	}
}

type Orchestrator struct {
	runner   Runner
	policy   RollbackPolicy
	stepByID map[string]Step
}

func NewOrchestrator(policy RollbackPolicy, opts ...OrchestratorOption) *Orchestrator {
	o := &Orchestrator{
		runner:   Runner{},
		policy:   policy,
		stepByID: map[string]Step{},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (o *Orchestrator) Execute(plan StagePlan) ExecutionResult {
	o.indexSteps(plan.Prepare)
	o.indexSteps(plan.Apply)

	prepareResult := o.runner.Run(StagePrepare, plan.Prepare)
	if !prepareResult.Success {
		result := ExecutionResult{Prepare: prepareResult, Err: prepareResult.Err}
		if o.policy.ShouldRollback(StagePrepare, prepareResult.Err) {
			result.Rollback = ExecuteRollback(prepareResult.Steps, o.stepByID)
			if !result.Rollback.Success {
				result.Err = errors.Join(prepareResult.Err, result.Rollback.Err)
			}
		}
		return result
	}

	applyResult := o.runner.Run(StageApply, plan.Apply)
	result := ExecutionResult{Prepare: prepareResult, Apply: applyResult}
	if applyResult.Success {
		return result
	}

	result.Err = applyResult.Err
	if o.policy.ShouldRollback(StageApply, applyResult.Err) {
		allSteps := append(prepareResult.Steps, applyResult.Steps...)
		result.Rollback = ExecuteRollback(allSteps, o.stepByID)
		if !result.Rollback.Success {
			result.Err = errors.Join(applyResult.Err, result.Rollback.Err)
		}
	}

	return result
}

func (o *Orchestrator) indexSteps(steps []Step) {
	for _, step := range steps {
		o.stepByID[step.ID()] = step
	}
}
