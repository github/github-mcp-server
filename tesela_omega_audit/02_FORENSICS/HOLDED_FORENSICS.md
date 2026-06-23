# HOLDED / ERP FORENSICS
**Fecha:** 2026-06-22 (UTC). Verificado lo que llega a Supabase; el panel/API directa de Holded es `N/D` (sin acceso) → `ACCESS_REQUESTS.md`.

## Lo verificable (en Supabase)
- 1 de 5 sociedades conectada (`sociedad`=1 fila).
- 28 clientes + 19 proveedores importados (Holded → erp.cliente/proveedor).
- 62 facturas en `factura_holded`. **Split venta/compra: N/D** (la columna de tipo difiere de la asumida en la consulta; no se afirma sin verificar el nombre real de columna).
- Tabla duplicada `holded_facturas` (1 fila) — intento paralelo de export; **unificar/eliminar**.
- Sincronización vía cron diario (verificada hoy). Keys en Vault.

## No verificable sin acceso a Holded
duplicados de contactos, contactos sin NIF/CIF, facturas sin proyecto, compras sin obra, tags, plan contable, conciliación bancaria, proyectos, webhooks. → `N/D`.

## Recomendaciones
- Conectar las 4 sociedades restantes (keys al Vault).
- Definir clave estándar de proyecto/obra en Holded para trazabilidad presupuesto→compra→certificación→factura→cobro (hoy imposible de garantizar).
- Validar contra referencia Palmeres (ventas ≈1,385M€ / compras ≈1,257M€, de bitácoras) — requiere datos en DuckDB/Holded.
