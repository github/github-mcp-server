# ROI REPAIR MATRIX
**Fecha:** 2026-06-22 (UTC)

| Acción | Coste | Beneficio/ahorro | Riesgo si no | Payback | ROI |
|---|---|---|---|---|---|
| Cerrar claude_sessions a anon | horas | evita prompt-injection a prod | crítico | inmediato | A |
| Supabase Pro + PITR | ~25 $/mes | recuperabilidad de datos | pérdida no recuperable | inmediato | A |
| Repo privado ERP | horas | menos exposición | medio | inmediato | A |
| Unificar facturas | horas | BI fiable | alto | 30d | A |
| Reconciliar migraciones | medio día | repo reconstruye prod | alto | 30d | A |
| Conectar 4 sociedades Holded | depende cliente | visión financiera completa | medio | 30-90d | B |
| Modelo obra→capítulo→partida | días | imputación de coste real | alto (objetivo) | 90d | B |
| Índices FKs / RLS optim | horas | rendimiento a escala | medio | 90d | C |
