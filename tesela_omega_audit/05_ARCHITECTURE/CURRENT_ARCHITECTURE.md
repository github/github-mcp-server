# CURRENT ARCHITECTURE
**Fecha:** 2026-06-22 (UTC)

- **Backend/Datos:** Supabase Postgres 17 (erp-grupo-tesela, UE). RLS por rol. 3 Edge Functions (app, sync-holded, emit-facturas). pg_cron+pg_net. Vault. Storage privado.
- **Frontend:** SPA estática servida por Edge Function `app` (supabase-js + RLS).
- **Integración contable:** Holded (1/5 sociedades) por API directa; emisión frenada (dryRun).
- **Analítica:** `Tesela-iA-v.0` (DuckDB/BC3) fuera de Supabase.
- **Satélites Supabase:** TeseLAB (event-sourcing, vacío), CONEXION ERP DATAS (BI estrella, semilla).
- **Coordinación IA:** tabla claude_sessions (anon r/w ⚠️).

**Debilidades estructurales:** múltiples fuentes de verdad; IaC con deriva; ERP en repo público; sin backup robusto; barandilla de IA ausente.
