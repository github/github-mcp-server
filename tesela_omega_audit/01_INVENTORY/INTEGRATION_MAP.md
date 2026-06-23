# INTEGRATION MAP
**Fecha:** 2026-06-22 (UTC)

| Integración | Estado | Evidencia | Riesgo |
|---|---|---|---|
| Holded → Supabase (lectura) | ✅ 1/5 sociedades, cron diario OK | sync-holded v6; cron OK 06:00 | 4 sociedades pendientes |
| Supabase → Holded (emisión) | 🟡 construida, frenada (dryRun) | emit-facturas v4 | activar sin IVA/serie = error fiscal |
| Holded vía Zapier | 🟡 mencionado en bitácoras | N/D en vivo | sin verificar |
| App web ↔ Supabase | ✅ supabase-js + RLS | Edge Function app v12 | — |
| pg_cron + pg_net | ✅ activo | cron.job/job_run_details | sin alertas de fallo |
| Vercel/Cloudflare/Notion/Google/n8n/Make | N/D | sin acceso | puntos ciegos |
