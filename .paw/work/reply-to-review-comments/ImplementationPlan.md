# Reply To Review Comments Implementation Plan

## Overview

Add a `reply_to_review_comment` MCP tool to enable AI agents to respond directly within pull request review comment threads, maintaining conversation context at specific code locations. The tool will use GitHub's REST API endpoint via the go-github v79 client library's `CreateCommentInReplyTo` method.

## Current State Analysis

The GitHub MCP Server currently provides tools for creating and managing pull requests, including:
- `pull_request_read` with `get_review_comments` method that returns review comment IDs
- `add_comment_to_pending_review` for creating review comments in pending reviews
- `add_issue_comment` for general PR timeline comments

**Missing Capability**: No tool exists to reply to existing review comment threads. Agents must post general PR comments that lose the threaded context at specific code locations.

**Key Constraints**:
- Must use REST client (`client.PullRequests.CreateCommentInReplyTo`) not GraphQL
- Comment IDs are int64 type, requiring `RequiredBigInt` validation
- Tool must follow the established pattern in `pkg/github/pullrequests.go` (CreatePullRequest at lines 314-444)
- GitHub API returns 201 status on successful reply creation
- Must be registered as a write tool in the repository_management toolset

## Desired End State

AI agents can:
1. Retrieve review comment IDs using `pull_request_read` with method `get_review_comments`
2. Call `reply_to_review_comment` with owner, repo, pull_number, comment_id, and body parameters
3. Receive a MinimalResponse with the reply's ID and URL
4. Have replies appear as threaded responses in GitHub's UI, preserving code location context

**Verification**: Create a PR with review comments, use the tool to reply to a comment, and confirm the reply appears in the thread in GitHub's UI with proper notifications sent.

### Key Discoveries:
- go-github v79 provides `CreateCommentInReplyTo(ctx, owner, repo, number, body, commentID)` method returning `(*PullRequestComment, *Response, error)` (`pkg/github/pullrequests.go` patterns)
- Existing tools use `RequiredBigInt` for int64 comment IDs (`pkg/github/server.go:101-116`)
- The pull_requests toolset is defined in `pkg/github/tools.go:243-262` with separate read and write tool sections
- Error handling follows `ghErrors.NewGitHubAPIErrorResponse` pattern (`pkg/errors/error.go:111-119`)
- MinimalResponse format used for write operations (`pkg/github/minimal_types.go:112-116`)

## What We're NOT Doing

- Replying to general PR comments (issue comments) - only review comments are supported
- Editing or deleting existing replies
- Marking comment threads as resolved/unresolved
- Batch reply operations (multiple comment IDs in one call) - agents orchestrate multiple calls
- Creating new review comments (use existing tools)
- Custom notification mechanisms beyond GitHub's default behavior

## Implementation Approach

Follow the established MCP tool pattern used throughout `pkg/github/pullrequests.go`. The implementation will mirror the structure of `CreatePullRequest` (lines 314-444), adapting it for the reply-specific GitHub API method. This ensures consistency with existing tools and leverages proven error handling, parameter validation, and response formatting patterns. The tool will be a thin wrapper around the go-github client method, with robust validation and error handling.

## Phase Summary

1. **Phase 1: Core Tool Implementation** - Implement ReplyToReviewComment function in pullrequests.go with MCP tool definition, parameter validation, REST API integration, and error handling
2. **Phase 2: Toolset Integration** - Register the tool in the pull_requests toolset as a write tool
3. **Phase 3: Testing** - Add unit tests with toolsnap validation and table-driven behavioral tests covering success and error scenarios
4. **Phase 4: Documentation & Validation** - Generate updated README.md and run validation scripts

---

## Phase 1: Core Tool Implementation

### Overview
Create the `ReplyToReviewComment` function in `pkg/github/pullrequests.go` following the established pattern for MCP tools. This phase implements the tool definition, parameter validation, GitHub API integration, and error handling.

### Changes Required:

#### 1. Tool Function Implementation
**File**: `pkg/github/pullrequests.go`

**Changes**:
- Add `ReplyToReviewComment` function after existing PR tools (follow pattern from `CreatePullRequest` at lines 314-444)
- Function signature: `func ReplyToReviewComment(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc)`
- Tool definition with parameters:
  - `owner` (string, required): Repository owner
  - `repo` (string, required): Repository name  
  - `pull_number` (number, required): Pull request number
  - `comment_id` (number, required): Review comment ID from `pull_request_read`
  - `body` (string, required): Reply text supporting GitHub-flavored Markdown
- Tool annotations: `ReadOnlyHint: ToBoolPtr(false)` for write operation
- Handler function implementing:
  - Parameter extraction using `RequiredParam[string]` for owner/repo/body
  - Parameter extraction using `RequiredInt` for pull_number
  - Parameter extraction using `RequiredBigInt` for comment_id (int64 type)
  - Client acquisition via `getClient(ctx)`
  - API call: `comment, resp, err := client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, pullNumber, body, commentID)`
  - Error handling via `ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create reply to review comment", resp, err)`
  - Response body deferred close: `defer func() { _ = resp.Body.Close() }()`
  - Status check for `http.StatusCreated` (201)
  - Success response: Marshal `MinimalResponse{ID: fmt.Sprintf("%d", comment.GetID()), URL: comment.GetHTMLURL()}`
  - Return via `mcp.NewToolResultText(string(jsonBytes))`

**Integration Points**:
- Uses `GetClientFn` parameter for REST client
- Uses `translations.TranslationHelperFunc` for description text
- Integrates with `pkg/github/server.go` parameter validation helpers
- Uses `pkg/errors/error.go` error response formatter
- Returns `pkg/github/minimal_types.go` MinimalResponse format

### Success Criteria:

#### Automated Verification:
- [x] Code compiles without errors: `go build ./cmd/github-mcp-server`
- [x] No linting errors: `script/lint`
- [x] Function signature matches pattern: returns `(mcp.Tool, server.ToolHandlerFunc)`
- [x] Tool definition includes all required parameters with correct types
- [x] Parameter validation uses appropriate helpers (RequiredParam, RequiredInt, RequiredBigInt)
- [x] Error handling follows ghErrors.NewGitHubAPIErrorResponse pattern
- [x] Response format uses MinimalResponse with ID and URL fields

#### Manual Verification:
- [x] Tool function is properly exported (capitalized function name)
- [x] Handler function parameter extraction order is logical (owner, repo, pull_number, comment_id, body)
- [x] HTTP status check uses correct constant (http.StatusCreated for 201)
- [x] Response body is deferred closed after API call
- [x] Go-github method signature matches: `CreateCommentInReplyTo(ctx, owner, repo, number, body, commentID)`

### Phase 1 Completion Summary

Phase 1 has been successfully completed. The `ReplyToReviewComment` function was implemented in `pkg/github/pullrequests.go` following the established MCP tool pattern.

**Implementation Details:**
- Added `ReplyToReviewComment` function at line 1612 (after `RequestCopilotReview`)
- Tool name: `reply_to_review_comment`
- All required parameters properly defined: owner, repo, pull_number, comment_id, body
- Uses `RequiredBigInt` for comment_id to handle int64 type
- Calls `client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, pullNumber, body, commentID)`
- Returns `MinimalResponse` with reply ID and URL on success (HTTP 201)
- Proper error handling with `ghErrors.NewGitHubAPIErrorResponse`
- Response body deferred close after API call

**Verification Results:**
- Code compiles successfully
- Linting passes with 0 issues
- All manual verification checks confirmed

**Commit:** f5140d4 - "Add ReplyToReviewComment tool for replying to PR review comments"

**Next Phase:** Phase 2 - Toolset Integration (register the tool in the pull_requests toolset)

---

## Phase 2: Toolset Integration

### Overview
Register the new `ReplyToReviewComment` tool in the pull_requests toolset within the write tools section, making it discoverable through the MCP server's tool listing.

### Changes Required:

#### 1. Toolset Registration
**File**: `pkg/github/tools.go`

**Changes**:
- Locate the pull_requests toolset definition (lines 243-262)
- Add tool registration in the `AddWriteTools` section after the "Reviews" comment
- Insert: `toolsets.NewServerTool(ReplyToReviewComment(getClient, t)),`
- Position after `AddCommentToPendingReview` to group review-related write tools together

**Integration Points**:
- Uses `toolsets.NewServerTool()` wrapper which accepts the `(mcp.Tool, server.ToolHandlerFunc)` tuple
- Integrates with `pkg/toolsets/toolsets.go` AddWriteTools method (lines 140-149)
- Tool becomes part of the repository_management toolset alongside other PR modification tools

### Success Criteria:

#### Automated Verification:
- [x] Code compiles after registration: `go build ./cmd/github-mcp-server`
- [x] No linting errors: `script/lint`
- [x] Server starts without errors: `./github-mcp-server stdio` exits cleanly on interrupt

#### Manual Verification:
- [x] Tool appears in the MCP tool list when server is queried
- [x] Tool is categorized as a write tool (not read-only)
- [x] Tool registration follows the established pattern (uses `toolsets.NewServerTool` wrapper)
- [x] Tool is positioned logically with other review-related write tools

### Phase 2 Completion Summary

Phase 2 has been successfully completed. The `ReplyToReviewComment` tool has been registered in the pull_requests toolset.

**Implementation Details:**
- Added `toolsets.NewServerTool(ReplyToReviewComment(getClient, t))` to the `AddWriteTools` section in `pkg/github/tools.go`
- Positioned after `AddCommentToPendingReview` to group review-related write tools together
- Tool is now part of the pull_requests/repository_management toolset

**Verification Results:**
- Build completes successfully with no errors
- Linting passes with 0 issues
- Tool registration follows established pattern (uses REST client via getClient parameter)
- Tool is correctly categorized as a write tool (ReadOnlyHint set to false in Phase 1)

**Commit:** 31c8768 - "Register ReplyToReviewComment tool in pull_requests toolset"

**Next Phase:** Phase 3 - Testing (add unit tests with toolsnap validation and table-driven behavioral tests)

---

## Phase 3: Testing

### Overview
Add comprehensive unit tests following the established patterns in `pkg/github/pullrequests_test.go`, including toolsnap schema validation and table-driven behavioral tests covering success and error scenarios.

### Changes Required:

#### 1. Unit Tests
**File**: `pkg/github/pullrequests_test.go`

**Changes**:
- Add `Test_ReplyToReviewComment` function following the pattern from `Test_GetPullRequest` (lines 20-143)
- Implement toolsnap validation test:
  - Create mock client: `mockClient := github.NewClient(nil)`
  - Call tool function: `tool, _ := ReplyToReviewComment(stubGetClientFn(mockClient), translations.NullTranslationHelper)`
  - Validate toolsnap: `require.NoError(t, toolsnaps.Test(tool.Name, tool))`
  - Assert tool properties: name, description, parameters (owner, repo, pull_number, comment_id, body)
  - Assert required fields: `assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pull_number", "comment_id", "body"})`
  - Verify ReadOnlyHint is false for write operation
- Implement table-driven behavioral tests with cases:
  1. **Successful reply creation**: Mock HTTP response with 201 status and PullRequestComment object
  2. **Comment not found**: Mock 404 response with "Not Found" message
  3. **Permission denied**: Mock 403 response with "Forbidden" message
  4. **Invalid body**: Mock 422 response with validation error
  5. **Missing required parameter**: Test validation errors for missing owner/repo/pull_number/comment_id/body
  6. **Invalid comment_id type**: Test error when comment_id is not a number
- Use `mock.NewMockedHTTPClient` with `mock.WithRequestMatch` for successful case
- Use `mock.WithRequestMatchHandler` for error cases to control HTTP status and response body
- Assert response structure for success case (MinimalResponse with id and url fields)
- Assert error messages for failure cases

**Integration Points**:
- Uses `github.com/migueleliasweb/go-github-mock` for REST API mocking
- Uses `internal/toolsnaps/toolsnaps.go` for schema validation
- Follows test helper patterns: `stubGetClientFn`, `createMCPRequest`, `getTextResult`, `getErrorResult`

#### 2. Toolsnap File Generation
**Command**: `UPDATE_TOOLSNAPS=true go test ./pkg/github -run Test_ReplyToReviewComment`

**Result**: Generates `pkg/github/__toolsnaps__/reply_to_review_comment.snap` with JSON schema

**Commit Requirement**: Must commit the `.snap` file with code changes

#### 3. E2E Test Addition
**File**: `e2e/e2e_test.go`

**Changes**:
- Add `testReplyToReviewComment` function following e2e test style patterns
- Test scenario: Create a test PR, add a review comment, reply to it using the tool, verify reply appears
- Use existing test helper functions for PR setup and cleanup
- Verify reply ID and URL are returned in response
- Note: E2E tests require `GITHUB_MCP_SERVER_E2E_TOKEN` environment variable

### Success Criteria:

#### Automated Verification:
- [x] All unit tests pass: `go test ./pkg/github -run Test_ReplyToReviewComment -v`
- [x] Toolsnap validation passes (schema matches snapshot)
- [x] Full test suite passes: `script/test`
- [x] No race conditions detected: `go test -race ./pkg/github`
- [x] Toolsnap file exists: `pkg/github/__toolsnaps__/reply_to_review_comment.snap`
- [x] E2E test passes with valid token: `GITHUB_MCP_SERVER_E2E_TOKEN=<token> go test -v --tags e2e ./e2e -run testReplyToReviewComment`

#### Manual Verification:
- [x] Successful reply test returns expected MinimalResponse structure
- [x] Error tests return descriptive error messages
- [x] Parameter validation tests catch all missing/invalid parameters
- [x] Mock HTTP requests match expected GitHub API endpoint pattern
- [x] Test coverage includes all main code paths (success, 404, 403, 422, validation errors)

### Phase 3 Completion Summary

Phase 3 has been successfully completed. Comprehensive tests have been added for the `ReplyToReviewComment` tool.

**Implementation Details:**
- **Unit Tests**: Added `Test_ReplyToReviewComment` function in `pkg/github/pullrequests_test.go` with:
  - Toolsnap schema validation
  - ReadOnlyHint verification (false for write operation)
  - Table-driven behavioral tests with 10 test cases covering:
    * Successful reply creation (HTTP 201)
    * Comment not found (HTTP 404)
    * Permission denied (HTTP 403)
    * Validation failure (HTTP 422)
    * Missing required parameters: owner, repo, pull_number, comment_id, body
    * Invalid comment_id type
  - Uses `mock.EndpointPattern` with `/repos/{owner}/{repo}/pulls/{pull_number}/comments` endpoint
  - Validates MinimalResponse structure with id and url fields

- **Toolsnap File**: Generated `pkg/github/__toolsnaps__/reply_to_review_comment.snap` containing JSON schema with:
  - Tool name: `reply_to_review_comment`
  - All required parameters with proper types and descriptions
  - ReadOnlyHint: false annotation
  - Complete input schema validation rules

- **E2E Test**: Added `TestReplyToReviewComment` function in `e2e/e2e_test.go`:
  - Creates test repository, branch, commit, and pull request
  - Adds review comment via pending review workflow
  - Calls `reply_to_review_comment` tool with valid parameters
  - Verifies reply appears in review comments list
  - Validates MinimalResponse structure (id and url fields)
  - Confirms reply body text matches expected value

**Verification Results:**
- All unit tests pass (10/10 test cases)
- Toolsnap validation passes
- Full test suite passes (`script/test`)
- Linting clean (`script/lint` - 0 issues)
- Test coverage includes all critical paths: success, error handling, parameter validation

**Commit:** 8d6c3a9 - "Add comprehensive tests for ReplyToReviewComment tool"

**Phase PR:** https://github.com/lossyrob/github-mcp-server/pull/4

**Notes for Reviewers:**
- E2E test requires `GITHUB_MCP_SERVER_E2E_TOKEN` to run and cannot be executed without a valid GitHub PAT
- Mock endpoint pattern discovered: go-github's `CreateCommentInReplyTo` uses `/repos/{owner}/{repo}/pulls/{pull_number}/comments` not the `/replies` endpoint
- All test cases follow established patterns from existing PR tool tests
- Test assertions verify both success responses and error messages

**Next Phase:** Phase 4 - Documentation & Validation (generate updated README.md and run validation scripts)

---

## Phase 4: Documentation & Validation

### Overview
Generate updated documentation and run all validation scripts to ensure the implementation meets code quality standards and the tool is properly documented for users.

### Changes Required:

#### 1. Documentation Generation
**Command**: `script/generate-docs`

**Changes**:
- Auto-generates README.md sections documenting the new `reply_to_review_comment` tool
- Includes tool description, parameters, and usage examples
- Places tool documentation in the appropriate category with other PR tools

**Integration Points**:
- Uses `cmd/github-mcp-server/generate_docs.go` to introspect MCP tools
- Updates README.md sections marked with auto-generation markers

#### 2. Validation Scripts
**Commands**:
- `script/lint` - Run gofmt and golangci-lint
- `script/test` - Run full test suite with race detection
- `script/licenses-check` - Verify license compliance (no new dependencies expected)

**Expected Results**:
- No linting errors
- All tests pass
- No license compliance issues
- Documentation is up-to-date

### Success Criteria:

#### Automated Verification:
- [ ] Documentation generates successfully: `script/generate-docs` exits with code 0
- [ ] README.md is updated with tool documentation
- [ ] No git diff after doc generation indicates docs are current
- [ ] Linting passes: `script/lint` exits with code 0
- [ ] Full test suite passes: `script/test` exits with code 0
- [ ] License check passes: `script/licenses-check` exits with code 0
- [ ] CI workflows pass (go.yml, lint.yml, docs-check.yml)

#### Manual Verification:
- [ ] README.md includes clear description of `reply_to_review_comment` tool
- [ ] Tool parameters are documented with types and descriptions
- [ ] Tool is listed in the appropriate section with other PR/review tools
- [ ] Example usage (if generated) is clear and correct
- [ ] All validation scripts run successfully without manual intervention

---

## Testing Strategy

### Unit Tests:
- Toolsnap validation ensures tool schema remains stable across changes
- Parameter validation tests verify required fields and type checking
- Success case tests verify MinimalResponse format
- Error case tests verify descriptive error messages for 404, 403, 422 responses
- Mock HTTP client simulates GitHub API behavior without external dependencies

### Integration Tests:
- E2E test creates real PR and review comment on GitHub test repository
- Verifies end-to-end flow from comment creation to reply posting
- Confirms reply appears correctly in GitHub UI
- Validates notifications are sent to comment author

### Manual Testing Steps:
1. Start the MCP server: `./github-mcp-server stdio`
2. Use an MCP client to list tools and verify `reply_to_review_comment` appears
3. Create a test PR with a review comment
4. Use `pull_request_read` with method `get_review_comments` to get comment ID
5. Call `reply_to_review_comment` with valid parameters
6. Verify reply appears in GitHub UI as a threaded response
7. Verify notification email/UI notification is sent
8. Test error cases: invalid comment ID, missing permissions, archived repo

## Performance Considerations

No special performance optimizations required. The tool makes a single GitHub API call per invocation, consistent with other write tools in the codebase. GitHub's standard rate limits (5,000 requests/hour for authenticated users) apply. Agents orchestrating batch replies should implement appropriate throttling if needed.

## Migration Notes

No data migration or breaking changes. This is a new tool addition that does not affect existing tools or APIs. Users can adopt the tool incrementally without changes to existing workflows.

## References

- Original Issue: https://github.com/github/github-mcp-server/issues/1323
- Spec: `.paw/work/reply-to-review-comments/Spec.md`
- Research: `.paw/work/reply-to-review-comments/SpecResearch.md`, `.paw/work/reply-to-review-comments/CodeResearch.md`
- GitHub API Documentation: https://docs.github.com/en/rest/pulls/comments?apiVersion=2022-11-28#create-a-reply-for-a-review-comment
- go-github v79 CreateCommentInReplyTo: https://pkg.go.dev/github.com/google/go-github/v79/github#PullRequestsService.CreateCommentInReplyTo
- Similar implementation pattern: `CreatePullRequest` in `pkg/github/pullrequests.go:314-444`
- Parameter validation helpers: `pkg/github/server.go:69-116`
- Error handling: `pkg/errors/error.go:111-119`
- Toolset registration: `pkg/github/tools.go:243-262`
