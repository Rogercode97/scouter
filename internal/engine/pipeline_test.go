package engine

import (
	"strings"
	"testing"

	"github.com/Rogercode97/scouter/internal/filter"
)

func TestApplyPipelineKeepLines(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	input := "hello\n\nworld\n\n"
	out, err := ApplyPipeline(ctx, f, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2: %v", len(lines), lines)
	}
}

func TestApplyPipelineChained(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
			{ActionName: "head", Params: map[string]any{"n": 2}},
		},
	}

	input := "a\nb\nc\nd\ne\n"
	out, err := ApplyPipeline(ctx, f, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 { // 2 kept + overflow msg
		t.Errorf("got %d lines: %v", len(lines), lines)
	}
}

func TestApplyPipelineUnknownAction(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "nonexistent"},
		},
	}

	_, err := ApplyPipeline(ctx, f, "input", nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestApplyPipelineEmptyInput(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	out, err := ApplyPipeline(ctx, f, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

func TestApplyPipelineGracefulDegradation(t *testing.T) {
	ctx := t.Context()
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{}}, // Missing pattern
		},
	}

	_, err := ApplyPipeline(ctx, f, "hello\nworld\n", nil)
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}