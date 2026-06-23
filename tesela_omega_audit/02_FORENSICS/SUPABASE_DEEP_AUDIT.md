# SUPABASE DEEP AUDIT
**Fecha:** 2026-06-22 (UTC). Fuente: Supabase MCP read-only (list_projects/tables/migrations/edge_functions, get_advisors, execute_sql sobre cron/pg_proc/storage/information_schema). Postgres 17.6.

## Proyectos
| Proyecto | Ref | Región | Creado |
|---|---|---|---|
| erp-grupo-tesela | jpojckqnhepiuwefyvdr | eu-west-3 | 2026-06-08 |
| TeseLAB Invest | umyejimabqcslsrymwus | eu-central-1 | 2026-06-20 |
| CONEXION ERP DATAS | akomftsfbnucktrladce | eu-central-1 | 2026-06-21 |

## erp-grupo-tesela — esquema y datos
- 20 tablas public (RLS habilitado en todas). Datos reales: cliente 28, proveedor 19, factura_holded 62, sociedad 1, perfil 1. claude_sessions 1, holded_facturas 1. Resto 0.
- Vistas: v_rentabilidad_promocion, v_comercializacion_promocion (LEFT JOIN fix), v_tesoreria_promocion, v_resumen_grupo.
- Funciones SECURITY DEFINER: current_rol, encolar_factura, es_direccion, get_holded_keys, handle_new_user, marcar_unidad_estado, puede_ver_promocion, rls_auto_enable. (`get_holded_keys` correctamente NO ejecutable por anon/authenticated.)
- Edge Functions: app v12, sync-holded v6, emit-facturas v4 (todas ACTIVE, verify_jwt=false).
- Cron: holded-sync-diario `0 6 * * *` activo; job_run_details = 3 succeeded, último 2026-06-22 06:00 → **fix de timeout (90s) confirmado**.
- Storage: bucket `documentos` privado.

## Advisors de seguridad (56 lints)
| Severidad | Lint | Nº | Lectura |
|---|---|---:|---|
| WARN | rls_policy_always_true (claude_sessions allow_all_anon ALL) | 1 | **P0** vector prompt-injection |
| WARN | anon_security_definer_function_executable (rls_auto_enable) | 1 | P1 revocar EXECUTE anon |
| WARN | authenticated_security_definer_function_executable (current_rol, es_direccion, puede_ver_promocion, rls_auto_enable) | 4 | P1 revisar |
| WARN | auth_leaked_password_protection | 1 | P2 activar |
| WARN | pg_graphql_anon_table_exposed | 24 | P3 (RLS filtra filas) |
| WARN | pg_graphql_authenticated_table_exposed | 24 | P3 |
| INFO | rls_enabled_no_policy (holded_facturas) | 1 | tabla bloqueada (sin política) |

## Advisors de rendimiento
- ~20 `unindexed_foreign_keys` (acceso_promocion, certificacion, cliente, contrato_*, documento, factura_*, fase, hito_pago, partida, presupuesto, promocion, proveedor, reserva).
- `auth_rls_initplan` (perfil, acceso_promocion): usar `(select auth.fn())`.
- `multiple_permissive_policies` en ~14 tablas (direccion_todo + ver_*): consolidar.
- `auth_db_connections_absolute` (Auth a 10 conexiones fijas).

## Deriva de esquema (P1)
Migraciones BD = 23 (última `20260621171853_create_claude_sessions`); repo = 22 (hasta `20260619_22`). En BD pero no en repo: tabla `holded_facturas`, funciones `rls_auto_enable`/`cleanup_old_sessions`. → repo no reconstruye producción.

## CONEXION ERP DATAS (akomftsfbnucktrladce)
- Tablas: claude_sessions(7), dim_sociedad(5), fact_ventas(2), fact_compras(3), fact_horas_mo(1), fact_tarifa_mo(1).
- Advisors: claude_sessions `anon_insert` (WARN), `cleanup_old_sessions`/`rls_auto_enable` SECURITY DEFINER anon-executable (WARN), dim/fact `rls_enabled_no_policy` (INFO), leaked_password_protection (WARN).
- Lectura: almacén BI estrella en estado semilla; mismo patrón inseguro de claude_sessions.

## TeseLAB Invest (umyejimabqcslsrymwus)
- 1 tabla `event` (append-only, 0 filas). Sin uso productivo aún.
