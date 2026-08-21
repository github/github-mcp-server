---
name: merge-pr
description: Get a PR to merge-ready state and merge it. Use when merging a pull request, checking if a PR is ready to merge, updating a PR branch, or converting a draft PR.
allowed-tools:
  - pull_request_read
  - merge_pull_request
  - update_pull_request_branch
  - update_pull_request_state
  - update_pull_request_draft_state
  - actions_get
---

# Merge Pull Request

Verify a PR is ready and merge it.

## Available Tools
- `pull_request_read` — check status, reviews, and CI
- `merge_pull_request` — merge the PR
- `update_pull_request_branch` — update branch if behind base
- `update_pull_request_draft_state` — convert draft to ready
- `actions_get` — check workflow run details

## Pre-Merge Checklist
1. CI: all checks must pass (use `pull_request_read` with get_status).
2. Reviews: required approvals present, no outstanding changes_requested.
3. Branch: if behind base, call `update_pull_request_branch`.
4. Draft: convert to ready with `update_pull_request_draft_state` if needed.
5. Merge method: match repo conventions (merge, squash, or rebase).

Never merge with failing checks. Never merge draft PRs without converting first.
