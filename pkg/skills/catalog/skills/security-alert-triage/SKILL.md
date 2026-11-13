---
name: security-alert-triage
description: "USE FOR: investigating GitHub security alerts — code scanning, Dependabot, secret scanning and advisories — and deciding what to fix now, schedule, dismiss, or escalate. NOT FOR: summarising overall repository health, or reviewing feedback on a pull request."
---

# Triaging security alerts

Turn four separate alert streams into one ordered list of decisions, each backed by
evidence a maintainer can check.

**This server reads alerts; it does not resolve them.** No tool here dismisses an
alert, merges a fix, or revokes a credential. The output is a recommendation with
evidence.

## Sequence

1. **Inventory every source that answers.** `list_code_scanning_alerts`,
   `list_dependabot_alerts` and `list_secret_scanning_alerts` with `state: open`.
   Enumerate `list_repository_security_advisories` separately: omit `state` for all
   advisories, or use `triage`, `draft`, `published` or `closed`. Record which
   sources answered and which did not — the difference matters, see the failure cases.
   When the question spans an organisation rather than one repository,
   `list_org_repository_security_advisories` with `org` gives the same advisory picture
   across every repository in it, and is the right starting point before fanning out.
2. **Handle secret scanning first**, ahead of any ordering by severity. A live
   credential is the only alert type where delay keeps causing harm.
3. **Order the rest by exploitability, not by count.** `severity` narrows the field;
   whether the code is reachable decides it.
4. **Read the individual alert** for anything you intend to recommend on:
   `get_code_scanning_alert`, `get_dependabot_alert` or `get_secret_scanning_alert`
   with the alert number. List responses are deliberately thin.
5. **Ground it in the code.** For a code scanning alert, read the flagged location
   with `get_file_contents`. For a dependency alert, `search_code` for the vulnerable
   symbol to find out whether the project reaches the affected path at all.
6. **Look up the advisory** behind a dependency alert with
   `list_global_security_advisories` or `get_global_security_advisory` by GHSA or CVE
   id. The advisory carries the patched versions and the real-world conditions.
7. **Recommend**, using the decision points below, naming the evidence each
   recommendation rests on.

`references/alert-sources.md` maps each source to its tools, its identifying fields,
and what resolution means there.

## Secret scanning comes first

An open secret scanning alert on a real credential means something is exposed right
now. The order is always: revoke, then rotate, then remove from history, then close
the alert. Revocation happens outside this server, so escalate to a person
immediately and name the credential type and where it appeared. Never recommend
dismissing a secret alert as a false positive without someone who can check the
credential confirming it — a wrongly dismissed live key stays live.

## Decision points

- **Fix now** — reachable, exploitable, and a fix exists. For a dependency that means
  `first_patched_version` is populated. Name the version to move to.
- **Schedule** — real, but not reachable in this project, or the fix needs a major
  upgrade. Give the reasoning and the trigger that would promote it.
- **Recommend dismissal** — genuinely not applicable: test-only code, a pattern the
  rule misreads, an unused path. State the rule, why it does not apply, and which
  dismissal reason fits. A person makes the call.
- **Escalate** — live credentials, anything already exploited, or anything you cannot
  assess with the data available. Escalating is a valid outcome, not a failure.

Severity is an input, not the answer. A critical alert in code that never runs
outranks nothing; a moderate one on a public entry point may outrank everything.

## Fields that matter

- `state` on alerts — `open` means outstanding. Advisory states are separate:
  `triage`, `draft`, `published`, `closed`; publication does not mean a fix is shipped.
- `rule.id` and `rule.security_severity_level` on a code scanning alert — the id lets
  you compare like with like across repositories.
- `most_recent_instance.location` — the file and line to actually read.
- `dependency.package` with `security_vulnerability.first_patched_version` — the two
  fields a dependency recommendation is built from.
- `secret_type` — identifies what kind of credential leaked, which decides how it is
  revoked and who owns the revocation.
- `resolution` on a closed secret alert — `revoked` is the only value that means the
  credential is dead.

## Confirm before anything is acted on

Every action this workflow leads to happens elsewhere: dismissing, upgrading,
revoking. Present the ordered recommendations with their evidence and get explicit
agreement before anyone acts. Never describe an alert as resolved on the strength of
a recommendation.

## Empty and failure cases

- **An empty list and a disabled feature look alike.** An empty result means no open
  alerts. An error mentioning permissions, or a feature not being enabled, means the
  source was not checked. Report those separately, and never fold an unchecked source
  into a clean result.
- **A source is unavailable** — name it as not assessed and continue with the rest.
- **No open alerts anywhere** — say which sources were checked and that all were
  clean. The list of sources is the useful part of that answer.
- **An advisory lookup fails** — recommend on the alert data alone and say the
  advisory could not be read.
