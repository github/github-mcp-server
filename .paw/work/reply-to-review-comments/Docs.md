# Reply To Review Comments - MCP Tool

## Overview

The `reply_to_review_comment` tool enables AI agents to participate in threaded code review discussions by replying directly to individual pull request review comments. This maintains the conversation context at specific code locations, mirroring how human developers respond to inline feedback.

**Problem Solved**: Previously, AI agents could only post general PR comments, which separated responses from the code they referenced. This made review conversations fragmented and difficult to navigate. Agents had to resort to listing all responses in a single comment, losing the threaded context that keeps discussions anchored to specific lines of code.

**Solution**: The tool provides direct access to GitHub's review comment reply API, allowing agents to respond within existing comment threads. Each reply appears in GitHub's UI as a threaded response at the relevant code location, preserving the familiar review experience while enabling agents to participate as full collaborators in the code review process.

## Architecture and Design

### High-Level Architecture

The tool follows the established MCP tool pattern used throughout the GitHub MCP Server:

```
MCP Client (AI Agent)
    ↓ (tool call with parameters)
GitHub MCP Server
    ↓ (ReplyToReviewComment handler)
Parameter Validation Layer
    ↓ (validated parameters)
GitHub REST Client (go-github v79)
    ↓ (CreateCommentInReplyTo API call)
GitHub API
    ↓ (threaded reply created)
Pull Request Review Thread
```

**Key Components**:
- **Tool Handler**: `ReplyToReviewComment` function in `pkg/github/pullrequests.go`
- **API Integration**: Uses `client.PullRequests.CreateCommentInReplyTo()` from go-github v79
- **Parameter Validation**: Leverages existing validation helpers (`RequiredParam`, `RequiredInt`, `RequiredBigInt`)
- **Error Handling**: Uses `ghErrors.NewGitHubAPIErrorResponse` for consistent error formatting
- **Response Format**: Returns `MinimalResponse` with reply ID and URL

### Design Decisions

**1. REST API over GraphQL**

The implementation uses GitHub's REST API endpoint rather than GraphQL for several reasons:
- The go-github v79 client provides a dedicated `CreateCommentInReplyTo` method with clean error handling
- REST endpoint explicitly requires the pull request number, making the API contract clear
- Consistent with other PR modification tools in the codebase
- Simpler error handling for API failures (404, 403, 422)

**2. Required Pull Request Number**

Both the pull request number and comment ID are required parameters, even though the comment ID technically uniquely identifies the comment. This design choice reflects:
- GitHub's API design: The endpoint path is `/repos/{owner}/{repo}/pulls/{pull_number}/comments`
- Better user experience: Agents already have the PR number from their review workflow context
- Validation opportunity: Ensures users are aware of which PR the comment belongs to
- Consistency: Matches the pattern of other PR-scoped tools

**3. int64 Comment ID Handling**

Comment IDs are validated using `RequiredBigInt` (not `RequiredInt`) because:
- GitHub uses int64 for comment IDs in the go-github client
- JavaScript's number type can represent int64 values safely in this range
- Using `RequiredBigInt` ensures proper type conversion without overflow
- Consistent with other tools that handle GitHub resource IDs

**4. Single-Reply Operations**

The tool processes one reply at a time rather than supporting batch operations:
- Simpler implementation and clearer error messages
- Agents can orchestrate batch operations by calling the tool multiple times
- GitHub's rate limits (5000 requests/hour) are sufficient for typical batch reply scenarios
- Allows granular error handling for each reply

**5. Minimal Response Format**

The tool returns only the reply ID and URL (not the full comment object):
- Consistent with other create/update tools in the codebase
- Sufficient for most agent workflows (verification, logging, linking)
- Reduces response payload size
- Full comment details can be retrieved via `pull_request_read` if needed

### Integration Points

**Toolset Registration**: The tool is registered in the `repository_management` toolset as a write tool, alongside other PR modification operations. It's positioned after `AddCommentToPendingReview` to group review-related write tools logically.

**Authentication**: The tool inherits the MCP server's GitHub authentication mechanism. Users must configure a personal access token with repository write permissions before using the tool.

**Companion Tools**: The tool works in coordination with:
- `pull_request_read` (with `get_review_comments` method): Retrieves review comment IDs needed for replies
- `add_comment_to_pending_review`: Creates new review comments (not replies)
- `pull_request_review_write`: Manages review workflow (submit, approve, request changes)

## User Guide

### Prerequisites

- GitHub MCP Server configured with a personal access token
- Token must have write access to the target repository
- Access to a pull request with existing review comments
- Review comment IDs obtained from `pull_request_read` tool

### Basic Usage

**Step 1: Retrieve Review Comments**

Use `pull_request_read` to get review comment IDs:

```json
{
  "owner": "myorg",
  "repo": "myproject",
  "pullNumber": 42,
  "method": "get_review_comments"
}
```

Response includes comment IDs:
```json
{
  "comments": [
    {
      "id": 12345,
      "body": "Consider using a more descriptive variable name here.",
      "path": "src/main.go",
      "line": 15
    }
  ]
}
```

**Step 2: Reply to a Comment**

Use `reply_to_review_comment` to respond:

```json
{
  "owner": "myorg",
  "repo": "myproject",
  "pull_number": 42,
  "comment_id": 12345,
  "body": "Good point! I've renamed it to `userSessionManager` in commit abc123."
}
```

Response confirms creation:
```json
{
  "id": "67890",
  "url": "https://github.com/myorg/myproject/pull/42#discussion_r67890"
}
```

**Step 3: Verify in GitHub UI**

Navigate to the PR in GitHub. The reply appears as a threaded response under the original review comment at line 15 of `src/main.go`. The reviewer receives a notification.

### Advanced Usage

**Batch Reply Workflow**

Agents can systematically address multiple review comments:

```python
# Pseudocode for agent workflow
review_comments = call_tool("pull_request_read", {
  "method": "get_review_comments",
  "owner": "myorg",
  "repo": "myproject", 
  "pullNumber": 42
})

for comment in review_comments["comments"]:
  response = generate_reply_for_comment(comment)
  call_tool("reply_to_review_comment", {
    "owner": "myorg",
    "repo": "myproject",
    "pull_number": 42,
    "comment_id": comment["id"],
    "body": response
  })
```

**Markdown Formatting**

Reply bodies support GitHub-flavored Markdown:

```json
{
  "body": "Fixed in commit abc123.\n\n```go\nuserSessionManager := NewSessionManager()\n```\n\nThe new name better reflects its purpose. cc @reviewer"
}
```

Supported formatting:
- Code blocks (inline and fenced)
- User mentions (`@username`)
- Issue/PR references (`#123`)
- Emoji (`:+1:`)
- Links, lists, headers, etc.

**Deferring Feedback**

Reply to indicate work will be addressed later:

```json
{
  "body": "Agreed! This refactoring is tracked in issue #456. I'll address it in a follow-up PR to keep this change focused."
}
```

### Configuration

No additional configuration is required beyond the GitHub MCP Server's standard authentication setup. The tool uses the server's configured personal access token.

## Technical Reference

### Tool Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner (username or organization) |
| `repo` | string | Yes | Repository name |
| `pull_number` | number | Yes | Pull request number containing the review comment |
| `comment_id` | number | Yes | Review comment ID (int64) from `pull_request_read` |
| `body` | string | Yes | Reply text supporting GitHub-flavored Markdown |

### Response Format

Successful calls return a `MinimalResponse` object:

```json
{
  "id": "67890",
  "url": "https://github.com/owner/repo/pull/42#discussion_r67890"
}
```

- `id`: The created reply's unique identifier (string representation of int64)
- `url`: Direct URL to the reply in GitHub's UI

### Error Handling

The tool returns descriptive error messages for common failure scenarios:

**404 Not Found** - Comment doesn't exist:
```
failed to create reply to review comment: Not Found
```
Causes: Comment was deleted, wrong comment ID, or general PR comment (not review comment)

**403 Forbidden** - Permission denied:
```
failed to create reply to review comment: Forbidden
```
Causes: No write access to repository, archived repository, or token lacks permissions

**422 Unprocessable Entity** - Validation failure:
```
failed to create reply to review comment: Validation failed
```
Causes: Empty reply body, invalid Markdown, or API validation constraints

**Parameter Validation Errors**:
```
missing required parameter: owner
missing required parameter: repo
missing required parameter: pull_number
missing required parameter: comment_id
missing required parameter: body
comment_id must be a number
```

### GitHub API Details

**Endpoint**: `POST /repos/{owner}/{repo}/pulls/{pull_number}/comments`

**Method**: `client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, number, body, commentID)`

**Success Status**: `201 Created`

**Rate Limits**: Standard GitHub API rate limits apply (5000 requests/hour for authenticated users)

**Notifications**: GitHub automatically sends notifications to:
- The original comment author
- Users subscribed to the PR
- Users mentioned in the reply body

## Usage Examples

### Example 1: Acknowledging a Fix

**Scenario**: Developer fixed an issue mentioned in review

```json
{
  "owner": "company",
  "repo": "backend-api",
  "pull_number": 158,
  "comment_id": 987654,
  "body": "Fixed in commit f3a2b1c. The race condition is now handled with proper mutex locking."
}
```

**Result**: Reply appears in thread at the relevant code location, reviewer sees the commit reference and explanation.

### Example 2: Requesting Clarification

**Scenario**: Agent needs more context about review feedback

```json
{
  "owner": "team",
  "repo": "frontend",
  "pull_number": 203,
  "comment_id": 445566,
  "body": "Could you clarify what you mean by 'edge case handling'? Are you referring to empty arrays or null values?"
}
```

**Result**: Reviewer receives notification and can provide additional context in the same thread.

### Example 3: Deferring Work

**Scenario**: Addressing feedback in a follow-up PR

```json
{
  "owner": "org",
  "repo": "platform",
  "pull_number": 91,
  "comment_id": 778899,
  "body": "This refactoring is tracked in issue #445. I'm keeping this PR focused on the bug fix, but will address the broader refactor in the next iteration."
}
```

**Result**: Sets expectations with reviewer while keeping conversation context intact.

## Edge Cases and Limitations

### Edge Cases

**Deleted Comments**: If a review comment is deleted after its ID is retrieved, the API returns 404 Not Found. Agents should handle this gracefully by logging the error and continuing with remaining comments.

**Closed/Merged PRs**: Replies are permitted on closed and merged pull requests. GitHub does not restrict commenting on completed PRs, allowing post-merge discussions to continue.

**Draft PRs**: Replies work on draft pull requests, supporting iterative feedback during development.

**Resolved Threads**: GitHub's "resolved" marker is UI-only and does not prevent replies. Agents can reply to resolved threads successfully.

**Archived Repositories**: Attempting to reply to comments in archived repositories returns 403 Forbidden. The error message indicates the repository is archived.

**Empty Body**: Empty or whitespace-only reply bodies are rejected by GitHub's API with 422 Unprocessable Entity.

### Limitations

**Review Comments Only**: The tool only works with review comments (inline code comments). General PR comments (issue comments) are not supported. Attempting to use an issue comment ID results in a 404 error.

**No Edit or Delete**: The tool cannot edit or delete existing replies. Use GitHub's web UI or other tools for those operations.

**No Thread Resolution**: The tool does not mark comment threads as resolved or unresolved. Thread resolution is a separate GitHub API operation.

**Single Reply Operations**: The tool processes one reply at a time. Agents orchestrate batch operations by calling the tool multiple times.

**No Custom Notifications**: The tool uses GitHub's standard notification system. Custom notification preferences or delivery mechanisms are not supported.

**Rate Limits**: Subject to GitHub's standard API rate limits (5000 requests/hour for authenticated users). Agents performing batch replies should implement appropriate throttling if needed.

**Comment Type Distinction**: Users must understand the difference between review comments (inline code comments) and issue comments (general PR comments). The tool's documentation emphasizes obtaining comment IDs from `pull_request_read` with the `get_review_comments` method to avoid confusion.

## Testing Guide

### How to Test This Feature

**Manual Testing Workflow**:

1. **Setup Test Environment**:
   - Fork a test repository or use an existing one where you have write access
   - Create a feature branch with a small code change
   - Open a pull request from the feature branch

2. **Create Review Comments**:
   - Add 2-3 inline review comments on different files/lines
   - Use specific feedback like "Consider renaming this variable" or "Add error handling here"
   - Note the pull request number (e.g., #42)

3. **Start GitHub MCP Server**:
   ```bash
   export GITHUB_PERSONAL_ACCESS_TOKEN="your_pat_here"
   ./github-mcp-server stdio
   ```

4. **Retrieve Review Comments**:
   - Call `pull_request_read` with method `get_review_comments`
   - Provide owner, repo, and pullNumber
   - Note the comment IDs returned (e.g., 12345, 12346, 12347)

5. **Reply to First Comment**:
   - Call `reply_to_review_comment` with:
     - owner, repo, pull_number (from step 2)
     - comment_id (from step 4)
     - body: "Thanks for the feedback! I'll address this in the next commit."
   - Verify response contains `id` and `url` fields
   - Open the `url` in a browser to confirm reply appears

6. **Verify Threading in GitHub UI**:
   - Navigate to the PR in GitHub
   - Locate the original review comment
   - Confirm your reply appears indented underneath as a threaded response
   - Verify the reply includes the exact text you provided

7. **Test Batch Replies**:
   - Reply to the remaining 2 comments with different messages
   - Verify all replies appear in their respective threads
   - Confirm threading is maintained for each reply

8. **Test Error Handling**:
   - Attempt to reply with a non-existent comment_id (e.g., 99999)
   - Verify you receive a 404 error message
   - Attempt to reply with an empty body string
   - Verify you receive a validation error

9. **Test Markdown Formatting**:
   - Reply to a comment with formatted text:
     ```
     Fixed! Here's the updated code:
     
     ```go
     if err != nil {
       return fmt.Errorf("failed: %w", err)
     }
     ```
     
     cc @reviewer for verification.
     ```
   - Verify Markdown renders correctly in GitHub UI

10. **Verify Notifications**:
    - Check that the original comment author receives an email/notification
    - Verify notification links directly to the reply

**Expected Results**:
- All replies appear as threaded responses at correct code locations
- Reply IDs and URLs are returned successfully
- Error messages are descriptive and actionable
- Markdown formatting renders correctly
- Notifications are sent to appropriate users
- No replies appear as general PR comments (all are threaded)

**Testing for Bug Fix**: N/A - This is a new feature, not a bug fix.

## Migration and Compatibility

### For New Users

No migration is required. Users can start using the `reply_to_review_comment` tool immediately after:
1. Configuring the GitHub MCP Server with a personal access token
2. Ensuring the token has repository write permissions
3. Familiarizing themselves with the `pull_request_read` tool for obtaining comment IDs

### For Existing Users

**No Breaking Changes**: This tool is a new addition and does not affect existing tools or APIs.

**Adoption Path**:
1. Update to the version that includes `reply_to_review_comment`
2. Existing workflows using `add_issue_comment` for general PR comments continue to work
3. Gradually migrate review-related responses to use `reply_to_review_comment` for better threading
4. No code changes required in existing agent implementations

**Workflow Enhancement**:

Before (using general PR comments):
```python
# Old approach: List all responses in one comment
responses = []
for comment in review_comments:
  responses.append(f"Re: {comment['path']}:{comment['line']} - {generate_response(comment)}")

call_tool("add_issue_comment", {
  "body": "\n\n".join(responses)
})
```

After (using threaded replies):
```python
# New approach: Reply directly in threads
for comment in review_comments:
  call_tool("reply_to_review_comment", {
    "pull_number": pr_number,
    "comment_id": comment["id"],
    "body": generate_response(comment)
  })
```

**Compatibility Notes**:
- The tool requires go-github v79 or later (includes `CreateCommentInReplyTo` method)
- No database migrations or schema changes
- No configuration file updates required
- Tool is backward compatible with existing MCP client implementations

### Deprecation

No existing functionality is deprecated. Both approaches remain valid:
- Use `add_issue_comment` for general PR-level comments (announcements, summaries)
- Use `reply_to_review_comment` for responding to specific review feedback at code locations

## References

- **Implementation Plan**: `.paw/work/reply-to-review-comments/ImplementationPlan.md`
- **Specification**: `.paw/work/reply-to-review-comments/Spec.md`
- **Research**: `.paw/work/reply-to-review-comments/SpecResearch.md`, `.paw/work/reply-to-review-comments/CodeResearch.md`
- **GitHub API Documentation**: [Create a reply for a review comment](https://docs.github.com/en/rest/pulls/comments?apiVersion=2022-11-28#create-a-reply-for-a-review-comment)
- **go-github Library**: [CreateCommentInReplyTo Method](https://pkg.go.dev/github.com/google/go-github/v79/github#PullRequestsService.CreateCommentInReplyTo)
- **Original Issue**: [github/github-mcp-server#1323](https://github.com/github/github-mcp-server/issues/1323)
- **Phase PRs**:
  - Phase 1: Core Tool Implementation - PR #1
  - Phase 2: Toolset Integration - PR #2
  - Phase 3: Testing - PR #4
  - Phase 4: Documentation & Validation - (this phase)
