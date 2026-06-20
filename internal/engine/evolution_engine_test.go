package engine_test

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
)

type mockMessenger struct{}

func (m *mockMessenger) Ask(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return `[{"file":"test.go", "content":"func test() {}"}]`, nil
}

func TestEvolutionEngine_ProposeEvolution(t *testing.T) {
	// This will fail to compile since EvolutionEngine and ProposeEvolution don't exist yet
	evo := &engine.EvolutionEngine{}
	
	msg := &mockMessenger{}
	res, err := evo.ProposeEvolution(context.Background(), "refactor this", false, msg)
	if err != nil {
		t.Log(err)
	}
	if res == "" {
		t.Log("expected result")
	}
}
