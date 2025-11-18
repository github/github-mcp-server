# Remote GitHub MCP Server 🚀

[![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_Server-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D) [![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Server-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D&quality=insiders)

Easily connect to the GitHub MCP Server using the hosted version – no local setup or runtime required.

**URL:** https://api.githubcopilot.com/mcp/

## About

The remote GitHub MCP server is built using this repository as a library, and binding it into GitHub server infrastructure with an internal repository. You can open issues and propose changes in this repository, and we regularly update the remote server to include the latest version of this code.

The remote server has [additional tools](#toolsets-only-available-in-the-remote-mcp-server) that are not available in the local MCP server, such as the `create_pull_request_with_copilot` tool for invoking Copilot coding agent.

## Remote MCP Toolsets

Below is a table of available toolsets for the remote GitHub MCP Server. Each toolset is provided as a distinct URL so you can mix and match to create the perfect combination of tools for your use-case. Add `/readonly` to the end of any URL to restrict the tools in the toolset to only those that enable read access. We also provide the option to use [headers](#headers) instead.

<!-- START AUTOMATED TOOLSETS -->
| Name           | Description                                      | API URL                                               | 1-Click Install (VS Code)                                                                                                                                                                                                 | Read-only Link                                                                                                 | 1-Click Read-only Install (VS Code)                                                                                                                                                                                                 |
|----------------|--------------------------------------------------|-------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| all            | All available GitHub MCP tools                    | https://api.githubcopilot.com/mcp/                    | [Install](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D)                                      | [read-only](https://api.githubcopilot.com/mcp/readonly)                                                      | [Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Freadonly%22%7D) |
| Stargazers     | GitHub Stargazers related tools                  | https://api.githubcopilot.com/mcp/x/stargazers        | [Install](https://insiders.vscode.dev/redirect/mcp/install?name=gh-stargazers&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fstargazers%22%7D)                   | [read-only](https://api.githubcopilot.com/mcp/x/stargazers/readonly)                                           | [Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=gh-stargazers&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fstargazers%2Freadonly%22%7D)                                                                    |

<!-- END AUTOMATED TOOLSETS -->

### Additional _Remote_ Server Toolsets

These toolsets are only available in the remote GitHub MCP Server and are not included in the local MCP server.

| Name                 | Description                                   | API URL                                     | 1-Click Install (VS Code)                                                                                                                                                                          | Read-only Link                                                    | 1-Click Read-only Install (VS Code)                                                                                                                                                                                     |
| -------------------- | --------------------------------------------- | ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Copilot  | Copilot related tools | https://api.githubcopilot.com/mcp/x/copilot | [Install](https://insiders.vscode.dev/redirect/mcp/install?name=gh-copilot&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fcopilot%22%7D)             | [read-only](https://api.githubcopilot.com/mcp/x/copilot/readonly)                                        | [Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=gh-copilot&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fcopilot%2Freadonly%22%7D)                                                              |
| Copilot Spaces  | Copilot Spaces tools | https://api.githubcopilot.com/mcp/x/copilot_spaces    | [Install](https://insiders.vscode.dev/redirect/mcp/install?name=gh-copilot_spaces&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fcopilot_spaces%22%7D)             | [read-only](https://api.githubcopilot.com/mcp/x/copilot_spaces/readonly)                                        | [Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=gh-copilot_spaces&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fcopilot_spaces%2Freadonly%22%7D)                                                              |
| GitHub support docs search | Retrieve documentation to answer GitHub product and support questions. Topics include: GitHub Actions Workflows, Authentication, ... | https://api.githubcopilot.com/mcp/x/github_support_docs_search | [Install](https://insiders.vscode.dev/redirect/mcp/install?name=gh-support&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fgithub_support_docs_search%22%7D) | [read-only](https://api.githubcopilot.com/mcp/x/github_support_docs_search/readonly) | [Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=gh-support&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Fx%2Fgithub_support_docs_search%2Freadonly%22%7D) |

### Optional Headers

The Remote GitHub MCP server has optional headers equivalent to the Local server env vars:

- `X-MCP-Toolsets`: Comma-separated list of toolsets to enable. E.g. "repos,issues".
    - Equivalent to `GITHUB_TOOLSETS` env var for Local server.
    - If the list is empty, default toolsets will be used. Invalid or unknown toolsets are silently ignored without error and will not prevent the server from starting. Whitespace is ignored.
- `X-MCP-Readonly`: Enables only "read" tools.
    - Equivalent to `GITHUB_READ_ONLY` env var for Local server.
    - If this header is empty, "false", "f", "no", "n", "0", or "off" (ignoring whitespace and case), it will be interpreted as false. All other values are interpreted as true.

Example:

```json
{
    "type": "http",
    "url": "https://api.githubcopilot.com/mcp/",
    "headers": {
        "X-MCP-Toolsets": "repos,issues",
        "X-MCP-Readonly": "true"
    }
}
```

### URL Path Parameters

The Remote GitHub MCP server supports the following URL path patterns:

- `/` - Default toolset (see ["default" toolset](../README.md#default-toolset))
- `/readonly` - Default toolset in read-only mode
- `/x/all` - All available toolsets
- `/x/all/readonly` - All available toolsets in read-only mode
- `/x/{toolset}` - Single specific toolset
- `/x/{toolset}/readonly` - Single specific toolset in read-only mode

Note: `{toolset}` can only be a single toolset, not a comma-separated list. To combine multiple toolsets, use the `X-MCP-Toolsets` header instead.

Example:

```json
{
    "type": "http",
    "url": "https://api.githubcopilot.com/mcp/x/issues/readonly"
}
```