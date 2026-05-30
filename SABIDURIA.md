# 🧠 SABIDURIA (Wisdom) - Scouter

Este documento resume las decisiones arquitectónicas fundamentales y el "alma" técnica de Scouter. Es el mapa de navegación para cualquier agente o arquitecto que trabaje en el motor.

## 🔱 Deep AST Intelligence (Wave 15)

### Naming de Funciones Anónimas
Para que un Call Graph sea persistente y útil para el análisis de impacto, los nombres sintéticos de las funciones anónimas (closures/lambdas) deben ser **estables**.
- **Regla:** Se utiliza el formato jerárquico `Parent.funcN` (ej. `MyClass.MyMethod.func1`).
- **Por qué:** Los nombres basados en línea/columna (`func_12_5`) se rompen con cualquier cambio de formato. El nombre jerárquico sobrevive a refactorizaciones locales y permite seguir el flujo de datos de forma lógica.

### LSP Warp Speed (Persistent Daemon)
El arranque de un servidor LSP (como `gopls`) toma ~3 segundos. En un flujo de herramientas CLI, esto es inaceptable.
- **Implementación:** Scouter utiliza un **Auto-Daemon** persistente vía Unix Sockets (`scouter-gopls.sock`).
- **Protocolo:** El `lsp.Manager` intenta conectar al socket primero. Si falla, arranca el daemon en segundo plano (detacheado) y reconecta. Esto reduce la latencia de análisis de segundos a milisegundos.

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
