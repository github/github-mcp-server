# GLOSSARY
**Fecha:** 2026-06-22 (UTC)

- **RLS:** Row Level Security (Postgres). Filtra filas por política según rol.
- **PITR:** Point-In-Time Recovery (restaurar a un instante). Requiere Supabase Pro.
- **IaC:** Infrastructure as Code (aquí: migraciones que reconstruyen la BD).
- **Drift/deriva:** la BD real difiere del código versionado.
- **publishable/anon key:** clave pública de cliente Supabase; segura si RLS protege los datos.
- **service_role key:** clave con bypass de RLS; NUNCA en frontend/repo.
- **Prompt injection:** instrucciones maliciosas insertadas en datos que un agente IA lee y obedece.
- **P0-P3:** prioridad (P0 crítico … P3 diferible).
- **TWOS:** event-sourcing de TeseLAB Invest (tabla `event`).
