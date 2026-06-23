# TESELA OMEGA — Informe Ejecutivo

**Fecha (UTC):** 2026-06-22 · **Modo:** Auditoría read-only (sin cambios) · **Auditor:** Agente IA (Claude Code)
**Alcance verificable real:** workspace `github-mcp-server` (código ERP + app + migraciones), 3 proyectos Supabase de la org `uladuspfccwdyrmyklnk`, 7 repos GitHub de `israel2606`, ecosistema MCP conectado a esta sesión.
**Fuera de alcance por falta de acceso desde esta sesión:** Vercel, Cloudflare/DNS, Holded (panel/API directa), n8n/Make/Zapier en ejecución, Google Workspace, Notion, OpenAI/Anthropic billing. → ver `ACCESS_REQUESTS.md`.

> Todo lo afirmado aquí tiene evidencia en `09_APPENDICES/COMMAND_LOG.md` y en los informes de `02_FORENSICS/` y `03_SECURITY/`. Lo no verificable está marcado `N/D`.

---

## Resumen brutal

Tienes un **ERP real y funcionando** (Supabase + app web + sincronización diaria de Holded verificada hoy), pero montado sobre un **ecosistema disperso y sin gobierno**: el mismo "cerebro" vive repartido en **3 proyectos Supabase + 5 repos privados + 1 repo público** que se solapan, sin una única fuente de la verdad. La seguridad de los datos de negocio (clientes, proveedores, 62 facturas reales) está **correctamente protegida por RLS**, y **no hay secretos filtrados en el repositorio** (verificado). Pero hay un **agujero de control concreto**: una tabla de coordinación entre sesiones de IA (`claude_sessions`) es **escribible por cualquiera en internet** y su contenido **dirige a agentes IA que sí pueden escribir en producción** (GitHub y Supabase). Eso convierte un detalle "menor" en el **riesgo nº1**.

No estás roto. Estás **sin consolidar y sin barandillas**. Es barato de arreglar ahora; es caro si esperas a tener 20 obras y 10 personas dentro.

## Situación actual (qué hay, qué funciona)

- **ERP operativo (Supabase `erp-grupo-tesela`, eu-west-3 / UE):** 20 tablas, RLS por rol (dirección/obra/comercial), app web pública por Edge Function, 3 Edge Functions activas (`app`, `sync-holded`, `emit-facturas`).
- **Holded conectado (1 de 5 sociedades):** 28 clientes + 19 proveedores + 62 facturas reales importadas. **Cron diario 06:00 UTC verificado** (3 ejecuciones correctas, última hoy). Keys cifradas en Vault, leídas solo por `service_role` (correcto).
- **Emisión de facturas (`emit-facturas`):** construida, segura por defecto (`dryRun=true`, exige service-role key). **Aún NO operativa para uso real** (falta IVA/serie). Bien que esté frenada.
- **Datos protegidos:** acceso anónimo a tablas de negocio bloqueado por RLS; bucket documental privado; sin `.env` ni `service_role` en el repo.

## Riesgos P0 (críticos — leer antes de nada)

1. **`claude_sessions` escribible por anónimos y consumida por agentes IA con permisos de escritura en producción.** Cualquiera con la URL del proyecto y la *publishable key* (ambas públicas por diseño en la app web) puede **leer, modificar o inyectar** filas de coordinación que otras sesiones de IA leen y obedecen. Es un **vector de inyección de instrucciones (prompt injection) hacia agentes que pueden hacer commits/push y `execute_sql`**. Presente en 2 proyectos (`erp-grupo-tesela` y `CONEXION ERP DATAS`). *(Evidencia: advisor `rls_policy_always_true`; política `allow_all_anon` ALL; columnas `repo/task/status/next_action/branch`.)*
2. **No hay copia de seguridad robusta de datos reales (plan Free, sin PITR).** El ERP guarda 62 facturas reales + 47 contactos en un proyecto en plan **Free** (sin point-in-time-recovery). Un borrado accidental o un `drop` de un agente mal dirigido (ver P0-1) **no es recuperable** finamente. *(Evidencia: plan Free; sin backups gestionados verificables.)*

> Por el protocolo Omega me **detengo aquí en cuanto a cambios**: estos P0 se reportan y **no ejecuto ninguna reparación** sin tu aprobación explícita (ver `06_REPAIR/REPAIR_PLAN.md`).

## Riesgos P1 (altos)

3. **Sin única fuente de la verdad.** 3 Supabase + DuckDB (`Tesela-iA-v.0`) + Holded almacenan datos de negocio solapados; **2 tablas de facturas duplicadas** en el mismo proyecto (`factura_holded` 62 filas vs `holded_facturas` 1 fila). BI financiero **no fiable** mientras esto siga así.
4. **Deriva de esquema (Infra-as-Code rota).** La BD tiene 23 migraciones; el repo, 22. `holded_facturas` y las funciones `rls_auto_enable`/`cleanup_old_sessions` se crearon **directamente en producción** sin quedar en el repo → no auditable, no reproducible.
5. **Sprawl de repositorios.** ≥5 repos privados Tesela + el **código ERP dentro de un fork PÚBLICO** (`github-mcp-server`): expone arquitectura, modelo de datos y URL del proyecto. La *publishable key* es pública por diseño, pero el ERP no debería vivir en un repo público.
6. **`auth_leaked_password_protection` desactivado** y varias funciones `SECURITY DEFINER` innecesariamente ejecutables por `anon`/`authenticated` (`rls_auto_enable`, `current_rol`, `es_direccion`, `puede_ver_promocion`).

## Qué sobra / qué cuesta dinero sin aportar valor

- **Proyectos/repos solapados** (coste de mantenimiento y confusión, no tanto € directo: casi todo está en planes Free). El gasto real es **tiempo directivo y riesgo**, no factura mensual.
- **Tabla `holded_facturas`** (duplicado de `factura_holded`).
- **Canales de coordinación múltiples** ya marcados obsoletos en sesiones previas; `claude_sessions` los reemplaza pero introduce el P0-1.

## Decisiones que dirección debe tomar

| # | Decisión | Recomendación |
|---|---|---|
| D1 | ¿Cuál es la **fuente única de la verdad** del ERP? | `erp-grupo-tesela` (Supabase) para operación; `Tesela-iA-v.0` (DuckDB) para análisis BC3/Holded histórico. Consolidar, no multiplicar. |
| D2 | ¿Sacar el código ERP del repo **público**? | Sí: mover `supabase/`, `app/`, `docs/erp-grupo-tesela/` a un repo privado dedicado. |
| D3 | ¿Subir el ERP a **Supabase Pro** (PITR/backups)? | Sí, en cuanto haya datos reales que perder (ya los hay). ~25 $/mes. |
| D4 | ¿Cerrar el P0-1 de `claude_sessions`? | Sí, inmediato: quitar acceso `anon`, o sacar la coordinación de una tabla pública. |

## Plan 7 días (P0 + quick wins, todo reversible)
1. **Cerrar `claude_sessions` a `anon`** en los 2 proyectos (revocar policy/grant) — corta el vector de prompt-injection.
2. **Activar leaked-password protection** en Auth (1 clic).
3. **Revocar EXECUTE a `anon`** de `rls_auto_enable` (y a `authenticated` si no se usa por API).
4. **Subir `erp-grupo-tesela` a Pro** y verificar PITR / exportar un backup lógico manual ya.
5. **Decidir D1/D2** (fuente única + repo privado).

## Plan 30 días (P1)
- Consolidar facturas en **una sola tabla**, eliminar `holded_facturas`.
- **Reconciliar migraciones**: traer al repo la migración `create_claude_sessions` y las funciones de drift; prohibir DDL directo (todo por migración).
- Mover ERP a repo privado; dejar `github-mcp-server` solo como fork técnico.
- Conectar las **4 sociedades Holded** restantes.
- Crear los **usuarios reales** del equipo en Auth y validar aislamiento por rol con cuentas reales.

## Plan 90 días (P2-P3, escalabilidad)
- Modelo **obra → capítulo → partida** para imputar costes (presupuesto→pedido→compra→certificación→factura→cobro).
- BI consolidado (caja, margen por promoción) sobre fuente única.
- Índices en FKs (rendimiento al crecer), optimizar políticas RLS (`(select auth.fn())`).
- Política de **un repo = un propósito + owner**; arquitectura objetivo documentada (`05_ARCHITECTURE/TARGET_ARCHITECTURE.md`).

## Ahorro / inversión / ROI
- **Ahorro inmediato (€):** bajo — casi todo en Free; el ahorro real es **tiempo directivo y reducción de riesgo**.
- **Inversión recomendada:** Supabase Pro (~25 $/mes) = seguro de continuidad sobre datos reales. **ROI A** (evita riesgo crítico).
- **Coste de no actuar:** pérdida de datos no recuperable (P0-2) o manipulación vía agentes (P0-1). Desproporcionado frente al coste de arreglarlo (horas, no miles de €).

## Bloqueos
- Sin acceso desde esta sesión a Vercel/Cloudflare/Holded-panel/Notion/Google/n8n → esas capas están `N/D` (ver `ACCESS_REQUESTS.md`). No invento su estado.

## Conclusión
El núcleo es **bueno y recuperable**. No hace falta reconstruir: hace falta **consolidar, poner barandillas y respaldar**. Cierra los 2 P0 esta semana (horas de trabajo), decide la fuente única y el repo privado, y tendrás una base **auditable y escalable** para crecer a 20 obras sin rehacer nada.
