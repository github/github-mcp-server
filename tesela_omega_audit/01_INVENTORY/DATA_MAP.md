# DATA MAP — dónde vive cada dato
**Fecha:** 2026-06-22 (UTC)

| Dominio | Sistema canónico hoy | Duplicado/competencia | Veredicto |
|---|---|---|---|
| Clientes/Proveedores | erp.cliente(28)/proveedor(19) | Holded (origen), DuckDB | OK; origen Holded |
| Facturas | erp.factura_holded(62) | erp.holded_facturas(1) ⚠️, Holded, DuckDB | **Unificar** a una tabla |
| Promociones/Obra | erp (promocion/fase/unidad/...) 0 filas | — | Esquema listo, sin datos |
| Presupuesto/partida | erp (presupuesto/partida) 0 filas | DuckDB/BC3 | Falta modelo capítulo |
| BI agregados | CONEXION ERP DATAS (dim/fact, semilla) | v_* vistas en erp | Decidir cuál manda |
| Eventos financieros | TeseLAB `event` (0 filas) | — | Sin uso aún |
| Coordinación IA | claude_sessions (erp + CONEXION) | Drive (obsoleto) | ⚠️ sacar de tabla pública |

**Conclusión:** la verdad está repartida. Definir D1 (fuente única) es prerequisito de BI fiable.
