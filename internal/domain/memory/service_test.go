package memory_test

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/Rogercode97/scouter/internal/types"
)

type dummyDistiller struct{}

func (d dummyDistiller) Distill(ctx context.Context, logs []memory.Observation) (memory.Summary, error) {
	return memory.Summary{}, nil
}

func (d dummyDistiller) DistillTranscript(ctx context.Context, transcript []memory.Message) ([]memory.DistilledMemory, error) {
	return nil, nil
}

type dummyMemory struct{}

func (d dummyMemory) GetRecentObservations(ctx context.Context, project string, hours int) ([]memory.Observation, error) {
	return nil, nil
}
func (d dummyMemory) SaveObservation(ctx context.Context, project string, mem memory.DistilledMemory) error {
	return nil
}
func (d dummyMemory) SearchInsights(ctx context.Context, query string, limit int) ([]types.MemoryInsight, error) {
	return nil, nil
}
func (d dummyMemory) SaveSummary(ctx context.Context, project string, summary memory.Summary) error {
	return nil
}

func TestAppService_Signatures(t *testing.T) {
	svc := memory.NewAppService(dummyMemory{})
	distiller := dummyDistiller{}
	
	err := svc.PassiveDistill(context.Background(), "scouter", nil, distiller)
	if err != nil {
		t.Log(err)
	}
	
	err = svc.DistillAndSave(context.Background(), "scouter", 24, distiller)
	if err != nil {
		t.Log(err)
	}
}
