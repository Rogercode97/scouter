# 📋 CHECKLIST DE IMPLEMENTACIÓN: MCP SHIELD PURIFICATION

## 🔱 ESTADO DEL ALMA
- **Cambio**: `mcp-shield-purification`
- **Fase**: Implementation Tasks (Wave 8.9)
- **Status**: ⚔️ READY FOR EXECUTION

---

## 🏗️ FASE 1: CIMIENTOS Y DEPENDENCIAS (FOUNDATION)
- [ ] **[CLI]** Instalar el SDK oficial de MCP: `go get github.com/modelcontextprotocol/go-sdk/server` y `go get github.com/modelcontextprotocol/go-sdk/protocol`.
- [ ] **[CLI]** Ejecutar `go mod tidy` para purificar el `go.sum`.

## ⚡ FASE 2: REFACTOR DE HANDLERS (LOGIC PURIFICATION)
- [ ] **[FILE]** `internal/mcp/handlers.go`: Refactorizar `HandleSearch`, `HandleIndex`, etc., para que acepten `context.Context` y sigan la firma de `mcp.ToolHandler`.
- [ ] **[FILE]** `internal/mcp/handlers.go`: Eliminar dependencias de `os.Stdout` dentro de los handlers; usar inyección de dependencias para resultados.
- [ ] **[FILE]** `internal/mcp/types.go`: Definir los structs de Request/Response basados en el esquema de herramientas del SDK oficial.

## 🛡️ FASE 3: RECONSTRUCCIÓN DEL SERVIDOR (MCP SHIELDING)
- [ ] **[FILE]** `internal/mcp/server.go`: Implementar `NewServer()` usando `mcp.NewServer()`.
- [ ] **[FILE]** `internal/mcp/server.go`: Registrar todas las herramientas (Search, Index, Read, etc.) usando `server.RegisterTool()`.
- [ ] **[FILE]** `internal/mcp/server.go`: Implementar el transporte JSON-RPC sobre `stdin/stdout` usando la abstracción nativa del SDK (`protocol.NewStdioServerTransport`).
- [ ] **[FILE]** `internal/mcp/resolver.go`: Eliminar el código legacy de captura de logs que secuestraba `os.Stdout`.

## 🔌 FASE 4: INTEGRACIÓN Y CABLEADO (WIRING)
- [ ] **[FILE]** `cmd/scouter/main.go`: Actualizar el entrypoint del modo MCP para inicializar y arrancar el nuevo `Server`.
- [ ] **[FILE]** `internal/cli/cli.go`: Asegurar que los flags de MCP invoquen correctamente la nueva infraestructura purificada.

## ✅ FASE 5: VERIFICACIÓN EMPÍRICA (TRUTH KERNEL)
- [ ] **[FILE]** `internal/mcp/server_test.go`: Crear suite de tests de integración usando mocks del transporte para verificar el handshake JSON-RPC.
- [ ] **[FILE]** `internal/mcp/handlers_test.go`: Validar cada herramienta individualmente con inputs/outputs controlados.
- [ ] **[CLI]** Ejecutar `go test ./internal/mcp/... -cover` y garantizar >90% de cobertura.
- [ ] **[CLI]** Verificar conexión con un cliente MCP real (e.g., Claude Desktop o inspector de MCP) para validar el protocolo.

---
**Sovereign Note**: La purificación del canal Stdout es Crítica. Ninguna función debe imprimir a Stdout excepto el transporte del SDK. 
Hakai.