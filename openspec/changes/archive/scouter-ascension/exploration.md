## Exploration: Scouter Ascension (MCP Wave 12.0)

### Current State
El servidor MCP de Scouter, definido en `internal/mcp/server.go` y `internal/mcp/handlers.go`, funciona como una pasarela para el `TruthEngine`. Aunque es robusto, opera bajo un paradigma pre-Wave 12.0:
- **Sobrecarga de Contexto:** Las listas (como resultados de búsqueda o llamadores) se devuelven en masa (con un tope estático de 500), lo que puede ahogar el contexto del LLM.
- **Opacidad Cognitiva:** Las respuestas son volcados de structs JSON. Faltan bloques de razonamiento (`<thought>`) que expliquen por qué se encontró un símbolo o cómo interpretar el radio de explosión (blast radius).
- **Subutilización de "Resources":** Toda interacción se da a través de Tools (llamadas de función), sin exponer el estado subyacente (ej. ADRs) como Recursos leíbles.

### Affected Areas
- `internal/mcp/handlers.go` — Requiere reescribir los retornos para inyectar `<thought>`, filtrar el JSON al "Truth Kernel" y soportar paginación.
- `internal/mcp/server.go` — Necesita exponer `Resources` y registrar adecuadamente las intenciones (`Annotations`).
- `internal/store/` y `internal/engine/` — Necesitarán soporte para `limit` y `offset` en las consultas a la base de datos SQLite.

### Approaches

1. **La Actualización Quirúrgica (Sovereign Patch)**
   - Solo añadir paginación a `handleSearch` y truncar los JSON de respuesta (Truth Kernels).
   - *Pros:* Rápido, bajo riesgo de regresiones.
   - *Cons:* No cumple completamente el estándar Wave 12.0. Faltan recursos y evaluación formal.
   - *Effort:* Low.

2. **La Ascensión Completa (Wave 12.0 Compliance) [RECOMENDADO]**
   - **Paginación:** Implementar `offset` en los structs de parámetros.
   - **Truth Kernels & Thoughts:** Envolver las salidas de las herramientas en un formato XML/JSON estructurado que empiece con el razonamiento del servidor.
   - **Evaluación:** Crear `evaluation.xml` como prueba de fuego.
   - **Recursos:** Exponer la carpeta `docs/adr/` y el estado general vía MCP Resources.
   - *Pros:* Resuelve de raíz el consumo de tokens y mejora drásticamente la utilidad agentica.
   - *Cons:* Requiere modificar la firma de las consultas al store.
   - *Effort:* Medium-High.

### Recommendation
Proceder con la **Ascensión Completa (Enfoque 2)**. Un servidor MCP como Scouter, diseñado para manejar ASTs complejos y call graphs, no puede permitirse ineficiencias de token. Reducir la huella de contexto paginando y destilando la señal compensará el esfuerzo de refactorización en pocos días de uso intensivo.

### Risks
- Modificar el esquema de los parámetros (`offset`) puede romper agentes que tengan las firmas de las herramientas cacheadas. Se requiere un aviso de cambio de versión.
- El rendimiento de SQLite con consultas paginadas muy profundas (`OFFSET 1000`) podría degradarse, aunque en contexto de IA rara vez pasaremos de la página 3.

### Ready for Proposal
**Yes**. La propuesta está madura. Podemos proceder a la Fase 2 del HTDAG y comenzar a refactorizar los Handlers.
