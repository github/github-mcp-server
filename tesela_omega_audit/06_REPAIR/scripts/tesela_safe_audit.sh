#!/usr/bin/env bash
set -euo pipefail
echo "TESELA OMEGA SAFE AUDIT (read-only)"
OUT="${1:-tesela_omega_audit/01_INVENTORY}"
mkdir -p "$OUT"
find . -maxdepth 5 -type f ! -path "./.git/*" ! -path "./node_modules/*" \
  ! -path "./dist/*" ! -path "./build/*" ! -path "./tesela_omega_audit/*" \
  | sed 's#^\./##' | sort > "$OUT/file_inventory.txt"
git status --short || true
echo "Inventory written to $OUT/file_inventory.txt"
