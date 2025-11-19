# Feature Specification: Reply To Review Comments

**Branch**: feature/reply-to-review-comments  |  **Created**: 2025-11-19  |  **Status**: Draft
**Input Brief**: Enable AI agents to reply directly to individual pull request review comment threads, maintaining conversation context at the code level

## Overview

When a developer receives inline code review feedback on a pull request, GitHub's natural workflow involves replying directly to specific comment threads to acknowledge changes, request clarification, or provide context. Currently, AI agents using the GitHub MCP Server cannot participate in these threaded conversations. An agent reviewing comments from Copilot or human reviewers must resort to posting a single general PR comment that lists all responses, losing the threaded context and making it difficult for reviewers to track which specific code lines are being discussed. This workaround creates a fragmented conversation history where responses are separated from the code they reference.

The reply-to-review-comments feature enables AI agents to respond directly within review comment threads, just as human developers do. When an agent addresses feedback—whether by making code changes, asking clarifying questions, or explaining design decisions—it can now reply in the appropriate thread, maintaining the conversation context at the specific line of code. Reviewers receive notifications for each reply and can continue the discussion without needing to correlate external comments back to their original feedback locations.

This feature integrates seamlessly into existing code review workflows. An agent can retrieve review comments using existing tools, identify which comments require responses, and reply to each thread individually. The threaded replies appear in the GitHub UI exactly as human-authored replies do, preserving the familiar review experience while enabling AI agents to participate as full collaborators in the code review process. This maintains GitHub's conversation model where discussions about specific code patterns remain anchored to their source locations, making review history more navigable and actionable.

## Objectives

- Enable AI agents to reply directly to individual pull request review comment threads, maintaining conversation context at specific code locations
- Provide a tool that integrates with GitHub's native review workflow, allowing agents to participate in threaded code review discussions
- Allow agents to acknowledge code changes, request clarifications, or defer work by responding within the appropriate comment thread
- Support batch response workflows where agents iterate through multiple review comments and reply to each thread systematically (Rationale: this enables agents to comprehensively address reviewer feedback in a single operation)
- Maintain compatibility with existing PR comment retrieval tools, allowing comment IDs obtained from review listings to be used for replies

## User Scenarios & Testing

### User Story P1 – Reply to Single Review Comment

Narrative: As an AI agent addressing code review feedback, I need to reply to a specific review comment thread so that my response appears in context at the relevant code location rather than as a disconnected general comment.

Independent Test: Create a reply to an existing review comment and verify it appears as a threaded response in the GitHub UI.

Acceptance Scenarios:
1. Given a pull request with at least one review comment, When the agent replies to that comment with acknowledgment text (providing both PR number and comment ID), Then the reply appears as a child comment in the review thread and the reviewer receives a notification.
2. Given a review comment ID and pull request number obtained from the pull_request_read tool, When the agent provides both IDs with reply body text, Then the reply is successfully posted to the correct thread.
3. Given a review comment on a specific code line, When the agent replies with "Fixed in commit abc123" (providing both PR number and comment ID), Then the reply is visible in the GitHub UI as part of that code line's conversation thread.

### User Story P2 – Error Handling for Invalid Comments

Narrative: As an AI agent attempting to reply to review feedback, I need clear error messages when the target comment doesn't exist or I lack permissions so that I can provide useful feedback to the user about why the reply failed.

Independent Test: Attempt to reply to a non-existent comment ID and verify a descriptive error is returned.

Acceptance Scenarios:
1. Given a comment ID that does not exist, When the agent attempts to create a reply, Then a 404 error is returned with a message indicating the comment was not found.
2. Given a repository where the user lacks write access, When the agent attempts to reply to a review comment, Then a 403 error is returned with a message indicating insufficient permissions.
3. Given a general PR comment ID (not a review comment), When the agent attempts to use it with the reply tool, Then an error is returned indicating the comment type is incompatible.

### User Story P3 – Batch Reply Workflow

Narrative: As an AI agent managing code review responses, I need to iterate through multiple review comments and reply to each one systematically so that I can comprehensively address all reviewer feedback in a single coordinated operation.

Independent Test: Retrieve multiple review comments, reply to each one sequentially, and verify all replies appear in their respective threads.

Acceptance Scenarios:
1. Given a pull request with five review comments, When the agent retrieves the comments (including PR number and comment IDs) and posts replies to all five, Then each reply appears in its corresponding thread and all five reviewers receive notifications.
2. Given a mix of addressed and deferred feedback, When the agent replies with commit references for fixed items and "tracking in issue #N" for deferred items (providing both PR number and comment ID for each), Then each reply contains the appropriate context-specific message.
3. Given a review comment list obtained from pull_request_read, When the agent iterates through the list and calls reply_to_review_comment for each ID (along with the PR number), Then all calls succeed and maintain the correct ID-to-thread mapping.

### Edge Cases

- **Deleted Comments**: When a reply is attempted for a review comment that has been deleted, the API returns 404 Not Found, indicating the target comment no longer exists.
- **Closed or Merged PRs**: Replies to review comments are permitted on closed and merged pull requests, allowing post-merge discussions to continue.
- **Draft PRs**: Replies to review comments are allowed on draft pull requests, supporting iterative feedback during development.
- **Resolved Comment Threads**: GitHub's "resolved" state is a UI marker and does not prevent replies via the API; replies to resolved threads are successful.
- **Archived Repositories**: Attempting to reply to comments in archived repositories returns 403 Forbidden with a message about the archived state.
- **Empty Reply Body**: If the reply body is empty or contains only whitespace, the API returns 422 Unprocessable Entity.
- **Markdown Content**: Reply bodies support GitHub-flavored Markdown, including code blocks, mentions, and emoji, consistent with other comment creation tools.

## Requirements

### Functional Requirements

- FR-001: The system shall provide a tool that accepts repository owner, repository name, pull request number, review comment ID, and reply body text as parameters (Stories: P1, P2, P3)
- FR-002: The system shall create a reply to the specified review comment using GitHub's REST API POST endpoint for comment replies (Stories: P1, P3)
- FR-003: The system shall return a minimal response containing the created reply's ID and URL upon successful creation (Stories: P1, P3)
- FR-004: The system shall return a 404 error with descriptive message when the target review comment does not exist (Stories: P2)
- FR-005: The system shall return a 403 error with descriptive message when the authenticated user lacks write access to the repository (Stories: P2)
- FR-006: The system shall return a 403 error with descriptive message when attempting to reply to comments in an archived repository (Stories: P2)
- FR-007: The system shall return a 422 error with descriptive message when the reply body is empty or invalid (Stories: P2)
- FR-008: The system shall accept review comment IDs in the same numeric format returned by the pull_request_read tool's get_review_comments method (Stories: P1, P3)
- FR-009: The system shall support GitHub-flavored Markdown in reply body text, including code blocks, mentions, and emoji (Stories: P1, P3)
- FR-010: The system shall validate all required parameters (owner, repo, pull_number, comment_id, body) before making API calls, returning descriptive errors for missing values (Stories: P2)
- FR-011: The system shall preserve GitHub's standard rate limiting behavior, returning rate limit errors when thresholds are exceeded (Stories: P2)
- FR-012: The system shall allow replies to review comments on pull requests in any state: open, closed, merged, or draft (Stories: P1, P3)

### Key Entities

- **Review Comment**: An inline comment on a pull request associated with a specific file path and line number, created during code review. Distinguished from general PR comments which are not tied to code locations.
- **Review Comment ID**: A numeric identifier (int64) uniquely identifying a review comment within a repository, obtained from review comment listings.
- **Pull Request Number**: The integer number that identifies the pull request containing the review comment. Required because the GitHub API endpoint uses the PR number in the URL path.
- **Reply**: A threaded response to a review comment, appearing as a child comment in the same conversation thread.
- **Repository Owner**: The GitHub username or organization name that owns the repository.
- **Repository Name**: The name of the repository containing the pull request.

### Cross-Cutting / Non-Functional

- **Error Consistency**: All API errors shall be wrapped using the existing error response formatter to provide consistent error messages across all MCP tools.
- **Parameter Validation**: All required parameters shall be validated using the existing parameter validation helpers before API calls are made.
- **Authentication**: The tool shall use the existing GitHub client authentication mechanism, requiring a valid personal access token with appropriate repository permissions.
- **Toolset Integration**: The tool shall be registered as a write tool in the repository_management toolset, alongside other PR modification tools.

## Success Criteria

- SC-001: An agent can retrieve a review comment ID from pull_request_read and successfully create a reply that appears as a threaded response in the GitHub UI (FR-001, FR-002, FR-003, FR-008)
- SC-002: When an agent replies to a review comment, the repository owner and comment author receive GitHub notifications (FR-002)
- SC-003: An agent attempting to reply to a non-existent comment ID receives a clear error message indicating the comment was not found (FR-004)
- SC-004: An agent attempting to reply without write access receives a clear error message indicating insufficient permissions (FR-005)
- SC-005: An agent can reply to review comments on closed, merged, and draft pull requests without errors (FR-012)
- SC-006: An agent can create replies containing Markdown formatting, including code blocks and mentions, and the formatting renders correctly in GitHub UI (FR-009)
- SC-007: When required parameters are missing or invalid, the agent receives validation errors before any API call is made (FR-010)
- SC-008: An agent can iterate through a list of review comment IDs and create replies to multiple threads in sequence, with each reply appearing in the correct thread (FR-001, FR-002, FR-008)
- SC-009: The tool follows the same error handling patterns as existing PR tools, providing consistent error messages across the MCP server (FR-004, FR-005, FR-006, FR-007)
- SC-010: The tool integrates into the repository_management toolset and is discoverable alongside other PR modification tools (FR-001)

## Assumptions

- The go-github v79 client library provides a CreateCommentInReplyTo method that accepts owner, repo, pull request number, body text, and comment ID to create threaded replies.
- The GitHub API requires the pull request number in the URL path (/repos/{owner}/{repo}/pulls/{pull_number}/comments) even though the comment ID theoretically uniquely identifies the comment. This is a GitHub API design constraint.
- Users will obtain both review comment IDs and pull request numbers through the existing pull_request_read tool's get_review_comments method before calling the reply tool.
- GitHub's standard authentication and rate limiting mechanisms are sufficient for this endpoint; no special handling is required.
- The distinction between review comments (inline code comments) and general PR comments (issue comments) is clear to users through tool documentation.
- Users have already configured the MCP server with a GitHub personal access token that has appropriate repository access permissions.
- The minimal response format (ID and URL) used by other create/update tools is sufficient for reply operations; users do not require the full reply object to be returned.
- GitHub's notification system automatically handles notifying relevant users when replies are created; no additional notification logic is needed.
- The tool will inherit the MCP server's existing error handling infrastructure without requiring custom error types or special error formatting logic.

## Scope

In Scope:
- Creating threaded replies to existing pull request review comments
- Parameter validation for owner, repo, comment_id, and body
- Error handling for API failures (404, 403, 422, rate limits)
- Integration with the repository_management toolset
- Support for Markdown formatting in reply bodies
- Compatibility with review comments on PRs in any state (open, closed, merged, draft)

Out of Scope:
- Replying to general PR comments (issue comments) that are not review comments
- Editing or deleting existing replies
- Marking comment threads as resolved or unresolved
- Retrieving or listing replies to review comments (this is handled by pull_request_read)
- Creating new review comments (this is handled by add_comment_to_pending_review and pull_request_review_write)
- Batch reply operations that accept multiple comment IDs in a single tool call (users can orchestrate batch operations by calling the tool multiple times)
- Custom notification preferences or delivery mechanisms
- Rate limiting or throttling beyond GitHub's default API limits

## Dependencies

- go-github v79 client library with CreateReply method support for pull request comments
- GitHub REST API endpoint: POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies
- Existing MCP server authentication mechanism providing a valid GitHub personal access token
- Existing error handling utilities (ghErrors.NewGitHubAPIErrorResponse)
- Existing parameter validation helpers (RequiredParam, OptionalParam, RequiredInt)
- pull_request_read tool as the standard method for obtaining review comment IDs
- repository_management toolset for tool registration

## Risks & Mitigations

- **Risk**: Users may confuse review comment IDs with general PR comment IDs, leading to 404 errors. Mitigation: Clearly document in the tool description that only review comment IDs (obtained from pull_request_read with get_review_comments) are valid; include an error message that explicitly states the comment type distinction when a 404 occurs.
- **Risk**: Users may expect the tool to support batch operations natively, leading to performance concerns when replying to many comments. Mitigation: Document that the tool is designed for single-reply operations and that orchestration tools or agents should handle iteration; GitHub's rate limits (5000 requests/hour) are sufficient for typical batch reply scenarios.
- **Risk**: The go-github v79 library may not expose the CreateReply method or may have a different signature than expected. Mitigation: Research confirms the library provides this method with the expected signature (owner, repo, commentID, body); validate during implementation and adjust parameter handling if needed.
- **Risk**: GitHub's API may return unexpected error codes or messages for edge cases not covered by research. Mitigation: Use the existing error response wrapper (ghErrors.NewGitHubAPIErrorResponse) which preserves all HTTP response details, allowing users to see the full GitHub error message.
- **Risk**: Archived repository detection may not be obvious from error messages, confusing users. Mitigation: The existing error handler captures and returns GitHub's descriptive error messages; test with archived repositories to ensure clarity.

## References

- Issue: https://github.com/github/github-mcp-server/issues/1323
- Research: .paw/work/reply-to-review-comments/SpecResearch.md
- External: 
  - GitHub REST API Documentation: https://docs.github.com/en/rest/pulls/comments?apiVersion=2022-11-28#create-a-reply-for-a-review-comment
  - API Endpoint: POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies

## Glossary

- **Review Comment**: An inline comment on a pull request that references a specific file and line number, created during code review. These are distinct from general PR comments (issue comments).
- **Comment Thread**: A conversation consisting of an original review comment and zero or more replies.
- **Minimal Response**: A JSON object containing only the ID and URL of a newly created resource, used as a standard return format for create/update operations in the MCP server.
- **Repository Management Toolset**: A grouping of MCP tools related to repository and pull request operations, including PR creation, updates, and comment management.
