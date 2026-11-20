# Read CI check details

The `actions_get` and `actions_list` tools support GitHub Actions and checks from external providers.
Enable the Actions toolset with `--toolsets=default,actions`.
These read-only methods use the GitHub Checks API.
Fine-grained tokens and GitHub Apps need repository **Checks: read** permission.
Classic tokens need `repo` scope for private repositories.

## Select a check

Use `actions_list` with `method: "list_check_runs"`.
Provide `owner`, `repo`, and exactly one selector:

| Selector | Meaning |
| --- | --- |
| `ref` | A commit SHA, branch, or tag. Use a SHA for a stable investigation. |
| `resource_id` | A check suite ID, passed as a string. Workflow-run summaries include `check_suite_id`. |

Use `check_runs_filter` to specify `check_name`, `status`, or `filter`.
The status values are `queued`, `in_progress`, and `completed`.
The default filter is `latest`. Use `all` to include earlier attempts.
The optional `app_id` filter only applies to reference lookup.

Check summaries include IDs, provider identity, status, conclusion, and browser links.
They do not include diagnostic output or annotation bodies.
Different attempts can have the same name. Use the check ID, not the name, for subsequent requests.

You can also get check IDs from these existing calls:

- `pull_request_read` with `get_check_runs` lists checks for the current PR head.
- `actions_list` with `list_workflow_jobs` returns `check_run_id` for each job with an upstream check link.

Job IDs and check-run IDs are not interchangeable.
The server extracts the check ID from the API-provided link.
It does not fetch that URL or add requests to enrich workflow lists.

## Read output and annotations

Call `actions_get` with these arguments to read one check's output:

```json
{
  "method": "get_check_run",
  "owner": "OWNER",
  "repo": "REPO",
  "resource_id": "CHECK_RUN_ID"
}
```

The result includes the output title, summary, text, and annotation count when GitHub provides them.
It does not include annotation bodies or fetch logs.
Provider text uses the server's content sanitizer.

Call `actions_list` with these arguments to read one annotation page:

```json
{
  "method": "list_check_run_annotations",
  "owner": "OWNER",
  "repo": "REPO",
  "resource_id": "CHECK_RUN_ID",
  "page": 1,
  "perPage": 30
}
```

Check lists and annotation lists return `next_page` when another page exists.
Pass that number as `page` to continue.
Each call fetches one page. The default page size is 30, and the maximum is 100.
Annotation pages contain file paths, line and column ranges, annotation levels, messages, and raw diagnostic details.
Individual diagnostic messages are not truncated.

## Response compatibility

Legacy clients receive a JSON text result.
Modern clients receive a `method` field and a method-specific result field in both JSON text and `structuredContent`.
The result fields are `check_runs`, `check_run`, and `check_run_annotations`.

These methods do not change `pull_request_read(get_status)`, which reads commit statuses rather than Checks API output.
Some Actions checks have no useful output or annotations. Use `get_job_logs` when the failure requires Actions logs.
