#!/bin/bash
# Automated secret/key scan for the repo
# Run every 12 hours via cron or CI

SCAN_REPORT="/workspaces/github-mcp-server/scripts/secret-scan-report-$(date +%Y%m%d%H%M).txt"

# Use gitleaks if available, else fallback to grep
if command -v gitleaks &> /dev/null; then
  gitleaks detect --source /workspaces/github-mcp-server --report-path "$SCAN_REPORT"
else
  echo "gitleaks not found, using grep fallback" > "$SCAN_REPORT"
  grep -rE '(private|secret|key|credential|password|token|api|pem|env|wallet|json|signer|controller|authority)' /workspaces/github-mcp-server >> "$SCAN_REPORT"
fi

# Print summary
if [ -s "$SCAN_REPORT" ]; then
  echo "[!] Secrets or keys found. See $SCAN_REPORT"
else
  echo "[+] No secrets or keys found."
fi
