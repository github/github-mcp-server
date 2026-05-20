---
name: create-pr
description: Create a well-structured pull request that reviews smoothly. Use when opening a new PR, pushing changes for review, or submitting code changes to a repository.
allowed-tools:
  - create_pull_request
  - get_file_contents
  - create_branch
  - push_files
  - request_pull_request_reviewers
  - list_pull_requests
  - search_pull_requests
---

# Create Pull Request

Create a PR that communicates intent clearly and reviews smoothly.

## Available Tools
- `create_pull_request` — create the PR
- `get_file_contents` — read PR templates from repo
- `create_branch` — create a feature branch
- `push_files` — push multiple files in one commit
- `request_pull_request_reviewers` — request reviewers
- `list_pull_requests` / `search_pull_requests` — check for existing PRs

## Workflow
1. Look for PR template in `.github/`, `docs/`, or root (`pull_request_template.md`).
2. Check for existing PRs on the same branch with `list_pull_requests`.
3. Create PR with template-structured description.
4. Link issues using "Closes #N" or "Fixes #N" in the body.
5. Request reviewers who know the affected code areas.

Never create a PR without a description. Use the template if one exists.
