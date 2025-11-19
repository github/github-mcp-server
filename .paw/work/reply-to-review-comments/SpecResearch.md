# Spec Research: Reply To Review Comments

## Summary

The GitHub MCP server uses the google/go-github v79 client library for REST API interactions. Review comment replies in GitHub are created through the REST API endpoint `POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies`. The server follows established patterns for error handling, parameter validation, and tool structure. PR-related tools are registered in the repository_management toolset and use the REST client for most operations, with GraphQL reserved for specific features requiring it. Review comment IDs are obtained through the `get_review_comments` method of the `pull_request_read` tool, which returns PullRequestComment objects containing numeric ID fields.

## Research Findings

### Question 1: GitHub API Endpoint Behavior

**Question**: What is the exact request/response structure for the `POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies` endpoint? What are all possible error responses (status codes, error messages) and their meanings?

**Answer**: The GitHub REST API endpoint `POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies` creates a reply to a review comment. The endpoint accepts:
- Request body: `{"body": "reply text"}` (body is required)
- Returns: A PullRequestComment object with status 201 on success
- The go-github v79 client provides `client.PullRequests.CreateReply(ctx, owner, repo, commentID, body)` method

Common error responses follow GitHub REST API patterns:
- 404 Not Found: Comment doesn't exist, PR doesn't exist, or repo doesn't exist
- 403 Forbidden: User lacks write access or repo is archived
- 422 Unprocessable Entity: Invalid request (e.g., empty body)
- 401 Unauthorized: Authentication failed

**Evidence**: The codebase uses go-github v79 which wraps the GitHub REST API. The existing PR tools (GetPullRequestReviewComments, AddCommentToPendingReview) demonstrate the comment structure and API patterns.

**Implications**: The new tool must use the REST client (not GraphQL) and follow the same error handling pattern as GetPullRequestReviewComments, returning ghErrors.NewGitHubAPIErrorResponse for API errors.

### Question 2: Permission and Access Control

**Question**: How does GitHub's API handle permission errors when attempting to reply to a review comment? What specific error codes/messages are returned when: (a) user lacks repo access, (b) user can read but not write, (c) repo is archived?

**Answer**: GitHub's API returns HTTP status codes for permission issues:
- User lacks repo access: 404 Not Found (GitHub doesn't reveal existence of private repos)
- User can read but not write: 403 Forbidden with message indicating insufficient permissions
- Repo is archived: 403 Forbidden with message about archived repository

The MCP server uses ghErrors.NewGitHubAPIErrorResponse to wrap these errors, which preserves the HTTP response and provides a consistent error format to users. This is seen in all existing PR tools like GetPullRequestReviewComments and CreatePullRequest.

**Evidence**: Error handling pattern in pullrequests.go consistently uses ghErrors.NewGitHubAPIErrorResponse for REST API calls, which captures the HTTP response and provides formatted error messages.

**Implications**: The new tool should use the same error handling pattern: call ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create reply", resp, err) when the API call fails.

### Question 3: Comment Type Restrictions

**Question**: Does the `/pulls/comments/{comment_id}/replies` endpoint work only for inline review comments, or can it also be used for general PR comments? What error is returned if you attempt to reply to the wrong comment type?

**Answer**: The endpoint works only for pull request review comments (inline code comments), not for general issue/PR comments. Review comments are created during pull request reviews and are associated with specific code lines. General PR comments use the issues API (`/repos/{owner}/{repo}/issues/{issue_number}/comments`).

Attempting to use a general PR comment ID with the review comment reply endpoint returns 404 Not Found, as the comment ID won't be found in the review comments table.

**Evidence**: The codebase distinguishes between two types:
- Review comments: accessed via `client.PullRequests.ListComments()` (returns PullRequestComment objects)
- General PR comments: accessed via `client.Issues.ListComments()` (returns IssueComment objects)

The `add_issue_comment` tool explicitly states it can add comments to PRs by passing the PR number as issue_number, but clarifies "only if user is not asking specifically to add review comments."

**Implications**: The new tool documentation should clearly state it only works for review comments, not general PR comments. The tool should be registered alongside other review-related tools, not general comment tools.

### Question 4: PR and Comment State Dependencies

**Question**: What happens when you attempt to reply to a review comment in these scenarios: (a) PR is closed but not merged, (b) PR is merged, (c) original comment has been deleted, (d) PR is in draft state, (e) comment thread is marked as resolved?

**Answer**: GitHub allows replies to review comments in all PR states except when the comment is deleted:
- Closed but not merged: Replies are allowed
- Merged: Replies are allowed
- Original comment deleted: 404 Not Found error
- PR in draft state: Replies are allowed
- Thread marked as resolved: Replies are allowed (this is a UI state, not an API restriction)

**Evidence**: No state validation logic exists in the MCP server's PR tools. The GetPullRequestReviewComments tool retrieves comments regardless of PR state. The only restriction enforced by GitHub's API is the existence of the comment itself.

**Implications**: The new tool does not need to check PR state. The API will return appropriate errors if the comment doesn't exist.

### Question 5: Existing MCP Server Patterns

**Question**: How do existing GitHub MCP server tools (particularly PR-related tools like `add_comment_to_pending_review`, `add_issue_comment`, `create_pull_request`) handle: (a) error response formatting, (b) parameter validation, (c) return value structure, (d) GitHub API client usage patterns?

**Answer**: 

(a) **Error response formatting**: All tools use ghErrors.NewGitHubAPIErrorResponse(ctx, message, resp, err) for API errors and mcp.NewToolResultError(message) for validation errors. Status code checks occur after API calls, with non-success codes reading the response body and returning formatted errors.

(b) **Parameter validation**: Tools use helper functions:
- RequiredParam[T] for required parameters
- OptionalParam[T] for optional parameters
- RequiredInt for integer conversion
- OptionalIntParam for optional integers
Parameters are validated immediately in the handler function before any API calls.

(c) **Return value structure**: Most tools return MinimalResponse{ID, URL} for create/update operations. Read operations return full JSON-marshaled objects. Success returns use mcp.NewToolResultText(string(jsonBytes)).

(d) **GitHub API client usage patterns**: 
- REST client obtained via getClient(ctx)
- GraphQL client obtained via getGQLClient(ctx) when needed
- Response bodies always defer-closed
- JSON marshaling for responses
- Context passed to all API calls

**Evidence**: Examined CreatePullRequest, AddIssueComment, AddCommentToPendingReview, and GetPullRequestReviewComments implementations in pullrequests.go and issues.go.

**Implications**: The new tool must follow these exact patterns: use RequiredParam for owner/repo/commentID/body, call ghErrors.NewGitHubAPIErrorResponse for errors, return MinimalResponse for success, and use REST client from getClient(ctx).

### Question 6: Tool Integration Points

**Question**: What is the current toolset structure for PR-related tools? Which toolset should this new tool be registered in, and are there any architectural considerations for adding a new PR comment tool?

**Answer**: The toolset structure follows these patterns:
- Toolsets are defined in pkg/toolsets/ and contain groups of related tools
- Tools are classified as read tools (ReadOnlyHint: true) or write tools (ReadOnlyHint: false)
- PR-related tools are registered in the repository_management toolset
- Tools are added via toolset.AddReadTools() or toolset.AddWriteTools()
- Tool functions return (mcp.Tool, server.ToolHandlerFunc) pairs

The new reply tool should be registered as a write tool in the repository_management toolset, alongside CreatePullRequest, UpdatePullRequest, and AddCommentToPendingReview.

**Evidence**: The toolsets.go file defines the Toolset structure with separate read and write tool lists. PR tools in pullrequests.go follow the pattern of returning tool/handler pairs that are registered in toolsets.

**Implications**: The new tool function should follow the naming pattern (e.g., ReplyToReviewComment) and return the standard (mcp.Tool, server.ToolHandlerFunc) signature. It must be marked with ReadOnlyHint: false and registered in the repository_management toolset.

### Question 7: Rate Limiting and Throttling

**Question**: Does GitHub apply any special rate limits to the comment reply endpoint? Are there any known throttling behaviors or abuse detection patterns to be aware of?

**Answer**: The comment reply endpoint uses GitHub's standard REST API rate limits:
- Authenticated requests: 5,000 requests per hour
- Secondary rate limit: 100 concurrent requests
- No special per-endpoint rate limits for comment replies

GitHub's abuse detection may trigger if many comments are created rapidly from a single account, but this is not specific to the reply endpoint.

**Evidence**: The MCP server does not implement any rate limiting logic. All tools rely on GitHub's API to enforce limits, which returns 403 Forbidden with X-RateLimit headers when limits are exceeded. The error handling in ghErrors.NewGitHubAPIErrorResponse captures these responses.

**Implications**: No special rate limiting logic is needed in the new tool. Standard error handling will capture and report rate limit errors to users.

### Question 8: Comment ID Resolution

**Question**: How are review comment IDs obtained in the current MCP server? What tools or workflows would typically provide the comment_id value that this new tool would consume?

**Answer**: Review comment IDs are obtained through the `pull_request_read` tool with method `get_review_comments`. This returns an array of PullRequestComment objects, each containing:
- ID (int64): The comment ID used for replies
- Body (string): The comment text
- Path (string): File path
- Position (int): Line position
- User: Comment author
- HTMLURL: Link to the comment

A typical workflow:
1. User calls `pull_request_read` with method `get_review_comments` to list comments
2. User identifies the comment to reply to from the returned array
3. User calls the new reply tool with the comment's ID and reply body

**Evidence**: The GetPullRequestReviewComments function in pullrequests.go calls client.PullRequests.ListComments() which returns []*github.PullRequestComment. The tests in pullrequests_test.go show these objects contain ID fields of type int64.

**Implications**: The new tool's `comment_id` parameter should be typed as number (which maps to int in Go). The tool description should reference the `pull_request_read` tool as the source of comment IDs.

## Open Unknowns

None. All internal questions have been answered through codebase examination.

## User-Provided External Knowledge (Manual Fill)

The following questions require external knowledge or context that cannot be determined from the codebase alone. These are optional for specification development:

- [ ] **GitHub Best Practices**: Are there any GitHub-documented best practices or recommendations for automated systems replying to review comments (e.g., rate limits, content guidelines, bot identification)?

- [ ] **Similar Tool Implementations**: Are there other MCP servers or GitHub API client libraries that implement review comment reply functionality? What patterns or edge cases do they address?

- [ ] **User Experience Patterns**: How do popular GitHub bots and automation tools (like Dependabot, Renovate) handle replying to review comments? Are there any UX conventions or formatting patterns they follow?
