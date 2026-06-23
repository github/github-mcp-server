# ACCESS MATRIX (lo verificable)
**Fecha:** 2026-06-22 (UTC)

| Rol/identidad | Sistema | Acceso | Evidencia |
|---|---|---|---|
| `anon` (público) | erp tablas negocio | SELECT por grant pero **0 filas** (RLS) | advisors pg_graphql_*; RLS por rol |
| `anon` | erp.claude_sessions | **r/w total** ⚠️ | advisor rls_policy_always_true |
| `anon` | RPC rls_auto_enable | EXECUTE ⚠️ | advisor anon_security_definer |
| `authenticated` | erp por rol (dirección/obra/comercial) | según políticas | migraciones 05/06; helpers |
| `service_role` | Vault get_holded_keys | EXECUTE (correcto) | NO aparece en advisors anon/auth |
| `israel2606` | 7 repos GitHub | admin | get_me/search_repositories |
| esta sesión | GitHub MCP | scope israel2606/github-mcp-server | system scope |

**Pendiente:** usuarios reales del equipo (hoy solo 1 perfil demo).
