---
agent: 'PAW-01B Spec Researcher'
---
# Spec Research Prompt: Reply To Review Comments

Perform research to answer the following questions.

Target Branch: feature/reply-to-review-comments
Issue URL: https://github.com/github/github-mcp-server/issues/1323
Additional Inputs: none

## Questions

1. **GitHub API Endpoint Behavior**: What is the exact request/response structure for the `POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/replies` endpoint? What are all possible error responses (status codes, error messages) and their meanings?

2. **Permission and Access Control**: How does GitHub's API handle permission errors when attempting to reply to a review comment? What specific error codes/messages are returned when: (a) user lacks repo access, (b) user can read but not write, (c) repo is archived?

3. **Comment Type Restrictions**: Does the `/pulls/comments/{comment_id}/replies` endpoint work only for inline review comments, or can it also be used for general PR comments? What error is returned if you attempt to reply to the wrong comment type?

4. **PR and Comment State Dependencies**: What happens when you attempt to reply to a review comment in these scenarios: (a) PR is closed but not merged, (b) PR is merged, (c) original comment has been deleted, (d) PR is in draft state, (e) comment thread is marked as resolved?

5. **Existing MCP Server Patterns**: How do existing GitHub MCP server tools (particularly PR-related tools like `add_comment_to_pending_review`, `add_issue_comment`, `create_pull_request`) handle: (a) error response formatting, (b) parameter validation, (c) return value structure, (d) GitHub API client usage patterns?

6. **Tool Integration Points**: What is the current toolset structure for PR-related tools? Which toolset should this new tool be registered in, and are there any architectural considerations for adding a new PR comment tool?

7. **Rate Limiting and Throttling**: Does GitHub apply any special rate limits to the comment reply endpoint? Are there any known throttling behaviors or abuse detection patterns to be aware of?

8. **Comment ID Resolution**: How are review comment IDs obtained in the current MCP server? What tools or workflows would typically provide the comment_id value that this new tool would consume?

### Optional External / Context

1. **GitHub Best Practices**: Are there any GitHub-documented best practices or recommendations for automated systems replying to review comments (e.g., rate limits, content guidelines, bot identification)?

2. **Similar Tool Implementations**: Are there other MCP servers or GitHub API client libraries that implement review comment reply functionality? What patterns or edge cases do they address?

3. **User Experience Patterns**: How do popular GitHub bots and automation tools (like Dependabot, Renovate) handle replying to review comments? Are there any UX conventions or formatting patterns they follow?
