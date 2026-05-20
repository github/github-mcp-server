---
name: discover-github
description: Search for users, organizations, and repositories. Use when finding GitHub users, looking up organizations, discovering repos by topic or language, or managing your starred repositories.
allowed-tools:
  - search_users
  - search_orgs
  - search_repositories
  - list_starred_repositories
  - star_repository
  - unstar_repository
---

# Discover GitHub

Search for users, organizations, and repositories across GitHub.

## Available Tools
- `search_users` — find users by name, location, or profile
- `search_orgs` — find organizations
- `search_repositories` — find repos by name, topic, language, org
- `list_starred_repositories` — your starred repos
- `star_repository` / `unstar_repository` — manage stars

## Search Tips
- Use qualifiers: language:go, org:github, topic:mcp, stars:>100.
- Use separate `sort` and `order` parameters — don't put sort: in query strings.
- Star useful repos to build a personal reference library.
