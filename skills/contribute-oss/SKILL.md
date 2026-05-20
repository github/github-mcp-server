---
name: contribute-oss
description: Fork, branch, and submit PRs to external repositories. Use when contributing to open source, forking a repo to make changes, or submitting a pull request to a project you don't own.
allowed-tools:
  - fork_repository
  - create_branch
  - push_files
  - create_pull_request
  - get_file_contents
  - search_repositories
  - pull_request_read
---

# Contribute to Open Source

Workflow for contributing to repos you don't have write access to.

## Available Tools
- `fork_repository` — fork upstream to your account
- `create_branch` — create feature branch on your fork
- `push_files` — push changes to your fork
- `create_pull_request` — PR from your fork to upstream
- `get_file_contents` — read CONTRIBUTING.md and templates
- `search_repositories` — find the repo
- `pull_request_read` — track your PR status

## Workflow
1. Read CONTRIBUTING.md and CODE_OF_CONDUCT.md first.
2. Fork the repo, create a feature branch (not main).
3. Keep changes small and focused — one concern per PR.
4. Follow the project's existing code style.
5. Create PR with clear description linking related issues.

Look for good-first-issue labels to find starter tasks. Don't submit large PRs without discussing scope first in an issue.
