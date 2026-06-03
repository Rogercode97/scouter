package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	// Resource: Workspace Information
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "scouter://workspace",
		Name:        "Workspace Information",
		Description: "Returns the current workspace path and Scouter version.",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		cwd, _ := os.Getwd()
		project := utils.GetRepoName(ctx)
		content := fmt.Sprintf("Scouter Version: 12.0.0-ascension\nWorkspace: %s\nProject: %s", cwd, project)
		
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "scouter://workspace",
					MIMEType: "text/plain",
					Text:     content,
				},
			},
		}, nil
	})

	// Resource: Dependency Graph (Sovereign Resource)
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "scouter://graph/dependencies",
		Name:        "Dependency Graph",
		Description: "Returns the full project dependency graph as JSON.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		res, err := s.store.GetDependencies(ctx)
		if err != nil {
			return nil, err
		}
		
		out, _ := json.MarshalIndent(res, "", "  ")
		
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "scouter://graph/dependencies",
					MIMEType: "application/json",
					Text:     string(out),
				},
			},
		}, nil
	})

	// Resource: Staging Ledger (Sovereign Resource)
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "scouter://ledger/staging",
		Name:        "Staging Ledger",
		Description: "Shows pending changes and mission status.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		// Use TruthEngine's ripple engine ledger if available
		// Actually, let's just use the persistence file for total sovereignty
		data, err := os.ReadFile(".scouter/ledger.json")
		if err != nil {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "scouter://ledger/staging",
						MIMEType: "application/json",
						Text:     `{"message": "No active mission or staging area is empty."}`,
					},
				},
			}, nil
		}
		
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "scouter://ledger/staging",
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	})

	// Resource: List ADRs
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "scouter://adrs",
		Name:        "Architecture Decision Records (ADRs)",
		Description: "Lists all Architecture Decision Records (ADRs) available in the workspace.",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		cwd, _ := os.Getwd()
		adrDir := filepath.Join(cwd, "docs", "adr")
		entries, err := os.ReadDir(adrDir)
		
		var list strings.Builder
		list.WriteString("Available ADRs:\n")
		
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					list.WriteString(fmt.Sprintf("- scouter://adrs/%s\n", entry.Name()))
				}
			}
		} else {
			list.WriteString("No ADRs found or directory docs/adr does not exist.\n")
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "scouter://adrs",
					MIMEType: "text/plain",
					Text:     list.String(),
				},
			},
		}, nil
	})

	// Resource Template: Read specific ADR
	s.mcpServer.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "scouter://adrs/{id}",
		Name:        "Read ADR",
		Description: "Read the content of a specific Architecture Decision Record.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		parts := strings.Split(uri, "scouter://adrs/")
		if len(parts) != 2 || parts[1] == "" {
			return nil, fmt.Errorf("invalid ADR URI")
		}
		
		filename := parts[1]
		cwd, _ := os.Getwd()
		adrPath := filepath.Join(cwd, "docs", "adr", filename)
		
		// Path Traversal Protection
		cleanPath := filepath.Clean(adrPath)
		if !strings.HasPrefix(cleanPath, filepath.Join(cwd, "docs", "adr")) {
			return nil, fmt.Errorf("security violation: path traversal detected")
		}
		
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read ADR: %v", err)
		}
		
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      uri,
					MIMEType: "text/markdown",
					Text:     string(content),
				},
			},
		}, nil
	})

	// Resource: SDD Roadmap
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "scouter://sdd/roadmap",
		Name:        "SDD Roadmap",
		Description: "Returns the project trajectory and current phase.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		res, err := s.sdd.GetSDDRoadmap(ctx)
		if err != nil {
			return nil, err
		}
		out, _ := json.MarshalIndent(res, "", "  ")
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "scouter://sdd/roadmap",
					MIMEType: "application/json",
					Text:     string(out),
				},
			},
		}, nil
	})

	// Resource: SDD Tasks
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "scouter://sdd/tasks",
		Name:        "SDD Tasks",
		Description: "Returns the current SDD task list.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		res, err := s.sdd.GetSDDTasks(ctx)
		if err != nil {
			return nil, err
		}
		out, _ := json.MarshalIndent(res, "", "  ")
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "scouter://sdd/tasks",
					MIMEType: "application/json",
					Text:     string(out),
				},
			},
		}, nil
	})
}
