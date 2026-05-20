---
name: self-review-pr
description: Review your own PR before requesting team review. Use when you want to self-check your PR, verify CI status, polish description, or prepare your changes for review.
allowed-tools:
  - pull_request_read
  - get_file_contents
  - search_code
  - actions_get
  - get_job_logs
  - update_pull_request
  - update_pull_request_body
  - update_pull_request_title
  - request_pull_request_reviewers
---

# Self-Review PR

Review your own PR before asking others. Catch what you can so reviewers focus on what matters.

## Available Tools
- `pull_request_read` — read your diff, CI status, and files
- `get_file_contents` — check PR template compliance
- `search_code` — verify changes match codebase patterns
- `actions_get` / `get_job_logs` — investigate CI failures
- `update_pull_request` / `update_pull_request_body` / `update_pull_request_title` — fix PR metadata
- `request_pull_request_reviewers` — request reviewers when ready

## Checklist
1. Read your own diff — look for debug code, TODOs, unintended changes.
2. Check CI passes — if failing, fix before requesting review.
3. Verify description links relevant issues and follows the PR template.
4. Verify title follows repo conventions (conventional commits, etc.).
5. Request reviewers who own the affected code.

Don't request review with failing CI. Reviewers notice when you haven't self-reviewed.
