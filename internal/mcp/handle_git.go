package mcp

import (
	"context"
	"fmt"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ProvenanceParams struct {
	FilePath   string `json:"filePath" jsonschema:"REQUIRED. The absolute or relative path to the file"`
	LineNumber int    `json:"lineNumber,omitempty" jsonschema:"Optional: Specific 1-based line number to blame"`
}

func (s *Server) handleProvenance(ctx context.Context, req *mcp.CallToolRequest, args ProvenanceParams) (*mcp.CallToolResult, any, error) {
	if args.FilePath == "" {
		return s.presenter.FormatError(fmt.Errorf("missing filePath")), nil, nil
	}

	path, err := utils.ValidatePath(args.FilePath)
	if err != nil {
		return s.presenter.FormatError(err), nil, nil
	}

	repoPath := "." // Defaulting to current working directory for now

	provs, err := engine.GetFileProvenance(ctx, repoPath, path)
	if err != nil {
		return s.presenter.FormatError(fmt.Errorf("Provenance failed: %v", err)), nil, nil
	}

	if args.LineNumber > 0 {
		if args.LineNumber > len(provs) {
			return s.presenter.FormatError(fmt.Errorf("lineNumber out of bounds")), nil, nil
		}
		p := provs[args.LineNumber-1]
		thought := fmt.Sprintf("On-Demand Provenance: Traced line %d in %s to commit %s by %s (Era: %s)",
			args.LineNumber, path, p.Commit, p.Author, p.EngineeringEra)
		res, err := s.presenter.FormatResult(thought, p)
		return res, nil, err
	}

	thought := fmt.Sprintf("On-Demand Provenance: Extracted full blame for %s (%d lines)", path, len(provs))
	res, err := s.presenter.FormatResult(thought, provs)
	return res, nil, err
}
