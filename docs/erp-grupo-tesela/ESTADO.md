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

**Vía B (reserva, ya desplegada): Edge Function `sync-holded`.** Mejor para la
**sincronización masiva inicial** de todos los contactos (la API de Holded sí permite
listar). Requiere poner el secreto `HOLDED_API_KEY` en Supabase e invocar la función.
Código en `supabase/functions/sync-holded/index.ts`.

> Plan: usar Zapier para el día a día transaccional y la Edge Function para la carga
> inicial masiva de contactos. El mapeo de campos se valida contra la cuenta real.

## Próximos pasos

1. **Holded:** añadir el secreto `HOLDED_API_KEY` y lanzar la primera sincronización.
2. **Fase 1:** tabla de usuarios + roles, módulo Promociones/Obras y Documental, CRM con Attio.
