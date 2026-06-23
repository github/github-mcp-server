# CEO Decision Brief — Grupo Tesela
**Fecha:** 2026-06-22 (UTC) · 1 página · decisiones, no literatura.

## Lo que necesito que decidas (4)
| # | Decisión | Recomendación | Coste | Si no decides |
|---|---|---|---|---|
| D1 | Fuente única de la verdad | Supabase `erp-grupo-tesela` (operación) + `Tesela-iA-v.0`/DuckDB (análisis) | 0 € organizativo | Datos contradictorios, BI no fiable |
| D2 | Sacar ERP del repo público | Repo privado dedicado | horas | Exposición de arquitectura/URL |
| D3 | Supabase Pro (backups/PITR) | Sí, ya hay datos reales | ~25 $/mes | Pérdida de datos no recuperable |
| D4 | Cerrar `claude_sessions` a anon | Sí, esta semana | horas | Manipulación de agentes IA vía internet |

## Los 2 P0 que no debes tolerar
1. Tabla de coordinación IA **escribible por cualquiera** y obedecida por agentes que tocan producción.
2. **Sin backup fino** (plan Free) sobre 62 facturas + 47 contactos reales.

## Lo bueno (no tirar nada)
ERP funcionando, RLS por rol correcto, Holded sincronizando a diario (verificado hoy), sin secretos en el repo. Es **consolidar y respaldar**, no reconstruir.
