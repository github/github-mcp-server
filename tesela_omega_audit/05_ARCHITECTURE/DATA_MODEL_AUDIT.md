# DATA MODEL AUDIT
**Fecha:** 2026-06-22 (UTC). Fuente: list_tables, information_schema, migraciones.

## ERP (erp-grupo-tesela) — 20 tablas
Núcleo: sociedad, promocion, fase, unidad, cliente, proveedor, reserva, contrato_venta, hito_pago, presupuesto, partida, contrato_obra, certificacion, documento, perfil, acceso_promocion, factura_pendiente, factura_holded.
Añadidas (drift/no en docs): **holded_facturas**, **claude_sessions**.

### Problemas
1. **Duplicado de facturas:** `factura_holded`(62, usada por app) vs `holded_facturas`(1, sin política, fuera de migración). → unificar.
2. **`claude_sessions`** mezcla coordinación de IA dentro del esquema de negocio + anon r/w. → sacar/asegurar.
3. **Imputación de coste incompleta:** existen presupuesto/partida/contrato_obra/certificacion pero a 0 filas y sin el eslabón **capítulo** ni la cadena trazable presupuesto→pedido→compra→certificación→factura→cobro.
4. **FKs sin índice** (rendimiento) y **RLS subóptima** (ver advisors).

## BI (CONEXION ERP DATAS): dim_sociedad + fact_ventas/compras/horas_mo/tarifa_mo (estrella, semilla, sin políticas).
## Event-sourcing (TeseLAB): tabla `event` única (0 filas).

## Recomendación de modelo objetivo
- Una sola tabla de facturas con `tipo` (venta/compra) y `sociedad_id`, `proyecto/obra`, `capitulo`, `partida`.
- Catálogo capítulos/partidas (BC3) como dimensión; imputación de compras/certificaciones a partida.
- Claves Holded (`holded_id`) como enlace; `proyecto` obligatorio en facturas/compras para trazabilidad.
