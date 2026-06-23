# SYSTEM MAP
**Fecha:** 2026-06-22 (UTC)

```
[Navegador] --https--> [Edge Function `app` (verify_jwt=false)] --supabase-js + RLS--> [Postgres erp-grupo-tesela]
                                                                       ^
[Holded API] <--API directa-- [Edge Function `sync-holded`] <--cron 06:00-- [pg_cron + pg_net]
[Holded API] <--API directa-- [Edge Function `emit-facturas` (dryRun, service-role)] <-- cola factura_pendiente
[Vault holded_sociedades] --get_holded_keys() (service_role)--> sync/emit
[Storage bucket `documentos` privado] <-- RLS por promoción
```
- **Producción de facto:** `erp-grupo-tesela` (UE/París).
- **Satélites:** `TeseLAB Invest` (event-sourcing, vacío), `CONEXION ERP DATAS` (BI estrella, semilla), `Tesela-iA-v.0` (DuckDB analítico, fuera de Supabase).
- **Coordinación IA:** tabla `claude_sessions` (⚠️ anon r/w).
