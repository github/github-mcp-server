---
name: account-and-team-context
description: "USE FOR: establishing who the authenticated account is, which organisations and teams it belongs to, who sits on a team, and who has write access to a repository. NOT FOR: finding an unfamiliar repository, or choosing what work to do next."
---

# Establishing whose account this is and what it reaches

Answer two questions before anything that depends on identity: who is acting, and what
that account can actually see or change.

## Sequence

1. **Resolve the acting account.** `get_me` returns the authenticated user. Its `login`
   is the value every later question is asked in terms of — "my pull requests", "issues
   assigned to me", "repositories I can push to" all mean that `login`.
2. **Resolve organisation membership** with `get_teams`. Called with no argument it
   returns the teams the authenticated account belongs to; pass `user` to ask the same
   about someone else, which only works for organisations the current credentials can
   already see.
3. **Expand a team into people** with `get_team_members`, using the organisation as
   `org` and the team's `team_slug`. The slug is the hyphenated form from the team's
   address, not its display name.
4. **Establish repository access** with `list_repository_collaborators`. Use
   `affiliation` to separate the populations: `direct` for people granted access on the
   repository itself, `outside` for collaborators who are not organisation members, and
   `all` for the combined view. The distinction matters because `all` includes people
   whose access comes from a team, who are invisible under `direct`.
5. **Resolve a name to an account** with `search_users` when you have a person and need
   their `login`. Qualify the `query` — a bare personal name matches widely.
6. **Say what you established, and what you could not.** An account's view of GitHub is
   bounded by its credentials, and a report that does not say so reads as complete.

## Decision points

- **"Me" is not a guess.** Any request phrased in the first person — my issues, my
  reviews, repositories I maintain — depends on `login`. Resolve it rather than
  inferring it from a repository owner or from earlier conversation.
- **Membership is not permission.** Being on a team says nothing on its own about a
  given repository; the team may have no grant there. Check access where access
  matters.
- **Absence is ambiguous.** A team or organisation missing from `get_teams` may not
  exist, or may simply be invisible to these credentials. Both are limited to
  organisations the current credentials reach, and neither can tell you which case you
  are in.
- **Names are not identities.** Display names collide and change; `login` does not.
  When a request names a person, resolve to a `login` before acting on it, and say
  which account you resolved to.

## Fields that matter

- `login` — the only durable identifier for an account. Report it alongside a display
  name whenever the two differ.
- `type` — distinguishes a user from an organisation. A great deal of confusion comes
  from treating an organisation as a person.
- `team_slug` versus a team's display name — only the slug addresses a team.
- `affiliation` — `direct` and `all` answer different questions, and quoting one as the
  other overstates or understates who can change the repository.
- `nextPage` — collaborator results are paginated. A large organisation's list is not
  the first page, and treating it as the whole set produces a confidently wrong answer
  about who has access.

## Confirm before acting on identity

Nothing here changes anything: all five are read-only. The care needed is about what is
done with the answer rather than about the calls themselves.

- **Confirm the person before acting on their behalf.** If a request names someone and
  the lookup returns more than one plausible account, ask which — do not pick the
  closest match and go on to assign, request, or mention them.
- **Membership lists are about people.** Report them for the question asked. Compiling
  a roster of an organisation because a lookup made it easy is not part of the task,
  and should be agreed before it is produced.
- **Do not narrate credentials.** Report what the account can reach; do not report the
  token, its scopes, or anything else about how access was obtained.

## Empty and failure cases

- **No teams returned** — the account belongs to none the credentials can see. Say
  exactly that. It is not the same as "the account belongs to no teams".
- **The team slug is wrong** — a display name was used in place of a slug. Re-derive it
  from the team's address rather than guessing a hyphenation.
- **Collaborators are refused** — listing collaborators needs push access to the
  repository. Report that the access question could not be answered, rather than
  reporting an empty list, which reads as "nobody has access".
- **A person search returns nothing** — the name may be a display name only, the account
  may have been renamed, or it may not exist. Say which you could not distinguish, and
  ask for a `login`.
- **A person search returns many** — do not choose. Present the candidates with enough
  to tell them apart and ask.
