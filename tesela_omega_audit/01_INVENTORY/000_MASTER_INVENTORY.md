# 000 — Inventario Maestro
**Fecha:** 2026-06-22 (UTC). Solo activos verificados en vivo o en el workspace.

## Workspace (repo github-mcp-server, rama claude/connector-recommendations-hzwhi4)
567 ficheros (excl. .git/node_modules/build). Artefactos Tesela:
- `supabase/migrations/` (22 .sql) · `supabase/functions/` (app, sync-holded, emit-facturas) · `supabase/config.toml`
- `app/` (index.html, app.js, styles.css, config.js, README)
- `docs/erp-grupo-tesela/` (ARQUITECTURA, MODELO-DATOS, ESTADO, INDICE, README, prototipo/)
- Resto: proyecto upstream GitHub MCP Server (Go).

## Supabase (org uladuspfccwdyrmyklnk) — 3 proyectos
| Proyecto | Ref | Región | Tablas | Datos reales |
|---|---|---|---|---|
| erp-grupo-tesela | jpojckqnhepiuwefyvdr | eu-west-3 | 20 | cliente 28, proveedor 19, factura_holded 62, sociedad 1, perfil 1 |
| TeseLAB Invest | umyejimabqcslsrymwus | eu-central-1 | 1 (event) | 0 |
| CONEXION ERP DATAS | akomftsfbnucktrladce | eu-central-1 | 6 (claude_sessions, dim_sociedad, fact_*) | 1-7 semilla |

- Edge Functions (erp): app v12, sync-holded v6, emit-facturas v4 (ACTIVE).
- Cron (erp): holded-sync-diario 0 6 * * * activo (último OK 2026-06-22 06:00).
- Storage (erp): bucket `documentos` privado. Vault: `holded_sociedades`.

## GitHub (israel2606): 7 repos → `REPOSITORY_MAP.md`.
## MCP ecosystem → `02_FORENSICS/MCP_ECOSYSTEM_REPORT.md`.
## No inventariable desde esta sesión → `ACCESS_REQUESTS.md`.
