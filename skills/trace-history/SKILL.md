---
name: trace-history
description: Understand why code changed by tracing commits and PRs. Use when investigating git history, finding who changed something, understanding the motivation behind a change, or tracking down when a bug was introduced.
allowed-tools:
  - list_commits
  - get_commit
  - search_pull_requests
  - pull_request_read
---

# Trace Code History

Understand why code changed by following the commit to PR to discussion chain.

## Available Tools
- `list_commits` — commit history, filterable by path
- `get_commit` — full commit details and diff
- `search_pull_requests` — find PRs by commit SHA or keywords
- `pull_request_read` — read PR description and review discussion

## Workflow
1. `list_commits` with path filter to find relevant commits.
2. `get_commit` to see what changed.
3. `search_pull_requests` to find the PR (search by commit SHA or title keywords).
4. `pull_request_read` for the PR description and review comments — this has the *why*.

Commit messages say *what*. PR descriptions say *why*. Review comments say *what was considered*.
