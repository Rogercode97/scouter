package mcp

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClientProfile categorizes the connected MCP client to adapt behaviors and prompts.
type ClientProfile string

const (
	ClientAntigravity ClientProfile = "antigravity"
	ClientToon        ClientProfile = "toon"
	ClientOpenCode    ClientProfile = "opencode"
	ClientCodex       ClientProfile = "codex"
	ClientGeneric     ClientProfile = "generic"
)

// IdentifyClient inspects the MCP session to determine the client profile.
func IdentifyClient(session *mcp.ServerSession) ClientProfile {
	if session == nil {
		return ClientGeneric
	}

	params := session.InitializeParams()
	if params == nil || params.ClientInfo == nil {
		return ClientGeneric
	}

	name := strings.ToLower(params.ClientInfo.Name)

	switch {
	case strings.Contains(name, "toon"):
		return ClientToon
	case strings.Contains(name, "antigravity"):
		return ClientAntigravity
	case strings.Contains(name, "opencode"):
		return ClientOpenCode
	case strings.Contains(name, "codex"):
		return ClientCodex
	default:
		return ClientGeneric
	}
}

// AdaptSystemPrompt modifies a baseline prompt according to the client profile.
func AdaptSystemPrompt(baseline string, profile ClientProfile) string {
	var builder strings.Builder
	builder.WriteString(baseline)

	switch profile {
	case ClientToon:
		builder.WriteString("\n\n[TOON CLIENT DETECTED]: You are interacting with the 'Toon' agent. Ensure your outputs are highly visual, structured, and use clear markdown formatting. Emphasize step-by-step logic.")
	case ClientAntigravity:
		builder.WriteString("\n\n[ANTIGRAVITY DETECTED]: Use Pure Signal. Minimize conversational slop. Use `<thought>` blocks if complex logic is needed before responding.")
	case ClientOpenCode:
		builder.WriteString("\n\n[OPENCODE DETECTED]: Strict code delivery. Output raw, clean code over extensive markdown explanations where possible. Avoid markdown wrappers if the parser is strict.")
	case ClientCodex:
		builder.WriteString("\n\n[CODEX DETECTED]: Optimize for strict token limits. Be extremely concise. Truncate outputs where possible.")
	}

	return builder.String()
}
