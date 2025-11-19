---
date: 2025-11-19T17:07:20-05:00
git_commit: ec6afa776d8bebe0c0ed36926562b411e14c5bf4
branch: feature/reply-to-review-comments
repository: github-mcp-server
topic: "Reply To Review Comments - Implementation Details"
tags: [research, codebase, pullrequests, mcp-tools, github-api]
status: complete
last_updated: 2025-11-19
---

# Research: Reply To Review Comments - Implementation Details

**Date**: 2025-11-19 17:07:20 EST
**Git Commit**: ec6afa776d8bebe0c0ed36926562b411e14c5bf4
**Branch**: feature/reply-to-review-comments
**Repository**: github-mcp-server

## Research Question

Document the implementation patterns, file locations, and integration points required to add a `reply_to_review_comment` tool to the GitHub MCP server. This tool will enable AI agents to reply directly to individual pull request review comment threads using the GitHub REST API endpoint `POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies`.

## Summary

The GitHub MCP server follows consistent patterns for implementing tools that interact with the GitHub API. The new reply-to-review-comment feature should be implemented in `pkg/github/pullrequests.go` following the same patterns used by existing PR comment tools like `CreatePullRequest`, `AddCommentToPendingReview`, and `GetPullRequestReviewComments`. The implementation requires:

1. A new function that returns `(mcp.Tool, server.ToolHandlerFunc)` following the naming pattern `ReplyToReviewComment`
2. Use of the REST client obtained via `getClient(ctx)` and the `client.PullRequests.CreateComment` method (note: go-github v79 uses `CreateComment` with a parent comment ID parameter, not a separate `CreateReply` method)
3. Parameter validation using `RequiredParam` and `RequiredBigInt` helpers from `pkg/github/server.go`
4. Error handling via `ghErrors.NewGitHubAPIErrorResponse` from `pkg/errors/error.go`
5. MinimalResponse return format with ID and URL fields from `pkg/github/minimal_types.go`
6. Registration as a write tool in the pull_requests toolset in `pkg/github/tools.go`
7. Unit tests following the table-driven pattern with toolsnap validation in `pkg/github/pullrequests_test.go`

## Detailed Findings

### Component 1: Tool Implementation Pattern

**Location**: `pkg/github/pullrequests.go`

**Pattern**: All PR tools follow a consistent structure with these key elements:

1. **Function Signature** (`pullrequests.go:314-361`):
   ```go
   func ReplyToReviewComment(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc)
   ```
   - Returns a tuple of `(mcp.Tool, server.ToolHandlerFunc)`
   - Accepts `getClient` function and translation helper as parameters
   - Function name should be exported (capitalized)

2. **Tool Definition** (`pullrequests.go:315-360`):
   Uses `mcp.NewTool()` with:
   - Tool name (e.g., `"reply_to_review_comment"`)
   - Description via translation helper
   - Tool annotation with title and `ReadOnlyHint: ToBoolPtr(false)` for write tools
   - Parameter definitions using `mcp.WithString()`, `mcp.WithNumber()`, etc.
   - Required parameters marked with `mcp.Required()`

3. **Handler Function** (`pullrequests.go:362-444`):
   Anonymous function with signature:
   ```go
   func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
   ```
   
   Handler structure:
   - Parameter extraction and validation at the top
   - Client acquisition via `getClient(ctx)`
   - GitHub API call
   - Error handling with `ghErrors.NewGitHubAPIErrorResponse`
   - Response status check
   - Success response marshaling

**Example from CreatePullRequest** (`pullrequests.go:314-444`):
```go
func CreatePullRequest(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
    return mcp.NewTool("create_pull_request",
        mcp.WithDescription(t("TOOL_CREATE_PULL_REQUEST_DESCRIPTION", "...")),
        mcp.WithToolAnnotation(mcp.ToolAnnotation{
            Title:        t("TOOL_CREATE_PULL_REQUEST_USER_TITLE", "..."),
            ReadOnlyHint: ToBoolPtr(false),
        }),
        mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner")),
        // ... more parameters
    ),
    func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        owner, err := RequiredParam[string](request, "owner")
        if err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        // ... parameter extraction
        
        client, err := getClient(ctx)
        if err != nil {
            return nil, fmt.Errorf("failed to get GitHub client: %w", err)
        }
        
        pr, resp, err := client.PullRequests.Create(ctx, owner, repo, newPR)
        if err != nil {
            return ghErrors.NewGitHubAPIErrorResponse(ctx,
                "failed to create pull request",
                resp,
                err,
            ), nil
        }
        defer func() { _ = resp.Body.Close() }()
        
        if resp.StatusCode != http.StatusCreated {
            body, err := io.ReadAll(resp.Body)
            // ... error handling
        }
        
        minimalResponse := MinimalResponse{
            ID:  fmt.Sprintf("%d", pr.GetID()),
            URL: pr.GetHTMLURL(),
        }
        
        r, err := json.Marshal(minimalResponse)
        // ... return result
    }
}
```

### Component 2: Parameter Validation Utilities

**Location**: `pkg/github/server.go:69-219`

**Available Helpers**:

1. **RequiredParam[T]** (`server.go:69-88`):
   - Generic function for any comparable type
   - Returns error if parameter missing, wrong type, or zero value
   - Usage: `owner, err := RequiredParam[string](request, "owner")`

2. **RequiredInt** (`server.go:90-99`):
   - Converts float64 to int
   - Usage: `pullNumber, err := RequiredInt(request, "pullNumber")`

3. **RequiredBigInt** (`server.go:101-116`):
   - Converts float64 to int64 with overflow check
   - Usage for comment IDs: `commentID, err := RequiredBigInt(request, "comment_id")`
   - Important: Review comment IDs are int64 in go-github

4. **OptionalParam[T]** (`server.go:118-135`):
   - Returns zero value if parameter not present
   - Usage: `body, err := OptionalParam[string](request, "body")`

**Pattern**: All parameter validation happens at the beginning of the handler function, before any API calls. Validation errors return immediately with `mcp.NewToolResultError(err.Error()), nil`.

### Component 3: Error Handling Infrastructure

**Location**: `pkg/errors/error.go`

**Key Function**: `NewGitHubAPIErrorResponse` (`error.go:111-119`):
```go
func NewGitHubAPIErrorResponse(ctx context.Context, message string, resp *github.Response, err error) *mcp.CallToolResult
```

**Usage Pattern** (`pullrequests.go:419-424`):
```go
pr, resp, err := client.PullRequests.Create(ctx, owner, repo, newPR)
if err != nil {
    return ghErrors.NewGitHubAPIErrorResponse(ctx,
        "failed to create pull request",
        resp,
        err,
    ), nil
}
defer func() { _ = resp.Body.Close() }()
```

**Standard Error Checking** (`pullrequests.go:426-433`):
```go
if resp.StatusCode != http.StatusCreated {
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %w", err)
    }
    return mcp.NewToolResultError(fmt.Sprintf("failed to create pull request: %s", string(body))), nil
}
```

**Error Flow**:
1. API call returns `(result, *github.Response, error)`
2. If `err != nil`, wrap with `ghErrors.NewGitHubAPIErrorResponse` and return
3. Always defer close response body
4. Check status code and read body for non-success responses
5. Return `mcp.NewToolResultError` for validation errors

### Component 4: MinimalResponse Pattern

**Location**: `pkg/github/minimal_types.go:112-116`

**Definition**:
```go
type MinimalResponse struct {
    ID  string `json:"id"`
    URL string `json:"url"`
}
```

**Usage for Create Operations** (`pullrequests.go:435-444`):
```go
minimalResponse := MinimalResponse{
    ID:  fmt.Sprintf("%d", pr.GetID()),
    URL: pr.GetHTMLURL(),
}

r, err := json.Marshal(minimalResponse)
if err != nil {
    return nil, fmt.Errorf("failed to marshal response: %w", err)
}

return mcp.NewToolResultText(string(r)), nil
```

**Pattern**: Write tools (create/update) return MinimalResponse with ID and URL. Read tools return full JSON-marshaled objects.

### Component 5: GitHub REST Client Usage

**Client Acquisition**: Tools receive a `GetClientFn` parameter and call it to obtain the client:
```go
client, err := getClient(ctx)
if err != nil {
    return nil, fmt.Errorf("failed to get GitHub client: %w", err)
}
```

**go-github v79 Client** (`go.mod:6`):
- Library: `github.com/google/go-github/v79 v79.0.0`
- Type: `*github.Client`
- PR comment operations via `client.PullRequests.*` methods

**Review Comment API Methods** (`pullrequests.go:252-278`):
- **ListComments**: `client.PullRequests.ListComments(ctx, owner, repo, pullNumber, opts)` returns `[]*github.PullRequestComment`
- **CreateComment**: Used for creating review comments and replies (single method for both)
- **Expected for replies**: `client.PullRequests.CreateComment(ctx, owner, repo, commentID, comment)` where comment includes `InReplyTo` field

**Important**: The spec research mentions `CreateReply` method, but go-github v79 typically uses `CreateComment` with an `InReplyTo` field to create replies. The actual go-github API should be verified during implementation.

**Response Pattern**:
```go
comments, resp, err := client.PullRequests.ListComments(ctx, owner, repo, pullNumber, opts)
if err != nil {
    return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get comments", resp, err), nil
}
defer func() { _ = resp.Body.Close() }()
```

### Component 6: Toolset Registration

**Location**: `pkg/github/tools.go:243-262`

**Pull Requests Toolset** (`tools.go:243-262`):
```go
pullRequests := toolsets.NewToolset(ToolsetMetadataPullRequests.ID, ToolsetMetadataPullRequests.Description).
    AddReadTools(
        toolsets.NewServerTool(PullRequestRead(getClient, t, flags)),
        toolsets.NewServerTool(ListPullRequests(getClient, t)),
        toolsets.NewServerTool(SearchPullRequests(getClient, t)),
    ).
    AddWriteTools(
        toolsets.NewServerTool(MergePullRequest(getClient, t)),
        toolsets.NewServerTool(UpdatePullRequestBranch(getClient, t)),
        toolsets.NewServerTool(CreatePullRequest(getClient, t)),
        toolsets.NewServerTool(UpdatePullRequest(getClient, getGQLClient, t)),
        toolsets.NewServerTool(RequestCopilotReview(getClient, t)),
        
        // Reviews
        toolsets.NewServerTool(PullRequestReviewWrite(getGQLClient, t)),
        toolsets.NewServerTool(AddCommentToPendingReview(getGQLClient, t)),
    )
```

**Integration Point**: The new `ReplyToReviewComment` tool should be added to the `AddWriteTools` section, likely in the "Reviews" subsection after `AddCommentToPendingReview`:
```go
toolsets.NewServerTool(ReplyToReviewComment(getClient, t)),
```

**Toolset Structure** (`pkg/toolsets/toolsets.go:54-66`):
- Toolsets separate read and write tools
- Write tools require `ReadOnlyHint: ToBoolPtr(false)` in tool annotations
- Tools are wrapped with `toolsets.NewServerTool()` which accepts the `(mcp.Tool, server.ToolHandlerFunc)` tuple

### Component 7: Testing Patterns

**Location**: `pkg/github/pullrequests_test.go`

**Test Structure** (`pullrequests_test.go:20-143`):

1. **Toolsnap Validation** (verifies tool schema):
   ```go
   func Test_ReplyToReviewComment(t *testing.T) {
       mockClient := github.NewClient(nil)
       tool, _ := ReplyToReviewComment(stubGetClientFn(mockClient), translations.NullTranslationHelper)
       require.NoError(t, toolsnaps.Test(tool.Name, tool))
       
       assert.Equal(t, "reply_to_review_comment", tool.Name)
       assert.NotEmpty(t, tool.Description)
       assert.Contains(t, tool.InputSchema.Properties, "owner")
       assert.Contains(t, tool.InputSchema.Properties, "repo")
       assert.Contains(t, tool.InputSchema.Properties, "comment_id")
       assert.Contains(t, tool.InputSchema.Properties, "body")
       assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "comment_id", "body"})
   ```

2. **Table-Driven Behavioral Tests** (`pullrequests_test.go:56-143`):
   ```go
   tests := []struct {
       name           string
       mockedClient   *http.Client
       requestArgs    map[string]interface{}
       expectError    bool
       expectedResult interface{}
       expectedErrMsg string
   }{
       {
           name: "successful reply creation",
           mockedClient: mock.NewMockedHTTPClient(
               mock.WithRequestMatch(
                   mock.PostReposPullsCommentsByOwnerByRepoByCommentNumber,
                   mockReplyComment,
               ),
           ),
           requestArgs: map[string]interface{}{
               "owner": "owner",
               "repo": "repo",
               "comment_id": float64(12345),
               "body": "Thanks for the review!",
           },
           expectError: false,
       },
       {
           name: "comment not found",
           mockedClient: mock.NewMockedHTTPClient(
               mock.WithRequestMatchHandler(
                   mock.PostReposPullsCommentsByOwnerByRepoByCommentNumber,
                   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
                       w.WriteHeader(http.StatusNotFound)
                       _, _ = w.Write([]byte(`{"message": "Not Found"}`))
                   }),
               ),
           ),
           requestArgs: map[string]interface{}{
               "owner": "owner",
               "repo": "repo",
               "comment_id": float64(99999),
               "body": "Reply",
           },
           expectError: true,
           expectedErrMsg: "failed to create reply",
       },
   }
   
   for _, tc := range tests {
       t.Run(tc.name, func(t *testing.T) {
           // ... test implementation
       }
   }
   ```

3. **Test Helpers** (`pullrequests_test.go`):
   - `stubGetClientFn(client)`: Returns a GetClientFn that returns the mock client
   - `createMCPRequest(args)`: Creates mcp.CallToolRequest from argument map
   - `getTextResult(t, result)`: Extracts text content from successful result
   - `getErrorResult(t, result)`: Extracts error content from error result

4. **Toolsnap Files**: Tests generate/validate JSON schema snapshots in `pkg/github/__toolsnaps__/*.snap`
   - Run `UPDATE_TOOLSNAPS=true go test ./...` to update snapshots after schema changes
   - Must commit `.snap` files with code changes

**Mock Library**: Uses `github.com/migueleliasweb/go-github-mock` for REST API mocking

### Component 8: GitHub API Integration Points

**Review Comment ID Source** (`pullrequests.go:252-278`):
- Tool: `pull_request_read` with method `get_review_comments`
- Returns: `[]*github.PullRequestComment` objects
- Comment ID field: `comment.GetID()` returns `int64`
- Comment structure includes: ID, Body, Path, Position, User, HTMLURL

**API Endpoint**: `POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies`
- Expected response: `*github.PullRequestComment` object with 201 status
- Error responses: 404 (not found), 403 (forbidden), 422 (invalid)

**go-github Type**: `github.PullRequestComment` (`pullrequests.go:267-278`)
- Has `GetID()` method returning int64
- Has `GetHTMLURL()` method returning string
- Used for both original comments and replies

### Component 9: Documentation Generation

**Location**: `cmd/github-mcp-server/generate_docs.go`

**Process**:
1. Tools are introspected via MCP server
2. README.md sections are auto-generated
3. Run `script/generate-docs` after adding new tools
4. Commit updated README.md with code changes

**CI Validation**: `docs-check.yml` workflow verifies README.md is up-to-date

## Code References

- `pkg/github/pullrequests.go:314-444` - CreatePullRequest tool implementation (primary reference pattern)
- `pkg/github/pullrequests.go:252-278` - GetPullRequestReviewComments (comment listing)
- `pkg/github/pullrequests.go:1363-1552` - AddCommentToPendingReview (GraphQL review comment tool)
- `pkg/github/server.go:69-116` - Parameter validation helpers (RequiredParam, RequiredBigInt)
- `pkg/errors/error.go:111-119` - NewGitHubAPIErrorResponse error wrapper
- `pkg/github/minimal_types.go:112-116` - MinimalResponse type definition
- `pkg/github/tools.go:243-262` - Pull requests toolset registration
- `pkg/toolsets/toolsets.go:140-149` - AddWriteTools method
- `pkg/github/pullrequests_test.go:20-143` - Test_GetPullRequest (test pattern example)
- `internal/toolsnaps/toolsnaps.go` - Toolsnap validation infrastructure

## Architecture Documentation

**Tool Implementation Flow**:
1. Tool function created in `pkg/github/pullrequests.go`
2. Tool registered in `pkg/github/tools.go` pullRequests toolset
3. Tests added in `pkg/github/pullrequests_test.go`
4. Toolsnap generated via `UPDATE_TOOLSNAPS=true go test ./...`
5. Documentation updated via `script/generate-docs`

**Data Flow for Reply Operation**:
1. User calls `pull_request_read` with method `get_review_comments` → receives comment list with IDs
2. User identifies target comment and extracts `comment.ID` (int64)
3. User calls `reply_to_review_comment` with owner, repo, comment_id, body
4. Tool validates parameters → acquires REST client → calls GitHub API
5. GitHub creates reply comment → returns PullRequestComment object
6. Tool returns MinimalResponse with reply ID and URL

**Error Handling Chain**:
1. Parameter validation errors → immediate return with `mcp.NewToolResultError`
2. Client acquisition errors → return with `fmt.Errorf` wrapping
3. API call errors → wrap with `ghErrors.NewGitHubAPIErrorResponse`
4. Non-success status codes → read body and return `mcp.NewToolResultError`

**Testing Strategy**:
1. Toolsnap test validates tool schema matches snapshot
2. Table-driven tests cover success and error scenarios
3. Mock HTTP client simulates GitHub API responses
4. Tests verify both response structure and error messages

## Implementation Checklist

Based on the research, the implementation requires:

- [ ] Add `ReplyToReviewComment` function to `pkg/github/pullrequests.go`
- [ ] Define tool with required parameters: owner, repo, comment_id, body
- [ ] Use `RequiredBigInt` for comment_id parameter (int64 type)
- [ ] Implement handler with parameter validation at the top
- [ ] Call `client.PullRequests.CreateComment` (or similar) with reply parameters
- [ ] Handle errors with `ghErrors.NewGitHubAPIErrorResponse`
- [ ] Return `MinimalResponse` with reply ID and URL on success
- [ ] Register tool in pull_requests toolset in `pkg/github/tools.go`
- [ ] Add unit tests in `pkg/github/pullrequests_test.go` with toolsnap validation
- [ ] Generate toolsnap with `UPDATE_TOOLSNAPS=true go test ./...`
- [ ] Update documentation with `script/generate-docs`
- [ ] Run `script/lint` and `script/test` before committing
- [ ] Update e2e tests in `e2e/e2e_test.go` if applicable

## Open Questions

1. **go-github Method Name**: The spec research mentions `CreateReply`, but go-github v79 may use `CreateComment` with an `InReplyTo` field or similar parameter. The exact method signature needs verification in the go-github library source or documentation during implementation.

2. **Expected Status Code**: Need to verify if GitHub API returns 201 (Created) or 200 (OK) for successful reply creation. Most create operations return 201, but this should be confirmed.

3. **Comment ID Type**: Confirmed as int64 in go-github types, but need to verify the exact parameter name and type for the reply API call.

