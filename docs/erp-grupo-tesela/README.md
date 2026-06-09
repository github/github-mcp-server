# ERP Grupo Tesela

ERP a medida para **construcción, estudio de arquitectura y promoción inmobiliaria**,
construido sobre Supabase e integrado con las herramientas del grupo.

## 📚 Documentación

| Documento | Contenido |
|-----------|-----------|
| [ARQUITECTURA.md](./ARQUITECTURA.md) | Stack, capas, módulos, hoja de ruta y plan de costes |
| [MODELO-DATOS.md](./MODELO-DATOS.md) | Modelo relacional (tablas + diagrama ER) |
| [ESTADO.md](./ESTADO.md) | Bitácora de avance y pasos pendientes |
| [prototipo/pantallas.html](./prototipo/pantallas.html) | Diseño de las 5 pantallas (abrir en navegador) |

## 🧱 Stack

- **Base de datos / backend:** Supabase (PostgreSQL, Auth, Storage, Edge Functions) — región UE.
- **Contabilidad:** Holded (integrado vía Zapier + Edge Function).
- **Automatización:** Make.com / Zapier. **CRM:** Attio. **BI:** Coupler.io. **Diseño:** Figma.

## 🗄️ Proyecto Supabase

- Proyecto: `erp-grupo-tesela` · Ref: `jpojckqnhepiuwefyvdr` · Región: eu-west-3 (París, UE).
- URL API: https://jpojckqnhepiuwefyvdr.supabase.co
- Migraciones versionadas en [`supabase/migrations/`](../../supabase/migrations/).

## ✅ Capacidades implementadas

- 14 tablas de negocio (sociedades, promociones, fases, unidades, terceros, ventas, obra, documental).
- Usuarios y **roles** (dirección / obra / comercial) con **seguridad por fila (RLS)** por promoción.
- **Almacén documental** (bucket privado con permisos por promoción).
- **BI:** vistas `v_rentabilidad_promocion`, `v_comercializacion_promocion`, `v_tesoreria_promocion`.
- **Facturación automática:** cola `factura_pendiente` que se llena al escriturar una compraventa.
- Integración Holded preparada (Edge Function `sync-holded` + acciones de Zapier).

## 🔐 Seguridad

- RLS activo en todas las tablas. Las funciones helper de RLS no están expuestas en el API público.
- Las credenciales (claves de API, secretos) **no se guardan en el repositorio**; van en el panel de Supabase.

## 🚀 Puesta en marcha (pendiente del cliente)

1. **Figma:** asignar a tu usuario un asiento **Editor** (no Viewer) para retomar el diseño nativo.
2. **Holded:** autorizar Zapier (OAuth) y/o poner el secret `HOLDED_SOCIEDADES` en Supabase.
3. **Tu usuario:** registrarte para asignarte el rol `direccion`.
4. **Seguridad:** rotar las API keys de Holded si pasaron por canales no seguros.
