---
name: explore-repo
description: Understand an unfamiliar codebase quickly. Use when exploring a new repo, understanding project structure, finding entry points, or getting oriented in code you haven't seen before.
allowed-tools:
  - get_repository_tree
  - get_file_contents
  - search_code
  - list_commits
  - list_branches
  - list_tags
---

# Explore Repository

Understand a new codebase systematically without reading every file.

## Available Tools
- `get_repository_tree` — full directory tree at any ref
- `get_file_contents` — read files and directories
- `search_code` — find patterns across the codebase
- `list_commits` — recent commit history
- `list_branches` / `list_tags` — branches and tags

## Workflow
1. `get_repository_tree` at root for structure overview.
2. Read README.md, CONTRIBUTING.md, and build/config files.
3. `list_commits` on main branch to find actively-changing areas.
4. `search_code` for imports and entry points to understand architecture.

Start with structure, then drill into active areas. Don't read every file.
