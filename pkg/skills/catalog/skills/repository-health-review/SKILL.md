---
name: repository-health-review
description: "USE FOR: judging how well a repository is maintained overall — release cadence, branch and proposal age, issue inflow, build reliability, how many people can maintain it, and outstanding alert counts. NOT FOR: acting on any individual issue, alert or proposed change, or cutting a specific release."
---

# Assessing repository health

Produce an evidence-backed picture of how a repository is doing across a few
independent dimensions.

**This workflow makes no changes.** Every tool it uses is read-only. If a specific
issue, alert or pull request needs action, that is separate work — name it and hand it
over rather than starting it here.

## Sequence

Gather each dimension independently, so one unavailable source does not sink the
report.

1. **Activity.** `list_commits` with `since` set to ninety days ago gives commit
   volume and who is contributing. `list_releases` gives cadence — the gaps between
   `published_at` values say more than the count.
2. **Branch hygiene.** `list_branches`, then check the age of each branch's head
   commit. Many long-untouched branches usually mean the project does not prune, which
   is a mild signal on its own and a stronger one alongside slow pull request flow.
3. **Flow.** `list_pull_requests` with `state: open`, `sort: created` and
   `direction: asc` puts the oldest first; the age of the oldest and the shape of the
   distribution are the numbers worth reporting. `list_issues` with `state: OPEN`,
   `orderBy: CREATED_AT` and `direction: ASC` does the same for issues.
4. **CI reliability.** `actions_list` with `method: list_workflow_runs`, filtered to
   the default branch, over a recent window. The ratio of `conclusion: success` to
   failures on the default branch is the signal; failures on feature branches are
   expected and are not.
5. **Maintainer coverage.** `list_repository_collaborators` shows how many people can maintain
   the repository. One active committer is a risk worth naming plainly.
6. **Outstanding risk.** `list_code_scanning_alerts` and `list_dependabot_alerts` with
   `state: open`, **counted by severity only**. Do not open individual alerts here.
7. **Report per dimension**, each with the numbers behind it, and state which
   dimensions could not be assessed.

## Decision points

- **Absent history is not poor health.** A repository with few commits, no releases
  and no alerts may be new. Say *insufficient history* rather than scoring it.
- **Low activity is not decay.** A stable, finished library should look quiet. Weigh
  activity against open issues and unanswered pull requests, not on its own.
- **What counts as stale** depends on the project's own cadence. Derive the threshold
  from its release and merge intervals rather than importing a fixed number, and state
  the threshold you used.
- **When to stop.** This produces an assessment. If it surfaces something urgent — a
  critical alert, a default branch that has been failing for weeks — name it clearly
  and hand it over.

## Fields that matter

- `published_at` across releases — the intervals are the cadence.
- The head commit date per branch — branch names carry no age.
- `created_at` on the oldest open pull request — a single number that captures review
  responsiveness better than a count.
- `conclusion` and `head_branch` on workflow runs — filter to the default branch, or
  the failure rate is meaningless.
- `state` on alerts — count `open` only; `dismissed` and `fixed` are noise here.
- `affiliation` on collaborators — `direct` and `outside` answer different questions
  about who really maintains the repository.

## Confirm before changing anything

Nothing here changes anything, so no confirmation is needed to run it. If the
assessment leads to a proposal — pruning branches, closing stale issues, upgrading a
dependency — that proposal is a separate decision that needs its own agreement before
anyone acts.

## Empty and failure cases

- **A dimension cannot be assessed** — mark it *not assessed* and name the reason.
  Never infer health from a missing signal, and never let one unavailable source
  suppress the rest of the report.
- **No releases** — report no release history rather than a bad cadence; plenty of
  repositories legitimately never release.
- **No open issues or pull requests** — that is a clean result for flow, but check
  activity before calling it healthy, since it also describes an abandoned repository.
- **Alert sources unavailable** — say the risk dimension was not assessed. An empty
  result and a disabled feature are different answers.
- **A very large repository** — page rather than raising limits, and if a dimension is
  too large to sample fairly, say the number is approximate and how it was derived.
