---
name: manage-project
description: Track and update work items in GitHub Projects (v2). Use when managing a project board, updating issue status fields, adding items to a project, querying project items, or posting project status updates.
allowed-tools:
  - projects_list
  - projects_get
  - projects_write
  - search_issues
  - search_pull_requests
---

# Manage Project Board

Track and update work items in GitHub Projects (v2).

## Available Tools
- `projects_list` — find projects for a user, org, or repo
- `projects_get` — get project details, fields, items, status updates
- `projects_write` — update project items, fields, and status
- `search_issues` / `search_pull_requests` — find items to add

## Workflow
1. `projects_list` to find the project.
2. `projects_get` with list_project_fields to understand field names, IDs, and types.
3. `projects_get` with list_project_items to browse current items.
4. `projects_write` to update fields, add items, or post status updates.

## Critical Rules
- Always call list_project_fields first — use EXACT field names (case-insensitive). Never guess field IDs.
- Paginate: loop while pageInfo.hasNextPage=true using after=pageInfo.nextCursor.
- Keep query, fields, and per_page identical across pages.

## Query Syntax for list_project_items
- AND: space-separated (label:bug priority:high)
- OR: comma inside qualifier (label:bug,critical)
- NOT: leading dash (-label:wontfix)
- State: state:open, state:closed, state:merged
- Type: is:issue, is:pr
- Assignment: assignee:@me
