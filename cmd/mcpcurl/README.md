# mcpcurl

A CLI tool that dynamically builds commands based on schemas retrieved from MCP servers that can
be executed against the configured MCP server.

## Overview

`mcpcurl` is a command-line interface that:

1. Connects to an MCP server via stdio
2. Dynamically retrieves the available tools schema
3. Generates CLI commands corresponding to each tool
4. Handles parameter validation based on the schema
5. Executes commands and displays responses

## Installation

## Usage

```bash
mcpcurl --stdio-server-cmd="<command to start MCP server>" <command> [flags]
```

The `--stdio-server-cmd` flag is required for all commands and specifies the command to run the MCP server.

### Available Commands

- `tools`: Contains all dynamically generated tool commands from the schema
- `schema`: Fetches and displays the raw schema from the MCP server
- `help`: Shows help for any command

### Examples

List available tools in Anthropic's MCP server:

```bash
% ./mcpcurl --stdio-server-cmd "docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN mcp/github" tools --help
Contains all dynamically generated tool commands from the schema

Usage:
  mcpcurl tools [command]

Available Commands:
  add_issue_comment     Add a comment to an existing issue
  create_branch         Create a new branch in a GitHub repository
  create_issue          Create a new issue in a GitHub repository
  create_or_update_file Create or update a single file in a GitHub repository
  create_pull_request   Create a new pull request in a GitHub repository
  create_repository     Create a new GitHub repository in your account
  fork_repository       Fork a GitHub repository to your account or specified organization
  get_file_contents     Get the contents of a file or directory from a GitHub repository
  get_issue             Get details of a specific issue in a GitHub repository.
  get_issue_comments    Get comments for a GitHub issue
  list_commits          Get list of commits of a branch in a GitHub repository
  list_issues           List issues in a GitHub repository with filtering options
  push_files            Push multiple files to a GitHub repository in a single commit
  search_code           Search for code across GitHub repositories
  search_issues         Search for issues and pull requests across GitHub repositories
  search_repositories   Search for GitHub repositories
  search_users          Search for users on GitHub
  update_issue          Update an existing issue in a GitHub repository

Flags:
  -h, --help   help for tools

Global Flags:
      --pretty                    Pretty print MCP response (only for JSON responses) (default true)
      --stdio-server-cmd string   Shell command to invoke MCP server via stdio (required)

Use "mcpcurl tools [command] --help" for more information about a command.
```

Get help for a specific tool:

```bash
 % ./mcpcurl --stdio-server-cmd "docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN mcp/github" tools get_issue --help
Get details of a specific issue in a GitHub repository.

Usage:
  mcpcurl tools get_issue [flags]

Flags:
  -h, --help                 help for get_issue
      --issue_number float   
      --owner string         
      --repo string

Global Flags:
      --pretty                    Pretty print MCP response (only for JSON responses) (default true)
      --stdio-server-cmd string   Shell command to invoke MCP server via stdio (required)

```

Use one of the tools:

```bash
 % ./mcpcurl --stdio-server-cmd "docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN mcp/github" tools get_issue --owner golang --repo go --issue_number 1
{
  "active_lock_reason": null,
  "assignee": null,
  "assignees": [],
  "author_association": "CONTRIBUTOR",
  "body": "by **rsc+personal@swtch.com**:\n\n\u003cpre\u003eWhat steps will reproduce the problem?\n1. Run build on Ubuntu 9.10, which uses gcc 4.4.1\n\nWhat is the expected output? What do you see instead?\n\nCgo fails with the following error:\n\n{{{\ngo/misc/cgo/stdio$ make\ncgo  file.go\ncould not determine kind of name for C.CString\ncould not determine kind of name for C.puts\ncould not determine kind of name for C.fflushstdout\ncould not determine kind of name for C.free\nthrow: sys·mapaccess1: key not in map\n\npanic PC=0x2b01c2b96a08\nthrow+0x33 /media/scratch/workspace/go/src/pkg/runtime/runtime.c:71\n    throw(0x4d2daf, 0x0)\nsys·mapaccess1+0x74 \n/media/scratch/workspace/go/src/pkg/runtime/hashmap.c:769\n    sys·mapaccess1(0xc2b51930, 0x2b01)\nmain·*Prog·loadDebugInfo+0xa67 \n/media/scratch/workspace/go/src/cmd/cgo/gcc.go:164\n    main·*Prog·loadDebugInfo(0xc2bc0000, 0x2b01)\nmain·main+0x352 \n/media/scratch/workspace/go/src/cmd/cgo/main.go:68\n    main·main()\nmainstart+0xf \n/media/scratch/workspace/go/src/pkg/runtime/amd64/asm.s:55\n    mainstart()\ngoexit /media/scratch/workspace/go/src/pkg/runtime/proc.c:133\n    goexit()\nmake: *** [file.cgo1.go] Error 2\n}}}\n\nPlease use labels and text to provide additional information.\u003c/pre\u003e\n",
  "closed_at": "2014-12-08T10:02:16Z",
  "closed_by": null,
  "comments": 12,
  "comments_url": "https://api.github.com/repos/golang/go/issues/1/comments",
  "created_at": "2009-10-22T06:07:26Z",
  "events_url": "https://api.github.com/repos/golang/go/issues/1/events",
  [...]
}
```

## Dynamic Commands

All tools provided by the MCP server are automatically available as subcommands under the `tools` command. Each generated command has:

- Appropriate flags matching the tool's input schema
- Validation for required parameters
- Type validation
- Enum validation (for string parameters with allowable values)
- Help text generated from the tool's description

## How It Works

1. `mcpcurl` makes a JSON-RPC request to the server using the `tools/list` method
2. The server responds with a schema describing all available tools
3. `mcpcurl` dynamically builds a command structure based on this schema
4. When a command is executed, arguments are converted to a JSON-RPC request
5. The request is sent to the server via stdin, and the response is printed to stdout
