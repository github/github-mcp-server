# RAW FINDINGS
**Fecha:** 2026-06-22 (UTC). Evidencia cruda (resumen). Detalle de comandos en `COMMAND_LOG.md`.

- Supabase org uladuspfccwdyrmyklnk: 3 proyectos (jpojckqnhepiuwefyvdr eu-west-3; umyejimabqcslsrymwus eu-central-1; akomftsfbnucktrladce eu-central-1 / creado 2026-06-21).
- erp tablas/filas: cliente 28, proveedor 19, factura_holded 62, sociedad 1, perfil 1, claude_sessions 1, holded_facturas 1, resto 0.
- Advisors erp seguridad: 56 (1 rls_policy_always_true, 1 anon_secdef rls_auto_enable, 4 auth_secdef, 1 leaked_password, 24+24 graphql_exposed, 1 rls_no_policy holded_facturas).
- Advisors erp rendimiento: ~20 unindexed_fk, auth_rls_initplan, multiple_permissive (~14), auth_db_connections_absolute.
- Migraciones BD 23 (última 20260621171853_create_claude_sessions) vs repo 22.
- cron jobid 2 holded-sync-diario 0 6 * * * activo; 3 succeeded; último 2026-06-22 06:00.
- storage bucket documentos privado. Edge: app v12, sync-holded v6, emit-facturas v4.
- SECURITY DEFINER funcs: current_rol, encolar_factura, es_direccion, get_holded_keys (no anon/auth), handle_new_user, marcar_unidad_estado, puede_ver_promocion, rls_auto_enable.
- GitHub israel2606: 7 repos (público: github-mcp-server[fork,ERP], awesome-diagrams; privados: SOT, Tesela-iA-v.0, Grupo-Tesela-ERP-Cerebro, client-s-command-center, Inversiones-MyInvestor).
- git grep: sin secretos reales en repo; app/config.js publishable key (pública por diseño).
- claude_sessions row: repo "Grupo-Tesela-ERP-Cerebro", task ERP Cerebro, branch claude/frontend-deploy-ci (PR #11).
