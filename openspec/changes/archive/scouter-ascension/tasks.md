<tasks project="scouter" change="scouter-ascension">
  <task id="1.0.0" status="pending" type="milestone">
    <name>Fase 1: Foundation (Paginación en Store/Engine)</name>
    <files>internal/store/store.go, internal/engine/search.go</files>
    <action>
      Modificar las interfaces del Store y Engine para soportar los parámetros 'limit' y 'offset' en las consultas de búsqueda y obtención de Callers.
    </action>
    <verify>go test ./internal/store/... ./internal/engine/...</verify>
    <commit>feat(engine): add offset support for pagination</commit>
  </task>

  <task id="1.1.0" status="done" depends_on="1.0.0" type="task">
    <name>Actualizar structs de parámetros MCP</name>
    <files>internal/mcp/handlers.go</files>
    <action>
      Añadir los campos `Limit int` y `Offset int` a las estructuras `SearchParams`, `CallersParams` y `CriticalParams` en `handlers.go`.
      Asegurar que los valores por defecto sean razonables (ej. Limit=50).
    </action>
    <verify>go build ./internal/mcp/...</verify>
    <commit>feat(mcp): add pagination fields to param structs</commit>
  </task>

  <task id="2.0.0" status="pending" depends_on="1.1.0" type="milestone">
    <name>Fase 2: Refactorización de Handlers (Truth Kernels & Thoughts)</name>
    <files>internal/mcp/handlers.go</files>
    <action>
      Refactorizar `handleSearch`, `handleCallers` y `handleCritical` para usar los nuevos campos de paginación.
      Modificar el retorno de los resultados para inyectar un `<thought>` inicial explicando brevemente la búsqueda realizada, seguido de los resultados filtrados (Truth Kernel).
      Implementar `isError: true` explícito en fallos lógicos.
    </action>
    <verify>go test ./internal/mcp/...</verify>
    <commit>refactor(mcp): implement pagination and reasoning blocks in list handlers</commit>
  </task>

  <task id="2.1.0" status="done" depends_on="2.0.0" type="task">
    <name>Integrar RTK Muscle en handleRead</name>
    <files>internal/mcp/handlers.go</files>
    <action>
      Refactorizar `handleRead` para que, antes de intentar la lectura manual, verifique la existencia del binario `rtk` y delegue la ejecución a `rtk read <file> --pointer <pointer> --ultra-compact`.
    </action>
    <verify>go build ./internal/mcp/...</verify>
    <commit>feat(mcp): delegate file reads to rtk for pure signal</commit>
  </task>

  <task id="3.0.0" status="done" depends_on="2.1.0" type="milestone">
    <name>Fase 3: Resources y Anotaciones</name>
    <files>internal/mcp/server.go</files>
    <action>
      Añadir anotaciones `destructiveHint: true` a las herramientas que mutan estado (`evolve`, `self_heal`, `ripple_refactor`).
      Implementar el registro de `Resources` para exponer el estado interno (ej. la versión actual, el path del workspace y los ADRs) como lecturas seguras.
    </action>
    <verify>go run ./cmd/scouter --version</verify>
    <commit>feat(mcp): add tool annotations and register read-only resources</commit>
  </task>

  <task id="4.0.0" status="pending" depends_on="3.0.0" type="milestone">
    <name>Fase 4: Hakaishin Trial (Evaluación)</name>
    <files>tests/evaluation.xml</files>
    <action>
      Crear el archivo `tests/evaluation.xml` con 10+ pares de Pregunta/Respuesta multi-salto que exijan usar `search`, `callers` e `impact` de manera paginada para verificar el correcto funcionamiento del servidor MCP.
    </action>
    <verify>python scripts/evaluation.py tests/evaluation.xml</verify>
    <commit>test(mcp): forge evaluation suite for hakaishin trial</commit>
  </task>
</tasks>