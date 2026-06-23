# SECURITY REPORT
**Fecha:** 2026-06-22 (UTC). Fuente: Supabase advisors (security), git grep, lectura de código y config.

## Veredicto
Los **datos de negocio están bien protegidos** por RLS y **no hay secretos en el repo**. El problema de seguridad real es una **barandilla de IA ausente** (R1) y la **falta de backup** (R2), más endurecimientos pendientes.

## Hallazgos (por prioridad)
| ID | Hallazgo | Prioridad | Evidencia |
|---|---|---|---|
| S1 | `claude_sessions` anon r/w; consumida por agentes IA con escritura en prod | P0 | advisor rls_policy_always_true; columnas repo/task/next_action |
| S2 | Sin PITR (plan Free) con datos reales | P0 | plan Free; 62 facturas + 47 contactos |
| S3 | `rls_auto_enable()` SECURITY DEFINER ejecutable por anon | P1 | advisor anon_security_definer_function_executable |
| S4 | `current_rol`/`es_direccion`/`puede_ver_promocion` ejecutables por authenticated | P1 | advisor authenticated_security_definer |
| S5 | Leaked-password protection desactivado | P2 | advisor auth_leaked_password_protection |
| S6 | Esquema visible en GraphQL a anon/authenticated (RLS filtra filas) | P3 | 24+24 advisors pg_graphql_* |
| S7 | `holded_facturas` RLS sin política (tabla bloqueada, pero drift) | P3 | advisor rls_enabled_no_policy |
| S8 | Mismo patrón inseguro de claude_sessions en CONEXION ERP DATAS | P1 | advisors proyecto akomftsfbnucktrladce |

## Lo que está BIEN (no romper)
- RLS habilitado en las 20 tablas; acceso anónimo a datos de negocio = 0 filas.
- `get_holded_keys()` solo `service_role` (no expuesto a anon/authenticated).
- `service_role` key NO está en el repo; bucket `documentos` privado; emit-facturas exige service-role y dryRun por defecto.

## Acciones (todas reversibles; requieren aprobación — modo read-only)
1. `revoke all on public.claude_sessions from anon;` + eliminar política `allow_all_anon` (y `anon_insert` en CONEXION). Mover coordinación a repo privado o RLS por service_role.
2. Subir a Pro y habilitar PITR; backup lógico manual ya.
3. `revoke execute on function public.rls_auto_enable() from anon;` (y de authenticated si no se usa por API).
4. Activar leaked-password protection en Auth.
5. (P3) revocar SELECT de grant GraphQL a roles que no deban descubrir el esquema.
