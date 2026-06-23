# PRODUCTION RISK REPORT
**Fecha:** 2026-06-22 (UTC)

| Riesgo de producción | Estado | Prioridad |
|---|---|---|
| Pérdida de datos (sin PITR, Free) | Presente | P0 |
| Manipulación vía agentes (claude_sessions anon) | Presente | P0 |
| Deriva de esquema (repo no reconstruye prod) | Presente | P1 |
| emit-facturas activado sin IVA/serie | Mitigado (dryRun) | P1 |
| Sin alertas de fallo cron/sync | Presente | P2 |
| Edge Functions verify_jwt=false | Aceptable (app pública + sync con auth propia + emit con service-role) | Info |

## Continuidad
- 1 sola persona (israel2606) admin de todo; sesiones IA artesanales. **Bus factor = 1** → R15.
- Recomendado: backups verificados, runbook de incidentes (`INCIDENT_RESPONSE_PLAN.md`), onboarding versionado.
