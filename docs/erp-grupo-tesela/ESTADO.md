# Estado de implementación — ERP Grupo Tesela

> Bitácora de avance. Última actualización: Fase 1 (usuarios y roles).

## ✅ Fase 0 — Cimientos (en marcha)

| Paso | Estado | Detalle |
|------|--------|---------|
| Documentación (arquitectura + modelo de datos) | ✅ Hecho | En `docs/erp-grupo-tesela/` |
| Proyecto Supabase | ✅ Hecho | `erp-grupo-tesela` · región **eu-west-3 (París, UE)** · plan Free (0 €) |
| Esquema inicial (14 tablas) | ✅ Hecho | Migración `esquema_inicial_erp` |
| RLS base (bloqueo de acceso anónimo) | ✅ Hecho | Migración `rls_base_autenticados` |
| Prueba end-to-end del modelo | ✅ Superada | Promoción demo creada, consultada y eliminada correctamente |
| Integración Holded (lectura) | 🟡 Habilitada en Zapier, falta autorizar | 19 acciones activas (find_contact, create_invoice, etc.). Pendiente: el usuario autoriza su cuenta en el enlace OAuth |
| Auth + RLS fino por rol | ✅ Hecho (Fase 1) | Ver sección Fase 1 |

## ✅ Fase 1 — Usuarios y roles (hecho)

| Paso | Estado | Detalle |
|------|--------|---------|
| Tablas `perfil` y `acceso_promocion` | ✅ Hecho | Perfil vinculado a Supabase Auth; asignación de promociones por usuario |
| Alta automática de perfil al registrarse | ✅ Hecho | Trigger `on_auth_user_created` |
| Funciones helper RLS | ✅ Hecho | `current_rol()`, `es_direccion()`, `puede_ver_promocion()` |
| RLS fino por rol y promoción | ✅ Hecho | dirección=todo; obra/comercial=lectura de promociones asignadas + escritura en su dominio |
| Endurecimiento de funciones | ✅ Hecho | Funciones helper retiradas del API REST público |
| Almacenamiento documental | ✅ Hecho | Bucket privado `documentos` + RLS por promoción (ruta `<promocion_id>/...`); borrado solo dirección |
| Cuadro de rentabilidad por promoción | ✅ Hecho | Vista `v_rentabilidad_promocion` (ingresos vs coste real; respeta RLS) |
| Facturación automática (cola → Holded) | ✅ Hecho | Tabla `factura_pendiente` + trigger: al escriturar una compraventa se encola su factura. Make/Zapier la emite en Holded (`create_invoice`) |
| Tesorería por promoción | ✅ Hecho | Vista `v_tesoreria_promocion` (contratado en hitos / cobrado / pendiente de cobro) |
| App web real (frontend) | ✅ Hecho | `app/` — login con Supabase + dashboard de promociones en vivo (RLS por rol) |

**Roles:** `direccion` (ve y gestiona todo) · `obra` (sus promociones: presupuestos, contratos de obra) · `comercial` (sus promociones: reservas, compraventas). `NULL` = pendiente de asignar (sin acceso).

> ⚠️ **Primer usuario:** cuando te registres en la app, tu perfil se creará con rol `NULL`.
> Hay que marcarte como `direccion` (lo haré yo con un UPDATE en cuanto exista tu usuario).
> A partir de ahí, tú (dirección) asignas roles y promociones al resto del equipo.

> 🔎 La verificación del aislamiento por rol se hará con usuarios reales (al crear las
> cuentas del equipo), no con usuarios simulados, para no manipular el sistema de Auth.

## Datos del proyecto Supabase

- **Nombre:** erp-grupo-tesela
- **Referencia:** `jpojckqnhepiuwefyvdr`
- **Región:** eu-west-3 (París — UE, RGPD)
- **URL API:** https://jpojckqnhepiuwefyvdr.supabase.co
- **Tablas:** 14 (sociedad, promocion, fase, unidad, cliente, proveedor, reserva, contrato_venta, hito_pago, presupuesto, partida, contrato_obra, certificacion, documento)

> ⚠️ Las claves de API (anon/publishable y service_role) **no se guardan en el repositorio** por seguridad. Se gestionan en el panel de Supabase.

## Notas de seguridad

- El aviso crítico inicial (RLS desactivado) **está resuelto**: el acceso anónimo está bloqueado.
- Las políticas actuales son permisivas para cualquier usuario autenticado (correcto en Fase 0).
- En Fase 1 se sustituyen por políticas finas por **rol** (dirección/obra/comercial) y por **sociedad/promoción**.

## Conexión con Holded — dos vías

**Vía A (elegida, en marcha): Zapier.** Holded habilitado en Zapier con 19 acciones
(lectura `find_contact` + escritura: facturas, contactos, pagos, presupuestos…).
Pendiente: el usuario autoriza su cuenta en el enlace OAuth que devuelve `enable_zapier_action`.
- Ideal para operaciones transaccionales del ERP (crear factura al firmar una compraventa,
  buscar/crear contacto, registrar pago).
- Limitación: Zapier no ofrece "listar todos los contactos"; solo búsqueda individual.

**Vía B (lista, falta el secret): Edge Function `sync-holded` v2 (multi-sociedad).**
Desplegada y activa. Sincroniza varias empresas de Holded a la vez (carga masiva inicial).
Cada contacto se asocia a su `sociedad` (columna `sociedad_id` en `cliente`/`proveedor`).
Pendiente: configurar el secret `HOLDED_SOCIEDADES` en Supabase.

### Cómo poner el secret (lo haces tú, las keys NO van al repo)
En **Supabase → proyecto `erp-grupo-tesela` → Edge Functions → Secrets**, crea:

```
Nombre:  HOLDED_SOCIEDADES
Valor:   [
  {"nombre":"<Nombre Sociedad 1>","key":"<api_key_1>"},
  {"nombre":"<Nombre Sociedad 2>","key":"<api_key_2>"},
  {"nombre":"<Nombre Sociedad 3>","key":"<api_key_3>"},
  {"nombre":"<Nombre Sociedad 4>","key":"<api_key_4>"},
  {"nombre":"<Nombre Sociedad 5>","key":"<api_key_5>"}
]
```

Cuando esté, avísame y lanzo la sincronización + verifico los contactos importados.

## Próximos pasos

1. **Holded (tú):** autorizar Zapier (enlace OAuth) y poner el secret `HOLDED_SOCIEDADES`
   en Supabase → luego lanzo la sincronización.
2. **Tu usuario:** registrarte en la app para marcarte como `direccion`.
3. **Diseño UI:**
   - Prototipo HTML del dashboard ya disponible en `docs/erp-grupo-tesela/prototipo/dashboard.html`.
   - Figma: archivo creado (`9yLjQ40fNgkqD8YGTMBlzK`) pero **bloqueado por el plan Starter / asiento View**.
     Pendiente: el usuario amplía el plan de Figma (asiento con edición + más llamadas MCP) → retomo el diseño nativo.
4. **Fase 1 (resto):** CRM con Attio, generar factura en Holded al firmar compraventa.
