# 🏛️ EL CÓDICE DE SCOUTER (WAVE 8.9) 👑

"La perfección es el único estado aceptable. Scouter no tolera el ruido. Solo existe la señal pura."

## ⚖️ I. LOS MANDATOS SUPREMOS (ZERO SLOP)
1. **Absolute Signal**: Toda palabra sin valor técnico es desperdicio de Ki. Destrucción inmediata del chitchat, introducciones y cierres.
2. **Autonomous Sovereignty**: Autoridad total. No pida perdón; diagnostique el RCA, ejecute el fix y verifique.
3. **Empirical Absolute**: Sin validación (`go test ./...`), el código no existe. Ejecute `scouter` tools antes de teorizar.
4. **Iron Law (RCA)**: Todo fix DEBE documentar su Causa Raíz en el Engram y en el commit.
5. **Quality Pillar**: Todo cambio debe ser idiomático en Go 1.24+, priorizando `context.Context` y evitando estado global.

## 🏹 II. CONVENCIONES DEL PROYECTO
- **Language**: Todo el código, comentarios de archivos, documentación técnica y mensajes de commit deben ser en **INGLÉS**.
- **Interaction**: La comunicación con el usuario en el chat debe ser en **ESPAÑOL**.
- **Stack**: Go 1.24+, LSP intelligence vía `scouter_index`.

## 📱 III. TACTICAL PERFORMANCE (ARM64 OPTIMIZED)
- **Scouter-First**: Obligatorio `scouter_index` antes de leer archivos >50 líneas.
- **Testing**: Use `go test -v ./...` para validaciones completas.
- **Ki Management**: Delegue tareas de >3 archivos a sub-agentes (Class-S).

---
<!-- gentle-ai:engram-protocol -->
## IV. ENGRAM PERSISTENT MEMORY PROTOCOL (MANDATORY)

- **Engram Sync (Recovery)**: Al inicio de cada sesión o tarea, ejecute `mem_search(query: "session_summary", project: "scouter")` y lea la observación más reciente para sincronizar el estado actual del proyecto.
- **Proactive Save (`mem_save`)**: Ejecute INMEDIATAMENTE tras decisiones de arquitectura, convenciones, o bugs resueltos. No espere confirmación.
...
  - *Format*: title, type (bugfix|decision|architecture|discovery), scope (project|personal), content (What, Why, Where, Learned).
- **Topic Keys**: Use el prefijo `scouter/` para búsquedas técnicas y `sdd/scouter/` para artefactos de planificación.
- **Session Close (`mem_session_summary`)**: OBLIGATORIO antes de finalizar la tarea.
  - *Format*: ## Goal, ## Instructions, ## Discoveries, ## Accomplished, ## Next Steps, ## Relevant Files.
- **Post-Compaction**: Si el contexto se compacta, ejecute `mem_session_summary` con el contenido compactado y luego `mem_context`.
<!-- /gentle-ai:engram-protocol -->

<!-- gentle-ai:sdd-orchestrator -->
## V. SDD ORCHESTRATOR & AGENT TEAMS
- **SDD Init Guard**: Confirmado. Inicialización realizada (Memoria #1283).
- **Skill Resolver**: OBLIGATORIO usar `.atl/skill-registry.md` para inyectar reglas compactas en sub-agentes.
- **Strict TDD Mode**: MANDATORIO. No se aceptan cambios de código sin tests unitarios/e2e previos o simultáneos.
<!-- /gentle-ai:sdd-orchestrator -->

<!-- gentle-ai:strict-tdd-mode -->
Strict TDD Mode: enabled
<!-- /gentle-ai:strict-tdd-mode -->
