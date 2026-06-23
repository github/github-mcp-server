#!/usr/bin/env bash
set -euo pipefail
echo "TESELA OMEGA HEALTHCHECK (local only, no production)"
# Go (este repo)
if [ -f go.mod ]; then
  echo "== go build =="; go build ./... 2>&1 | tail -20 || echo "build issues (see above)"
fi
# Verifica presencia de variables sin imprimir valores
for v in SUPABASE_URL SUPABASE_SERVICE_ROLE_KEY; do
  if [ -n "${!v:-}" ]; then echo "$v: set"; else echo "$v: (unset)"; fi
done
echo "Healthcheck done. No production systems were contacted."
