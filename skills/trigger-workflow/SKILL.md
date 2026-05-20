---
name: trigger-workflow
description: Run, rerun, or cancel GitHub Actions workflow runs. Use when triggering a deployment, rerunning failed jobs, canceling a stuck workflow, or dispatching a workflow manually.
allowed-tools:
  - actions_run_trigger
  - actions_get
  - actions_list
  - get_job_logs
---

# Trigger Workflow

Run, rerun, or cancel GitHub Actions workflows.

## Available Tools
- `actions_run_trigger` — run_workflow, rerun_workflow_run, rerun_failed_jobs, cancel_workflow_run
- `actions_get` — list_workflows, get_workflow details
- `actions_list` — list recent runs
- `get_job_logs` — check results after run completes

## Tips
- Use rerun_failed_jobs instead of full rerun when only some jobs failed — faster.
- Check workflow definition for required inputs before triggering with run_workflow.
- Use cancel_workflow_run for stuck or unnecessary in-progress runs.
