---
name: get-context
description: Understand the current user, their permissions, and team membership. Use when starting any workflow, checking who you are, what you can access, or looking up team membership.
allowed-tools:
  - get_me
  - get_teams
  - get_team_members
---

# Get Context

Always call `get_me` first to establish who you are and what you can access.

## Available Tools
- `get_me` — your authenticated profile and permissions
- `get_teams` — teams you belong to
- `get_team_members` — members of a specific team
