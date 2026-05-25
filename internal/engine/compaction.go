package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
)

// CompactionEngine handles context hygiene and latent memory anchoring.
type CompactionEngine struct {
	store  store.Store
	Ledger *Ledger
}

func NewCompactionEngine(s store.Store, l *Ledger) *CompactionEngine {
	return &CompactionEngine{
		store:  s,
		Ledger: l,
	}
}

// SovereignBoundary represents a technical checkpoint in the conversation.
type SovereignBoundary struct {
	ID          string            `json:"boundary_id"`
	TruthKernel string            `json:"truth_kernel"` // Dense summary of discoveries
	StagingArea map[string]Patch  `json:"staging_area"` // Current Ledger state
	Budget      MissionStats      `json:"budget"`       // Ki/Turns consumption
	Timestamp   string            `json:"timestamp"`
}

// CreateBoundary generates a dense checkpoint of the current engine state.
func (e *CompactionEngine) CreateBoundary(ctx context.Context, truthKernel string) (*SovereignBoundary, error) {
	if truthKernel == "" {
		return nil, fmt.Errorf("truth kernel cannot be empty")
	}

	e.Ledger.mu.RLock()
	defer e.Ledger.mu.RUnlock()

	return &SovereignBoundary{
		ID:          fmt.Sprintf("boundary-%d", time.Now().Unix()),
		TruthKernel: truthKernel,
		StagingArea: e.Ledger.Staged,
		Budget:      e.Ledger.Stats,
		Timestamp:   time.Now().Format(time.RFC3339),
	}, nil
}

// PruneHistory reduces the token weight of tool results while preserving signal.
func (e *CompactionEngine) PruneHistory(toolResults []string) []string {
	pruned := make([]string, len(toolResults))
	for i, res := range toolResults {
		// Rule: If result is > 500 chars (approx 125 tokens), we summarize/truncate
		if len(res) > 500 {
			// Extract Pure Signal (e.g., just the first 100 and last 100 chars + pruning marker)
			pruned[i] = fmt.Sprintf("%s\n... [Sovereign Pruning: 60%% Ki Savings] ...\n%s", 
				res[:100], res[len(res)-100:])
		} else {
			pruned[i] = res
		}
	}
	return pruned
}

// CompactSession persists a boundary and provides the resumption prompt.
func (e *CompactionEngine) CompactSession(ctx context.Context, truthKernel string) (*types.CompactionResult, error) {
	boundary, err := e.CreateBoundary(ctx, truthKernel)
	if err != nil {
		return nil, err
	}

	cwd, _ := os.Getwd()
	anchorPath := filepath.Join(cwd, ".scouter", "boundary.json")
	
	data, _ := json.MarshalIndent(boundary, "", "  ")
	if err := os.MkdirAll(filepath.Dir(anchorPath), 0755); err != nil {
		return nil, err
	}
	
	if err := os.WriteFile(anchorPath, data, 0644); err != nil {
		return nil, err
	}

	return &types.CompactionResult{
		AnchorPath: anchorPath,
		Timestamp:  boundary.Timestamp,
		Message:    fmt.Sprintf("### 🔱 Sovereign Boundary Created\n\nContext has been compacted to protect the mission budget.\nTruth Kernel: %s\nLedger: %d staged files.\nBudget: %d Ki used.", 
			boundary.TruthKernel, len(boundary.StagingArea), boundary.Budget.TotalKi),
	}, nil
}
