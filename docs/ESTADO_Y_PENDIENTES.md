# Estado y Pendientes

> Última actualización: 2026-06-08. Rama: `claude/test-coverage-analysis-s33ri0` (PR #5).

## Nota sobre el proyecto

Este repositorio es el **GitHub MCP Server**, escrito en **Go** (no Python).
No hay `requirements.txt`, ni `src/`, ni carpeta `data/`, ni ficheros `.env`.
La autenticación se hace en runtime con la variable de entorno
`GITHUB_PERSONAL_ACCESS_TOKEN`; no se guardan claves en el repo.

## Qué hace este PR

Tests **complementarios** para `pkg/lockdown` (la frontera de confianza de
contenido). El PR #3, ya fusionado en `main`, había subido la cobertura del
paquete a **83.1%** con `safety_test.go`. Este PR añade los caminos que aún
quedaban sin cubrir, en un fichero nuevo (`safety_coverage_test.go`) para no
duplicar símbolos:

- `getRepoAccessInfo`: 67.9% → 96.4% (camino de "repo cacheado, usuario nuevo").
- `queryRepoAccessInfo`: 93.3% → 100% (error devuelto por el servidor GraphQL).
- `log`: 40% → 100% (ramas de emisión y de nivel por debajo del umbral).
- **Total `pkg/lockdown`: 83.1% → 98.7%.**

Sin cambios en código de producción.

## Pruebas (local)

| Comprobación | Resultado |
|---|---|
| `go build ./...` | ✅ OK |
| `go vet ./pkg/lockdown/` | ✅ limpio |
| `go test ./pkg/lockdown/` | ✅ pasa (incl. tests de `main`) |

## Seguridad

- ✅ Sin API keys ni secretos en el repo (escaneo de patrones de tokens: 0 hits).
- ✅ El token va por entorno (`GITHUB_PERSONAL_ACCESS_TOKEN`), no en fichero.
- ✅ `.gitignore` excluye binarios, `dist/`, `bin/`, `vendor`, artefactos de IDE.
- ⚠️ No existe `data/` ni `.env` en el repo; si se añadieran, habría que
  ignorarlos explícitamente.

## Pendiente / siguiente paso

- **CI**: el job `build (3.9/3.10/3.11)` (workflow de Python con `pytest`) falla
  en **cualquier** PR porque no hay tests Python en este repo Go (exit 5,
  "no tests ran"). Es un problema de configuración del repo, ajeno a este PR.
  Conviene desactivar o corregir ese workflow por separado.
- **Cobertura**: siguientes paquetes con hueco — `pkg/github/actions.go`
  (handlers de workflow jobs/artifacts/rerun), `internal/ghmcp` (hosts
  GHES/GHEC) y `cmd/mcpcurl`.
