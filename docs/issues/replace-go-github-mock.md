# Replace migueleliasweb/go-github-mock with stretchr/testify/mock

**Type:** Enhancement / Refactoring  
**Labels:** `enhancement`, `testing`, `technical-debt`

---

### Describe the feature or problem you'd like to solve

The current test suite uses [migueleliasweb/go-github-mock](https://github.com/migueleliasweb/go-github-mock) for mocking GitHub REST API responses in unit tests. While this library has served us well, there are several reasons to consider replacing it with [stretchr/testify/mock](https://github.com/stretchr/testify):

1. **Dependency Consolidation**: We already use `stretchr/testify` for assertions (`assert` and `require`). Using `testify/mock` would consolidate our testing dependencies.

2. **Interface-based Mocking**: `testify/mock` encourages interface-based mocking, which leads to better separation of concerns and more flexible test design.

3. **Maintenance & Activity**: `stretchr/testify` is one of the most widely used Go testing libraries with active maintenance.

4. **Type Safety**: Interface-based mocking provides compile-time type checking, whereas HTTP-level mocking relies on runtime matching.

5. **Test Clarity**: Mock expectations at the interface level make tests more readable and focused on behavior rather than HTTP transport details.

### Current State

The codebase currently uses `go-github-mock` extensively (metrics from `grep` search):

| Metric | Count |
|--------|-------|
| Files using go-github-mock | 16 |
| `mock.NewMockedHTTPClient` calls | ~449 |
| `mock.WithRequestMatchHandler` calls | ~267 |
| `mock.WithRequestMatch` calls | ~79 |
| Unique mock endpoint patterns | ~80+ |

*Note: Run `grep -c "mock.NewMockedHTTPClient" pkg/github/*_test.go` to verify current counts.*

**Files affected:**
- `pkg/github/*_test.go` (14 files)
- `pkg/raw/raw_test.go`
- `pkg/raw/raw_mock.go`

### Proposed solution

Replace HTTP-level mocking with interface-based mocking using `testify/mock`:

#### Phase 1: Create Mock Interfaces
1. Define interfaces for GitHub client operations (if not already present)
2. Create mock implementations using `testify/mock`
3. Update the codebase to depend on interfaces rather than concrete clients

#### Phase 2: Migrate Tests Incrementally
1. Start with a single test file to establish patterns
2. Create helper functions for common mock setups
3. Migrate remaining test files one at a time
4. Remove `go-github-mock` dependency when complete

#### Example Migration

**Before (HTTP-level mocking):**
```go
mockedClient := mock.NewMockedHTTPClient(
    mock.WithRequestMatch(
        mock.GetReposIssuesByOwnerByRepoByIssueNumber,
        mockIssue,
    ),
)
client := github.NewClient(mockedClient)
```

**After (Interface-based mocking):**
```go
mockClient := new(MockGitHubClient)
mockClient.On("GetIssue", ctx, "owner", "repo", 42).Return(mockIssue, nil)
```

### Benefits

1. **Simpler Test Setup**: No need to construct HTTP responses
2. **Better Error Testing**: Easy to mock error conditions without crafting HTTP error responses
3. **Faster Tests**: No HTTP round-trip overhead (even if mocked)
4. **Clearer Intent**: Tests read more like specifications
5. **Reduced Dependencies**: One less external dependency to maintain

### Considerations

1. **Migration Effort**: This is a significant refactoring with ~449 mock usages to update
2. **Interface Design**: Need to carefully design interfaces that balance granularity and usability
3. **GraphQL Mocking**: The existing `githubv4mock` for GraphQL is **out of scope** for this issue and will remain unchanged. It already provides a different mocking approach specific to GraphQL.
4. **Breaking Changes**: Test file changes only, no production code changes expected

### Implementation Plan

- [ ] Audit current mock usage patterns to identify common interfaces needed
- [ ] Design and implement mock interfaces for GitHub REST API client
- [ ] Create helper functions and test utilities for common mock scenarios
- [ ] Migrate test files incrementally (suggest starting with smallest files):
  - [ ] `pkg/github/code_scanning_test.go` (~4 mocks)
  - [ ] `pkg/github/secret_scanning_test.go` (~5 mocks)
  - [ ] `pkg/github/dependabot_test.go` (~6 mocks)
  - [ ] Continue with remaining files...
- [ ] Update `docs/testing.md` to reflect new mocking patterns
- [ ] Remove `go-github-mock` from `go.mod` after complete migration

### Additional context

**Current testing documentation reference:**
> Mocking is performed using [go-github-mock](https://github.com/migueleliasweb/go-github-mock) or `githubv4mock` for simulating GitHub REST and GQL API responses.

This will need to be updated to:
> Mocking is performed using [testify/mock](https://github.com/stretchr/testify#mock-package) for interface-based mocking or `githubv4mock` for simulating GitHub GQL API responses.

### Related

- [stretchr/testify documentation](https://pkg.go.dev/github.com/stretchr/testify/mock)
- Current testing guidelines: [`docs/testing.md`](../testing.md)
