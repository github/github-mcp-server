# REBUILD PLAN — eliminar / reparar / reconstruir
**Fecha:** 2026-06-22 (UTC)

| Activo | Estado | Acción | Motivo | Prioridad |
|---|---|---|---|---|
| ERP erp-grupo-tesela | útil, recuperable | **Reparar** | núcleo bueno; faltan barandillas | P0/P1 |
| claude_sessions (config actual) | peligroso | **Reparar** (asegurar) o **mover** | anon r/w | P0 |
| holded_facturas | duplicado | **Eliminar** (tras unificar) | confusión de datos | P1 |
| Migraciones (drift) | roto parcial | **Reparar** (reconciliar) | repo no reconstruye | P1 |
| Sprawl de repos | confuso | **Fusionar/declarar owner** | gobierno | P1 |
| CONEXION ERP DATAS / TeseLAB | inmaduros | **Decidir** (mantener/pausar) | semillas sin uso | P2 |
| Modelo de coste | incompleto | **Construir** obra→capítulo→partida | objetivo de negocio | P1 |

**No hace falta reconstruir el núcleo.** Reconstruir solo el **modelo de coste** (que no existe) y **el gobierno** (que falta).
