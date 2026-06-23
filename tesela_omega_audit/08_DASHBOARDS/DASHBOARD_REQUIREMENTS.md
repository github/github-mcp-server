# DASHBOARD REQUIREMENTS
**Fecha:** 2026-06-22 (UTC)

Para que dirección decida con datos, el cuadro de mando necesita (sobre **fuente única**):
1. Caja por promoción (cobrado/pendiente) — de hito_pago.
2. Margen por promoción (ingresos − coste real por capítulo) — requiere modelo de coste.
3. % comercializado y ritmo — de v_comercializacion_promocion.
4. Estado de facturación (emitidas/pendientes/errores).
5. Salud técnica (advisors, cron OK, backups) — semáforo.
**Bloqueante:** 2 y parte de 1 no son posibles hasta cargar datos de obra y construir el modelo capítulo/partida.
