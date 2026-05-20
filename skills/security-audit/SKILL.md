---
name: security-audit
description: Systematically review code scanning, secret, and dependency alerts. Use when auditing repo security, checking for vulnerabilities, reviewing CodeQL alerts, or investigating exposed secrets.
allowed-tools:
  - list_code_scanning_alerts
  - get_code_scanning_alert
  - list_secret_scanning_alerts
  - get_secret_scanning_alert
  - list_dependabot_alerts
  - get_dependabot_alert
  - get_file_contents
  - search_code
---

# Security Audit

Systematically review all security alerts across a repository.

## Available Tools
- `list_code_scanning_alerts` / `get_code_scanning_alert` — static analysis findings
- `list_secret_scanning_alerts` / `get_secret_scanning_alert` — exposed credentials
- `list_dependabot_alerts` / `get_dependabot_alert` — vulnerable dependencies
- `get_file_contents` / `search_code` — review code around alerts

## Triage Order
1. Secret scanning first — exposed credentials need immediate rotation.
2. Code scanning — static analysis alerts, prioritize critical/high severity.
3. Dependabot — vulnerable dependencies, prioritize by CVSS score.

For each alert: read full details, review the affected code, check if the same pattern exists elsewhere with `search_code`.

Don't dismiss alerts without understanding them. Check if previously-dismissed alerts were properly triaged.
