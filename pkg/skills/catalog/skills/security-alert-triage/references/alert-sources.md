# Alert sources

GitHub reports security problems through four independent systems. They use different
identifiers, different severity vocabularies, and different meanings for *closed*.
Keep them apart when reporting.

## Code scanning

- **Tools** — `list_code_scanning_alerts`, `get_code_scanning_alert`.
- **What it finds** — problems in this repository's own code, from analysis tools.
- **Identity** — alert number, plus `rule.id`. The rule id is stable across
  repositories, so it is the right thing to group or compare on.
- **Severity** — `rule.security_severity_level` for security rules,
  `rule.severity` for the rest. Filtering accepts both vocabularies, which is a
  common source of surprising empty results.
- **Where to look** — `most_recent_instance.location` gives file, line and column.
- **Closed means** — `fixed` when the code changed, `dismissed` when a person judged
  it inapplicable.

## Dependabot

- **Tools** — `list_dependabot_alerts`, `get_dependabot_alert`.
- **What it finds** — known vulnerabilities in declared dependencies. It reasons about
  manifests, not about whether the project calls the affected code.
- **Identity** — alert number, plus the advisory GHSA id.
- **Severity** — `low`, `medium`, `high`, `critical`.
- **The decisive fields** — `dependency.package` and
  `security_vulnerability.first_patched_version`. No patched version means an upgrade
  is not yet the answer.
- **Reachability** — the alert cannot tell you. Searching the code for the vulnerable
  symbol is what turns a list into a priority.
- **Closed means** — `fixed`, `dismissed`, or `auto_dismissed` where GitHub decided it
  no longer applies.

## Secret scanning

- **Tools** — `list_secret_scanning_alerts`, `get_secret_scanning_alert`.
- **What it finds** — credentials committed to the repository.
- **Identity** — alert number, plus `secret_type`.
- **Severity** — none. Treat every open alert as urgent until a person confirms the
  credential is dead.
- **The decisive fields** — `secret_type` says what to revoke and where; the alert's
  own state and resolution say whether anyone has already acted.
- **Closed means** — `resolution` is one of `revoked`, `false_positive`, `wont_fix`,
  `used_in_tests`, `pattern_edited`, `pattern_deleted`. Only `revoked` means the
  credential can no longer be used.

## Advisories

- **Tools** — `list_repository_security_advisories` for this repository's own
  advisories; `list_global_security_advisories` and `get_global_security_advisory` for
  the public database.
- **What it is** — the description of a vulnerability, not an alert about your code.
  Affected version ranges, patched versions and real-world context live here.
- **Identity** — GHSA id, usually with a CVE id.
- **Use it to** — explain *why* a Dependabot alert matters, and to check whether a
  proposed upgrade actually clears the affected range.

## Reporting across sources

- Count and name each source separately; a single total hides which system found what.
- Say explicitly which sources were checked. An unchecked source is not a clean one.
- Use each source's own severity words rather than flattening them onto one scale.
