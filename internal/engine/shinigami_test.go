package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type MockSolver struct {
	Candidates []FixCandidate
	Err        error
	Delay      time.Duration
}

func (m *MockSolver) Generate(ctx context.Context, diag *DiagnosticContext) (<-chan FixCandidate, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	ch := make(chan FixCandidate)
	go func() {
		defer close(ch)
		for _, c := range m.Candidates {
			select {
			case <-ctx.Done():
				return
			case <-time.After(m.Delay):
				ch <- c
			}
		}
	}()
	return ch, nil
}

type MockVerifier struct {
	AcceptID int
	Err      error
	Delay    time.Duration
}

func (m *MockVerifier) Evaluate(ctx context.Context, diag *DiagnosticContext, c *FixCandidate) (bool, error) {
	if m.Err != nil {
		return false, m.Err
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(m.Delay):
		return c.ID == m.AcceptID, nil
	}
}

func TestShinigamiPipeline_Run(t *testing.T) {
	tests := []struct {
		name          string
		solver        Solver
		verifier      Verifier
		ctxTimeout    time.Duration
		expectedID    int
		expectedError string
	}{
		{
			name: "Success: Valid candidate found",
			solver: &MockSolver{
				Candidates: []FixCandidate{
					{ID: 1, Code: "code 1", Score: 1.0},
					{ID: 2, Code: "code 2", Score: 1.0},
				},
			},
			verifier:      &MockVerifier{AcceptID: 2},
			ctxTimeout:    2 * time.Second,
			expectedID:    2,
			expectedError: "",
		},
		{
			name: "Error: All candidates rejected",
			solver: &MockSolver{
				Candidates: []FixCandidate{
					{ID: 1, Code: "code 1", Score: 1.0},
				},
			},
			verifier:      &MockVerifier{AcceptID: 99}, // No candidate matches 99
			ctxTimeout:    2 * time.Second,
			expectedID:    0,
			expectedError: "all candidates failed validation",
		},
		{
			name: "Error: Solver fails immediately",
			solver: &MockSolver{
				Err: errors.New("solver network error"),
			},
			verifier:      &MockVerifier{AcceptID: 1},
			ctxTimeout:    2 * time.Second,
			expectedID:    0,
			expectedError: "solver network error",
		},
		{
			name: "Timeout: Context canceled during solver",
			solver: &MockSolver{
				Candidates: []FixCandidate{{ID: 1}},
				Delay:      500 * time.Millisecond,
			},
			verifier:      &MockVerifier{AcceptID: 1},
			ctxTimeout:    100 * time.Millisecond,
			expectedID:    0,
			expectedError: "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.ctxTimeout)
			defer cancel()

			pipeline := NewShinigamiPipeline(tt.solver, tt.verifier)
			diag := &DiagnosticContext{FailingFile: "test.go"}

			got, err := pipeline.Run(ctx, diag)

			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedError)
				}
				if !strings.Contains(err.Error(), tt.expectedError) && !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("expected error %q, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got == nil {
				t.Fatalf("expected candidate, got nil")
			}

			if got.ID != tt.expectedID {
				t.Errorf("expected candidate ID %d, got %d", tt.expectedID, got.ID)
			}
		})
	}
}
