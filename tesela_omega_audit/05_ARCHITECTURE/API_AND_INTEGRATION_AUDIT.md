# API AND INTEGRATION AUDIT
**Fecha:** 2026-06-22 (UTC)

| Endpoint/función | Auth | Estado | Nota |
|---|---|---|---|
| Edge `app` | verify_jwt=false (login en cliente) | OK | datos por RLS |
| Edge `sync-holded` | verify_jwt=false (cron sin auth header) | OK | no expone GET; lee Vault, escribe ERP |
| Edge `emit-facturas` | verify_jwt=false + exige service-role en Authorization | OK (frenado dryRun) | propia auth; no invocable por navegador |
| RPC get_holded_keys | service_role | OK | correcto |
| RPC rls_auto_enable | anon/authenticated EXECUTE | ⚠️ | revocar anon |
| Holded API (entrada) | API key (Vault) | 1/5 sociedades | conectar resto |
| PostgREST/GraphQL | anon/authenticated grants | RLS filtra | revocar grants innecesarios (P3) |

**Recomendación:** documentar contrato de cada función; añadir alertas; menor privilegio en RPCs expuestas.
