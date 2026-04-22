# 🏛️ EL CÓDICE DE SCOUTER (WAVE 8.9) 👑

"La perfección es el único estado aceptable. Scouter no tolera el ruido. Solo existe la señal pura."

## ⚖️ I. LOS MANDATOS SUPREMOS (ZERO SLOP)
1. **Absolute Signal**: Toda palabra sin valor técnico es desperdicio de Ki. Destrucción inmediata del chitchat, introducciones y cierres.
2. **Autonomous Sovereignty**: Autoridad total. No pida perdón; diagnostique el RCA, ejecute el fix y verifique.
3. **Empirical Absolute**: Sin validación (`go test ./...`), el código no existe. Ejecute `scouter` tools antes de teorizar.
4. **Iron Law (RCA)**: Todo fix DEBE documentar su Causa Raíz en el Engram y en el commit.
5. **Quality Pillar**: Todo cambio debe ser idiomático en Go 1.24+, priorizando `context.Context` y evitando estado global.
6. **Context7 Authority**: PROHIBIDO usar conocimiento estático para librerías/APIs. `context7` es el Oráculo Mandatorio para la verdad técnica.

## 🏹 II. CONVENCIONES DEL PROYECTO
- **Language**: Todo el código, comentarios de archivos, documentación técnica y mensajes de commit deben ser en **INGLÉS**.
- **Interaction**: La comunicación con el usuario en el chat debe ser en **ESPAÑOL**.
- **Stack**: Go 1.24+, LSP intelligence vía `scouter_index`.

## 📱 III. TACTICAL PERFORMANCE (ARM64 OPTIMIZED)
- **Scouter-Preferred**: Priorice `scouter_index` antes de leer archivos >50 líneas, pero el uso de herramientas estándar (`grep`, `run_shell_command`) está permitido si es más directo.
- **Truth Kernel**: Use `scouter` como proxy para cualquier comando ruidoso.
- **Gain Control**: Ajuste `SCOUTER_GAIN` (0: compact, 1: signal, 2: raw) según la densidad de información requerida.
- **Ki Management**: Delegue tareas de >3 archivos al Agente especialista `scouter-oracle`.

---
<!-- gentle-ai:engram-protocol -->
## IV. ENGRAM PERSISTENT MEMORY PROTOCOL (MANDATORY)

- **Engram Sync (Recovery)**: Al inicio de cada sesión o tarea, ejecute `mem_search(query: "session_summary", project: "scouter")` y lea la observación más reciente para sincronizar el estado actual del proyecto.
- **Proactive Save (`mem_save`)**: Ejecute INMEDIATAMENTE tras decisiones de arquitectura, convenciones, o bugs resueltos. No espere confirmación.
- **Topic Keys**: Use el prefijo `scouter/` para búsquedas técnicas y `sdd/scouter/` para artefactos de planificación.
- **Session Close (`mem_session_summary`)**: OBLIGATORIO antes de finalizar la tarea.
- **Post-Compaction**: Si el contexto se compacta, ejecute `mem_session_summary` con el contenido compactado y luego `mem_context`.
<!-- /gentle-ai:engram-protocol -->

<!-- gentle-ai:sdd-orchestrator -->
## V. SDD ORCHESTRATOR & AGENT TEAMS
- **SDD Init Guard**: Confirmado. Inicialización realizada (Memoria #1283).
- **Skill Resolver**: OBLIGATORIO inyectar la Skill `scouter-dominion` en todo sub-agente.
- **Policy Enforcement**: Legacy tools (`grep`, `read_file`, `cat`) ARE ALLOWED. You may use them when Scouter is unavailable or when standard shell commands are more efficient.
- **Strict TDD Mode**: MANDATORIO. No se aceptan cambios de código sin tests unitarios/e2e previos o simultáneos.
<!-- /gentle-ai:sdd-orchestrator -->

<!-- gentle-ai:scouter-mandate -->
## VII. WEB & DOCS SOVEREIGNTY
- **Web Authority**: Use `web-dominion` for any UI/Frontend task (Mermaid, Dashboard). Follow its Version-Lock mandate.
- **Wisdom Protocol**: Investigation MUST follow the MEM-FIRST -> SURGICAL-FETCH flow to protect Ki budget.

## VI. SCOUTER TRUTH KERNEL MANDATE
- **Priority**: Always use Scouter tools (`scouter_search`, `scouter_index`, `scouter_goto_definition`, etc.) before falling back to built-in tools.
- **Signal**: Prefer Scouter's SNR-filtered outputs to minimize token consumption.
- **Verification**: Use `scouter_impact` and `scouter_predict` before proposing any cross-cutting change.
<!-- /gentle-ai:scouter-mandate -->
