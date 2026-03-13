# Error Handling

This document describes the error handling patterns used in the GitHub MCP Server, specifically how we handle GitHub API errors using the MCP SDK's `SetError`/`GetError` mechanism.

## Overview

The GitHub MCP Server uses the Go SDK's `CallToolResult.SetError()` to embed typed GitHub API errors directly in tool results. This approach enables:

1. **Tool Response Generation**: Return appropriate MCP tool error responses to clients
2. **Error Type Inspection**: Consumers can use `result.GetError()` with `errors.As` to extract typed errors for analysis

This is powered by the Go SDK v1.3.0+ `SetError`/`GetError` methods on `CallToolResult`, which embed a Go `error` in the result alongside the error text content.

## Error Types

### GitHubAPIError

Used for REST API errors from the GitHub API:

```go
type GitHubAPIError struct {
    Message  string           `json:"message"`
    Response *github.Response `json:"-"`
    Err      error            `json:"-"`
}
```

### GitHubGraphQLError

Used for GraphQL API errors from the GitHub API:

```go
type GitHubGraphQLError struct {
    Message string `json:"message"`
    Err     error  `json:"-"`
}
```

### GitHubRawAPIError

Used for raw HTTP API errors from the GitHub API:

```go
type GitHubRawAPIError struct {
    Message  string         `json:"message"`
    Response *http.Response `json:"-"`
    Err      error          `json:"-"`
}
```

## Usage Patterns

### For GitHub REST API Errors

When a GitHub REST API call fails, use:

```go
return ghErrors.NewGitHubAPIErrorResponse(ctx, message, response, err), nil
```

This function:
- Creates a `GitHubAPIError` with the provided message, response, and error
- Calls `result.SetError()` to embed the typed error in the tool result
- Returns a `CallToolResult` with `IsError: true` and error text content

### For GitHub GraphQL API Errors

```go
return ghErrors.NewGitHubGraphQLErrorResponse(ctx, message, err), nil
```

### For Raw HTTP API Errors

```go
return ghErrors.NewGitHubRawAPIErrorResponse(ctx, message, response, err), nil
```

### Extracting Errors from Results

Consumers (such as middleware or the remote server) can extract typed errors from results:

```go
if err := result.GetError(); err != nil {
    var apiErr *errors.GitHubAPIError
    if errors.As(err, &apiErr) {
        // Access apiErr.Response.StatusCode, apiErr.Message, etc.
    }

    var gqlErr *errors.GitHubGraphQLError
    if errors.As(err, &gqlErr) {
        // Access gqlErr.Message, gqlErr.Err, etc.
    }
}
```

## Design Principles

### User-Actionable vs. Developer Errors

- **User-actionable errors** (authentication failures, rate limits, 404s) should be returned as failed tool calls using the error response functions
- **Developer errors** (JSON marshaling failures, internal logic errors) should be returned as actual Go errors that bubble up through the MCP framework

### Type Safety with SetError/GetError

All GitHub API error types implement the `error` interface with `Unwrap()` support, enabling:
- `errors.As()` to extract the specific error type (e.g., `*GitHubAPIError`)
- `errors.Is()` to check for the underlying cause
- Standard Go error handling patterns

## Benefits

1. **Type Safety**: Errors are embedded in the result as typed Go errors, not just strings
2. **Observability**: Middleware can inspect the specific types of GitHub API errors using `errors.As`
3. **Simplicity**: No context-based error storage or middleware setup required
4. **Debugging**: Detailed error information (HTTP status codes, response objects) is preserved
5. **Privacy**: Error inspection can be done programmatically using `errors.Is`/`errors.As` checks

## Example Implementation

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

The error can then be inspected by consumers:

```go
result, err := handler(ctx, request)
if err == nil && result.IsError {
    if apiErr := result.GetError(); apiErr != nil {
        var ghErr *errors.GitHubAPIError
        if errors.As(apiErr, &ghErr) {
            log.Printf("GitHub API error: status=%d message=%s",
                ghErr.Response.StatusCode, ghErr.Message)
        }
    }
}
```
