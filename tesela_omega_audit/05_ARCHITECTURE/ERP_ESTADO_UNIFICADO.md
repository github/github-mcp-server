# ERP Grupo Tesela — Estado Unificado (fuente única de verdad)

**Fecha (UTC):** 2026-06-22 · **Verificado contra sistemas en vivo** (Supabase MCP + GitHub MCP, read-only).
**Propósito:** reconciliar en un solo documento lo que está realmente desplegado, frente a lo que dicen las bitácoras de sesiones anteriores (`docs/erp-grupo-tesela/ESTADO.md`). Donde difieren, **manda esta tabla** (tiene evidencia en vivo).

---

## 1. Proyectos Supabase reales (org `uladuspfccwdyrmyklnk`)

| Proyecto | Ref | Región | Creado | Rol real | Estado |
|---|---|---|---|---|---|
| **erp-grupo-tesela** | `jpojckqnhepiuwefyvdr` | eu-west-3 (París) | 2026-06-08 | **ERP operativo** (producción de facto) | ACTIVE_HEALTHY |
| TeseLAB Invest | `umyejimabqcslsrymwus` | eu-central-1 (Frankfurt) | 2026-06-20 | Event-sourcing "TWOS" (1 tabla `event`, 0 filas) | ACTIVE_HEALTHY |
| CONEXION ERP DATAS | `akomftsfbnucktrladce` | eu-central-1 (Frankfurt) | **2026-06-21** | Almacén BI estrella (dim/fact), semilla | ACTIVE_HEALTHY |

> ⚠️ **3 proyectos en 2 regiones distintas**, creados en 3 fechas, con propósitos solapados. No hay una única fuente de la verdad. **Decisión pendiente (D1).**

## 2. ERP en producción — `erp-grupo-tesela`

### 2.1 Datos reales (conteos en vivo)
| Tabla | Filas | Nota |
|---|---:|---|
| cliente | 28 | reales de Holded |
| proveedor | 19 | reales de Holded |
| factura_holded | **62** | facturas reales importadas (la app usa ESTA) |
| sociedad | 1 | 1 de 5 sociedades conectada |
| perfil | 1 | usuario demo dirección |
| holded_facturas | 1 | **DUPLICADO** (tabla paralela, fuera de migraciones) |
| claude_sessions | 1 | coordinación entre sesiones IA (ver riesgos) |
| promocion, fase, unidad, reserva, contrato_venta, hito_pago, presupuesto, partida, contrato_obra, certificacion, documento, acceso_promocion, factura_pendiente | 0 | vacías (demo purgada; aún sin datos reales de obra) |

### 2.2 Componentes desplegados (verificado)
- **Edge Functions (todas ACTIVE, `verify_jwt=false`):** `app` v12, `sync-holded` v6, `emit-facturas` v4.
- **Cron:** `holded-sync-diario` `0 6 * * *` **activo**; 3 ejecuciones correctas, última **2026-06-22 06:00 UTC** → el fix de timeout (90 s) **funciona**.
- **Vault:** keys Holded cifradas; `get_holded_keys()` solo `service_role` (verificado: NO ejecutable por anon/authenticated).
- **Storage:** bucket `documentos` privado.
- **Migraciones aplicadas:** 23 (BD) vs 22 (repo) → **deriva** (ver §4).
- **Vistas:** v_rentabilidad_promocion, v_comercializacion_promocion (fix LEFT JOIN aplicado), v_tesoreria_promocion, v_resumen_grupo.

### 2.3 Estado por módulo
| Módulo | Estado real | Detalle |
|---|---|---|
| Auth + RLS por rol | ✅ Operativo | dirección/obra/comercial; helpers SECURITY DEFINER |
| Importación Holded (lectura) | ✅ 1/5 sociedades | 47 contactos + 62 facturas; cron diario OK |
| Emisión Holded (escritura) | 🟡 Construida, **frenada** | `emit-facturas` dryRun; falta IVA/serie. **No usar en real aún** |
| App web (dashboard, detalle, ventas, certificaciones, contactos, facturas) | ✅ Operativa | publicada por Edge Function `app` |
| Comercialización (reserva→venta→hitos) | 🟡 Esquema listo, 0 datos | tablas vacías |
| Obra (presupuesto→capítulo→partida→certificación) | 🟡 Esquema parcial, 0 datos | falta modelo de imputación obra→capítulo→partida (D pendiente) |
| BI consolidado | ❌ No fiable | datos repartidos en 3 Supabase + DuckDB + Holded |

## 3. Discrepancias con `docs/erp-grupo-tesela/ESTADO.md` (bitácora previa)

| Afirmación de la bitácora | Realidad verificada hoy | Veredicto |
|---|---|---|
| "14 tablas" / "18 tablas" | **20 tablas** en `public` | Desactualizado |
| "56 facturas (30 ventas + 26 compras)" | **62 filas** en `factura_holded` | Creció; split venta/compra **N/D** (columna distinta a la asumida) |
| "Base lista para datos reales" | Cierto para ventas; **obra sin modelo de imputación** | Parcial |
| No menciona `holded_facturas` ni `claude_sessions` | Ambas existen en vivo | **Omisión** (deriva) |
| No menciona proyectos TeseLAB / CONEXION ERP DATAS | Existen, creados 06-20 y 06-21 | **Omisión** |

## 4. Deriva de esquema (Infra-as-Code rota) — P1

Existen en la **BD** pero **no en el repo** (`supabase/migrations/` termina en `20260619_22`):
- Migración `20260621171853_create_claude_sessions` (en BD, no en repo).
- Tabla `holded_facturas` (creada por SQL directo; sin política RLS).
- Funciones `rls_auto_enable()` y `cleanup_old_sessions()` (SECURITY DEFINER; ejecutables por anon → ver seguridad).

**Consecuencia:** el repositorio ya **no reconstruye** la base de datos real. Toda nueva sesión que "confíe en el repo" parte de un mapa incorrecto.

## 5. Riesgos que afectan al estado (resumen; detalle en `03_SECURITY/`)
- **P0** `claude_sessions` anon read/write + consumida por agentes IA con permisos de escritura → prompt-injection hacia producción.
- **P0** Plan **Free** sin PITR con datos reales → pérdida no recuperable.
- **P1** Duplicado `factura_holded`/`holded_facturas`; deriva de migraciones; sprawl de 3 Supabase.
- **P1** Código ERP en repo **público**.

## 6. Fuente única recomendada (objetivo)
- **Operación ERP:** `erp-grupo-tesela` (Supabase, UE) — único sistema transaccional. Subir a **Pro**.
- **Análisis/BC3/Holded histórico:** `Tesela-iA-v.0` (DuckDB) — único motor analítico.
- **Coordinación IA:** sacar de tabla pública; usar repo privado o tabla con RLS estricta.
- **Apagar/fusionar:** evaluar `CONEXION ERP DATAS` y `TeseLAB Invest` (¿aportan algo que no cubra lo anterior? hoy son semillas casi vacías).

> Este documento debe actualizarse **solo con evidencia en vivo**. Si una sesión futura cambia algo en producción, lo refleja aquí y en una migración del repo (no en SQL suelto).
