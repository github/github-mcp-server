# SECURITY BACKLOG (priorizado)
**Fecha:** 2026-06-22 (UTC)

| # | Acción | Prioridad | Reversible | Esfuerzo |
|---|---|---|---|---|
| 1 | Revocar anon en claude_sessions (erp + CONEXION) | P0 | sí | bajo |
| 2 | Supabase Pro + PITR + backup lógico | P0 | sí | bajo |
| 3 | Revocar EXECUTE anon en rls_auto_enable | P1 | sí | bajo |
| 4 | Revisar EXECUTE authenticated de helpers | P1 | sí | bajo |
| 5 | Activar leaked-password protection | P2 | sí | trivial |
| 6 | Reconciliar migraciones (drift) | P1 | sí | medio |
| 7 | Mover ERP a repo privado | P1 | sí | medio |
| 8 | Alertas de fallo cron/sync | P2 | sí | bajo |
| 9 | Revocar SELECT GraphQL innecesario | P3 | sí | bajo |
| 10 | Crear usuarios reales + validar RLS | P2 | sí | bajo |
