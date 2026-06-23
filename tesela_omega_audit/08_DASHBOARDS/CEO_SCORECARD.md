# CEO SCORECARD
**Fecha:** 2026-06-22 (UTC). 0-100 con evidencia.

| Área | Score | Evidencia | Riesgo | Próxima acción |
|---|---:|---|---|---|
| Arquitectura | 55 | núcleo OK; múltiples fuentes | P1 | fuente única |
| Backend | 70 | Supabase sano; edge OK; cron OK | P1 | reconciliar IaC |
| Frontend | 60 | app operativa | P2 | repo privado; XSS latente |
| Datos | 45 | duplicados; obra sin datos/modelo | P1 | unificar + modelo coste |
| IA | 50 | agentes útiles pero sin barandilla | P0 | asegurar claude_sessions |
| Automatización | 55 | cron OK; resto N/D | P2 | alertas; inventario |
| Seguridad | 60 | RLS OK, sin secretos; pero S1/S3 | P0 | revocar anon |
| Coste | 80 | casi todo Free | — | Pro (continuidad) |
| Escalabilidad | 45 | motor sí; gobierno no | P1 | modelo + única fuente |
| Operaciones | 55 | funciona; manual | P2 | usuarios reales |
| Documentación | 50 | existe pero desactualizada/derivada | P1 | ERP_ESTADO_UNIFICADO |
| Gobierno | 35 | sprawl, sin owner único | P1 | 1 repo=1 propósito |
| Continuidad | 35 | sin PITR, bus factor 1 | P0 | Pro+backup+runbook |

**Matriz:** P0 = {claude_sessions anon, sin PITR} · P1 = {fuente única, drift, repo público, sprawl, modelo coste} · P2 = {índices, RLS, alertas, usuarios} · P3 = {GraphQL grants, PR #13}.
