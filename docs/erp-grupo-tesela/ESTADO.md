# Estado de implementación — ERP Grupo Tesela

> Bitácora de avance. Última actualización: Fase 0.

## ✅ Fase 0 — Cimientos (en marcha)

| Paso | Estado | Detalle |
|------|--------|---------|
| Documentación (arquitectura + modelo de datos) | ✅ Hecho | En `docs/erp-grupo-tesela/` |
| Proyecto Supabase | ✅ Hecho | `erp-grupo-tesela` · región **eu-west-3 (París, UE)** · plan Free (0 €) |
| Esquema inicial (14 tablas) | ✅ Hecho | Migración `esquema_inicial_erp` |
| RLS base (bloqueo de acceso anónimo) | ✅ Hecho | Migración `rls_base_autenticados` |
| Prueba end-to-end del modelo | ✅ Superada | Promoción demo creada, consultada y eliminada correctamente |
| Integración Holded (lectura) | 🟡 Habilitada en Zapier, falta autorizar | 19 acciones activas (find_contact, create_invoice, etc.). Pendiente: el usuario autoriza su cuenta en el enlace OAuth |
| Auth + RLS fino por rol | ⏳ Pendiente | Se hace en Fase 1 con la tabla de usuarios |

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

1. **Holded:** añadir el secreto `HOLDED_API_KEY` y lanzar la primera sincronización.
2. **Fase 1:** tabla de usuarios + roles, módulo Promociones/Obras y Documental, CRM con Attio.
