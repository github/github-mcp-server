# TOP RIESGOS — Grupo Tesela
**Fecha:** 2026-06-22 (UTC). Conclusión / Evidencia / Impacto / Recomendación / Prioridad / Confianza.

### R1 — `claude_sessions` escribible por anónimos y consumida por agentes IA `P0` (Alta)
Evidencia: advisor `rls_policy_always_true` (política `allow_all_anon` ALL, rol anon); columnas `repo/task/next_action/branch`; `anon_insert` también en `akomftsfbnucktrladce`.
Impacto: cualquiera con URL+publishable key (públicas en la app) inyecta/edita coordinación que otras sesiones IA leen y obedecen; esos agentes hacen commits/push y `execute_sql`.
Recomendación: revocar `anon` (policy+grant); mover coordinación a repo privado/RLS estricta; tratar contenido como dato no confiable.

### R2 — Datos reales sin PITR (plan Free) `P0` (Alta)
Evidencia: proyecto Free; 62 facturas + 47 contactos reales. Impacto: borrado/`drop` no recuperable. Recomendación: Pro + backup lógico ya.

### R3 — Sin única fuente de la verdad `P1` (Alta)
Evidencia: 3 Supabase + DuckDB + Holded; `factura_holded`(62) vs `holded_facturas`(1). Impacto: BI no fiable. Recomendación: D1 + eliminar duplicado.

### R4 — Deriva de esquema (IaC rota) `P1` (Alta)
Evidencia: 23 migraciones BD vs 22 repo; `holded_facturas`, `rls_auto_enable()`, `cleanup_old_sessions()` solo en BD. Recomendación: traer a migraciones; prohibir DDL directo.

### R5 — Código ERP en repo público `P1` (Alta)
Evidencia: `github-mcp-server` (`private:false`) contiene supabase/app/docs. Recomendación: mover ERP a privado.

### R6 — Funciones SECURITY DEFINER ejecutables por anon/authenticated `P1` (Media)
Evidencia: advisors sobre `rls_auto_enable` (anon), `current_rol`/`es_direccion`/`puede_ver_promocion` (authenticated). Recomendación: revocar EXECUTE a anon; revisar authenticated.

### R7 — Leaked password protection desactivado `P2` (Alta)
Evidencia: advisor `auth_leaked_password_protection`. Recomendación: activar antes de usuarios reales.

### R8 — Sprawl de repos `P1` (Alta)
Evidencia: 5 repos Tesela + ERP en github-mcp-server. Recomendación: 1 repo = 1 propósito + owner.

### R9 — `emit-facturas` sin IVA/serie `P1` (Media)
Evidencia: código `items` sin `tax`; dryRun. Recomendación: no `dryRun:false` hasta validar fiscalidad.

### R10 — 4/5 sociedades Holded sin conectar `P2` (Alta)
Evidencia: `sociedad`=1. Impacto: visión financiera parcial.

### R11 — `CONEXION ERP DATAS`: RLS sin políticas, datos semilla `P2` (Alta)
Evidencia: advisor `rls_enabled_no_policy` dim/fact. Recomendación: definir propósito y políticas antes de cargar datos reales.

### R12 — FKs sin índice `P2` (Alta)
Evidencia: ~20 advisors `unindexed_foreign_keys`. Recomendación: índices al meter volumen.

### R13 — RLS re-evaluada por fila / múltiples permissive `P2` (Media)
Evidencia: advisors `auth_rls_initplan`, `multiple_permissive_policies`. Recomendación: `(select auth.fn())`; consolidar.

### R14 — Modelo obra→capítulo→partida incompleto `P1` (Media)
Evidencia: presupuesto/partida/contrato_obra/certificacion a 0 filas. Impacto: sin imputación de coste por capítulo.

### R15 — Continuidad: sesiones IA artesanales `P1` (Media)
Evidencia: coordinación vía claude_sessions/Drive. Recomendación: protocolo único versionado en repo.

### R16 — PR #13 base mal configurada `P3` (Alta)
Evidencia: base `claude/add-claude-documentation-4JuOo`. Recomendación: corregir o cerrar.

### R17 — Sin usuarios reales / RLS por rol no validado en real `P2` (Media)
Evidencia: `perfil`=1 (demo). Recomendación: cuentas reales + validar aislamiento.

### R18 — Esquema visible en GraphQL a anon/authenticated `P3` (Alta)
Evidencia: 24+24 advisors `pg_graphql_*` (RLS sigue filtrando filas). Recomendación: revocar SELECT de grant innecesario.

### R19 — Sin alertas de fallo de cron/sync `P2` (Media)
Evidencia: el timeout ciego anterior se detectó tarde. Recomendación: alerta proactiva.

### R20 — Capas no auditables (Vercel/Cloudflare/Notion/Google/automatizaciones) `P2` (Baja)
Evidencia: sin acceso (`ACCESS_REQUESTS.md`). Impacto: puntos ciegos.
