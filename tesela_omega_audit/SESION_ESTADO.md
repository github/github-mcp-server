# TESELA OMEGA — Estado de Sesión / Archivo de Continuación

**Última actualización (UTC):** 2026-06-23  
**Rama git:** `claude/connector-recommendations-hzwhi4` (repo `israel2606/github-mcp-server`)  
**Commit de la auditoría:** `253cbc2`  
**Modo de sesión:** Auditoría read-only completada. Reparaciones **pendientes de aprobación explícita del usuario**.

---

## Lo que se ha completado en esta sesión

La auditoría TESELA OMEGA v11 se ejecutó en modo **100 % read-only**. Se generaron **78 archivos** en `/tesela_omega_audit/` (10 directorios), todos commiteados en `253cbc2`.

### Directorios generados

| Directorio | Contenido |
|---|---|
| `00_EXECUTIVE/` | Informe ejecutivo, brief CEO, top 20 riesgos, top 20 oportunidades, resumen board |
| `01_INVENTORY/` | Inventario maestro, mapa de sistemas, repos, datos, integraciones, matriz de acceso, activos desconocidos |
| `02_FORENSICS/` | Auditoría profunda Supabase, GitHub, MCP ecosystem, Holded, + 12 stubs N/D (Vercel, Cloudflare, n8n, Notion, Google, etc.) |
| `03_SECURITY/` | Informe de seguridad, exposición de secretos, permisos, riesgos de producción, plan de respuesta a incidentes, backlog |
| `04_COSTS/` | Burn rate, duplicaciones, servicios sin uso, matriz ROI, plan de ahorro |
| `05_ARCHITECTURE/` | Estado unificado ERP, arquitectura actual/objetivo, deuda técnica, modelo de datos, API, escalabilidad, resiliencia |
| `06_REPAIR/` | Plan de reparación Fases A/B/C, plan de reconstrucción, migraciones, limpieza, rollback, tests, changelog; scripts/ |
| `07_PRODUCT_AND_BUSINESS/` | Realidad del producto, features vs promesas, procesos de negocio, roadmap de automatización, árbol de KPIs, modelo operativo |
| `08_DASHBOARDS/` | Scorecards CEO/CTO/CFO/COO, requisitos de dashboard |
| `09_APPENDICES/` | Log de comandos, supuestos, limitaciones, hallazgos raw, glosario |

---

## Estado del sistema verificado en vivo (2026-06-22)

### Supabase — proyecto ERP principal
- **Ref:** `jpojckqnhepiuwefyvdr` · región `eu-west-3` · plan **Free**
- **20 tablas** en `public`; 62 facturas reales + 47 contactos Holded
- **Edge Functions activas:** `app` v12, `sync-holded` v6, `emit-facturas` v4 (todas `verify_jwt=false`)
- **Cron:** `holded-sync-diario` `0 6 * * *` — 3 ejecuciones OK, última 2026-06-22 06:00 UTC
- **Migraciones:** 23 en BD vs 22 en repo → **deriva** (ver §4 de `05_ARCHITECTURE/ERP_ESTADO_UNIFICADO.md`)
- **Vault:** keys Holded cifradas; `get_holded_keys()` solo `service_role` ✅

### Otros proyectos Supabase
- `umyejimabqcslsrymwus` — TeseLAB Invest (event-sourcing "TWOS", 1 tabla, 0 filas)
- `akomftsfbnucktrladce` — CONEXION ERP DATAS (esquema BI estrella, semilla)

### GitHub
- 7 repos bajo `israel2606`; código ERP dentro del fork **público** `github-mcp-server`
- **Sin secretos reales en el repo** (verificado con `git grep`)

---

## ⚠️ Riesgos P0 — REPORTADOS, NO REPARADOS

Estos dos P0 fueron identificados y reportados. **No se ejecutó ninguna reparación** porque el protocolo Omega requiere aprobación explícita del usuario antes de modificar producción.

### P0-1 — `claude_sessions` escribible por anónimos
- **Qué es:** la tabla `claude_sessions` en `jpojckqnhepiuwefyvdr` (y también en `akomftsfbnucktrladce`) tiene política `allow_all_anon` ALL — cualquiera con la URL + publishable key puede leer y escribir filas que los agentes de IA consumen para coordinar acciones (commits, push, `execute_sql`).
- **Riesgo real:** vector de prompt-injection hacia agentes con permisos de escritura en producción.
- **Fix (1 SQL, reversible, ~5 min):** revocar la política `allow_all_anon` + `REVOKE ALL ON claude_sessions FROM anon`. El SQL está en `06_REPAIR/scripts/tesela_autorepair_draft.sh` y en `06_REPAIR/REPAIR_PLAN.md` §Fase A.
- **Estado:** ❌ BLOQUEADO — requiere aprobación del usuario.

### P0-2 — Sin backup robusto en plan Free (sin PITR)
- **Qué es:** los datos reales (62 facturas, 47 contactos) viven en un proyecto Free sin point-in-time-recovery. Un borrado accidental o un DROP por agente mal dirigido no es recuperable finamente.
- **Fix:** subir `erp-grupo-tesela` a **Supabase Pro** (~25 $/mes). También se puede hacer ya un backup lógico manual (`pg_dump`) como medida puente.
- **Estado:** ❌ BLOQUEADO — requiere aprobación + acción manual en el panel de Supabase.

---

## Decisiones pendientes del usuario (D1–D4)

| # | Pregunta | Recomendación | Estado |
|---|---|---|---|
| D1 | ¿Cuál es la **fuente única de la verdad** del ERP? | `erp-grupo-tesela` (Supabase) para operación; `Tesela-iA-v.0` (DuckDB) para análisis histórico BC3/Holded. Consolidar, no multiplicar. | ⏳ Pendiente |
| D2 | ¿Sacar el código ERP del repo **público** `github-mcp-server`? | Sí: mover `supabase/`, `app/`, `docs/erp-grupo-tesela/` a un repo privado dedicado. | ⏳ Pendiente |
| D3 | ¿Subir `erp-grupo-tesela` a **Supabase Pro** (PITR/backups)? | Sí, ya hay datos reales que perder. ~25 $/mes. Cierra P0-2. | ⏳ Pendiente |
| D4 | ¿Cerrar `claude_sessions` a acceso `anon`? | Sí, urgente esta semana. Cierra P0-1. SQL listo y revisado. | ⏳ Pendiente |

---

## Accesos no disponibles en esta sesión

Ver `ACCESS_REQUESTS.md` para detalle completo. Sistemas con estado **N/D** (no verificados):

- **Vercel** — despliegues frontend, URLs, variables de entorno
- **Cloudflare / DNS** — dominios, proxies, WAF
- **Holded panel / API directa** — estado de las 4 sociedades no conectadas, catálogo de cuentas
- **n8n / Make / Zapier** — flujos activos, credenciales, errores
- **Notion** — docs de procesos, wiki interna
- **Google Workspace** — Drive, Calendar, usuarios, grupos
- **OpenAI / Anthropic billing** — consumo, límites, claves activas
- **Repos privados** `TeseLAB`, `Tesela-iA`, etc. — código no inspeccionado

---

## Cómo continuar en la próxima sesión

### Si quieres ejecutar las reparaciones P0 (recomendado esta semana)

1. Di explícitamente: **"Apruebo ejecutar Fase A del plan de reparación"** (o el ítem concreto).
2. El agente ejecutará los SQLs de `06_REPAIR/REPAIR_PLAN.md` §Fase A y los scripts de `06_REPAIR/scripts/`.
3. Cada cambio se documenta en `06_REPAIR/CHANGELOG.md`.

**Checklist Fase A (orden sugerido):**
- [ ] Revocar acceso `anon` en `claude_sessions` (ambos proyectos) → cierra P0-1
- [ ] Revocar EXECUTE de `rls_auto_enable()` desde `anon` → reduce superficie P1-6
- [ ] Activar leaked-password protection en Auth → 1 clic en panel Supabase
- [ ] Subir a Supabase Pro + verificar PITR + exportar backup lógico manual → cierra P0-2

### Si quieres continuar el desarrollo del ERP (Fase 1/2 del plan de producto)

El plan de producto está en `07_PRODUCT_AND_BUSINESS/` y en el plan original del repo (ver `CLAUDE.md`). Los siguientes hitos de producto son:
- Conectar las 4 sociedades Holded restantes
- Módulo de comercialización (reservas → arras → compraventa → hitos de pago) — esquema listo, 0 datos reales
- Modelo de obra → capítulo → partida (presupuesto → certificación → factura)
- Crear usuarios reales del equipo en Auth y validar RLS por rol

### Si quieres auditar las capas N/D

Proporcionar acceso (o credenciales de solo lectura) a: Vercel, Holded API, repos privados. Ver `ACCESS_REQUESTS.md` para la lista completa de lo que se necesita y por qué.

---

## Archivos clave para referenciar

| Propósito | Archivo |
|---|---|
| Visión ejecutiva completa | `00_EXECUTIVE/TESELA_OMEGA_EXECUTIVE_REPORT.md` |
| Estado actual del ERP (fuente única) | `05_ARCHITECTURE/ERP_ESTADO_UNIFICADO.md` |
| Plan de reparación + SQLs | `06_REPAIR/REPAIR_PLAN.md` |
| Scripts de reparación (safe por defecto) | `06_REPAIR/scripts/tesela_autorepair_draft.sh` |
| Riesgos de seguridad con evidencia | `03_SECURITY/SECURITY_REPORT.md` |
| Accesos que faltan | `ACCESS_REQUESTS.md` |
| Log de comandos ejecutados en la auditoría | `09_APPENDICES/COMMAND_LOG.md` |

---

> Este archivo debe actualizarse al inicio de cada nueva sesión de trabajo para reflejar qué decisiones se han tomado y qué reparaciones se han ejecutado.
