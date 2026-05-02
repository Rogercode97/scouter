package mcp

import (
	"context"
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
		URI:         "file:///scouter/workspace",
		Name:        "Workspace Information",
		Description: "Returns the current workspace path and Scouter version.",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		cwd, _ := os.Getwd()
		project := utils.GetRepoName(ctx)
		content := fmt.Sprintf("Scouter Version: 8.0.0-wave11\nWorkspace: %s\nProject: %s", cwd, project)
		
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "file:///scouter/workspace",
					MIMEType: "text/plain",
					Text:     content,
				},
			},
		}, nil
	})

	// Resource: List ADRs
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "file:///scouter/adrs",
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
					list.WriteString(fmt.Sprintf("- file:///scouter/adrs/%s\n", entry.Name()))
				}
			}
		} else {
			list.WriteString("No ADRs found or directory docs/adr does not exist.\n")
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "file:///scouter/adrs",
					MIMEType: "text/plain",
					Text:     list.String(),
				},
			},
		}, nil
	})

	// Resource Template: Read specific ADR
	s.mcpServer.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "file:///scouter/adrs/{id}",
		Name:        "Read ADR",
		Description: "Read the content of a specific Architecture Decision Record.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		parts := strings.Split(uri, "file:///scouter/adrs/")
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
}