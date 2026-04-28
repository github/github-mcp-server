---
name: triage-issues
description: Categorize, deduplicate, and prioritize incoming issues. Use when triaging issues, labeling bugs, organizing a backlog, closing duplicates, or processing new issue reports.
allowed-tools:
  - list_issues
  - search_issues
  - issue_read
  - list_issue_types
  - issue_write
  - update_issue_labels
  - update_issue_type
  - update_issue_milestone
  - update_issue_state
  - update_issue_title
  - update_issue_body
  - update_issue_assignees
  - add_issue_comment
  - set_issue_fields
  - list_labels
  - get_label
---

# Triage Issues

Systematically process incoming issues: categorize, deduplicate, and prioritize.

## Available Tools
- `list_issues` / `search_issues` / `issue_read` — find and read issues
- `list_issue_types` — discover org issue types
- `update_issue_labels` / `update_issue_type` / `update_issue_milestone` — categorize
- `update_issue_state` — close duplicates or invalid issues
- `add_issue_comment` — ask for info or note triage decisions
- `list_labels` / `get_label` — check available labels

## Workflow
1. `list_issue_types` to understand the org's issue taxonomy.
2. For each new issue:
   a. `search_issues` for duplicates before doing anything else.
   b. Apply labels for type (bug, feature, docs) and priority.
   c. Set issue type if the org uses typed issues.
   d. Assign to milestone if applicable.
   e. Close duplicates with state_reason not_planned and link to the original.
3. Comment on issues that need more info from the reporter.

Always set state_reason when closing: completed or not_planned. Never close without a reason.
