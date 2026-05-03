---
name: debug-ci
description: Investigate and fix failing GitHub Actions workflows. Use when CI is failing, a workflow run errored, you need to read build logs, or debug why tests aren't passing.
allowed-tools:
  - actions_get
  - get_job_logs
  - actions_list
  - get_file_contents
  - pull_request_read
---

# Debug CI Failure

Investigate failing GitHub Actions systematically.

## Available Tools
- `actions_get` — workflow run details, job list (use get_workflow_run, list_workflow_jobs)
- `get_job_logs` — logs from a specific failed job
- `actions_list` — list recent runs for comparison
- `get_file_contents` — read workflow YAML definitions
- `pull_request_read` — check PR-linked CI status

## Workflow
1. `actions_get` with get_workflow_run for the failed run.
2. `actions_get` with list_workflow_jobs to find which jobs failed.
3. `get_job_logs` for EACH failed job — don't stop at the first one.
4. Read the workflow file in .github/workflows/ to understand the pipeline.
5. Compare with recent passing runs via `actions_list` to spot what changed.

## Anti-Patterns
- Don't just rerun without reading logs — flaky tests need fixes, not retries.
- Don't read only the first failure — later jobs may reveal the root cause.
- Check if the failure is in workflow config vs application code.
