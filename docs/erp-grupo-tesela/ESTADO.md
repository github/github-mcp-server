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
| Integración Holded (lectura) | 🟡 Lista, falta activar | Edge Function `sync-holded` desplegada. Falta el secreto `HOLDED_API_KEY` |
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

## Cómo activar la sincronización con Holded

1. Saca tu API key en **Holded → Configuración → Desarrolladores → API**.
2. En **Supabase → Project Settings → Edge Functions → Secrets**, añade:
   `HOLDED_API_KEY = <tu_api_key>`
3. Invoca la función `sync-holded` (yo puedo lanzarla por ti una vez esté el secreto).
4. Verás los contactos de Holded volcados en las tablas `cliente` y `proveedor`
   con su `holded_id` poblado.

> El código vive en `supabase/functions/sync-holded/index.ts`. El mapeo de campos
> conviene validarlo contra la doc vigente de Holded (developers.holded.com).

## Próximos pasos

1. **Holded:** añadir el secreto `HOLDED_API_KEY` y lanzar la primera sincronización.
2. **Fase 1:** tabla de usuarios + roles, módulo Promociones/Obras y Documental, CRM con Attio.
