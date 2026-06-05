package tui

import (
	"github.com/Rogercode97/scouter/internal/engine/apply"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

type StepProgressMsg struct {
	StepID string
	Status apply.StepStatus
	Err    error
}

type PipelineDoneMsg struct {
	Result apply.ExecutionResult
}

type Model struct {
	Progress        ProgressState
	Execution       apply.ExecutionResult
	Err             error
	pipelineRunning bool
}

func NewModel() Model {
	return Model{
		Progress: NewProgressState([]string{"Initialize", "Apply Patches", "Verify"}),
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		if m.pipelineRunning {
			return m, tickCmd()
		}
		return m, nil
	case StepProgressMsg:
		idx := m.findProgressItem(msg.StepID)
		if idx >= 0 {
			switch msg.Status {
			case apply.StepStatusRunning:
				m.Progress.Start(idx)
			case apply.StepStatusSucceeded:
				m.Progress.Mark(idx, string(msg.Status))
			case apply.StepStatusFailed:
				m.Progress.Mark(idx, string(msg.Status))
			}
		}
		return m, nil
	case PipelineDoneMsg:
		m.Execution = msg.Result
		m.pipelineRunning = false
		m.Progress = ProgressFromExecution(msg.Result)
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) findProgressItem(stepID string) int {
	for i, item := range m.Progress.Items {
		if item.Label == stepID {
			return i
		}
	}
	return -1
}

func (m Model) View() string {
	return m.Progress.Render()
}
