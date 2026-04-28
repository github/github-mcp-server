---
name: search-code
description: Find code patterns, symbols, and examples across GitHub. Use when searching for code, finding how something is implemented, locating files, or looking for usage examples across repositories.
allowed-tools:
  - search_code
  - search_repositories
  - get_file_contents
---

# Search Code

Find specific code patterns across GitHub repositories.

## Available Tools
- `search_code` — search code with language:, org:, path: qualifiers
- `search_repositories` — find repos by name, topic, language
- `get_file_contents` — read full file context around matches

## Query Tips
- Use qualifiers in query: `language:go`, `org:github`, `path:src/`.
- Do NOT put `sort:` in the query string — use the separate `sort` parameter.
- After finding matches, read the full file with `get_file_contents` for context.
