# 🏛️ EL LIBRO DE SABIDURÍA DE SCOUTER (v1.0.0) 👑

Este documento contiene la visión técnica, lecciones aprendidas y el backlog de trascendencia de Scouter.

## ⚖️ Mandatos de Arquitectura

| Mandato | Descripción |
| :--- | :--- |
| **Absolute Signal** | Reducción de ruido en terminal para optimizar Ki (tokens). |
| **Context Sovereignty** | Gestión estricta de `context.Context` (Go 1.24+) para evitar bloqueos. |
| **Glasswall Validation** | Integridad de datos infranqueable mediante tags de validación. |
| **Hexagonal Isolation** | Desacoplamiento total entre lógica de negocio y persistencia. |

## 🧬 Evolución: El ADN de HAKAI

Integración de capacidades de alta eficiencia y análisis dinámico inspiradas en el ecosistema *jCodeHakai*.

### ⚡ Capacidades Core

| Capacidad | Objetivo | Mecánica |
| :--- | :--- | :--- |
| **HAKAI Density Format** | Reducción drástica del "Impuesto de Tokens" (Ki). | Serialización compacta con *path interning* para respuestas MCP masivas. |
| **PageRank (Inteligencia)** | Identificar los "hubs" arquitectónicos del sistema. | Algoritmo iterativo sobre el grafo de llamadas para ponderar importancia. |
| **Tectonic Mapping** | Comprender la fragilidad basada en historial. | Análisis de `git log` para detectar acoplamiento temporal (Co-Churn). |
| **AI Summaries** | Contexto instantáneo sin leer implementación. | Micro-resúmenes de funciones críticas generados por LLM. |
| **Freshness Scoring** | Garantizar la soberanía de la información. | Metadato de estado (`fresh`, `stale`) basado en hashes y mtime. |

## 🏹 Análisis de Mercado y Posicionamiento

Scouter se posiciona como el sucesor inteligente de los proxies de reducción de ruido:

| Herramienta | Enfoque | Diferencia vs Scouter |
| :--- | :--- | :--- |
| **RTK / RTK-AI** | Proxy CLI universal | Basado en patrones estáticos; Scouter usa AST. |
| **Tokf / Tokf.net** | Filtrado via TOML | Requiere autoría manual; Scouter es autónomo. |
| **Chop / getchop.run** | Interceptor de agentes | Heurístico; Scouter es semántico (Global Call Graph). |
| **Unlog** | Ingesta de logs + RCA | Post-mortem; Scouter es interactivo y preventivo. |

**El Diferencial Soberano**: Mientras otros "recortan" texto, Scouter **entiende la estructura**. Gracias al *Global Call Graph* y el *Blast Radius Analysis*, Scouter inyecta el contexto crítico que otros filtros eliminan por error.

## 📜 SDD: Spec-Driven Development

Scouter utiliza un flujo de trabajo basado en especificaciones (SDD) para garantizar trazabilidad:
- **Exploración Nativa**: `explore_sdd` permite navegar por propuestas y tareas sin salir del contexto.
- **Recursos Soberanos**: `scouter://sdd/roadmap` unifica el estado del proyecto.
- **Sincronización con Engram**: El estado de SDD se ancla a la memoria persistente.

## 🕸️ El Motor de Impacto (Blast Radius)

| Componente | Implementación |
| :--- | :--- |
| **Recursive CTE** | Uso de recursión nativa en SQLite para trazar impacto en grafos cíclicos. |
| **Fuzzy Joining** | Unión de nodos por nombre o path completo para máxima precisión. |
| **Cycle Immunity** | Implementado mediante `UNION` determinista para evitar bucles infinitos. |

### 📊 Métricas de Riesgo
- **Centrality (Indegree)**: Importancia estructural basada en el número de dependientes.
- **Fragility**: Probabilidad de fallo basada en el historial de tests y la profundidad de impacto.

## 🔮 Backlog de Omnisciencia (Futuras Implementaciones)

- [ ] **Búsqueda Híbrida**: Combinar BM25 (FTS5) con Embeddings vectoriales locales.
- [ ] **Mapa de Calor Visual**: Colorización de nodos de riesgo en gráficos Mermaid.
- [ ] **Traceado de Interfaces**: Resolución dinámica de llamadas a través de interfaces.
- [ ] **CI/CD Oracle**: Generación de planes de ejecución de tests optimizados para pipelines.
- [ ] **Persistent ULMEN Cache**: Almacenar hashes SHA-256 en SQLite indexados por `mtime` (Zero-Latency Oracle).

## 📜 Registro de Batalla (Lecciones Aprendidas)

| Incidente / Desafío | Resolución / Lección |
| :--- | :--- |
| **SQLITE_BUSY** | Resuelto mediante `signal.NotifyContext` y WAL mode. |
| **Foreign Key Sovereignty** | No se puede indexar una llamada sin un archivo previo. |
| **t.Context()** | Esencial para la limpieza de recursos en tests modernos de Go. |
| **CTE Efficiency** | La recursión en DB es órdenes de magnitud más rápida que en la app. |
| **SQL Metric Synthesis** | Calcular riesgos en SQL evita latencia de I/O en la aplicación. |
| **MCP Stabilization** | CLI Disconnection resuelto con `sync.Mutex` para `os.Stdout`. |

---
*Scouter no solo busca código; entiende su propósito.*