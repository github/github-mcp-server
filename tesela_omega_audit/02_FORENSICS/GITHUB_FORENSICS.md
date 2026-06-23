# GITHUB FORENSICS
**Fecha:** 2026-06-22 (UTC). Fuente: git local + GitHub MCP (get_me, search_repositories, list_pull_requests).

## Repo de trabajo (github-mcp-server)
- Rama actual: `claude/connector-recommendations-hzwhi4`; remotas: main, claude/connector-recommendations-hzwhi4. Sin tags, sin stash, working tree limpio (solo `tesela_omega_audit/` sin trackear).
- Historial Tesela (selección): 64a37f8 fix dashboard promoción sin unidades · b35751c emit-facturas + fix timeout · 59af5bd alta promociones · 23b05d9 sync diario · 190a6f3 importar facturas · 7c9e61c Holded 1 sociedad.
- **Hallazgo:** este repo es un **fork público** de GitHub MCP Server que además aloja el ERP (supabase/app/docs). Mezcla de propósitos + exposición pública del ERP (R5).

## Cuenta israel2606
- 7 repos (2 públicos: github-mcp-server, awesome-diagrams; 5 privados Tesela). Ver `01_INVENTORY/REPOSITORY_MAP.md`.

## PRs abiertos
- #14 (draft): cobertura tests `cmd/mcpcurl` (upstream técnico, sin riesgo).
- #13: `list_issue_types` repo-scoped, **base apunta a `claude/add-claude-documentation-4JuOo`** en vez de `main` → merge accidental a rama equivocada. Corregir base o cerrar.

## No verificado (sin acceso a repos privados desde el conector)
Forense por repo (SOT, Tesela-iA-v.0, Cerebro, command-center, Inversiones): `N/D` → `ACCESS_REQUESTS.md`.

## Secretos en repo
`git grep` de patrones (service_role/JWT/sk-/ghp_/.env): **sin hallazgos reales**. Solo placeholders de docs y el código del propio github-mcp-server que maneja tokens `ghp_`. La publishable key en `app/config.js` es pública por diseño. Detalle: `03_SECURITY/SECRETS_EXPOSURE_REPORT.md`.
