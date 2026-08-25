---
name: address-pr-feedback
description: Handle review comments on your PR and push fixes. Use when you received PR feedback, need to respond to reviewer comments, resolve threads, or push fixes based on review.
allowed-tools:
  - pull_request_read
  - add_reply_to_pull_request_comment
  - resolve_review_thread
  - push_files
  - create_or_update_file
  - update_pull_request_branch
  - request_pull_request_reviewers
---

# Address PR Feedback

You received review feedback. Address it systematically, not piecemeal.

## Available Tools
- `pull_request_read` — read all review comments and threads
- `add_reply_to_pull_request_comment` — respond to reviewer comments
- `resolve_review_thread` — mark threads as resolved
- `push_files` / `create_or_update_file` — push fixes
- `update_pull_request_branch` — rebase/merge with base branch
- `request_pull_request_reviewers` — re-request review after addressing

## Workflow
1. Read ALL comments before responding — comments may be related.
2. Group related feedback and address together in one commit.
3. Reply to each comment explaining what you changed (or why you disagree).
4. Resolve threads only after addressing the concern — not before.
5. Push fixes, then re-request review.

Don't resolve threads without responding. Don't push fixes without explaining them in the thread.
