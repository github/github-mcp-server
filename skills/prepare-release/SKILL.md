---
name: prepare-release
description: Compile release notes from commits and merged PRs. Use when preparing a release, writing a changelog, summarizing changes since last version, or reviewing what shipped.
allowed-tools:
  - list_releases
  - get_latest_release
  - get_release_by_tag
  - list_tags
  - get_tag
  - list_commits
  - search_pull_requests
---

# Prepare Release

Compile release notes from merged PRs and commits since the last release.

## Available Tools
- `list_releases` / `get_latest_release` / `get_release_by_tag` — browse releases
- `list_tags` / `get_tag` — version tags
- `list_commits` — commits since last release
- `search_pull_requests` — find merged PRs in the range

## Workflow
1. `get_latest_release` to find the last version tag.
2. `list_commits` since that tag to see all changes.
3. `search_pull_requests` for merged PRs in the range — PR descriptions are richer than commits.
4. Group changes: breaking changes, features, bug fixes, docs.
5. Link PR numbers in release notes for traceability.

Use PR titles and labels for categorization — commit messages alone are often too terse.
