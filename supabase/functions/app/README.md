# Edge Function `app` — publicación de la web del ERP

Sirve la app web del ERP (login + dashboard) como HTML autocontenido en una URL pública,
para poder abrir el ERP desde el navegador sin desplegar en otro hosting.

- **URL:** https://jpojckqnhepiuwefyvdr.supabase.co/functions/v1/app
- Desplegada con `verify_jwt=false` (es una web pública; el login lo hace supabase-js
  contra Supabase Auth, y los datos quedan protegidos por RLS).
- Fuente canónica de la app (versión multiarchivo, editable): carpeta [`/app`](../../../app).
  Esta función es una variante inline de esa misma app para publicación rápida.

Para actualizarla, re-desplegar la función con el HTML actualizado, o migrar a un hosting
estático (Vercel/Netlify/Cloudflare Pages) sirviendo la carpeta `/app`.
