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
