---
name: manage-sub-issues
description: Break down large issues into trackable sub-tasks. Use when decomposing epics, creating task breakdowns, organizing work into smaller pieces, or managing parent-child issue relationships.
allowed-tools:
  - issue_read
  - create_issue
  - sub_issue_write
  - add_sub_issue
  - remove_sub_issue
  - reprioritize_sub_issue
  - search_issues
---

# Manage Sub-Issues

Break down epics and large issues into small, trackable sub-tasks.

## Available Tools
- `issue_read` — read parent issue details
- `create_issue` — create sub-issue
- `add_sub_issue` — link sub-issue to parent
- `remove_sub_issue` — unlink a sub-issue
- `reprioritize_sub_issue` — reorder sub-issues by priority
- `search_issues` — find related issues

## Workflow
1. Read the parent issue to understand full scope.
2. Break into small, independently completable pieces — each should map to one PR.
3. `add_sub_issue` to link each to the parent.
4. `reprioritize_sub_issue` to order by dependency (do X before Y).

Keep parent issue description updated as the breakdown evolves.
