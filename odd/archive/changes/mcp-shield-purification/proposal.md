# PROPOSAL: Fase 1 - Blindaje y Purificación de MCP

## 🎯 INTENT
Corregir la arquitectura deficiente del servidor MCP identificada por el Veredicto del Juicio Supremo (Rating 1.9/10.0). Se requiere aislar el entorno de MCP de los canales estándar (`os.Stdout`) para prevenir la corrupción del protocolo JSON-RPC con logs de sistema, eliminar dependencias de variables globales y garantizar robustez empírica mediante validación total de pruebas.

## 🛡️ SCOPE
- **IN SCOPE**:
  - Modificación de `internal/mcp/server.go` para inyección explícita de dependencias (`io.ReadCloser` e `io.WriteCloser`).
  - Eliminación completa del secuestro y mutación global de `os.Stdout`.
  - Refactorización de la instanciación del servidor para eliminar dependencias de variables y estados globales.
  - Corrección estricta del framing del protocolo JSON-RPC para evitar colisiones e interferencias.
  - Implementación de suite de tests con un mandato del 100% de cobertura (Zero Slop) para el paquete `internal/mcp`.
- **OUT OF SCOPE**:
  - Adición de nuevas herramientas (tools) de MCP.
  - Modificación de la capa de transporte ajena al paquete `internal/mcp`.
  - Refactorización de otros módulos o subsistemas en `internal/` (e.g., `telemetry`, `engine`).

## 📦 CAPABILITIES
### Modified Capabilities
- `mcp-server-initialization`: Transición de estado global a inicialización pura e inyectada.
- `mcp-json-rpc-transport`: Aislamiento de comunicación (Zero System Log Collision).

### New Capabilities
- `mcp-empirical-validation`: Implementación de tests de unidad e integración (cobertura total en `internal/mcp`).

## 🗺️ AFFECTED AREAS
- `internal/mcp/server.go`
- `internal/mcp/handlers.go`
- `internal/mcp/resolver.go`
- Archivos de prueba correspondientes (ej. `internal/mcp/server_test.go`, `internal/mcp/handlers_test.go`).

## ⏪ ROLLBACK PLAN
- Deshacer commits de la rama de la propuesta.
- Restaurar `internal/mcp/server.go` al modelo actual (basado en OS pipes y globales).
- Re-ejecutar la validación completa del CLI de Scouter para confirmar que el entorno legacy sigue operativo.