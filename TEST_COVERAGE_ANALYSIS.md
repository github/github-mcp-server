# Test Coverage Analysis - GitHub MCP Server

**Analysis Date:** 2025-11-21
**Overall Coverage:** 58.7%

## Executive Summary

The GitHub MCP Server has a moderate level of test coverage (58.7%) with strong testing in core business logic areas but significant gaps in infrastructure, initialization, and integration code. This analysis identifies specific areas where additional tests would provide the most value.

## Current Coverage by Package

### Well-Tested Packages (>80%)
- **pkg/errors**: 94.6% ✅
- **pkg/raw**: 90.9% ✅
- **internal/toolsnaps**: 84.6% ✅
- **pkg/log**: 81.8% ✅

### Moderately Tested Packages (40-80%)
- **pkg/github**: 69.3% ⚠️
- **pkg/toolsets**: 40.2% ⚠️

### Untested Packages (0%)
- **cmd/github-mcp-server**: 0.0% ❌
- **cmd/mcpcurl**: 0.0% ❌
- **internal/ghmcp**: 0.0% ❌
- **internal/profiler**: 0.0% ❌
- **pkg/buffer**: 0.0% ❌
- **pkg/translations**: 0.0% ❌

## Priority Areas for Improvement

### 1. Critical Infrastructure Code (HIGH PRIORITY)

#### A. Server Initialization (`internal/ghmcp/server.go`)
**Current Coverage:** 0.0%
**Impact:** High - This is the core server setup code

**Missing Tests:**
- `NewMCPServer()` - Server creation and configuration
- `parseAPIHost()` - GitHub API host parsing (github.com, GHES, GHEC)
- Authentication transport setup
- Client initialization logic

**Recommended Tests:**
```go
// Test cases to add:
- Test NewMCPServer with github.com host
- Test NewMCPServer with GHES host (https://github.enterprise.com)
- Test NewMCPServer with GHEC host (https://subdomain.ghe.com)
- Test invalid host parsing
- Test token authentication setup
- Test user agent configuration with client info
```

**Why This Matters:** This code handles critical setup including authentication, API endpoint configuration, and client initialization. Bugs here affect all functionality.

#### B. Profiler (`internal/profiler/profiler.go`)
**Current Coverage:** 0.0%
**Impact:** Medium - Performance monitoring and diagnostics

**Missing Tests:**
- Memory delta calculations (especially `safeMemoryDelta` with overflow handling)
- Profile collection and formatting
- Global profiler initialization
- Environment variable parsing

**Recommended Tests:**
```go
// Test cases to add:
- Test safeMemoryDelta with normal values
- Test safeMemoryDelta with values > MaxInt64 (overflow handling)
- Test ProfileFunc when enabled/disabled
- Test ProfileFuncWithMetrics captures lines and bytes
- Test IsProfilingEnabled with various env var values
- Test global profiler initialization
```

**Why This Matters:** The profiler includes complex overflow handling logic that should be tested. Memory calculation bugs could cause crashes or incorrect diagnostics.

#### C. Buffer Utilities (`pkg/buffer/buffer.go`)
**Current Coverage:** 0.0%
**Impact:** Medium - Used for log streaming

**Missing Tests:**
- Ring buffer logic for log tailing
- Line counting and truncation
- Edge cases (empty logs, single line, exactly maxLines, more than maxLines)

**Recommended Tests:**
```go
// Test cases to add:
- Test ProcessResponseAsRingBufferToEnd with empty response
- Test with fewer lines than buffer size
- Test with exactly buffer size lines
- Test with more lines than buffer size (ring buffer wraparound)
- Test with very long lines (>2000 chars, should truncate per docs)
- Test scanner error handling
```

**Why This Matters:** Ring buffer logic is error-prone, especially at boundary conditions. Used in job log retrieval which is a user-facing feature.

### 2. Translation System (MEDIUM PRIORITY)

#### Translation Helper (`pkg/translations/translations.go`)
**Current Coverage:** 0.0%
**Impact:** Medium - i18n support

**Missing Tests:**
- Translation key lookup
- Environment variable overrides (`GITHUB_MCP_*` prefix)
- Config file loading
- Translation map dumping

**Recommended Tests:**
```go
// Test cases to add:
- Test NullTranslationHelper returns default value
- Test TranslationHelper loads from config file
- Test TranslationHelper uses env var overrides
- Test TranslationHelper caches keys
- Test DumpTranslationKeyMap creates valid JSON
```

**Why This Matters:** Translation system is user-facing and affects all tool descriptions. Bugs here could break tool discovery.

### 3. Dynamic Toolset Discovery (MEDIUM PRIORITY)

#### Dynamic Tools (`pkg/github/dynamic_tools.go`)
**Current Coverage:** 0.0%
**Impact:** Medium - Feature discovery system

**Missing Tests:**
- `EnableToolset()` handler
- `ListAvailableToolsets()` handler
- `GetToolsetsTools()` handler
- Toolset already enabled scenarios
- Invalid toolset name handling

**Recommended Tests:**
```go
// Test cases to add:
- Test EnableToolset enables a disabled toolset
- Test EnableToolset returns message when already enabled
- Test EnableToolset returns error for non-existent toolset
- Test ListAvailableToolsets returns all toolsets with status
- Test GetToolsetsTools returns correct tools for toolset
- Test GetToolsetsTools handles invalid toolset name
```

**Why This Matters:** This is a beta feature that helps with tool choice. Good test coverage ensures reliability as it moves to GA.

### 4. GitHub API Handlers - Edge Cases (MEDIUM PRIORITY)

Many handlers in `pkg/github/` have 66-79% coverage. The missing coverage is typically:
- Error path testing
- Pagination edge cases
- Empty result sets
- API error response handling

#### Areas to Improve:

**Code Scanning (`code_scanning.go`)**: 66.7%
- Test error responses from GitHub API
- Test empty alert lists
- Test pagination

**Dependabot (`dependabot.go`)**: 66.7%
- Test error responses
- Test empty alert lists
- Test severity filtering

**Issues (`issues.go`)**: Various functions at 66-79%
- Test create issue with all optional parameters
- Test issue type enumeration edge cases
- Test sub-issue API error handling

**Actions (`actions.go`)**: Multiple functions at 76-88%
- Test workflow run functions that aren't covered:
  - `GetWorkflowRun()`: 0.0%
  - `GetWorkflowRunLogs()`: 0.0%
  - `ListWorkflowJobs()`: 0.0%
  - `RerunWorkflowRun()`: 0.0%
  - `RerunFailedJobs()`: 0.0%

**Recommended Approach:**
For each handler, add test cases for:
```go
// Common missing test patterns:
- API returns 404 (resource not found)
- API returns 403 (permission denied)
- API returns 500 (server error)
- API returns empty list/null response
- Network timeout/connection error
- Invalid pagination parameters
- Missing required fields in response
```

### 5. Command-Line Tools (LOW PRIORITY)

#### Main Binary (`cmd/github-mcp-server/main.go`)
**Current Coverage:** 0.0%
**Impact:** Low - Mostly initialization code, covered by e2e tests

**Rationale for Low Priority:**
- Already covered by e2e tests
- Mostly thin wrappers around tested packages
- Configuration parsing is harder to unit test
- Cost/benefit ratio is lower than other areas

**If Testing:**
Consider integration tests rather than unit tests for:
- Command-line flag parsing
- Environment variable handling
- Config file loading
- Exit code behavior

#### Documentation Generator (`cmd/github-mcp-server/generate_docs.go`)
**Current Coverage:** 0.0%
**Impact:** Low - Developer tooling

**Rationale for Low Priority:**
- Not production code
- Output is human-verified
- Changes infrequently
- Manual testing is straightforward

### 6. Toolset Management (MEDIUM PRIORITY)

#### Toolsets Package (`pkg/toolsets/toolsets.go`)
**Current Coverage:** 40.2%
**Impact:** Medium - Core functionality

**Missing Coverage:**
- `RegisterTools()`: 0.0%
- `AddResourceTemplates()`: 0.0%
- `AddPrompts()`: 0.0%
- `GetActiveTools()`: 0.0%
- `RegisterAll()`: 0.0%
- Dynamic toolset operations

**Recommended Tests:**
```go
// Test cases to add:
- Test RegisterTools adds tools to toolset
- Test AddResourceTemplates adds templates correctly
- Test AddPrompts adds prompts correctly
- Test GetActiveTools returns only enabled tools
- Test GetAvailableTools returns all tools
- Test SetReadOnly filters write tools
- Test EnableToolsets with "all" keyword
- Test EnableToolset with invalid name returns error
- Test toolset is/enabled status tracking
```

## Testing Patterns to Follow

Based on the existing tests, the project follows these patterns:

### 1. Handler Test Structure
```go
func Test_HandlerName(t *testing.T) {
    // 1. Verify tool definition once
    mockClient := github.NewClient(nil)
    tool, _ := HandlerFunc(stubGetClient(mockClient), translations.NullTranslationHelper)

    assert.Equal(t, "expected_name", tool.Name)
    assert.NotEmpty(t, tool.Description)
    // Verify schema properties

    // 2. Table-driven behavioral tests
    tests := []struct {
        name           string
        mockedClient   *http.Client
        requestArgs    map[string]any
        expectError    bool
        expectedErrMsg string
    }{
        // Test cases...
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 2. Use Testify Assertions
- Use `require` for critical checks (continuing is not meaningful)
- Use `assert` for non-critical checks
- Always use descriptive test names

### 3. Mock GitHub API
- Use `go-github-mock` for REST API calls
- Use `githubv4mock` for GraphQL calls
- Create realistic response objects

### 4. Tool Schema Snapshots
- Every tool must have a snapshot test
- Update snapshots with `UPDATE_TOOLSNAPS=true`
- CI fails on missing snapshots

## Recommended Testing Plan

### Phase 1: High Priority Infrastructure (Week 1-2)
1. Add tests for `internal/ghmcp/server.go` (focus on `parseAPIHost`, `NewMCPServer`)
2. Add tests for `internal/profiler/profiler.go` (focus on `safeMemoryDelta`)
3. Add tests for `pkg/buffer/buffer.go` (ring buffer logic)
4. Target: Increase overall coverage to ~65%

### Phase 2: Medium Priority Features (Week 3-4)
1. Add tests for `pkg/translations/translations.go`
2. Add tests for `pkg/github/dynamic_tools.go`
3. Improve coverage in `pkg/toolsets/toolsets.go` to >80%
4. Target: Increase overall coverage to ~70%

### Phase 3: Complete GitHub Handler Coverage (Ongoing)
1. Identify functions with 0% coverage in `pkg/github/`
2. Add tests for uncovered functions (prioritize by usage)
3. Add edge case tests for functions with 66-79% coverage
4. Target: Increase pkg/github coverage to >85%

### Phase 4: Integration and Edge Cases
1. Expand e2e test coverage for critical workflows
2. Add error injection tests
3. Add performance/stress tests using profiler
4. Target: Overall coverage >75%

## Metrics to Track

| Metric | Current | Target (3 months) |
|--------|---------|-------------------|
| Overall Coverage | 58.7% | 75%+ |
| pkg/github Coverage | 69.3% | 85%+ |
| pkg/toolsets Coverage | 40.2% | 80%+ |
| Critical Infrastructure | 0% | 90%+ |
| Packages with 0% Coverage | 6 | 0 |

## Testing Anti-Patterns to Avoid

Based on the existing test suite:

1. ❌ **Don't test main() functions directly** - Use e2e or integration tests instead
2. ❌ **Don't test global state mutations without cleanup** - E.g., mark all notifications as read
3. ❌ **Don't skip snapshot tests** - They prevent breaking schema changes
4. ❌ **Don't use `_test` package suffix** - Tests prefer internal package access
5. ✅ **Do use table-driven tests** - Easier to add cases
6. ✅ **Do test error paths** - Most missing coverage is error handling
7. ✅ **Do use mocks for GitHub API** - Fast, reliable, no rate limits

## Conclusion

The codebase has solid test coverage in core business logic (pkg/errors, pkg/raw, pkg/log) but needs improvement in:

1. **Infrastructure code** (0% coverage) - server initialization, profiling, buffering
2. **Feature discovery** (0% coverage) - translation system, dynamic toolsets
3. **Edge cases** - API error handling, pagination, empty results
4. **Toolset management** (40% coverage) - needs doubling

Focusing on infrastructure tests first will provide the most value, as these areas affect all functionality and currently have no safety net. The existing test patterns are solid and should be followed for consistency.

## Quick Wins

These tests would be fast to write and provide immediate value:

1. **pkg/buffer/buffer.go** - Single file, pure logic, no external deps (~2 hours)
2. **internal/profiler/profiler.go** - `safeMemoryDelta` function is critical (~3 hours)
3. **pkg/github/dynamic_tools.go** - Small API surface, clear behavior (~3 hours)
4. **GitHub handler functions with 0% coverage** - Follow existing patterns (~1-2 hours each)

**Estimated Time for Quick Wins:** ~2 days of focused work would add 5-7% to overall coverage and eliminate most critical gaps.
