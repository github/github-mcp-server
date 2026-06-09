---
description: Show the current state of a PR in israel2606/github-mcp-server (checks, comments, mergeability)
argument-hint: <pr-number>
allowed-tools: mcp__github__pull_request_read, mcp__github__get_job_logs, mcp__github__list_pull_requests
---

Read PR `$ARGUMENTS` from `israel2606/github-mcp-server` and report:

1. Title, state, `mergeable_state`, head SHA, branch.
2. Check runs: group by status (success/failure/pending/skipped). For each
   failure, fetch the job log (`tail_lines: 100`) and identify the root
   cause in one line.
3. Review threads: count resolved vs unresolved; for unresolved, show the
   path and one-line summary.
4. Any new commits on the branch since the last review.

Output a compact summary, not the raw API payloads.

Do NOT modify the PR or push anything.
