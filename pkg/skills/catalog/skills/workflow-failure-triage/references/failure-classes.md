# Telling failure classes apart

Use this when the first log read is ambiguous — when the failure could plausibly be the
code, the infrastructure, or the configuration, and the choice changes what you
recommend. Every check below is available through `actions_list`, `actions_get` and
`get_job_logs`; none of it requires anything outside them.

## The three questions worth asking

### 1. Does it fail at the same step every time?

A run that has already been rerun has more than one attempt. Pull the job for the
current attempt with `actions_get` and `method: get_workflow_job`, and compare which
step holds the first non-successful `conclusion`.

- **Same step, same place** — the code or the configuration. Reruns will keep failing.
- **Different step each attempt, or no consistent place** — infrastructure. The work
  itself is probably fine.
- **It passed on a later attempt with no change in between** — record this as an
  unreliable job, not as a pass. It will recur, and reporting it as fixed is how it
  stays unfixed.

### 2. Did sibling jobs in the same run fail too?

`actions_list` with `method: list_workflow_jobs` gives every job's `conclusion` in one
call.

- **One job failed, the rest succeeded** — the failure belongs to that job. Look at what
  is specific to it: its matrix values, its inputs, the step that broke.
- **Every job failed at roughly the same point** — something shared broke: a
  credential, a service the whole run depends on, a checkout step. This is rarely a code
  failure, and rerunning one job will not tell you anything.
- **Jobs failed with no log at all** — they never started. A job they depended on failed
  first; that upstream job is the real subject.

### 3. Is it specific to this branch or this event?

`actions_list` with `method: list_workflow_runs` and a `workflow_runs_filter` lets you
ask the same workflow about a different `branch` or a different `event`.

- **Failing on this branch, passing on the default branch** — the change is implicated.
  This is the most useful signal available and it costs one call.
- **Failing everywhere, including runs that predate the change** — the failure is not
  the change. Say so explicitly, because it redirects the whole investigation.
- **Failing only for one `event`** — usually permissions. Runs from forks and scheduled
  runs get a narrower token and fewer secrets than runs from a push to the repository.

## What each answer implies

| Evidence | Class | Is a rerun meaningful? |
| --- | --- | --- |
| Same step every attempt, one job, branch-specific | Code | No |
| Same step every attempt, every job, all branches | Configuration | No |
| Different step or place each attempt | Infrastructure | Yes, once |
| Passed on a later attempt with nothing changed | Unreliable job | Already answered — report it |
| No log, job never started | Upstream job failed | No — triage the upstream job |

## When it stays ambiguous

Say so. Two honest lines — what the evidence shows and what it does not settle — are
worth more than a confident guess, and they let the person reading decide whether to
spend more on it.

Do not resolve ambiguity by rerunning repeatedly until the answer is convenient. If a
rerun really is the next step, it is one rerun, proposed openly, with agreement first.
