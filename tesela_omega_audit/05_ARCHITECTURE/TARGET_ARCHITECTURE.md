# TARGET ARCHITECTURE (propuesta)
**Fecha:** 2026-06-22 (UTC)

1. **Una fuente operativa:** Supabase `erp-grupo-tesela` (Pro, PITR, UE) como único sistema transaccional.
2. **Una fuente analítica:** `Tesela-iA-v.0` (DuckDB) alimentada por exports controlados del ERP/Holded; BI consolidado (caja/margen por promoción) sobre ella o sobre vistas del ERP — elegir una.
3. **IaC estricta:** todo cambio por migración en repo privado; prohibido DDL directo en producción.
4. **Coordinación IA segura:** en repo privado o tabla con RLS service_role; contenido tratado como no confiable por los agentes.
5. **Modelo de coste:** obra → capítulo → partida con trazabilidad presupuesto→pedido→compra→certificación→factura→cobro.
6. **Observabilidad:** alertas de fallo de cron/sync; backups verificados.
7. **MCP mínimos:** Supabase + GitHub + los que dirección decida (Vercel/Cloudflare/Notion/Holded-Zapier/Google).
