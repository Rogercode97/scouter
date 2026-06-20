package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SnapshotASTParams struct {
	FilePath string `json:"filePath" jsonschema:"REQUIRED. The absolute or relative path to the file to snapshot"`
}

type VerifyASTParams struct {
	SnapshotID string `json:"snapshotId" jsonschema:"REQUIRED. The ID of the snapshot to verify against"`
	FilePath   string `json:"filePath" jsonschema:"REQUIRED. The absolute or relative path to the edited file to verify"`
}

// storeSnapshot saves a snapshot with LRU eviction when MaxSnapshots is reached.
func (s *Server) storeSnapshot(snap *engine.ChronosSnapshot) {
	s.snapshotsMu.Lock()
	defer s.snapshotsMu.Unlock()

	// Evict oldest if at capacity
	if len(s.snapshotOrder) >= MaxSnapshots {
		evictID := s.snapshotOrder[0]
		s.snapshotOrder = s.snapshotOrder[1:]
		delete(s.snapshots, evictID)
	}

	s.snapshots[snap.ID] = snap
	s.snapshotOrder = append(s.snapshotOrder, snap.ID)
}

// getSnapshot retrieves a snapshot by ID.
func (s *Server) getSnapshot(id string) (*engine.ChronosSnapshot, bool) {
	s.snapshotsMu.RLock()
	defer s.snapshotsMu.RUnlock()
	snap, ok := s.snapshots[id]
	return snap, ok
}

func (s *Server) handleSnapshotAST(ctx context.Context, req *mcp.CallToolRequest, args SnapshotASTParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return s.presenter.FormatError(fmt.Errorf("missing filePath")), nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return s.presenter.FormatError(err), nil, nil
	}

	snapshot, err := s.chronos.TakeSnapshot(ctx, path)
	if err != nil {
		return s.presenter.FormatError(fmt.Errorf("Failed to take snapshot: %v", err)), nil, nil
	}

	s.storeSnapshot(snapshot)

	thought := fmt.Sprintf("Chronos Engine: Snapshot taken for %s. Captured %d structural nodes. Snapshot ID: %s.",
		filepath.Base(path), len(snapshot.Symbols), snapshot.ID)

	return s.presenter.FormatTextResult(thought, fmt.Sprintf("[ZON Snapshot: %s]\nSYMBOLS | %d", snapshot.ID, len(snapshot.Symbols))), nil, nil
}

func (s *Server) handleVerifyAST(ctx context.Context, req *mcp.CallToolRequest, args VerifyASTParams) (*mcp.CallToolResult, any, error) {
	if args.SnapshotID == "" || args.FilePath == "" {
		return s.presenter.FormatError(fmt.Errorf("missing snapshotId or filePath")), nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return s.presenter.FormatError(err), nil, nil
	}

	snapshot, exists := s.getSnapshot(args.SnapshotID)
	if !exists {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Snapshot ID %s not found. Max capacity is %d snapshots (oldest are evicted).", args.SnapshotID, MaxSnapshots)}},
			IsError: true,
		}, nil, nil
	}

	diff, err := s.chronos.CompareSnapshot(ctx, snapshot, path)
	if err != nil {
		return s.presenter.FormatError(fmt.Errorf("Failed to verify snapshot: %v", err)), nil, nil
	}

	outStr := engine.EncodeZONVerify(diff)

	status := "✅ Structurally Safe"
	isError := false
	if len(diff.MissingSymbols) > 0 || len(diff.MangledSymbols) > 0 {
		status = "❌ Structural Breakage Detected"
		isError = true
	}

	thought := fmt.Sprintf("Chronos Engine: Verified %s against snapshot %s. %s\nMissing: %d | Mangled: %d | Added: %d | Unchanged: %d",
		filepath.Base(path), args.SnapshotID, status,
		len(diff.MissingSymbols), len(diff.MangledSymbols), len(diff.AddedSymbols), diff.Unchanged)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: thought + outStr},
		},
		IsError: isError,
	}, nil, nil
}

type SemanticDiffParams struct {
	DiffTarget string `json:"diff_target,omitempty" jsonschema:"Optional. The git target to diff against (e.g., HEAD, main). Defaults to HEAD."`
}

func (s *Server) handleSemanticDiff(ctx context.Context, req *mcp.CallToolRequest, args SemanticDiffParams) (*mcp.CallToolResult, any, error) {
	target := args.DiffTarget
	if target == "" {
		target = "HEAD"
	}

	report, err := s.chronos.SemanticDiff(ctx, target)
	if err != nil {
		return s.presenter.FormatError(err), nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("[Semantic Diff against %s]\n%s", target, report)},
		},
	}, nil, nil
}
