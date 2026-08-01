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
  - **所需 OAuth Scopes**：`repo`
  - **MCP App UI**：`ui://github-mcp-server/pr-write`
  - `base`: Branch to merge into (string, 必需)
  - `body`: PR description (string, 可选)
  - `draft`: Create as draft PR (boolean, 可选)
  - `head`: Branch containing changes (string, 必需)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], 可选)
  - `title`: PR title (string, 必需)

- **get_me** - Get my user profile
  - **MCP App UI**：`ui://github-mcp-server/get-me`
  - 无需参数

- **issue_write** - Create or update issue/pull request
  - **所需 OAuth Scopes**：`repo`
  - **MCP App UI**：`ui://github-mcp-server/issue-write`
  - `assignees`: Usernames to assign to this issue (string[], 可选)
  - `body`: Issue body content (string, 可选)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'. (number, 可选)
  - `issue_fields`: Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'. (object[], 可选)
  - `issue_number`: Issue number to update (number, 可选)
  - `labels`: Labels to apply to this issue (string[], 可选)
  - `method`: Write operation to perform on a single issue.
    Options are:
    - 'create' - creates a new issue.
    - 'update' - updates an existing issue.
     (string, 必需)
  - `milestone`: Milestone number (number, 可选)
  - `owner`: Repository owner (string, 必需)
  - `repo`: Repository name (string, 必需)
  - `state`: New state (string, 可选)
  - `state_reason`: Reason for the state change. Ignored unless state is changed. (string, 可选)
  - `title`: Issue title (string, 可选)
  - `type`: Type of this issue. Only use if issue types are enabled for this repository. Use list_issue_types tool to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter. (string, 可选)

- **ui_get** - Get UI data
  - **所需 OAuth Scopes（任一）**：`repo`, `read:org`
  - **可接受 OAuth Scopes**：`admin:org`, `read:org`, `repo`, `write:org`
  - `method`: The type of data to fetch (string, 必需)
  - `owner`: Repository owner (required for all methods) (string, 必需)
  - `repo`: Repository name (required for labels, assignees, milestones, branches, issue fields, reviewers) (string, 可选)

- **update_pull_request** - Edit pull request
  - **所需 OAuth Scopes**：`repo`
  - **MCP App UI**：`ui://github-mcp-server/pr-edit`
  - `base`: New base branch name (string, 可选)
  - `body`: New description (string, 可选)
  - `draft`: Mark pull request as draft (true) or ready for review (false) (boolean, 可选)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, 可选)
  - `owner`: Repository owner (string, 必需)
  - `pullNumber`: Pull request number to update (number, 必需)
  - `repo`: Repository name (string, 必需)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], 可选)
  - `state`: New state (string, 可选)
  - `title`: New title (string, 可选)

### `file_blame`

- **get_file_blame** - Get file blame information
  - **所需 OAuth Scopes**：`repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, 可选)
  - `end_line`: Optional 1-based ending line of the window of interest. Must be >= start_line when both are provided. (number, 可选)
  - `owner`: Repository owner (username or organization) (string, 必需)
  - `path`: Path to the file in the repository, relative to the repository root (string, 必需)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `ref`: Git reference (branch, tag, or commit SHA). Defaults to the repository's default branch (HEAD). (string, 可选)
  - `repo`: Repository name (string, 必需)
  - `start_line`: Optional 1-based starting line of the window of interest. Only ranges overlapping [start_line, end_line] are returned, clamped to the window. (number, 可选)

### `issue_dependencies`

- **issue_dependency_read** - Read issue dependencies
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The number of the issue (number, 必需)
  - `method`: The read operation to perform on a single issue's dependencies.
    Options are:
    1. get_blocked_by - List the issues that block this issue (this issue is blocked by them).
    2. get_blocking - List the issues that this issue blocks.
     (string, 必需)
  - `owner`: The owner of the repository (string, 必需)
  - `page`: Page number for pagination (min 1) (number, 可选)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, 可选)
  - `repo`: The name of the repository (string, 必需)

- **issue_dependency_write** - Change issue dependency
  - **所需 OAuth Scopes**：`repo`
  - `issue_number`: The number of the subject issue (number, 必需)
  - `method`: The action to perform.
    Options are:
    - 'add' - create the dependency relationship.
    - 'remove' - delete the dependency relationship. (string, 必需)
  - `owner`: The owner of the subject issue's repository (string, 必需)
  - `related_issue_number`: The number of the related issue to link or unlink (number, 必需)
  - `related_owner`: The owner of the related issue's repository. Defaults to 'owner' when omitted. (string, 可选)
  - `related_repo`: The name of the related issue's repository. Defaults to 'repo' when omitted. (string, 可选)
  - `repo`: The name of the subject issue's repository (string, 必需)
  - `type`: The relationship direction relative to the subject issue.
    Options are:
    - 'blocked_by' - the subject issue is blocked by the related issue.
    - 'blocking' - the subject issue blocks the related issue. (string, 必需)

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
