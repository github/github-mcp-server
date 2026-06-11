# App web ERP Grupo Tesela

Frontend real (sin framework, sin build) conectado a Supabase. Login con email/contraseña
y dashboard de promociones que carga datos **en vivo** desde las vistas, respetando el RLS
por rol (cada usuario ve solo sus promociones).

## Archivos
- `index.html` — estructura (login + dashboard).
- `styles.css` — estilos.
- `config.js` — URL y clave **pública** de Supabase (segura para el frontend gracias al RLS).
- `app.js` — lógica: auth, carga de `v_comercializacion_promocion` + `v_rentabilidad_promocion`.

## Cómo ejecutarla en local
Necesita servirse por HTTP (no `file://`). Con Node instalado:

```bash
npx serve app          # o:  python3 -m http.server -d app 8080
```

Abre la URL que indique (p. ej. http://localhost:3000) e inicia sesión.

## Crear el primer usuario
1. En el panel de Supabase → **Authentication → Users → Add user** (email + contraseña).
2. Ese usuario nace con rol `NULL` (sin acceso). Dirección le asigna el rol y promociones,
   o se marca como `direccion` para verlo todo.

## Despliegue (opciones)
- **Vercel / Netlify / Cloudflare Pages:** subir la carpeta `app/` como sitio estático.
- **Supabase Storage / hosting estático** o cualquier CDN.

> La clave de `config.js` es la *publishable/anon key*: es pública por diseño. La seguridad
> real la da el RLS de la base de datos, no ocultar esta clave.
