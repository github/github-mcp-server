# GitHub MCP Server

The GitHub MCP Server is a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction)
server that provides seamless integration with GitHub APIs, enabling advanced
automation and interaction capabilities for developers and tools.

[![Install with Docker in VS Code](https://img.shields.io/badge/VS_Code-Install_Server-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&inputs=%5B%7B%22id%22%3A%22github_token%22%2C%22type%22%3A%22promptString%22%2C%22description%22%3A%22GitHub%20Personal%20Access%20Token%22%2C%22password%22%3Atrue%7D%5D&config=%7B%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-e%22%2C%22GITHUB_PERSONAL_ACCESS_TOKEN%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%2C%22env%22%3A%7B%22GITHUB_PERSONAL_ACCESS_TOKEN%22%3A%22%24%7Binput%3Agithub_token%7D%22%7D%7D) [![Install with Docker in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Server-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&inputs=%5B%7B%22id%22%3A%22github_token%22%2C%22type%22%3A%22promptString%22%2C%22description%22%3A%22GitHub%20Personal%20Access%20Token%22%2C%22password%22%3Atrue%7D%5D&config=%7B%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-e%22%2C%22GITHUB_PERSONAL_ACCESS_TOKEN%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%2C%22env%22%3A%7B%22GITHUB_PERSONAL_ACCESS_TOKEN%22%3A%22%24%7Binput%3Agithub_token%7D%22%7D%7D&quality=insiders)

## Use Cases

- Automating GitHub workflows and processes.
- Extracting and analyzing data from GitHub repositories.
- Building AI powered tools and applications that interact with GitHub's ecosystem.

## Prerequisites

1. To run the server in a container, you will need to have [Docker](https://www.docker.com/) installed.
2. [Create a GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new).
The MCP server can use many of the GitHub APIs, so enable the permissions that you feel comfortable granting your AI tools (to learn more about access tokens, please check out the [documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)).



## Installation

### Usage with VS Code

For quick installation, use one of the one-click install buttons at the top of this README.

For manual installation, add the following JSON block to your User Settings (JSON) file in VS Code. You can do this by pressing `Ctrl + Shift + P` and typing `Preferences: Open User Settings (JSON)`.

Optionally, you can add it to a file called `.vscode/mcp.json` in your workspace. This will allow you to share the configuration with others.

> Note that the `mcp` key is not needed in the `.vscode/mcp.json` file.

```json
{
  "mcp": {
    "inputs": [
      {
        "type": "promptString",
        "id": "github_token",
        "description": "GitHub Personal Access Token",
        "password": true
      }
    ],
    "servers": {
      "github": {
        "command": "docker",
        "args": [
          "run",
          "-i",
          "--rm",
          "-e",
          "GITHUB_PERSONAL_ACCESS_TOKEN",
          "ghcr.io/github/github-mcp-server"
        ],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
        }
      }
    }
  }
}
```

More about using MCP server tools in VS Code's [agent mode documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers).

### Usage with Claude Desktop

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "<YOUR_TOKEN>"
      }
    }
  }
}
```

### Build from source

If you don't have Docker, you can use `go` to build the binary in the
`cmd/github-mcp-server` directory, and use the `github-mcp-server stdio`
command with the `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable set to
your token.

## GitHub Enterprise Server

The flag `--gh-host` and the environment variable `GH_HOST` can be used to set
the GitHub Enterprise Server hostname.

## i18n / Overriding Descriptions

The descriptions of the tools can be overridden by creating a
`github-mcp-server-config.json` file in the same directory as the binary.

The file should contain a JSON object with the tool names as keys and the new
descriptions as values. For example:

```json
{
  "TOOL_ADD_ISSUE_COMMENT_DESCRIPTION": "an alternative description",
  "TOOL_CREATE_BRANCH_DESCRIPTION": "Create a new branch in a GitHub repository"
}
```

You can create an export of the current translations by running the binary with
the `--export-translations` flag.

This flag will preserve any translations/overrides you have made, while adding
any new translations that have been added to the binary since the last time you
exported.

```sh
./github-mcp-server --export-translations
cat github-mcp-server-config.json
```

You can also use ENV vars to override the descriptions. The environment
variable names are the same as the keys in the JSON file, prefixed with
`GITHUB_MCP_` and all uppercase.

For example, to override the `TOOL_ADD_ISSUE_COMMENT_DESCRIPTION` tool, you can
set the following environment variable:

```sh
export GITHUB_MCP_TOOL_ADD_ISSUE_COMMENT_DESCRIPTION="an alternative description"
```

## Tools

### Users

- **get_me** - Get details of the authenticated user
  - No parameters required

### Issues

- **get_issue** - Gets the contents of an issue within a repository

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `issue_number`: Issue number (number, required)

- **get_issue_comments** - Get comments for a GitHub issue

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `issue_number`: Issue number (number, required)

- **create_issue** - Create a new issue in a GitHub repository

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `title`: Issue title (string, required)
  - `body`: Issue body content (string, optional)
  - `assignees`: Usernames to assign to this issue (string[], optional)
  - `labels`: Labels to apply to this issue (string[], optional)

- **add_issue_comment** - Add a comment to an issue

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `issue_number`: Issue number (number, required)
  - `body`: Comment text (string, required)

- **list_issues** - List and filter repository issues

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `state`: Filter by state ('open', 'closed', 'all') (string, optional)
  - `labels`: Labels to filter by (string[], optional)
  - `sort`: Sort by ('created', 'updated', 'comments') (string, optional)
  - `direction`: Sort direction ('asc', 'desc') (string, optional)
  - `since`: Filter by date (ISO 8601 timestamp) (string, optional)
  - `page`: Page number (number, optional)
  - `perPage`: Results per page (number, optional)

- **update_issue** - Update an existing issue in a GitHub repository

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `issue_number`: Issue number to update (number, required)
  - `title`: New title (string, optional)
  - `body`: New description (string, optional)
  - `state`: New state ('open' or 'closed') (string, optional)
  - `labels`: New labels (string[], optional)
  - `assignees`: New assignees (string[], optional)
  - `milestone`: New milestone number (number, optional)

- **search_issues** - Search for issues and pull requests
  - `query`: Search query (string, required)
  - `sort`: Sort field (string, optional)
  - `order`: Sort order (string, optional)
  - `page`: Page number (number, optional)
  - `per_page`: Results per page (number, optional)

### Pull Requests

- **get_pull_request** - Get details of a specific pull request

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)

- **list_pull_requests** - List and filter repository pull requests

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `state`: PR state (string, optional)
  - `sort`: Sort field (string, optional)
  - `direction`: Sort direction (string, optional)
  - `perPage`: Results per page (number, optional)
  - `page`: Page number (number, optional)

- **merge_pull_request** - Merge a pull request

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `commit_title`: Title for the merge commit (string, optional)
  - `commit_message`: Message for the merge commit (string, optional)
  - `merge_method`: Merge method (string, optional)

- **get_pull_request_files** - Get the list of files changed in a pull request

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)

- **get_pull_request_status** - Get the combined status of all status checks for a pull request

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)

- **update_pull_request_branch** - Update a pull request branch with the latest changes from the base branch

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `expectedHeadSha`: The expected SHA of the pull request's HEAD ref (string, optional)

- **get_pull_request_comments** - Get the review comments on a pull request

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)

- **get_pull_request_reviews** - Get the reviews on a pull request

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)

- **create_pull_request_review** - Create a review on a pull request review

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `body`: Review comment text (string, optional)
  - `event`: Review action ('APPROVE', 'REQUEST_CHANGES', 'COMMENT') (string, required)
  - `commitId`: SHA of commit to review (string, optional)
  - `comments`: Line-specific comments array of objects, each object with path (string), position (number), and body (string) (array, optional)

- **create_pull_request** - Create a new pull request

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `title`: PR title (string, required)
  - `body`: PR description (string, optional)
  - `head`: Branch containing changes (string, required)
  - `base`: Branch to merge into (string, required)
  - `draft`: Create as draft PR (boolean, optional)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, optional)

### Repositories

- **create_or_update_file** - Create or update a single file in a repository

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `path`: File path (string, required)
  - `message`: Commit message (string, required)
  - `content`: File content (string, required)
  - `branch`: Branch name (string, optional)
  - `sha`: File SHA if updating (string, optional)

- **push_files** - Push multiple files in a single commit

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `branch`: Branch to push to (string, required)
  - `files`: Files to push, each with path and content (array, required)
  - `message`: Commit message (string, required)

- **search_repositories** - Search for GitHub repositories

  - `query`: Search query (string, required)
  - `sort`: Sort field (string, optional)
  - `order`: Sort order (string, optional)
  - `page`: Page number (number, optional)
  - `perPage`: Results per page (number, optional)

- **create_repository** - Create a new GitHub repository

  - `name`: Repository name (string, required)
  - `description`: Repository description (string, optional)
  - `private`: Whether the repository is private (boolean, optional)
  - `autoInit`: Auto-initialize with README (boolean, optional)

- **get_file_contents** - Get contents of a file or directory

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `path`: File path (string, required)
  - `ref`: Git reference (string, optional)

- **fork_repository** - Fork a repository

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `organization`: Target organization name (string, optional)

- **create_branch** - Create a new branch

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `branch`: New branch name (string, required)
  - `sha`: SHA to create branch from (string, required)

- **list_commits** - Gets commits of a branch in a repository
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `sha`: Branch name, tag, or commit SHA (string, optional)
  - `path`: Only commits containing this file path (string, optional)
  - `page`: Page number (number, optional)
  - `perPage`: Results per page (number, optional)

### Search

- **search_code** - Search for code across GitHub repositories

  - `query`: Search query (string, required)
  - `sort`: Sort field (string, optional)
  - `order`: Sort order (string, optional)
  - `page`: Page number (number, optional)
  - `perPage`: Results per page (number, optional)

- **search_users** - Search for GitHub users
  - `query`: Search query (string, required)
  - `sort`: Sort field (string, optional)
  - `order`: Sort order (string, optional)
  - `page`: Page number (number, optional)
  - `perPage`: Results per page (number, optional)

### Code Scanning

- **get_code_scanning_alert** - Get a code scanning alert

  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `alertNumber`: Alert number (number, required)

- **list_code_scanning_alerts** - List code scanning alerts for a repository
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `ref`: Git reference (string, optional)
  - `state`: Alert state (string, optional)
  - `severity`: Alert severity (string, optional)

## Resources

### Repository Content

- **Get Repository Content**
  Retrieves the content of a repository at a specific path.

  - **Template**: `repo://{owner}/{repo}/contents{/path*}`
  - **Parameters**:
    - `owner`: Repository owner (string, required)
    - `repo`: Repository name (string, required)
    - `path`: File or directory path (string, optional)

- **Get Repository Content for a Specific Branch**
  Retrieves the content of a repository at a specific path for a given branch.

  - **Template**: `repo://{owner}/{repo}/refs/heads/{branch}/contents{/path*}`
  - **Parameters**:
    - `owner`: Repository owner (string, required)
    - `repo`: Repository name (string, required)
    - `branch`: Branch name (string, required)
    - `path`: File or directory path (string, optional)

- **Get Repository Content for a Specific Commit**
  Retrieves the content of a repository at a specific path for a given commit.

  - **Template**: `repo://{owner}/{repo}/sha/{sha}/contents{/path*}`
  - **Parameters**:
    - `owner`: Repository owner (string, required)
    - `repo`: Repository name (string, required)
    - `sha`: Commit SHA (string, required)
    - `path`: File or directory path (string, optional)

- **Get Repository Content for a Specific Tag**
  Retrieves the content of a repository at a specific path for a given tag.

  - **Template**: `repo://{owner}/{repo}/refs/tags/{tag}/contents{/path*}`
  - **Parameters**:
    - `owner`: Repository owner (string, required)
    - `repo`: Repository name (string, required)
    - `tag`: Tag name (string, required)
    - `path`: File or directory path (string, optional)

- **Get Repository Content for a Specific Pull Request**
  Retrieves the content of a repository at a specific path for a given pull request.

  - **Template**: `repo://{owner}/{repo}/refs/pull/{prNumber}/head/contents{/path*}`
  - **Parameters**:
    - `owner`: Repository owner (string, required)
    - `repo`: Repository name (string, required)
    - `prNumber`: Pull request number (string, required)
    - `path`: File or directory path (string, optional)

## License

This project is licensed under the terms of the MIT open source license. Please refer to [MIT](./LICENSE) for the full terms.
