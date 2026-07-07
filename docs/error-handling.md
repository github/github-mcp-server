# Error Handling

This document describes the error handling patterns used in the GitHub MCP Server, specifically how we handle GitHub API errors and avoid direct use of mcp-go error types.

## Overview

The GitHub MCP Server implements a custom error handling approach that serves two primary purposes:

1. **Tool Response Generation**: Return appropriate MCP tool error responses to clients
2. **Middleware Inspection**: Store detailed error information in the request context for middleware analysis

This dual approach enables better observability and debugging capabilities, particularly for remote server deployments where understanding the nature of failures (rate limiting, authentication, 404s, 500s, etc.) is crucial for validation and monitoring.

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

## Usage Patterns

### For GitHub REST API Errors

Instead of directly returning `mcp.NewToolResultError()`, use:

```go
return ghErrors.NewGitHubAPIErrorResponse(ctx, message, response, err), nil
```

This function:
- Creates a `GitHubAPIError` with the provided message, response, and error
- Stores the error in the context for middleware inspection
- Returns an appropriate MCP tool error response

### For GitHub GraphQL API Errors

```go
return ghErrors.NewGitHubGraphQLErrorResponse(ctx, message, err), nil
```

### Context Management

The error handling system uses context to store errors for later inspection:

```go
// Initialize context with error tracking
ctx = errors.ContextWithGitHubErrors(ctx)

// Retrieve errors for inspection (typically in middleware)
apiErrors, err := errors.GetGitHubAPIErrors(ctx)
graphqlErrors, err := errors.GetGitHubGraphQLErrors(ctx)
```

## Design Principles

### User-Actionable vs. Developer Errors

- **User-actionable errors** (authentication failures, rate limits, 404s) should be returned as failed tool calls using the error response functions
- **Developer errors** (JSON marshaling failures, internal logic errors) should be returned as actual Go errors that bubble up through the MCP framework

### Context Limitations

This approach was designed to work around current limitations in mcp-go where context is not propagated through each step of request processing. By storing errors in context values, middleware can inspect them without requiring context propagation.

### Graceful Error Handling

Error storage operations in context are designed to fail gracefully - if context storage fails, the tool will still return an appropriate error response to the client.

## Benefits

1. **Observability**: Middleware can inspect the specific types of GitHub API errors occurring
2. **Debugging**: Detailed error information is preserved without exposing potentially sensitive data in logs
3. **Validation**: Remote servers can use error types and HTTP status codes to validate that changes don't break functionality
4. **Privacy**: Error inspection can be done programmatically using `errors.Is` checks without logging PII

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

This approach ensures that both the client receives an appropriate error response and any middleware can inspect the underlying GitHub API error for monitoring and debugging purposes.
