package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/types"
)

type mockDistiller struct {
	summary       memory.Summary
	err           error
	memories      []memory.DistilledMemory
	transcriptErr error
}

func (m *mockDistiller) Distill(ctx context.Context, logs []memory.Observation) (memory.Summary, error) {
	return m.summary, m.err
}

func (m *mockDistiller) DistillTranscript(ctx context.Context, transcript []memory.Message) ([]memory.DistilledMemory, error) {
	return m.memories, m.transcriptErr
}

type mockMemory struct {
	observations   []memory.Observation
	getErr         error
	saveErr        error
	saveSummaryErr error
	savedMemories  []memory.DistilledMemory
	savedSummaries []memory.Summary
}

func (m *mockMemory) GetRecentObservations(ctx context.Context, project string, hours int) ([]memory.Observation, error) {
	return m.observations, m.getErr
}
func (m *mockMemory) SaveObservation(ctx context.Context, project string, mem memory.DistilledMemory) error {
	m.savedMemories = append(m.savedMemories, mem)
	return m.saveErr
}
func (m *mockMemory) SearchInsights(ctx context.Context, query string, limit int) ([]types.MemoryInsight, error) {
	return nil, nil
}
func (m *mockMemory) SaveSummary(ctx context.Context, project string, summary memory.Summary) error {
	m.savedSummaries = append(m.savedSummaries, summary)
	return m.saveSummaryErr
}

func TestAppService_DistillAndSave(t *testing.T) {
	t.Run("fails when extraction fails", func(t *testing.T) {
		mem := &mockMemory{getErr: errors.New("get err")}
		svc := memory.NewAppService(mem)
		err := svc.DistillAndSave(context.Background(), "scouter", 24, &mockDistiller{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("fails when no observations found", func(t *testing.T) {
		mem := &mockMemory{observations: nil}
		svc := memory.NewAppService(mem)
		err := svc.DistillAndSave(context.Background(), "scouter", 24, &mockDistiller{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("fails when distillation fails", func(t *testing.T) {
		mem := &mockMemory{observations: []memory.Observation{{Content: "test"}}}
		dist := &mockDistiller{err: errors.New("distill err")}
		svc := memory.NewAppService(mem)
		err := svc.DistillAndSave(context.Background(), "scouter", 24, dist)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("fails when save summary fails", func(t *testing.T) {
		mem := &mockMemory{
			observations:   []memory.Observation{{Content: "test"}},
			saveSummaryErr: errors.New("save err"),
		}
		dist := &mockDistiller{summary: memory.Summary{}}
		svc := memory.NewAppService(mem)
		err := svc.DistillAndSave(context.Background(), "scouter", 24, dist)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		mem := &mockMemory{
			observations: []memory.Observation{{Content: "test"}},
		}
		dist := &mockDistiller{summary: memory.Summary{ADRs: []string{"Use Go"}}}
		svc := memory.NewAppService(mem)
		err := svc.DistillAndSave(context.Background(), "scouter", 24, dist)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(mem.savedSummaries) != 1 {
			t.Fatalf("expected 1 summary saved, got %d", len(mem.savedSummaries))
		}
		if len(mem.savedSummaries[0].ADRs) != 1 || mem.savedSummaries[0].ADRs[0] != "Use Go" {
			t.Errorf("summary not saved correctly")
		}
	})
}

func TestAppService_PassiveDistill(t *testing.T) {
	t.Run("fails when distillation fails", func(t *testing.T) {
		mem := &mockMemory{}
		dist := &mockDistiller{transcriptErr: errors.New("distill err")}
		svc := memory.NewAppService(mem)
		err := svc.PassiveDistill(context.Background(), "scouter", nil, dist)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns early if no memories distilled", func(t *testing.T) {
		mem := &mockMemory{}
		dist := &mockDistiller{memories: nil}
		svc := memory.NewAppService(mem)
		err := svc.PassiveDistill(context.Background(), "scouter", nil, dist)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("filters out duplicates", func(t *testing.T) {
		mem := &mockMemory{
			observations: []memory.Observation{
				{Content: "duplicate content"},
			},
		}
		dist := &mockDistiller{
			memories: []memory.DistilledMemory{
				{Content: "duplicate content"},
				{Content: "new content"},
			},
		}
		svc := memory.NewAppService(mem)
		err := svc.PassiveDistill(context.Background(), "scouter", nil, dist)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(mem.savedMemories) != 1 {
			t.Fatalf("expected 1 memory saved, got %d", len(mem.savedMemories))
		}
		if mem.savedMemories[0].Content != "new content" {
			t.Errorf("expected 'new content' to be saved")
		}
	})

	t.Run("handles get err gracefully and continues to save", func(t *testing.T) {
		mem := &mockMemory{
			getErr: errors.New("get err"),
		}
		dist := &mockDistiller{
			memories: []memory.DistilledMemory{
				{Content: "new content"},
			},
		}
		svc := memory.NewAppService(mem)
		err := svc.PassiveDistill(context.Background(), "scouter", nil, dist)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(mem.savedMemories) != 1 {
			t.Fatalf("expected 1 memory saved, got %d", len(mem.savedMemories))
		}
	})

	t.Run("handles save err gracefully and continues to save others", func(t *testing.T) {
		mem := &mockMemory{
			saveErr: errors.New("save err"),
		}
		dist := &mockDistiller{
			memories: []memory.DistilledMemory{
				{Content: "new content 1"},
				{Content: "new content 2"},
			},
		}
		svc := memory.NewAppService(mem)
		err := svc.PassiveDistill(context.Background(), "scouter", nil, dist)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(mem.savedMemories) != 2 {
			t.Fatalf("expected 2 save attempts, got %d", len(mem.savedMemories))
		}
	})
}
