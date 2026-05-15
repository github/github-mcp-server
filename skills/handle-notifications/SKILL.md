---
name: handle-notifications
description: Systematically triage the current user's GitHub notifications inbox — enumerate unread items, prioritize by notification reason (review requests, mentions, assignments, security alerts), act on the high-priority ones, then dismiss the rest. Use when the user asks "what should I work on?", "catch me up on GitHub", "triage my inbox", "what needs my attention?", or otherwise wants to clear their notifications backlog.
---

## When to use

Use this skill when the user asks about their GitHub inbox, pending work, or outstanding notifications — any of:

- "What should I work on next?"
- "Catch me up on GitHub."
- "Triage my inbox."
- "What needs my attention?"
- "Clear my notifications."

## Workflow

1. **Enumerate.** Call `list_notifications` with `filter: "default"` (unread only — the common case). Switch to `filter: "include_read"` only if the user explicitly asks for a full sweep. Pass `since` as an RFC3339 timestamp to scope to recent activity (e.g. the last day or since the last triage).

2. **Partition by `reason`.** Each notification carries a `reason` field. Group into priority buckets:

   - **High — act or respond promptly:**
     - `review_requested` — someone is waiting on your review.
     - `mention` / `team_mention` — you were @-referenced.
     - `assign` — you were assigned an issue or PR.
     - `security_alert` — security advisory or Dependabot alert.
   - **Medium — read and decide:**
     - `author` — updates on threads you opened.
     - `comment` — replies on threads you participated in.
     - `state_change` — issue/PR closed or reopened.
   - **Low — usually safe to mark read without reading:**
     - `ci_activity` — workflow runs. Look only if you own CI for this repo.
     - `subscribed` — repo-watch updates on threads you haven't participated in.

3. **Drill in on high-priority.** For each high-priority notification, call `get_notification_details` to inspect the item, then take the appropriate action — leave a review (see the `review-pr` skill), comment, close, etc.

4. **Dismiss as you go.** After acting on (or deciding to skip) each high-priority item, call `dismiss_notification` with the `threadID` and a `state`:
   - `state: "done"` archives the notification so it no longer appears in default queries. Use for items you've fully resolved.
   - `state: "read"` keeps the notification visible but marks it acknowledged. Use for "I've seen this, coming back later."

5. **Bulk-close the noise.** After the high-priority pass, if a large medium/low bucket remains and the user is comfortable, call `mark_all_notifications_read`. Only do this with explicit user approval — a blanket mark-read can bury something the partitioning rules missed.

## Caveats

- **`read` vs `done` matters.** `read` leaves the notification in the default inbox; `done` removes it. Pick intentionally based on whether there's follow-up.
- **Silence chatty threads.** If one issue/PR is generating a flood, call `manage_notification_subscription` with action `ignore` to silence that specific thread. For an entire noisy repository, use `manage_repository_notification_subscription`.
- **Surface decisions, don't hide them.** After each bucket, summarize to the user what you acted on, what you dismissed, and what's left open for them. Do not silently mark-read a pile of notifications.
- **Respect scope.** If the user narrows to a specific repo ("triage my inbox for `owner/repo`"), pass `owner` and `repo` to `list_notifications` rather than filtering client-side after fetching everything.
