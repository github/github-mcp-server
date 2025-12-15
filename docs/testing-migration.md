# Testing Infrastructure Migration

## Overview

This document describes the ongoing migration from `migueleliasweb/go-github-mock` to `stretchr/testify`-based HTTP mocking infrastructure.

## Motivation

As described in issue #1492, we are migrating from `go-github-mock` to consolidate our testing dependencies:

1. **Dependency Consolidation**: We already use `stretchr/testify` for assertions
2. **Maintenance**: Reduce the number of external dependencies
3. **Flexibility**: Custom HTTP mocking provides more control over test scenarios
4. **Clarity**: Simpler, more readable test setup

## New Testing Infrastructure

The new mocking infrastructure is located in `pkg/github/helper_test.go` and provides:

### MockHTTPClientWithHandlers

Creates an HTTP client with multiple handlers for different API endpoints:

```go
mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    "GET /repos/{owner}/{repo}": mockResponse(t, http.StatusOK, mockRepo),
    "GET /repos/{owner}/{repo}/git/trees/{tree}": mockResponse(t, http.StatusOK, mockTree),
})
```

### Path Pattern Matching

Supports GitHub API path patterns with parameter placeholders:
- `{owner}`, `{repo}`, `{tree}`, etc. are treated as wildcards
- Exact paths are matched first, then patterns
- Query parameters can be validated using the existing `expectQueryParams` helper

### Helper Functions

- `mockResponse(t, statusCode, body)` - Creates JSON responses
- `expectQueryParams(t, params).andThen(handler)` - Validates query parameters
- `MockHTTPClientWithHandler(handler)` - Single handler for all requests

## Migration Pattern

### Before (using go-github-mock):

```go
mockedClient := mock.NewMockedHTTPClient(
    mock.WithRequestMatch(
        mock.GetReposByOwnerByRepo,
        mockRepo,
    ),
    mock.WithRequestMatchHandler(
        mock.GetReposGitTreesByOwnerByRepoByTreeSha,
        mockResponse(t, http.StatusOK, mockTree),
    ),
)
```

### After (using new infrastructure):

```go
mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    "GET /repos/{owner}/{repo}": mockResponse(t, http.StatusOK, mockRepo),
    "GET /repos/{owner}/{repo}/git/trees/{tree}": mockResponse(t, http.StatusOK, mockTree),
})
```

## Migration Status

### Completed
- ✅ `pkg/github/git_test.go` - 196 lines (pilot migration)
- ✅ `pkg/github/code_scanning_test.go` - 258 lines

### Remaining (in order of size, smallest first)
- ⏳ `pkg/github/secret_scanning_test.go` - 263 lines
- ⏳ `pkg/github/dependabot_test.go` - 269 lines
- ⏳ `pkg/github/repository_resource_test.go` - 314 lines
- ⏳ `pkg/github/context_tools_test.go` - 498 lines
- ⏳ `pkg/github/security_advisories_test.go` - 546 lines
- ⏳ `pkg/github/gists_test.go` - 621 lines
- ⏳ `pkg/github/search_test.go` - 760 lines
- ⏳ `pkg/github/notifications_test.go` - 783 lines
- ⏳ `pkg/github/actions_test.go` - 1442 lines
- ⏳ `pkg/github/projects_test.go` - 1684 lines
- ⏳ `pkg/github/pullrequests_test.go` - 3263 lines
- ⏳ `pkg/github/repositories_test.go` - 3472 lines
- ⏳ `pkg/github/issues_test.go` - 3705 lines
- ⏳ `pkg/raw/raw_mock.go` - Special case (defines endpoint patterns)

**Total remaining: ~14,210 lines of test code across 14 files**

## API Endpoint Patterns

Common GitHub API endpoints and their patterns:

| Endpoint | Pattern |
|----------|---------|
| Get Repository | `GET /repos/{owner}/{repo}` |
| Get Tree | `GET /repos/{owner}/{repo}/git/trees/{tree}` |
| Get Issue | `GET /repos/{owner}/{repo}/issues/{issue_number}` |
| List Issues | `GET /repos/{owner}/{repo}/issues` |
| Get Pull Request | `GET /repos/{owner}/{repo}/pulls/{pull_number}` |
| Code Scanning Alert | `GET /repos/{owner}/{repo}/code-scanning/alerts/{alert_number}` |
| Secret Scanning Alert | `GET /repos/{owner}/{repo}/secret-scanning/alerts/{alert_number}` |

For a complete reference, see the [GitHub REST API documentation](https://docs.github.com/en/rest).

## Testing Best Practices

1. **Use pattern matching for flexibility**: `{owner}`, `{repo}`, etc.
2. **Validate query parameters when needed**: Use `expectQueryParams`
3. **Keep test setup close to test cases**: Define handlers inline
4. **Reuse mock data across test cases**: Define once at the top
5. **Test both success and failure cases**: Include error scenarios

## Next Steps

1. Continue migrating test files incrementally (smallest first)
2. Once all files are migrated, remove `go-github-mock` dependency
3. Update `pkg/raw/raw_mock.go` to use new patterns
4. Run `go mod tidy` to clean up dependencies
5. Update third-party licenses with `./script/licenses`

## References

- Issue: #1492
- testify documentation: https://pkg.go.dev/github.com/stretchr/testify
- GitHub API: https://docs.github.com/en/rest
