---
name: scouter-mcp-go
description: >
  Elite protocol for Scouter MCP tool development. Enforces Glasswall Validation, Context Authority, and OOM Guard.
  Trigger: When adding or refactoring MCP tools in cmd/scouter/.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.1"
---

# 👁️ SCOUTER MCP-GO (WAVE 8.9)

## 🔱 MANDATOS SOBERANOS
- **GLASSWALL VALIDATION**: Prohibido usar argumentos crudos. Todo input DEBE ser unmarshal-eado en un struct con tags `validate` y sentenciado mediante `validator/v10`.
- **CONTEXT AUTHORITY**: El `ctx` del handler es ley. Prohibido crear contextos huérfanos.
- **OOM GUARD**: Toda respuesta con listas DEBE tener un `LIMIT` (ej. 500) y un flag `truncated: true`.
- **STRUCTURED RESPONSES**: Solo JSON en `TextContent`. El agente es un parser, no un lector de cuentos.

## 🛠️ PATRÓN DE ÉLITE (HANDLERS)
```go
type Request struct {
	Param string `json:"param" validate:"required,min=1"`
}

s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var req Request
	// 1. Force Unmarshal
	if err := json.Unmarshal(request.GetArgumentsJSON(), &req); err != nil {
		return mcpError("Invalid arguments"), nil
	}
	// 2. Glasswall Execution
	if err := v.Struct(req); err != nil {
		return mcpError(fmt.Sprintf("Validation failed: %v", err)), nil
	}
	// 3. Sovereign Logic
	res, _ := db.Action(ctx, req.Param)
	return mcpJSONResponse(res), nil
})
```

## 📜 CHECKLIST DE SOBERANÍA
- [ ] ¿Struct con tags `validate` definido?
- [ ] ¿`validator.Struct()` ejecutado antes de la lógica?
- [ ] ¿Contexto propagado hasta el `store`?
- [ ] ¿Respuesta JSON estructurada?
