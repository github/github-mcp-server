#!/usr/bin/env bash
set -euo pipefail
# TESELA OMEGA AUTOREPAIR - DRAFT SAFE MODE
# Por defecto NO hace nada destructivo. Requiere --confirm y pasos explícitos.
DRY_RUN=1
for a in "$@"; do
  case "$a" in
    --confirm) DRY_RUN=0 ;;
    *) ;;
  esac
done
echo "TESELA OMEGA AUTOREPAIR - DRAFT SAFE MODE"
echo "No destructive actions will be executed. DRY_RUN=$DRY_RUN"
echo
echo "Planned (manual + approved only):"
echo " 1. [DB] revoke anon on public.claude_sessions; drop policy allow_all_anon;  (erp + CONEXION)"
echo " 2. [Supabase] upgrade to Pro + enable PITR + manual logical backup"
echo " 3. [DB] revoke execute on function public.rls_auto_enable() from anon;"
echo " 4. [Auth] enable leaked password protection"
echo " 5. [Repo] git checkout -b omega-audit-repair/$(date +%Y%m%d)  (move ERP to private repo)"
if [ "$DRY_RUN" -eq 1 ]; then
  echo
  echo "DRY-RUN: nothing executed. Re-run with --confirm AND uncomment guarded blocks after approval."
  exit 0
fi
echo "Refusing to auto-execute destructive/production changes. Apply via reviewed migrations with backup + rollback."
exit 1
