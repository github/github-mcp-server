# SECRETS EXPOSURE REPORT
**Fecha:** 2026-06-22 (UTC). Método: `git grep` de patrones (api_key/secret/token/password/private_key/service_role/sk-/ghp_/AIza/AKIA/JWT) sobre ficheros trackeados. Valores **no impresos**.

## Resultado: SIN SECRETOS REALES EN EL REPO ✅
- **No** hay `service_role` key, **no** hay `.env` trackeado, **no** hay JWT de Supabase.
- Coincidencias = ruido legítimo:
  - `app/config.js`: `anonKey` con prefijo `sb_publishable_...` → **clave pública por diseño** (segura si RLS se respeta).
  - `docs/*`: placeholders `ghp_your_token_here`, `<SERVICE_ROLE_KEY>`.
  - `pkg/**`, `internal/**`: código del propio GitHub MCP Server que detecta tokens `ghp_` (no son secretos).

## Matiz importante
La publishable key es pública, pero como `claude_sessions` permite acceso anon, esa clave **basta para leer/escribir esa tabla** (ver S1). El riesgo no es "fuga de clave" sino "RLS demasiado abierta en una tabla".

## Recomendaciones
- Mantener la disciplina: nunca commitear `service_role`/.env.
- Considerar mover `app/config.js` a un repo privado junto con el resto del ERP (R5) por higiene, aunque la clave sea pública.
- Rotación: no urge (no hay fuga); si se hace público algo sensible en el futuro, rotar publishable key es trivial.
