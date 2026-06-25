# MASTER REVIEW — Sistema Digital Grupo Tesela

**Documento:** Revisión maestra consolidada de todo el ecosistema  
**Fecha (UTC):** 2026-06-25  
**Modo:** Read-only · sin cambios en producción · sin secretos expuestos  
**Base de evidencia:** auditoría TESELA OMEGA v11 (`tesela_omega_audit/`, commit `253cbc2`) + verificación en vivo Supabase/GitHub MCP  
**Veredicto en una línea:** núcleo **bueno y recuperable**; falta **consolidar, poner barandillas y respaldar**.

---

## 0. Cómo leer este documento

Este MASTER REVIEW es el **punto de entrada único** al estado del sistema. Resume y enlaza el detalle que vive en `tesela_omega_audit/`. Donde haya conflicto entre una bitácora antigua y este documento, **manda este** (tiene evidencia en vivo). Todo lo no verificable está marcado `N/D`.

| Si buscas… | Ve a… |
|---|---|
| La foto ejecutiva | §1 y §2 de este doc + `00_EXECUTIVE/TESELA_OMEGA_EXECUTIVE_REPORT.md` |
| Qué arreglar ya | §4 (P0) + `06_REPAIR/REPAIR_PLAN.md` |
| Estado real del ERP | §3 + `05_ARCHITECTURE/ERP_ESTADO_UNIFICADO.md` |
| Decisiones que dependen de ti | §6 |
| Dónde retomar | §8 + `tesela_omega_audit/SESION_ESTADO.md` |

---

## 1. Resumen ejecutivo (60 segundos)

Grupo Tesela tiene un **ERP real, desplegado y funcionando**: Supabase (Postgres, UE) + app web pública + sincronización diaria con Holded **verificada hoy**. Los datos de negocio (28 clientes, 19 proveedores, 62 facturas reales) están **correctamente protegidos por RLS** y **no hay secretos filtrados en el repositorio**.

El problema no es el núcleo, es el **gobierno**: el "cerebro" está repartido en **3 proyectos Supabase + varios repos** que se solapan, sin una única fuente de la verdad, y con **dos agujeros críticos (P0)** que hay que cerrar esta semana. Es barato de arreglar ahora; caro si se espera a crecer a 20 obras.

**Estado global:** 🟡 **Operativo con riesgo** — no roto, pero sin barandillas.

---

## 2. Mapa del sistema (qué hay y para qué sirve)

| Capa | Componente | Estado | Rol |
|---|---|---|---|
| Datos (operación) | Supabase `erp-grupo-tesela` (`jpojckqnhepiuwefyvdr`, eu-west-3, **Free**) | ✅ ACTIVE | ERP transaccional de facto |
| Datos (otros) | Supabase TeseLAB Invest + CONEXION ERP DATAS | 🟡 Semillas | Solapan; decisión D1 pendiente |
| Contabilidad | Holded (1 de 5 sociedades conectada) | 🟡 Parcial | Motor fiscal; lectura OK, emisión frenada |
| App | Edge Function `app` v12 (pública) | ✅ | Dashboard, ventas, certificaciones, facturas |
| Integración | Edge Functions `sync-holded` v6, `emit-facturas` v4 | ✅ / 🟡 | Import diario OK; emisión en `dryRun` |
| Código | Fork **público** `github-mcp-server` (contiene `supabase/`, `app/`, `docs/`) | ⚠️ | ERP en repo público (D2) |
| Analítica | `Tesela-iA-v.0` (DuckDB) | N/D | Análisis histórico BC3/Holded |
| Automatización | Make / Zapier / n8n | N/D | Sin acceso desde esta sesión |
| Frontend hosting | Vercel / Cloudflare | N/D | Sin acceso desde esta sesión |

> Detalle completo en `01_INVENTORY/` y `02_FORENSICS/`.

---

## 3. ERP en producción — estado real (verificado en vivo)

- **20 tablas** en `public` (la bitácora antigua decía 14/18 → desactualizada).
- **Datos reales:** 28 clientes, 19 proveedores, **62 facturas** (`factura_holded`), 1 sociedad, 1 perfil demo.
- **Tablas de negocio vacías** (esquema listo, 0 datos): promoción, fase, unidad, reserva, contrato_venta, hito_pago, presupuesto, partida, contrato_obra, certificación, documento.
- **RLS por rol** (dirección/obra/comercial) con helpers `SECURITY DEFINER` ✅.
- **Cron `holded-sync-diario`** `0 6 * * *`: 3 ejecuciones correctas, última 2026-06-22 06:00 UTC → fix de timeout (90 s) **funciona**.
- **Vault:** keys Holded cifradas; `get_holded_keys()` solo `service_role` ✅.
- **Deriva de esquema:** 23 migraciones en BD vs 22 en repo → el repo ya **no reconstruye** la BD real (P1).

| Módulo | Estado |
|---|---|
| Auth + RLS por rol | ✅ Operativo |
| Importación Holded (lectura) | ✅ 1/5 sociedades |
| Emisión Holded (escritura) | 🟡 Construida, frenada (falta IVA/serie) |
| App web | ✅ Operativa |
| Comercialización (reserva→venta→hitos) | 🟡 Esquema listo, 0 datos |
| Obra (presupuesto→partida→certificación) | 🟡 Esquema parcial, falta modelo de imputación |
| BI consolidado | ❌ No fiable (datos repartidos) |

> Fuente única: `05_ARCHITECTURE/ERP_ESTADO_UNIFICADO.md`.

---

## 4. Riesgos P0 — CRÍTICOS (reportados, NO reparados)

> Por protocolo Omega, estos se **reportan** y no se ejecuta reparación sin aprobación explícita.

### 🔴 P0-1 — `claude_sessions` escribible por anónimos
Tabla de coordinación entre agentes IA con política `allow_all_anon` ALL. Cualquiera con la URL + publishable key (públicas por diseño) puede **inyectar filas** que otros agentes leen y obedecen — y esos agentes pueden hacer commits/push y `execute_sql`. **Vector de prompt-injection hacia producción.** Presente en 2 proyectos.  
**Fix:** revocar policy/grant `anon` (1 SQL, reversible, ~5 min). → cierra con **D4**.

### 🔴 P0-2 — Sin backup robusto (plan Free, sin PITR)
62 facturas + 47 contactos reales en plan **Free** sin point-in-time-recovery. Un borrado accidental o un DROP de un agente mal dirigido (ver P0-1) **no es recuperable** finamente.  
**Fix:** subir a **Supabase Pro** (~25 $/mes) + backup lógico manual ya. → cierra con **D3**.

---

## 5. Riesgos P1 (altos)

| # | Riesgo | Acción |
|---|---|---|
| P1-3 | Sin única fuente de la verdad (3 Supabase + DuckDB + Holded); `factura_holded` vs `holded_facturas` duplicadas | Consolidar (D1); eliminar `holded_facturas` |
| P1-4 | Deriva de esquema (IaC rota): DDL directo en producción no versionado | Traer al repo migración + funciones; prohibir DDL suelto |
| P1-5 | Código ERP en repo **público** | Mover a repo privado (D2) |
| P1-6 | `auth_leaked_password_protection` off + funciones `SECURITY DEFINER` ejecutables por `anon` | Activar protección; revocar EXECUTE |

> Detalle y evidencia: `03_SECURITY/SECURITY_REPORT.md`.

---

## 6. Decisiones que dependen de dirección (D1–D4)

| # | Decisión | Recomendación | Cierra |
|---|---|---|---|
| D1 | Fuente única de la verdad | `erp-grupo-tesela` (operación) + `Tesela-iA-v.0` (análisis) | P1-3 |
| D2 | Sacar ERP del repo público | Sí → repo privado dedicado | P1-5 |
| D3 | Subir a Supabase Pro | Sí, ya hay datos que perder (~25 $/mes) | P0-2 |
| D4 | Cerrar `claude_sessions` a `anon` | Sí, inmediato | P0-1 |

---

## 7. Planes de acción

**7 días (P0 + quick wins, todo reversible):**
1. Cerrar `claude_sessions` a `anon` en los 2 proyectos.
2. Activar leaked-password protection (1 clic).
3. Revocar EXECUTE a `anon` de `rls_auto_enable`.
4. Subir `erp-grupo-tesela` a Pro + verificar PITR + backup lógico manual.
5. Decidir D1/D2.

**30 días (P1):** consolidar facturas en 1 tabla; reconciliar migraciones; mover ERP a repo privado; conectar las 4 sociedades Holded restantes; crear usuarios reales y validar RLS por rol.

**90 días (P2-P3):** modelo obra→capítulo→partida; BI consolidado sobre fuente única; índices en FKs; un repo = un propósito + owner.

> Plan reparable paso a paso: `06_REPAIR/REPAIR_PLAN.md` (Fases A/B/C).

---

## 8. Dónde retomar

- **Estado de sesión y checklist:** `tesela_omega_audit/SESION_ESTADO.md`.
- **Para ejecutar reparaciones:** aprobar explícitamente "Fase A" → el agente corre los SQLs de `06_REPAIR/`.
- **Accesos que faltan** (capas N/D): `ACCESS_REQUESTS.md` (Vercel, Cloudflare, Holded panel, Notion, Google, n8n/Make/Zapier, repos privados).

---

## 9. Conclusión

El núcleo es **sólido y recuperable**. No hace falta reconstruir: hace falta **consolidar (D1/D2), respaldar (D3) y cerrar el agujero de control (D4)**. Las dos reparaciones P0 son horas de trabajo, reversibles, y eliminan el riesgo desproporcionado de pérdida de datos o manipulación vía agentes. Hecho eso, tienes una base **auditable y escalable** para crecer sin rehacer nada.

> Documento vivo. Actualizar tras cada decisión tomada o reparación ejecutada.
