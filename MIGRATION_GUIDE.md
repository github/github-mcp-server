# go-github-mock to testify Migration Guide

## Status

**Completed: 8/14 files (57%)**

**Next Priority Files:**
1. notifications_test.go (801 lines) - **Easiest** - only 4 simple WithRequestMatch patterns
2. search_test.go (776 lines) - 16 WithRequestMatchHandler (all with expectQueryParams)
3. projects_test.go (1,711 lines) - 31 WithRequestMatchHandler  
4. pullrequests_test.go (3,355 lines) - Mixed patterns
5. repositories_test.go (3,532 lines) - Mixed patterns
6. issues_test.go (3,755 lines) - **Largest** - mixed patterns

### ✅ Migrated Files
1. pkg/raw/raw_test.go 
2. pkg/github/actions_test.go (1,428 lines)
3. pkg/github/context_tools_test.go (530 lines)
4. pkg/github/dependabot_test.go (271 lines)
5. pkg/github/gists_test.go (617 lines)
6. pkg/github/repository_resource_test.go (307 lines)
7. pkg/github/secret_scanning_test.go (267 lines)
8. pkg/github/security_advisories_test.go (551 lines)

### ⏳ Remaining Files
1. pkg/github/issues_test.go (3,755 lines) - **Largest file**
2. pkg/github/pullrequests_test.go (3,355 lines)
3. pkg/github/repositories_test.go (3,532 lines)
4. pkg/github/projects_test.go (1,711 lines)
5. pkg/github/notifications_test.go (801 lines)
6. pkg/github/search_test.go (776 lines)

**Total remaining: ~13,930 lines**

## Migration Pattern

### Step 1: Remove Import
```go
// Remove this line
"github.com/migueleliasweb/go-github-mock/src/mock"
```

### Step 2: Replace Mock Client Creation

**Pattern A: Simple Mock with Data**
```go
// BEFORE:
mock.NewMockedHTTPClient(
    mock.WithRequestMatch(
        mock.GetAdvisories,
        mockData,
    ),
)

// AFTER:
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    GetAdvisories: mockResponse(t, http.StatusOK, mockData),
})
```

**Pattern B: Mock with Custom Handler**
```go
// BEFORE:
mock.NewMockedHTTPClient(
    mock.WithRequestMatchHandler(
        mock.GetUser,
        http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(mockUser)
        }),
    ),
)

// AFTER:
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    GetUser: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(mockUser)
    }),
})
```

**Pattern C: Mock with Query Parameter Expectations**
```go
// BEFORE:
mock.NewMockedHTTPClient(
    mock.WithRequestMatchHandler(
        mock.GetSearchRepositories,
        expectQueryParams(t, map[string]string{"q": "test"}).andThen(
            mockResponse(t, http.StatusOK, mockData),
        ),
    ),
)

// AFTER:
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    GetSearchRepositories: expectQueryParams(t, map[string]string{"q": "test"}).andThen(
        mockResponse(t, http.StatusOK, mockData),
    ),
})
```

**Pattern D: Empty Mock (for validation tests)**
```go
// BEFORE:
mock.NewMockedHTTPClient()

// AFTER:
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
```

### Step 3: Fix Constant Names

Replace old mock constants with new ones (note ID vs Id):
- `mock.GetGistsByGistId` → `GetGistsByGistID`
- `mock.GetNotificationsThreadsByThreadId` → `GetNotificationsThreadsByThreadID`
- `mock.PatchGistsByGistId` → `PatchGistsByGistID`

All endpoint constants are defined in `pkg/github/helper_test.go`.

## Special Cases

### Case 1: With expectQueryParams and andThen

When the handler uses `expectQueryParams(...).andThen(...)`, the structure must be carefully closed:

```go
// CORRECT:
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    GetSearchRepositories: expectQueryParams(t, map[string]string{
        "q": "test",
    }).andThen(
        mockResponse(t, http.StatusOK, data),
    ),  // <- Close andThen with ),
}),  // <- Close map with }),

// INCORRECT (missing map close):
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    GetSearchRepositories: expectQueryParams(t, map[string]string{
        "q": "test",
    }).andThen(
        mockResponse(t, http.StatusOK, data),
    ),  // <- Only closes andThen, missing }),
```

### Case 2: Multiple Endpoints in One Mock

```go
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPR),
    GetRawReposContentsByOwnerByRepoBySHAByPath: mockResponse(t, http.StatusOK, mockContent),
}),
```

## Common Issues and Solutions

### Issue 1: Extra Closing Braces
**Symptom:** Syntax errors with `expected operand, found '}'`

**Cause:** Regex replacement left extra `)` or `}` 

**Fix:** Check for patterns like:
- `mockResponse(t, http.StatusOK, data})` should be `mockResponse(t, http.StatusOK, data)`
- `}),` at wrong indentation - should be `})` to close map then `,`

### Issue 2: Missing Closing Braces
**Symptom:** `missing ',' in argument list`

**Fix:** Map literal should close with `})` not just `)`:
```go
MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
    GetUser: mockResponse(t, http.StatusOK, mockUser),
}),  // Note the closing }),
```

### Issue 3: Raw Content Endpoints
For raw content endpoints that handle paths with slashes (e.g., `pkg/github/actions.go`), use wildcard patterns:
- `GetRawReposContentsByOwnerByRepoByPath` uses `{path:.*}` pattern
- This is already configured in `helper_test.go`

## Testing After Migration

```bash
# Test specific file
go test ./pkg/github -run TestFunctionName -v

# Test all GitHub tests
go test ./pkg/github -v

# Run linter
script/lint
```

## Available Helper Functions

All defined in `pkg/github/helper_test.go`:

1. **MockHTTPClientWithHandlers** - Creates HTTP client with route handlers
2. **mockResponse** - Creates standard JSON response handler
3. **expectQueryParams** - Validates query parameters
4. **expectPath** - Validates request path
5. **expect** - Combines multiple expectations

## Endpoint Constants

All ~130 endpoint constants are in `pkg/github/helper_test.go`, including:
- User endpoints (GetUser, GetUserStarred, etc.)
- Repository endpoints (GetReposByOwnerByRepo, etc.)
- Issues endpoints (GetReposIssuesByOwnerByRepoByIssueNumber, etc.)
- Pull request endpoints
- Actions endpoints
- And many more...

## Final Steps (After All Files Migrated)

1. Remove `migueleliasweb/go-github-mock` from go.mod
2. Remove `pkg/raw/raw_mock.go` (temporarily restored for compatibility)
3. Run `go mod tidy`
4. Run `script/licenses` to update license files
5. Run full test suite: `script/test`
6. Run linter: `script/lint`
