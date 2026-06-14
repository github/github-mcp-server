# Insiders Features

Insiders Mode gives you access to experimental features in the GitHub MCP Server. These features may change, evolve, or be removed based on community feedback.

We created this mode to have a way to roll out experimental features and collect feedback. So if you are using Insiders, please don't hesitate to share your feedback with us! 

> [!NOTE]
> Features in Insiders Mode are experimental.

## Enabling Insiders Mode

| Method | Remote Server | Local Server |
|--------|---------------|--------------|
| URL path | Append `/insiders` to the URL | N/A |
| Header | `X-MCP-Insiders: true` | N/A |
| CLI flag | N/A | `--insiders` |
| Environment variable | N/A | `GITHUB_INSIDERS=true` |

For configuration examples, see the [Server Configuration Guide](./server-configuration.md#insiders-mode).

---

## Tools added or changed by Insiders Mode

The list below is generated from the Go source. It covers tool **inventory and schema deltas** introduced by each Insiders feature flag — newly registered tools, or existing tools whose input schema or MCP metadata changes when the flag is on. Flags that only affect runtime behavior (e.g. output formatting or extra field lookups behind an existing schema) won't appear here; those are documented in the prose sections of this file.

<!-- START AUTOMATED INSIDERS TOOLS -->

### `remote_mcp_ui_apps`

- **create_pull_request** - Open new pull request
  - **Required OAuth Scopes**: `repo`
  - **MCP App UI**: `ui://github-mcp-server/pr-write`
  - `base`: Branch to merge into (string, required)
  - `body`: PR description (string, optional)
  - `draft`: Create as draft PR (boolean, optional)
  - `head`: Branch containing changes (string, required)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `title`: PR title (string, required)

- **get_me** - Get my user profile
  - **MCP App UI**: `ui://github-mcp-server/get-me`
  - No parameters required

- **issue_write** - Create or update issue/pull request
  - **Required OAuth Scopes**: `repo`
  - **MCP App UI**: `ui://github-mcp-server/issue-write`
  - `assignees`: Usernames to assign to this issue (string[], optional)
  - `body`: Issue body content (string, optional)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'. (number, optional)
  - `issue_number`: Issue number to update (number, optional)
  - `labels`: Labels to apply to this issue (string[], optional)
  - `method`: Write operation to perform on a single issue.
    Options are:
    - 'create' - creates a new issue.
    - 'update' - updates an existing issue.
     (string, required)
  - `milestone`: Milestone number (number, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `state`: New state (string, optional)
  - `state_reason`: Reason for the state change. Ignored unless state is changed. (string, optional)
  - `title`: Issue title (string, optional)
  - `type`: Type of this issue. Only use if the repository has issue types configured. Use list_issue_types tool to get valid type values for the organization. If the repository doesn't support issue types, omit this parameter. (string, optional)

### `remote_mcp_issue_fields`

- **issue_write** - Create or update issue/pull request
  - **Required OAuth Scopes**: `repo`
  - `assignees`: Usernames to assign to this issue (string[], optional)
  - `body`: Issue body content (string, optional)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'. (number, optional)
  - `issue_fields`: Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'. (object[], optional)
  - `issue_number`: Issue number to update (number, optional)
  - `labels`: Labels to apply to this issue (string[], optional)
  - `method`: Write operation to perform on a single issue.
    Options are:
    - 'create' - creates a new issue.
    - 'update' - updates an existing issue.
     (string, required)
  - `milestone`: Milestone number (number, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `state`: New state (string, optional)
  - `state_reason`: Reason for the state change. Ignored unless state is changed. (string, optional)
  - `title`: Issue title (string, optional)
  - `type`: Type of this issue. Only use if the repository has issue types configured. Use list_issue_types tool to get valid type values for the organization. If the repository doesn't support issue types, omit this parameter. (string, optional)

- **list_issue_fields** - List issue fields
  - **Required OAuth Scopes**: `repo`, `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `repo`, `write:org`
  - `owner`: The account owner of the repository or organization. The name is not case sensitive. (string, required)
  - `repo`: The name of the repository. When provided, returns fields for this specific repository (inherited from its organization). When omitted, returns org-level fields directly. (string, optional)

- **list_issues** - List issues
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `direction`: Order direction. If provided, the 'orderBy' also needs to be provided. (string, optional)
  - `field_filters`: Filter by custom issue field values. Each entry takes a field_name and a value; the server looks up the field and coerces the value to its type (single-select option name, text, number, or YYYY-MM-DD date). (object[], optional)
  - `labels`: Filter by labels (string[], optional)
  - `orderBy`: Order issues by field. If provided, the 'direction' also needs to be provided. (string, optional)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `since`: Filter by date (ISO 8601 timestamp) (string, optional)
  - `state`: Filter by state, by default both open and closed issues are returned when not provided (string, optional)

### `file_blame`

- **get_file_blame** - Get file blame information
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `end_line`: Optional 1-based ending line of the window of interest. Must be >= start_line when both are provided. (number, optional)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path to the file in the repository, relative to the repository root (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `ref`: Git reference (branch, tag, or commit SHA). Defaults to the repository's default branch (HEAD). (string, optional)
  - `repo`: Repository name (string, required)
  - `start_line`: Optional 1-based starting line of the window of interest. Only ranges overlapping [start_line, end_line] are returned, clamped to the window. (number, optional)

<!-- END AUTOMATED INSIDERS TOOLS -->

---

## MCP Apps

[MCP Apps](https://modelcontextprotocol.io/docs/extensions/apps) is an extension to the Model Context Protocol that enables servers to deliver interactive user interfaces to end users. Instead of returning plain text that the LLM must interpret and relay, tools can render forms, profiles, and dashboards right in the chat using MCP Apps.

This means you can interact with GitHub visually: fill out forms to create issues, see user profiles with avatars, open pull requests — all without leaving your agent chat.

### Supported tools

The following tools have MCP Apps UIs:

| Tool | Description |
|------|-------------|
| `get_me` | Displays your GitHub user profile with avatar, bio, and stats in a rich card |
| `issue_write` | Opens an interactive form to create or update issues |
| `create_pull_request` | Provides a full PR creation form to create a pull request (or a draft pull request) |

### Client requirements

MCP Apps requires a host that supports the [MCP Apps extension](https://modelcontextprotocol.io/docs/extensions/apps). Currently tested and working with:

- **VS Code Insiders** — enable via the `chat.mcp.apps.enabled` setting
- **Visual Studio Code** — enable via the `chat.mcp.apps.enabled` setting

---

## CSV output for list tools

CSV output mode returns supported list tool responses as CSV instead of JSON. This is intended to reduce response context for agents when scanning or summarising lists of GitHub data.

CSV output applies only to tools in default toolsets whose names start with `list_`, such as `list_issues`, `list_pull_requests`, `list_commits`, and `list_branches`. It does not add new tools or expose a tool argument for selecting the format; the server controls the response format through the Insiders feature flag.

### Format

- Nested objects are flattened into dot-notation columns, for example `user.login`, `category.name`, or `head.ref`.
- Arrays are represented as compact single-cell values joined with `;`.
- `body` fields are whitespace-normalized so multiline Markdown does not expand a list response into many output lines.
- Response metadata present in wrapped responses, such as `pageInfo.*` and `totalCount`, is emitted as `#`-prefixed lines before the CSV rows, followed by a blank line. Tools that return a root JSON array do not include metadata preamble lines.

### Enabling CSV output

CSV output is enabled by Insiders Mode. For local development, it can also be enabled explicitly with the `csv_output` feature flag:

```bash
github-mcp-server stdio --features csv_output
```

Because this changes list tool response shape, clients that require JSON list responses should avoid enabling this feature.

---

## How feature flags are resolved

> [!NOTE]
> This section is for contributors. End users only need the table at the top of this page.

Insiders is a **meta feature flag** — the same shape as `default` or `all` for toolsets. It expands once at startup into a curated set of individual feature flags, and from that point on every code path keys off concrete flags, never `InsidersMode` directly. New experimental work should always get its own flag and then be added to the insiders expansion list, never folded into `insiders` as a catch-all.

### Resolution order

1. **User input.** Users may opt into specific features:
   - Local server: `--features=<flag>,<flag>` CLI flag (or `GITHUB_FEATURES` env var).
   - Self-hosted HTTP server: `X-MCP-Features: <flag>,<flag>` request header.
2. **Allowlist filter.** User-supplied flags are filtered against [`AllowedFeatureFlags`](../pkg/github/feature_flags.go). Anything not on the allowlist is silently dropped — flags missing from the allowlist can only be turned on by remote-server feature management, not by end users.
3. **Insiders expansion.** If insiders mode is on (`--insiders`, `/insiders` route, or `X-MCP-Insiders: true`), every flag in [`InsidersFeatureFlags`](../pkg/github/feature_flags.go) is unioned in. The insiders expansion is **not** re-validated against the allowlist — insiders is a server-controlled switch that can reach internal-only flags.
4. **Server-side fallback (remote server only).** Any flag not yet decided falls back to the remote server's feature manager, which can roll a feature out independently of user input or insiders membership.

`AllowedFeatureFlags` and `InsidersFeatureFlags` are deliberately independent sets:

- A flag in **`AllowedFeatureFlags` only** is a regular opt-in: users can turn it on, but insiders does not auto-enable it. Granular issues/PRs flags work this way.
- A flag in **`InsidersFeatureFlags` only** is reachable through insiders (and remote-server rollouts), but cannot be enabled by user input. Internal-only experiments work this way.
- A flag in **both** is opt-in for end users *and* automatically on under insiders.

### Adding a new feature flag

1. Add a constant in `pkg/github/feature_flags.go`.
2. Add it to `AllowedFeatureFlags` if end users should be able to opt in via `--features` / `X-MCP-Features`.
3. Add it to `InsidersFeatureFlags` if insiders mode should turn it on automatically.
4. Gate the behavior on the concrete flag (`deps.IsFeatureEnabled(ctx, FeatureFlagX)`), never on `cfg.InsidersMode`. There is a `TestGitHubPackageDoesNotReadInsidersMode` guard test that fails if `pkg/github` reads `InsidersMode` directly.
5. The MCP-diff CI workflow picks up new entries in `AllowedFeatureFlags` automatically — see `.github/workflows/mcp-diff.yml`.
