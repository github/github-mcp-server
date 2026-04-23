---
name: pull-requests
description: Submit a multi-comment GitHub pull request review using the pending-review workflow (pull_request_review_write → add_comment_to_pending_review → submit_pending). Use when leaving line-specific feedback on a pull request, when asked to review a PR, or whenever creating any review with more than one comment.
---

## When to use

Use this skill when submitting a pull request review that will include more than one comment, especially line-specific comments placed on particular files or diff lines.

**Skip this flow** — call `pull_request_review_write` with `method: "create"` and supply `body` and `event` directly — when:

- Leaving a single top-level comment with no line references.
- Approving or requesting changes without inline feedback.

## Workflow

Submit a multi-comment review using the three-step pending-review flow:

1. **Open a pending review.** Call `pull_request_review_write` with `method: "create"` **and no `event`**. Omitting `event` is what makes the review pending instead of submitting it immediately.
2. **Add each comment.** Call `add_comment_to_pending_review` once per comment, supplying `path` and a line reference (`line`/`side` for a single line, or `startLine`/`startSide` plus `line`/`side` for a multi-line range). This tool requires that a pending review already exists for the current user on this PR.
3. **Submit the review.** Call `pull_request_review_write` with `method: "submit_pending"`, an optional summary `body`, and an `event` indicating the review state — one of `APPROVE`, `REQUEST_CHANGES`, or `COMMENT`.

## Caveats

- **Always complete step 3.** A pending review is invisible to the PR author until `submit_pending` is called. If you stop partway through, the draft stays on the reviewer's side and can be resumed later or removed with `method: "delete_pending"`.
- **Do not pass `event` in step 1.** Providing `event` to `create` submits the review immediately and leaves no pending review for `add_comment_to_pending_review` to attach to.
