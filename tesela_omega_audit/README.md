# /tesela_omega_audit — Auditoría Omega v11 (Grupo Tesela)

**Fecha:** 2026-06-22 (UTC) · **Modo:** read-only (cero cambios) · **Evidencia:** Supabase MCP + GitHub MCP + workspace.

## Empieza por aquí
1. `00_EXECUTIVE/TESELA_OMEGA_EXECUTIVE_REPORT.md` — informe ejecutivo.
2. `00_EXECUTIVE/CEO_DECISION_BRIEF.md` — 4 decisiones + 2 P0.
3. `00_EXECUTIVE/TOP_20_RISKS.md` / `TOP_20_OPPORTUNITIES.md`.
4. `05_ARCHITECTURE/ERP_ESTADO_UNIFICADO.md` — estado real vs bitácora.
5. `ACCESS_REQUESTS.md` — qué falta para cerrar puntos ciegos.

## Carpetas
- 00_EXECUTIVE · 01_INVENTORY · 02_FORENSICS · 03_SECURITY · 04_COSTS · 05_ARCHITECTURE · 06_REPAIR · 07_PRODUCT_AND_BUSINESS · 08_DASHBOARDS · 09_APPENDICES

## Riesgos P0 (no ejecutar reparación sin aprobación)
- **R1** `claude_sessions` escribible por anónimos y consumida por agentes IA con escritura en producción.
- **R2** Datos reales sin PITR (Supabase plan Free).
