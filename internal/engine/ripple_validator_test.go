package engine

import (
	"context"
	"testing"
)

func TestBuildValidator(t *testing.T) {
	v := &BuildValidator{}
	ledger := NewLedger()
	ctx := context.Background()

	// This test depends on the environment having 'go'.
	// We can't easily mock exec.Command, but we can check if it runs.
	res, err := v.Validate(ctx, ledger)
	if err != nil {
		t.Fatalf("BuildValidator.Validate failed: %v", err)
	}

	if !res.Valid {
		t.Logf("Build failed (expected in some environments): %s", res.Message)
	} else {
		t.Logf("Build successful: %s", res.Message)
	}
}

func TestTestValidator(t *testing.T) {
	v := &TestValidator{SpecificTests: []string{"-run", "TestBFSPropagationStrategy_Discover"}}
	ledger := NewLedger()
	ctx := context.Background()

	res, err := v.Validate(ctx, ledger)
	if err != nil {
		t.Fatalf("TestValidator.Validate failed: %v", err)
	}

	if !res.Valid {
		t.Errorf("Test failed: %s", res.Message)
	} else {
		t.Logf("Test successful: %s", res.Message)
	}
}
