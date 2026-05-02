package lsp

import "encoding/json"

// Position represents a symbol's position in a text document.
type Position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}

// Range represents a range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location in a text document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier represents a text document's URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams represents parameters for requests at a position.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// DefinitionParams represents parameters for the textDocument/definition request.
type DefinitionParams struct {
	TextDocumentPositionParams
}

// ImplementationParams represents parameters for the textDocument/implementation request.
type ImplementationParams struct {
	TextDocumentPositionParams
}

// HoverParams represents parameters for the textDocument/hover request.
type HoverParams struct {
	TextDocumentPositionParams
}

// ReferenceContext represents context for the textDocument/references request.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams represents parameters for the textDocument/references request.
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// Hover represents the result of a hover request.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// CallHierarchyPrepareParams represents parameters for textDocument/prepareCallHierarchy.
type CallHierarchyPrepareParams struct {
	TextDocumentPositionParams
}

// CallHierarchyItem represents an item in a call hierarchy.
type CallHierarchyItem struct {
	Name           string      `json:"name"`
	Kind           int         `json:"kind"`
	Tags           []int       `json:"tags,omitempty"`
	Detail         string      `json:"detail,omitempty"`
	URI            string      `json:"uri"`
	Range          Range       `json:"range"`
	SelectionRange Range       `json:"selectionRange"`
	Data           interface{} `json:"data,omitempty"`
}

// CallHierarchyIncomingCallsParams represents parameters for callHierarchy/incomingCalls.
type CallHierarchyIncomingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

// CallHierarchyIncomingCall represents an incoming call.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// CallHierarchyOutgoingCallsParams represents parameters for callHierarchy/outgoingCalls.
type CallHierarchyOutgoingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

// CallHierarchyOutgoingCall represents an outgoing call.
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

// MarkupContent represents a markup content.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// WorkspaceFolder represents a workspace folder in LSP.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// InitializeParams represents parameters for the initialize request.
type InitializeParams struct {
	ProcessID        int                `json:"processId"`
	RootURI          string             `json:"rootUri,omitempty"`
	Capabilities     ClientCapabilities `json:"capabilities"`
	WorkspaceFolders []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}

// ClientCapabilities represents the capabilities of the LSP client.
type ClientCapabilities struct {
	TextDocument struct {
		Definition struct {
			DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
		} `json:"definition,omitempty"`
		References struct {
			DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
		} `json:"references,omitempty"`
		Hover struct {
			ContentFormat []string `json:"contentFormat,omitempty"`
		} `json:"hover,omitempty"`
	} `json:"textDocument,omitempty"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}
