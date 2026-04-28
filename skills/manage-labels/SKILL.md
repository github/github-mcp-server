---
name: manage-labels
description: Set up and maintain a consistent label scheme. Use when creating labels, organizing a label system, cleaning up labels, or standardizing label naming across a repository.
allowed-tools:
  - list_labels
  - list_label
  - label_write
  - search_issues
---

# Manage Labels

Create a consistent, useful label system for a repository.

## Available Tools
- `list_labels` / `list_label` — browse existing labels
- `label_write` — create, update, or delete labels
- `search_issues` — check label usage before deleting

## Best Practices
- Use prefixed names: type:bug, type:feature, priority:high, status:needs-triage.
- Use consistent colors within categories (all type: labels same color family).
- Write helpful descriptions — they appear in the label picker.
- Check label usage with `search_issues` before deleting or renaming.
- Aim for 15-25 labels total. Too many means none get used consistently.
