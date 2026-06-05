# 🧠 SABIDURIA (Wisdom) - Scouter

Este documento resume las decisiones arquitectónicas fundamentales y el "alma" técnica de Scouter. Es el mapa de navegación para cualquier agente o arquitecto que trabaje en el motor.

## 🔱 Deep AST Intelligence (Wave 15)

### Naming de Funciones Anónimas
Para que un Call Graph sea persistente y útil para el análisis de impacto, los nombres sintéticos de las funciones anónimas (closures/lambdas) deben ser **estables**.
- **Regla:** Se utiliza el formato jerárquico `Parent.funcN` (ej. `MyClass.MyMethod.func1`).
- **Unificación:** Esta convención está unificada en todos los lenguajes soportados (Go, TypeScript, Python, Rust) tanto en el motor AST nativo como en el motor Tree-sitter.
- **Por qué:** Los nombres basados en línea/columna (`func_12_5`) se rompen con cualquier cambio de formato. El nombre jerárquico sobrevive a refactorizaciones locales y permite seguir el flujo de datos de forma lógica.

### LSP Warp Speed (Persistent Daemon)
El arranque de un servidor LSP (como `gopls`) toma ~3 segundos. En un flujo de herramientas CLI, esto es inaceptable.
- **Implementación:** Scouter utiliza un **Auto-Daemon** persistente vía Unix Sockets (`scouter-gopls.sock`).
- **Protocolo:** El `lsp.Manager` intenta conectar al socket primero. Si falla, arranca el daemon en segundo plano (detacheado) y reconecta. Esto reduce la latencia de análisis de segundos a milisegundos.

### Mode Deep (SSA Analysis)
El análisis basado en AST simple es limitado para lenguajes con interfaces dinámicas como Go.
- **Implementación:** Scouter incorpora un motor de análisis SSA (Static Single Assignment) utilizando `CHA` (Class Hierarchy Analysis).
- **Activación:** Se activa mediante el flag `--deep` durante la indexación.
- **Ventaja:** Resuelve llamadas a través de interfaces y punteros con alta precisión, mapeando implementaciones concretas que el análisis sintético no puede detectar. Es fundamental para el "Impact Analysis" en sistemas desacoplados.

### Index Sharding (Enterprise Scale)
Para manejar codebases de millones de líneas sin degradación de performance, Scouter utiliza un sistema de sharding horizontal.
- **Estrategia:** Sharding por directorio raíz del proyecto o paquete (`shard-by-directory`).
- **Meta-Index:** Existe una base de datos maestra (`meta.db`) que actúa como catálogo de shards y almacena información global de dependencias y grafos de llamadas inter-shard.
- **Beneficio:** Permite paralelismo masivo en la indexación y evita que el índice FTS5 de SQLite crezca hasta volverse ineficiente.

## 🧱 Antifragilidad Termux/WASM (Wave 15.5)
Scouter se ejecuta frecuentemente bajo el intérprete Wazero en Termux (Android), el cual carece de JIT. El FFI entre Wasm y Host es un cuello de botella crítico.
- **Zero CGO:** Estrictamente prohibido. Usar `ncruces/go-sqlite3` y `goformer`.
- **Bulk Updates & Dual Pools:** Obligatorio usar agrupaciones masivas y Connection Pools duales en SQLite para evitar deadlocks ante el volumen paralelo del IndexerPipeline.

## 🏗️ Patrones de Diseño Core

### Hexagonal \u0026 Screaming Architecture
Scouter no es solo código; es una estructura que "grita" su intención.
- `internal/engine`: El cerebro. No conoce nada de MCP o CLI.
- `internal/mcp`: Los sentidos. Traduce el cerebro para el mundo exterior.
- **Sovereignty:** El motor de análisis es soberano; nunca depende de las herramientas que lo consumen.

### Staging Ledger (Verified Mutation)
Nunca escribimos directamente a disco durante refactorizaciones complejas.
- **Flujo:** Propuesta -> Ledger -> Impact Analysis -> Confirmación -> Commit.
- **Garantía:** Esto evita estados inconsistentes y permite al agente "sentir" el cambio antes de aplicarlo.

## 🛡️ Mandatos de Ingeniería
1. **TDD por Defecto:** Ninguna mejora en el motor se considera terminada sin su correspondiente integración en `treesitter_test.go` o `parser_test.go`.
2. **Context Efficiency:** Cada bit de información extraído del AST debe estar optimizado para no inflar el contexto del LLM innecesariamente (RTK Principles).

---
*La integridad estructural es la única fuente de verdad.*
