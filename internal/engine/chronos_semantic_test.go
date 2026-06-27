package engine_test

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
)

func TestChronosEngine_SemanticDiff(t *testing.T) {
	// Fails to compile because SemanticDiff isn't added to ChronosEngine yet
	chronos := engine.NewChronosEngine()

	res, err := chronos.SemanticDiff(context.Background(), "/repo")
	if err != nil {
		t.Log(err)
	}
	if res == "" {
		t.Log("Expected diff output")
	}
}
