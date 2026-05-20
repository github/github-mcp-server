---
name: create-issue
description: Create well-structured, searchable, actionable issues. Use when filing a bug report, requesting a feature, creating a task, or opening any new GitHub issue.
allowed-tools:
  - create_issue
  - search_issues
  - list_issue_types
  - get_file_contents
  - list_labels
---

# Create Issue

Create issues that are easy to find, understand, and act on.

## Available Tools
- `create_issue` — create the issue
- `search_issues` — check for duplicates first
- `list_issue_types` — discover available issue types
- `get_file_contents` — read issue templates in .github/ISSUE_TEMPLATE/
- `list_labels` — see available labels

## Workflow
1. Search for existing issues to avoid duplicates.
2. Check .github/ISSUE_TEMPLATE/ for templates and use them.
3. `list_issue_types` if the org supports typed issues.
4. Create with appropriate type, labels, and milestone.

Write actionable titles: "Fix X when Y" not "X is broken". Include reproduction steps for bugs.
