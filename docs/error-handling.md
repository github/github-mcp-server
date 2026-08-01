# 错误处理

本文档介绍 GitHub MCP Server 使用的错误处理模式，重点说明如何处理 GitHub API 错误，以及如何避免直接使用 mcp-go 错误类型。

## 概述

GitHub MCP Server 实现了一种自定义错误处理方式，主要用于：

1. **工具响应生成**：向客户端返回恰当的 MCP 工具错误响应
2. **中间件检查**：将详细错误信息存储在请求 context 中，供中间件分析

这种双重方式提供了更好的可观测性和调试能力，尤其适用于远程服务器部署。在这类部署中，了解失败性质（限流、身份验证、404、500 等）对验证和监控至关重要。

## 错误类型

### GitHubAPIError

用于 GitHub API 返回的 REST API 错误：

```go
type GitHubAPIError struct {
    Message  string           `json:"message"`
    Response *github.Response `json:"-"`
    Err      error            `json:"-"`
}
```

### GitHubGraphQLError

用于 GitHub API 返回的 GraphQL API 错误：

```go
type GitHubGraphQLError struct {
    Message string `json:"message"`
    Err     error  `json:"-"`
}
```

## 使用模式

### GitHub REST API 错误

请使用以下方式，而非直接返回 `mcp.NewToolResultError()`：

```go
return ghErrors.NewGitHubAPIErrorResponse(ctx, message, response, err), nil
```

此函数会：
- 使用提供的消息、响应和错误创建 `GitHubAPIError`
- 将错误存储在 context 中，供中间件检查
- 返回恰当的 MCP 工具错误响应

### GitHub GraphQL API 错误

```go
return ghErrors.NewGitHubGraphQLErrorResponse(ctx, message, err), nil
```

### Context 管理

错误处理系统使用 context 存储错误，以供稍后检查：

```go
// Initialize context with error tracking
ctx = errors.ContextWithGitHubErrors(ctx)

// Retrieve errors for inspection (typically in middleware)
apiErrors, err := errors.GetGitHubAPIErrors(ctx)
graphqlErrors, err := errors.GetGitHubGraphQLErrors(ctx)
```

## 设计原则

### 用户可处理的错误与开发者错误

- **用户可处理的错误**（身份验证失败、限流、404）应通过错误响应函数作为失败的工具调用返回
- **开发者错误**（JSON 编组失败、内部逻辑错误）应作为实际的 Go 错误返回，并经 MCP 框架向上传播

### Context 的限制

该方式旨在绕过 mcp-go 当前的限制：context 不会在请求处理的每一步中传播。通过将错误存储在 context 值中，中间件无需依赖 context 传播即可检查错误。

### 优雅的错误处理

context 中的错误存储操作被设计为优雅失败：即使 context 存储失败，工具仍会向客户端返回恰当的错误响应。

## 优点

1. **可观测性**：中间件可以检查发生的具体 GitHub API 错误类型
2. **调试**：保留详细错误信息，同时不在日志中暴露潜在的敏感数据
3. **验证**：远程服务器可使用错误类型和 HTTP 状态码验证变更没有破坏功能
4. **隐私**：可通过 `errors.Is` 检查以编程方式检查错误，而无需记录 PII

## 实现示例

```go
func GetIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
    return mcp.NewTool("get_issue", /* ... */),
        func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            owner, err := RequiredParam[string](request, "owner")
            if err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }
            
            client, err := getClient(ctx)
            if err != nil {
                return nil, fmt.Errorf("failed to get GitHub client: %w", err)
            }
            
            issue, resp, err := client.Issues.Get(ctx, owner, repo, issueNumber)
            if err != nil {
                return ghErrors.NewGitHubAPIErrorResponse(ctx,
                    "failed to get issue",
                    resp,
                    err,
                ), nil
            }
            
            return MarshalledTextResult(issue), nil
        }
}
```

该方式确保客户端能收到恰当的错误响应，并且任何中间件都能检查底层 GitHub API 错误，以用于监控和调试。
