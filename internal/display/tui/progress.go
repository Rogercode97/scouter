package tui

import (
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/apply"
)

type ProgressItem struct {
	Label  string
	Status string
}

const (
	ProgressStatusPending = "pending"
	ProgressStatusRunning = "running"
)

type ProgressState struct {
	Items   []ProgressItem
	Current int
	Logs    []string
}

func NewProgressState(labels []string) ProgressState {
	items := make([]ProgressItem, 0, len(labels))
	for _, label := range labels {
		items = append(items, ProgressItem{Label: label, Status: ProgressStatusPending})
	}

	return ProgressState{Items: items, Current: -1}
}

func (p *ProgressState) Start(index int) {
	if index < 0 || index >= len(p.Items) {
		return
	}
	p.Current = index
	p.Items[index].Status = ProgressStatusRunning
}

func (p *ProgressState) Mark(index int, status string) {
	if index < 0 || index >= len(p.Items) {
		return
	}
	p.Items[index].Status = status
	if p.Current == index {
		p.Current = -1
	}
}

func (p *ProgressState) AppendLog(format string, args ...any) {
	p.Logs = append(p.Logs, fmt.Sprintf(format, args...))
}

func ProgressFromExecution(result apply.ExecutionResult) ProgressState {
	var p ProgressState

	appendSteps := func(stage apply.StageResult) {
		for _, step := range stage.Steps {
			p.Items = append(p.Items, ProgressItem{
				Label:  step.StepID,
				Status: string(step.Status),
			})
		}
	}

	appendSteps(result.Prepare)
	appendSteps(result.Apply)
	appendSteps(result.Rollback)

	return p
}

func (p ProgressState) Render() string {
	var sb strings.Builder
	for _, item := range p.Items {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", item.Status, item.Label))
	}
	for _, log := range p.Logs {
		sb.WriteString(log + "\n")
	}
	return sb.String()
}
