package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rogercode97/scouter/internal/domain/memory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPDistiller struct {
	Session *mcp.ServerSession
}

func NewMCPDistiller(session *mcp.ServerSession) *MCPDistiller {
	return &MCPDistiller{Session: session}
}

func (d *MCPDistiller) Distill(ctx context.Context, logs []memory.Observation) (memory.Summary, error) {
	if len(logs) == 0 {
		return memory.Summary{}, nil
	}

	prompt := d.buildPrompt(logs)

	samplingRes, err := d.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: "You are the Scouter Dream Engine. Your output must be STRICTLY valid JSON.",
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: prompt}},
		},
		MaxTokens: 4096,
	})

	if err != nil {
		return memory.Summary{}, fmt.Errorf("MCP Sampling failed: %w", err)
	}

	txt, ok := samplingRes.Content.(*mcp.TextContent)
	if !ok {
		return memory.Summary{}, fmt.Errorf("unexpected sampling response type")
	}

	var summary memory.Summary
	responseText := txt.Text
	responseText = strings.TrimPrefix(responseText, "```json\n")
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	err = json.Unmarshal([]byte(responseText), &summary)
	if err != nil {
		return memory.Summary{}, fmt.Errorf("failed to unmarshal JSON from LLM: %w\nResponse: %s", err, responseText)
	}

	return summary, nil
}

func (d *MCPDistiller) buildPrompt(logs []memory.Observation) string {
	var sb strings.Builder
	sb.WriteString("Analyze the following development logs and distill them into a structured JSON summary.\n")
	sb.WriteString("RULES:\n")
	sb.WriteString("1. Extract Architectural Decisions (ADRs), Root Cause Bug Fixes, and Established Patterns.\n")
	sb.WriteString("2. IGNORE noise (typos, failed commands, routine edits).\n")
	sb.WriteString("3. If provided, prioritize AST structural data and structural links to correlate architectural boundaries.\n")
	sb.WriteString("4. OUTPUT MUST BE VALID JSON matching this structure: { \"adrs\": [], \"bug_fixes\": [], \"patterns\": [] }\n\n")
	sb.WriteString("LOGS:\n")

	for _, log := range logs {
		sb.WriteString("- ")
		sb.WriteString(log.Content)
		if log.ASTContext != "" {
			sb.WriteString(fmt.Sprintf(" [AST Context: %s]", log.ASTContext))
		}
		if len(log.StructuralLinks) > 0 {
			sb.WriteString(fmt.Sprintf(" [Links: %s]", strings.Join(log.StructuralLinks, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (d *MCPDistiller) DistillTranscript(ctx context.Context, transcript []memory.Message) ([]memory.DistilledMemory, error) {
	prompt := "Distill the architectural decisions, bug fixes, and patterns established in our current session into structured JSON: { \"memories\": [ { \"type\": \"architecture|bugfix|pattern\", \"title\": \"...\", \"content\": \"...\" } ] }"

	var messages []*mcp.SamplingMessage
	for _, msg := range transcript {
		messages = append(messages, &mcp.SamplingMessage{
			Role:    mcp.Role(msg.Role),
			Content: &mcp.TextContent{Text: msg.Content},
		})
	}

	// Always add the distillation request at the end
	messages = append(messages, &mcp.SamplingMessage{
		Role:    mcp.Role("user"),
		Content: &mcp.TextContent{Text: prompt},
	})

	samplingRes, err := d.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		SystemPrompt: "You are the Scouter Dream Engine. Your output must be STRICTLY valid JSON.",
		Messages:     messages,
		MaxTokens:    4096,
	})

	if err != nil {
		return nil, fmt.Errorf("MCP Sampling failed: %w", err)
	}

	txt, ok := samplingRes.Content.(*mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("unexpected sampling response type")
	}

	type responseWrapper struct {
		Memories []memory.DistilledMemory `json:"memories"`
	}

	var wrapper responseWrapper
	responseText := txt.Text
	responseText = strings.TrimPrefix(responseText, "```json\n")
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	err = json.Unmarshal([]byte(responseText), &wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON from LLM: %w\nResponse: %s", err, responseText)
	}

	return wrapper.Memories, nil
}
