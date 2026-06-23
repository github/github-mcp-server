# scripts/ — borradores SEGUROS (no destructivos)
**Fecha:** 2026-06-22 (UTC)

- `tesela_safe_audit.sh`: re-ejecuta el inventario read-only del workspace.
- `tesela_healthcheck.sh`: comprobaciones locales (build/lint/test si aplica) sin tocar producción.
- `tesela_autorepair_draft.sh`: **borrador**; por defecto solo `--dry-run`. No ejecuta acciones destructivas. Requiere flags explícitos y aprobación.

Ninguno toca Supabase/GitHub en producción. Las reparaciones de BD se harán por migración revisada, no por script suelto.
