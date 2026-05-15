# 🏛️ EL LIBRO DE SABIDURÍA DE SCOUTER (WAVE 9) 👑

Este documento contiene la visión técnica, lecciones aprendidas y el backlog de trascendencia de Scouter.

## ⚖️ Mandatos de Arquitectura
1. **Absolute Signal**: Reducción de ruido en terminal para optimizar Ki (tokens).
2. **Context Sovereignty**: Gestión estricta de `context.Context` (Go 1.24) para evitar bloqueos.
3. **Glasswall Validation**: Integridad de datos infranqueable mediante tags de validación.
4. **Hexagonal Isolation**: Desacoplamiento total entre lógica de negocio y persistencia.

## 🧬 Evolución: El ADN de HAKAI (Wave 14.0) 🧪
Integración de capacidades de alta eficiencia y análisis dinámico inspiradas en el ecosistema *jCodeHakai*.

### ⚡ Capacidades Core
1. **High Density Format (HAKAI)**:
   - **Objetivo**: Reducción drástica del "Impuesto de Tokens" (Ki).
   - **Implementación**: `internal/display/density.go`.
   - **Mecánica**: Serialización compacta (CSV-like) con *path interning* para respuestas MCP masivas.

2. **PageRank (Inteligencia Central)**:
   - **Objetivo**: Identificar los verdaderos "hubs" arquitectónicos del sistema.
   - **Implementación**: Evolución de `internal/engine/analyzer.go`.
   - **Mecánica**: Algoritmo iterativo sobre el grafo de llamadas para ponderar la importancia estructural.

3. **Tectonic Mapping (Git Co-Churn)**:
   - **Objetivo**: Comprender la fragilidad basada en el historial de cambios.
   - **Implementación**: `internal/engine/churn.go`.
   - **Mecánica**: Análisis de `git log` para detectar archivos que cambian simultáneamente (acoplamiento temporal).

4. **AI Summaries (Semantic Indexing)**:
   - **Objetivo**: Proporcionar contexto instantáneo sin leer la implementación completa.
   - **Implementación**: `internal/engine/enricher.go`.
   - **Mecánica**: Generación proactiva de micro-resúmenes de funciones críticas mediante LLM durante el proceso de *Enrich*.

5. **Freshness Scoring (Index Health)**:
   - **Objetivo**: Garantizar la soberanía de la información mostrada.
   - **Implementación**: `internal/engine/truth.go`.
   - **Mecánica**: Metadato de estado (`fresh`, `stale`, `edited`) basado en hashes y mtime comparados en tiempo real.

## 🏹 Análisis de Mercado y Posicionamiento (Omnisciencia)
Tras una investigación profunda del ecosistema (Abril 2026), Scouter se posiciona como el sucesor inteligente de los proxies de reducción de ruido:

| Herramienta | Enfoque | Diferencia vs Scouter |
| :--- | :--- | :--- |
| **RTK / RTK-AI** | Proxy CLI universal | Basado en patrones estáticos; Scouter usa AST. |
| **Tokf / Tokf.net** | Filtrado via TOML | Requiere autoría manual; Scouter es autónomo. |
| **Chop / getchop.run** | Interceptor de agentes | Heurístico; Scouter es semántico (Global Call Graph). |
| **Unlog** | Ingesta de logs + RCA | Post-mortem; Scouter es interactivo y preventivo. |

**El Diferencial Soberano (V2.0)**: Mientras otros "recortan" texto, Scouter **entiende la estructura**. Gracias al *Global Call Graph* y el *Blast Radius Analysis*, Scouter no solo ahorra tokens, sino que inyecta el contexto crítico que otros filtros eliminan por error.

## 📜 SDD: Spec-Driven Development (Sovereign Tracking)
Scouter utiliza un flujo de trabajo basado en especificaciones (SDD) para garantizar que los cambios arquitectónicos sean intencionales y trazables:
- **Exploración Nativa**: Mediante `explore_sdd`, los agentes pueden navegar por el historial de propuestas, especificaciones técnicas y tareas pendientes sin salir del contexto de ejecución.
- **Recursos Soberanos**: Las rutas `scouter://sdd/roadmap` y `scouter://sdd/tasks` proporcionan una visión unificada del estado del proyecto, eliminando la fragmentación entre archivos de texto planos.
- **Sincronización con Engram**: El estado de SDD se ancla a la memoria persistente, permitiendo la recuperación de contexto a través de sesiones.

## 🕸️ El Motor de Impacto (Blast Radius)
- **Recursive CTE**: Uso de recursión nativa en SQLite para trazar el impacto en grafos cíclicos.
- **Fuzzy Joining**: Capacidad de unir nodos por nombre o por path completo para máxima precisión.
- **Cycle Immunity**: Implementado mediante `UNION` determinista para evitar bucles infinitos.

## 📊 Métricas de Riesgo (Risk Map)
- **Centrality (Indegree)**: Importancia estructural basada en el número de dependientes.
- **Fragility**: Probabilidad de fallo basada en el historial de tests y la profundidad de impacto.

## 🔮 Backlog de Omnisciencia (Futuras Implementaciones)
1. **🧠 Búsqueda Híbrida**: Combinar BM25 (FTS5) con Embeddings vectoriales locales para búsqueda semántica.
2. **📈 Mapa de Calor Visual**: Colorización de nodos de riesgo en gráficos Mermaid (Gris -> Rojo).
3. **⚡ Predictive Testing**: Sugerir tests basados en el Blast Radius de cambios locales (Git Diff). [IMPLEMENTADO]
4. **🧬 Traceado de Interfaces**: Resolución dinámica de llamadas a través de interfaces mediante análisis de tipos.
5. **📡 CI/CD Oracle**: Generación de planes de ejecución de tests optimizados para pipelines.
6. **🛡️ Persistent ULMEN Cache**: Almacenar hashes SHA-256 en SQLite indexados por `mtime` para validación instantánea de contexto (Zero-Latency Oracle).

## 🗺️ Hoja de Ruta: Scouter Wave 12.0 (Sovereign & Lean) [BACKLOG]

1. **Paso 1: Persistent HAKAI (Optimización)**: Implementar la tabla `symbol_hashes` para persistir el Evidence Network (ULMEN) y evitar re-computación.
2. **Paso 2: Staging Ledger (Transparencia)**: Crear `internal/mcp/ledger.go` y el recurso `scouter://staging/ledger`. Evita "blind commits" y ahorra 30% de Ki.
3. **Paso 3: Protocolo Shinigami (Inteligencia)**: Evolucionar `healer.go` a un modelo de muestreo paralelo (Solver-Verifier) con tests de reproducción automáticos.
4. **Paso 4: Interface Omniscience (Ripple V2)**: Resolver la deuda técnica de propagación a través de interfaces detectada en la sesión del 2026-05-03.

## 📜 Registro de Batalla (Lecciones Aprendidas)
- **SQLITE_BUSY**: Resuelto mediante `signal.NotifyContext` y WAL mode.
- **Foreign Key Sovereignty**: No se puede indexar una llamada sin un archivo previo.
- **t.Context()**: Esencial para la limpieza de recursos en tests modernos.
- **⚡ CTE Efficiency**: La recursión en DB es órdenes de magnitud más rápida.
- **📉 SQL Metric Synthesis**: Calcular riesgos en SQL evita latencia de I/O en la aplicación.

---
*Scouter no solo busca código; entiende su propósito.*

## [2026-04-22] MCP Stabilization: Framing & Concurrency Victory
- **Symptom**: CLI Disconnection.
- **RCA**: Double newlines (\n\n) from manual injection and lack of Mutex for os.Stdout.
- **Resolution**: Implementation of 'mu sync.Mutex' in Server and removal of 'fmt.Fprint' and 'os.Stdout.Sync()'.
- **Verdict**: Divine Signal restored. Rating 10.0.
