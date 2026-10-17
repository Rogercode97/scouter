# 🏛️ CHECKLIST DE TAREAS: SOBERANÍA DE RUTAS (WAVE 8.9) 👑

Este documento desglosa la implementación técnica para erradicar el ruido de rutas absolutas y blindar la integridad del sistema de archivos en Scouter.

## 🛡️ FASE 1: FUNDAMENTOS Y DETECCIÓN DINÁMICA
- [ ] **[SECURITY]** Implementar `GetRepoRoot()` en `internal/utils/security.go`.
    - Debe buscar de forma ascendente `go.mod` o `.git`.
    - Retornar error si no se encuentra raíz (Hakai al contexto huérfano).
- [ ] **[SECURITY]** Definir la `Blacklist` de Pureza en `internal/utils/security.go`.
    - Incluir: `.git`, `.ssh`, `.env`, `.scouter`, `node_modules`, `vendor`, `dist`, `build`, `.vscode`, `.idea`, `.DS_Store`.

## ⚔️ FASE 2: REFACTOR DE VALIDACIÓN (NÚCLEO)
- [ ] **[UTILS]** Modificar `ValidatePath(path string) (string, error)` en `internal/utils/security.go`.
    - **Paso 1**: Rechazar rutas absolutas (`filepath.IsAbs`).
    - **Paso 2**: Resolución obligatoria de symlinks mediante `filepath.EvalSymlinks`.
    - **Paso 3**: Validar anclaje. La ruta resultante debe estar contenida en el `RepoRoot` o en `os.TempDir()`.
- [ ] **[SECURITY]** Integrar validación contra `Blacklist` en `ValidatePath`.
    - Cualquier segmento de la ruta que coincida con la lista negra debe disparar un error de seguridad.

## 🗑️ FASE 3: ELIMINACIÓN DE DEUDA Y HARDCODES
- [ ] **[REFACTOR]** Localizar y destruir hardcodes de Termux (`/data/data/com.termux/...`).
    - Usar `scouter_search` para identificar ocurrencias en el código base.
    - Reemplazar por llamadas dinámicas a `GetRepoRoot()` o rutas relativas validadas.

## 🧪 FASE 4: BLINDAJE EMPÍRICO (VERIFICACIÓN)
- [ ] **[TEST]** Actualizar `internal/utils/utils_test.go` con los escenarios de la Spec.
    - Test: Rechazo de rutas absolutas (e.g., `/etc/passwd`).
    - Test: Resolución de symlinks maliciosos apuntando fuera del root.
    - Test: Bloqueo de archivos en `Blacklist` (e.g., `src/.env`).
    - Test: Validación exitosa de archivos dentro del `RepoRoot`.
- [ ] **[CI]** Ejecutar suite completa: `go test ./internal/utils/... -v`.

---
**ESTADO: READY FOR IMPLEMENTATION**
*Hakai al ruido. Solo señal.*