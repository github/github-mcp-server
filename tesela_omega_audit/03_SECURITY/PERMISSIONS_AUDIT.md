# PERMISSIONS AUDIT
**Fecha:** 2026-06-22 (UTC)

## Roles Postgres (erp-grupo-tesela)
- `anon`: SELECT por grant en tablas (RLS → 0 filas en negocio) **pero r/w total en `claude_sessions`** (S1) y EXECUTE en `rls_auto_enable` (S3).
- `authenticated`: acceso por rol de negocio (dirección/obra/comercial) vía políticas + helpers SECURITY DEFINER; EXECUTE en current_rol/es_direccion/puede_ver_promocion (S4).
- `service_role`: get_holded_keys (correcto).

## Roles de negocio (RLS)
- `direccion`: todo. `obra`: sus promociones (presupuestos/contratos/certificaciones). `comercial`: sus promociones (reservas/ventas). `NULL`: sin acceso.
- Validación con **usuarios reales pendiente** (hoy 1 perfil demo) → R17.

## GitHub
- `israel2606` admin de 7 repos. Sesión MCP scoped a github-mcp-server.

## Recomendaciones
- Revocar accesos anon innecesarios (S1, S3).
- Crear cuentas reales y validar aislamiento por rol con ellas (no con usuarios simulados).
- Principio de menor privilegio en funciones expuestas por REST.
