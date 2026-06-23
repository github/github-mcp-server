# TEST PLAN
**Fecha:** 2026-06-22 (UTC)

| Cambio | Test | Criterio de éxito |
|---|---|---|
| Revocar anon claude_sessions | intentar SELECT/INSERT con anon key | 401/0 filas |
| Pro + PITR | restaurar a sandbox | datos íntegros |
| Unificar facturas | conteos antes/después | 62(+1) consolidadas, 0 pérdida |
| Reconciliar migraciones | `db reset` en branch | esquema == producción |
| Índices FKs | EXPLAIN en queries clave | uso de índice |
| Modelo coste | insertar caso obra demo | imputación capítulo/partida correcta |
| RLS por rol | usuarios reales dirección/obra/comercial | aislamiento correcto |
