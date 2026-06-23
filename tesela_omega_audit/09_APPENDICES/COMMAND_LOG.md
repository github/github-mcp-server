# COMMAND_LOG

Workspace: /home/user/github-mcp-server
UTC: Mon Jun 22 14:06:02 UTC 2026


## Evidence collected (read-only)
- Local: file/dir inventory, `git status/branch/log/remote/tag/stash`, `git grep` secret scan, read app/config.js, supabase/* sources.
- Supabase MCP (read-only): list_projects, list_tables (3 projects), get_advisors security+performance (erp + CONEXION), list_migrations, list_edge_functions, execute_sql (columns, cron.job, cron.job_run_details, storage.buckets, pg_proc, row counts).
- GitHub MCP (read-only): get_me, search_repositories (user:israel2606), list_pull_requests (open).

## Key raw facts
- Supabase org uladuspfccwdyrmyklnk: 3 projects (jpojckqnhepiuwefyvdr eu-west-3; umyejimabqcslsrymwus eu-central-1; akomftsfbnucktrladce eu-central-1 created 2026-06-21).
- erp-grupo-tesela public schema: 20 tables. Real data: cliente=28, proveedor=19, factura_holded=62, sociedad=1, perfil=1. claude_sessions=1, holded_facturas=1. All other ERP tables=0 rows.
- Security advisors (erp): 56 lints. WARN: claude_sessions allow_all_anon (ALL); rls_auto_enable SECURITY DEFINER anon-executable; current_rol/es_direccion/puede_ver_promocion authenticated-executable; auth_leaked_password_protection disabled; 24 anon + 24 authenticated GraphQL table-exposed (RLS still restricts rows). INFO: holded_facturas RLS-no-policy.
- Migrations in DB: 23 (last 20260621171853 create_claude_sessions). Repo migrations folder: 22 files (ends 20260619_22). => claude_sessions migration + holded_facturas table + rls_auto_enable/cleanup_old_sessions functions are DB-only (drift).
- cron.job: jobid 2 holded-sync-diario '0 6 * * *' active. job_run_details: 3 succeeded, last success 2026-06-22 06:00 UTC (timeout fix verified).
- storage.buckets: documentos (public=false).
- Edge functions ACTIVE: sync-holded v6, app v12, emit-facturas v4 (all verify_jwt=false).
- SECURITY DEFINER funcs: current_rol, encolar_factura, es_direccion, get_holded_keys, handle_new_user, marcar_unidad_estado, puede_ver_promocion, rls_auto_enable. get_holded_keys NOT anon/authenticated-exposed (correctly restricted).
- GitHub israel2606: 7 repos. PUBLIC: github-mcp-server (fork, holds ERP code), awesome-diagrams (fork, unrelated). PRIVATE (Tesela): SOT-SISTEMA-OPERATIVO-TESELA, Tesela-iA-v.0, Grupo-Tesela-ERP-Cerebro, client-s-command-center, Inversiones-MyInvestor.
- Open PRs on github-mcp-server: #14 (draft, mcpcurl coverage), #13 (list_issue_types; base misconfigured to a claude/ branch, not main).
- No secrets committed (git grep): no service_role/JWT/.env; only doc placeholders + upstream ghp_ handling code.
