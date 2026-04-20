---
name: pull-requests
description: Submit a multi-comment GitHub pull request review using the pending-review workflow (pull_request_review_write → add_comment_to_pending_review → submit_pending). Use when leaving line-specific feedback on a pull request, when asked to review a PR, or whenever creating any review with more than one comment.
---

## PR review workflow

PR review workflow: Always use 'pull_request_review_write' with method 'create' to create a pending review, then 'add_comment_to_pending_review' to add comments, and finally 'pull_request_review_write' with method 'submit_pending' to submit the review for complex reviews with line-specific comments.
