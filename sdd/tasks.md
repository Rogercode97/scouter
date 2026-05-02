# 🚀 Scouter Ascension (Wave 12.0) - HTDAG

Este archivo rastrea la evolución del servidor MCP de Scouter hacia los estándares de élite.

## 🔱 Mandatos Estratégicos
- [ ] **Paginación Soberana:** Implementar `limit` y `offset` en todos los listados.
- [ ] **Truth Kernels:** Filtrar respuestas JSON para maximizar la densidad de señal.
- [ ] **Reasoning Blocks:** Inyectar `<thought>` en los retornos de las herramientas.
- [ ] **RTK Muscle:** Delegar lecturas masivas al binario de RTK si está disponible.

## 📋 Tareas Pendientes

### Fase 1: Auditoría y Preparación
- [x] Investigar estándares Wave 12.0 y actualizar `mcp-builder`.
- [x] Auditar `internal/mcp/handlers.go` e identificar brechas.
- [x] Crear `openspec/changes/scouter-ascension/exploration.md`.

### Fase 2: Refactorización de Handlers
- [ ] Implementar paginación en `handleSearch`, `handleCallers` y `handleCritical`.
- [ ] Añadir bloques `<thought>` y discriminación de errores (`isError: true`).
- [ ] Integración explícita con `rtk read --ultra-compact` en `handleRead`.

### Fase 3: Recursos y Evaluación
- [ ] Exponer ADRs y Call Graph como `Resources` de MCP.
- [ ] Forjar `evaluation.xml` con 10+ casos de prueba QA.
- [ ] Ejecutar el "Hakaishin Trial" y verificar precisión > 90%.

### Fase 4: Sellado
- [ ] Actualizar `skill-registry.md`.
- [ ] Realizar autopsia de la mejora en Engram.

---
*El olvido es Slop. La estructura es Memoria. Hakai.*
