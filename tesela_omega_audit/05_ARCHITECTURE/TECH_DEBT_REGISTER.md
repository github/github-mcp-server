# TECH DEBT REGISTER
**Fecha:** 2026-06-22 (UTC)

| # | Deuda | Prioridad | Nota |
|---|---|---|---|
| TD1 | Deriva de migraciones (BD 23 / repo 22) | P1 | traer create_claude_sessions + funciones |
| TD2 | Tabla duplicada holded_facturas | P1 | unificar a factura_holded |
| TD3 | ERP en repo público | P1 | mover a privado |
| TD4 | FKs sin índice (~20) | P2 | crear al meter volumen |
| TD5 | RLS por fila / múltiples permissive | P2 | (select auth.fn()), consolidar |
| TD6 | 3 proyectos Supabase solapados | P1 | consolidar |
| TD7 | Modelo obra→capítulo→partida incompleto | P1 | diseñar |
| TD8 | Sin alertas observabilidad | P2 | cron/sync |
| TD9 | XSS latente comentado en app.js (onclick + esc) | P2 | revisar render de `referencia` |
