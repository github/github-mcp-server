# 🏢 ERP Grupo Tesela — Portada

Punto de entrada único: todos los accesos, el estado y la documentación del proyecto.

---

## 🔗 Accesos rápidos

| Qué | Dónde |
|-----|-------|
| 🖥️ **App del ERP (usar)** | https://jpojckqnhepiuwefyvdr.supabase.co/functions/v1/app |
| 🔑 **Usuario de prueba** | `demo@grupotesela.com` · contraseña compartida en el chat (rol Dirección) |
| 📂 **Código y docs (GitHub)** | Repo `israel2606/github-mcp-server` · rama `claude/connector-recommendations-hzwhi4` |
| 🗄️ **Base de datos (Supabase)** | https://supabase.com/dashboard/project/jpojckqnhepiuwefyvdr |
| 🎨 **Diseño (Figma)** | https://www.figma.com/design/9yLjQ40fNgkqD8YGTMBlzK |

---

## 📚 Documentación (en `docs/erp-grupo-tesela/`)

| Documento | Para qué |
|-----------|----------|
| [README.md](./README.md) | Resumen del proyecto y stack |
| [ESTADO.md](./ESTADO.md) | **Bitácora viva**: qué está hecho y qué falta |
| [ARQUITECTURA.md](./ARQUITECTURA.md) | Stack, capas, módulos, hoja de ruta, costes |
| [MODELO-DATOS.md](./MODELO-DATOS.md) | Tablas y diagrama entidad-relación |
| [prototipo/pantallas.html](./prototipo/pantallas.html) | Las 5 pantallas en HTML |
| `app/` (raíz del repo) | Código de la aplicación web |
| `supabase/migrations/` | Toda la base de datos versionada (14 migraciones) |

---

## ✅ Estado del ERP

**Funcionando (lo hago yo):**
- Base de datos (16 tablas) + usuarios y roles + seguridad por fila (RLS).
- Dashboard global del grupo + listado y detalle de promociones.
- BI: rentabilidad, comercialización y tesorería por promoción.
- Operar: registrar ventas desde la app (→ marca unidad vendida + encola factura).
- Facturación automática (cola hacia Holded) y almacén documental.
- App web publicada con login + usuario de prueba.

**Pendiente de ti (desbloqueos):**
1. 🔴 **Conectar Holded** — Zapier OAuth *o* secret `HOLDED_SOCIEDADES` en Supabase → emito las facturas reales.
2. 🟠 **Figma** — asignar a tu usuario un asiento *Editor* para retomar el diseño nativo.
3. 🟢 **Tu usuario real** — registrarte para asignarte rol `direccion`.
4. 🔒 **Seguridad** — rotar las 5 API keys de Holded que pasaron por el chat.

---

## 🧭 Próximos módulos propuestos (a elección)
Registrar obra/certificaciones · Registrar cobros · Postventa/incidencias · Exportar a PDF/CSV · Formulario de nueva promoción.
