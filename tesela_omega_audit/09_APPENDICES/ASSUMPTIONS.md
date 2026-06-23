# ASSUMPTIONS
**Fecha:** 2026-06-22 (UTC)

- Los 3 proyectos Supabase están en plan **Free** (de bitácoras; el panel billing no es accesible). Si ya hay Pro, R2/O2 cambian.
- `factura_holded` contiene facturas reales (62) según docs; el split venta/compra no se verifica (columna distinta a la asumida).
- "Producción" = `erp-grupo-tesela` (es el que sirve la app y sincroniza Holded).
- Las capas sin acceso (Vercel/Cloudflare/Notion/Google/automatizaciones/IA-billing) se asumen **desconocidas**, no buenas ni malas.
