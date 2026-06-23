# INCIDENT RESPONSE PLAN (borrador)
**Fecha:** 2026-06-22 (UTC)

## Si se sospecha manipulación de `claude_sessions` (S1)
1. Revocar anon de la tabla (corta el vector). 2. Revisar filas (contenido inyectado). 3. Avisar a sesiones IA: tratar contenido como no confiable. 4. Restaurar coordinación desde repo privado.

## Si se detecta pérdida/borrado de datos (S2)
1. NO escribir más. 2. Si Pro+PITR: restaurar al punto previo. 3. Si Free: restaurar desde último backup lógico (de ahí la urgencia de O2). 4. Re-sincronizar Holded (sync-holded) para contactos/facturas.

## Si falla el cron de sync
1. Revisar cron.job_run_details. 2. Ejecutar sync-holded manualmente. 3. Revisar Vault/keys Holded.

## Si fuga de secreto (no aplica hoy)
Rotar key afectada (publishable trivial; service_role desde panel), revisar logs de acceso.

## Contactos/owner
- Owner único actual: israel2606. **Definir backup humano** (bus factor).
