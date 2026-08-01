# mcpcurl

一个 CLI 工具，可根据从 MCP server 获取的 schema 动态构建命令，并可针对已配置的 MCP server 执行这些命令。

## 概述

`mcpcurl` 是一个命令行接口，可：

1. 通过 stdio 连接到 MCP server
2. 动态获取可用工具的 schema
3. 为每个工具生成相应的 CLI 命令
4. 根据 schema 处理参数验证
5. 执行命令并显示响应

## 安装

### 前提条件
- Go 1.24 or later
- 可通过 Docker 或本地构建访问 GitHub MCP Server

### 从源码构建
```bash
cd cmd/mcpcurl
go build -o mcpcurl
```

### 使用 Go Install
```bash
go install github.com/github/github-mcp-server/cmd/mcpcurl@latest
```

### 验证安装
```bash
./mcpcurl --help
```

## 用法

```console
mcpcurl --stdio-server-cmd="<command to start MCP server>" <command> [flags]
```

所有命令都需要 `--stdio-server-cmd` flag；它指定用于运行 MCP server 的命令。

### 可用命令

- `tools`：包含根据 schema 动态生成的所有工具命令
- `schema`：从 MCP server 获取并显示原始 schema
- `help`：显示任意命令的帮助信息

### 示例

列出 GitHub MCP server 中可用的工具：

```console
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
  get_issue             Get details of a specific issue in a GitHub repository
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

获取特定工具的帮助信息：

```console
 % ./mcpcurl --stdio-server-cmd "docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN mcp/github" tools get_issue --help
Get details of a specific issue in a GitHub repository

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

使用其中一个工具：

```console
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

## 动态命令

MCP server 提供的所有工具都会自动作为 `tools` 命令下的子命令提供。每个生成的命令具有：

- 与工具输入 schema 匹配的适当 flags
- 必填参数验证
- 类型验证
- Enum 验证（适用于具有允许值的字符串参数）
- 根据工具描述生成的帮助文本

## 工作原理

1. `mcpcurl` 使用 `tools/list` method 向 server 发起 JSON-RPC 请求
2. server 返回描述所有可用工具的 schema
3. `mcpcurl` 根据该 schema 动态构建命令结构
4. 执行命令时，arguments 会转换为 JSON-RPC 请求
5. 请求通过 stdin 发送到 server，响应打印到 stdout
