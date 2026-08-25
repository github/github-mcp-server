---
name: fix-dependabot
description: Handle vulnerable dependency alerts and update PRs. Use when fixing Dependabot alerts, updating vulnerable packages, reviewing dependency update PRs, or managing supply chain security.
allowed-tools:
  - list_dependabot_alerts
  - get_dependabot_alert
  - search_pull_requests
  - list_pull_requests
  - get_file_contents
---

# Fix Dependabot Alerts

Handle vulnerable dependency alerts systematically.

## Available Tools
- `list_dependabot_alerts` / `get_dependabot_alert` — list and inspect alerts
- `search_pull_requests` / `list_pull_requests` — find existing Dependabot PRs
- `get_file_contents` — read dependency files

## Workflow
1. List alerts sorted by severity — fix critical/high first.
2. Check if Dependabot already opened a PR for each alert.
3. For alerts with PRs: review the PR and merge if CI passes.
4. For alerts without PRs: check if the fix requires a major version bump.
5. Group related dependency updates into logical batches.

Check the alert's fixed_in version to understand the required update scope before acting.
