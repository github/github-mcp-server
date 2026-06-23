# ROLLBACK PLAN
**Fecha:** 2026-06-22 (UTC)

- **DB grants/policies:** snapshot de `pg_policies`/grants antes de cambiar; rollback = re-aplicar snapshot.
- **Migraciones:** cada cambio en su commit → `git revert`; en BD, migración inversa.
- **Tablas eliminadas:** restaurar desde export previo (CSV/SQL) o PITR (tras Pro).
- **Repo movido:** mantener espejo hasta confirmar.
- **Regla:** ningún paso destructivo sin (1) backup verificado y (2) rollback escrito.
