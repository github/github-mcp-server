package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test helpers ---

// makeWriteGuardRequest builds a *mcp.CallToolRequest with the given tool name and JSON arguments.
func makeWriteGuardRequest(t *testing.T, toolName string, args map[string]any) *mcp.CallToolRequest {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      toolName,
			Arguments: json.RawMessage(raw),
		},
	}
}

// passthroughHandler is a MethodHandler that records whether it was called and returns success.
type passthroughHandler struct {
	called bool
}

func (h *passthroughHandler) handle(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
	h.called = true
	return nil, nil
}

// makeInventoryWithTool creates a minimal inventory with one tool for testing.
func makeInventoryWithTool(t *testing.T, toolName string, readOnly bool) *inventory.Inventory {
	t.Helper()
	tool := inventory.NewServerToolFromHandler(
		mcp.Tool{
			Name: toolName,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: readOnly,
			},
		},
		inventory.ToolsetMetadata{
			ID:          "test",
			Description: "test toolset",
		},
		func(_ any) mcp.ToolHandler {
			return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return nil, nil
			}
		},
	)
	inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{tool}).Build()
	require.NoError(t, err)
	return inv
}

// makeEmptyInventory creates an inventory with no tools.
func makeEmptyInventory(t *testing.T) *inventory.Inventory {
	t.Helper()
	inv, err := inventory.NewBuilder().Build()
	require.NoError(t, err)
	return inv
}

// makeDepsWithRepoResponse creates a BaseDeps with a mock HTTP client that returns
// the given repo visibility for any GET /repos/{owner}/{repo} request.
func makeDepsWithRepoResponse(t *testing.T, isPrivate bool) ToolDependencies {
	t.Helper()
	mockHTTP := NewMockedHTTPClient(
		WithRequestMatch(
			EndpointPattern(GetReposByOwnerByRepo),
			&gogithub.Repository{
				Private: gogithub.Ptr(isPrivate),
			},
		),
	)
	client := gogithub.NewClient(mockHTTP)
	return BaseDeps{Client: client}
}

// makeDepsWithAPIError creates a BaseDeps with a mock HTTP client that returns
// a 500 error for any GET /repos/{owner}/{repo} request.
func makeDepsWithAPIError(t *testing.T) ToolDependencies {
	t.Helper()
	mockHTTP := NewMockedHTTPClient(
		WithRequestMatchHandler(
			EndpointPattern(GetReposByOwnerByRepo),
			mockResponse(t, http.StatusInternalServerError, nil),
		),
	)
	client := gogithub.NewClient(mockHTTP)
	return BaseDeps{Client: client}
}

// assertBlocked asserts that the result is a tool error (blocked).
func assertBlocked(t *testing.T, result mcp.Result) {
	t.Helper()
	toolResult, ok := result.(*mcp.CallToolResult)
	require.True(t, ok, "expected *mcp.CallToolResult, got %T", result)
	assert.True(t, toolResult.IsError, "expected IsError=true (blocked), got IsError=false")
}

// assertPassedThrough asserts that the next handler was called (not blocked).
func assertPassedThrough(t *testing.T, handler *passthroughHandler) {
	t.Helper()
	assert.True(t, handler.called, "expected request to pass through to next handler")
}

// --- Tests ---

// Test 1: Read-only tool passes through without any API call.
func TestWritePrivateOnlyMiddleware_ReadOnlyTool_PassThrough(t *testing.T) {
	inv := makeInventoryWithTool(t, "get_file_contents", true /* readOnly */)
	// Use deps that would fail if GetClient is called (no mock client).
	// If the guard incorrectly calls the API, this will panic or error.
	deps := BaseDeps{Client: nil}

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "get_file_contents", map[string]any{
		"owner": "myorg",
		"repo":  "myrepo",
		"path":  "README.md",
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err)
	assertPassedThrough(t, handler)
	assert.Nil(t, result, "read-only tool should pass through with nil result from next handler")
}

// Test 2: Write tool targeting a private repo passes through.
func TestWritePrivateOnlyMiddleware_WriteTool_PrivateRepo_PassThrough(t *testing.T) {
	inv := makeInventoryWithTool(t, "create_issue", false /* not readOnly */)
	deps := makeDepsWithRepoResponse(t, true /* isPrivate */)

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "create_issue", map[string]any{
		"owner": "myorg",
		"repo":  "myrepo",
		"title": "Bug report",
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err)
	assertPassedThrough(t, handler)
	assert.Nil(t, result)
}

// Test 3: Write tool targeting a public repo is blocked.
func TestWritePrivateOnlyMiddleware_WriteTool_PublicRepo_Blocked(t *testing.T) {
	inv := makeInventoryWithTool(t, "create_issue", false /* not readOnly */)
	deps := makeDepsWithRepoResponse(t, false /* isPrivate=false → public */)

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "create_issue", map[string]any{
		"owner": "myorg",
		"repo":  "publicrepo",
		"title": "Bug report",
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err, "blocking should return nil Go error")
	assert.False(t, handler.called, "next handler should NOT be called for public repo")
	assertBlocked(t, result)
}

// Test 4: Write tool with API error is blocked (fail-closed).
func TestWritePrivateOnlyMiddleware_WriteTool_APIError_Blocked(t *testing.T) {
	inv := makeInventoryWithTool(t, "create_issue", false /* not readOnly */)
	deps := makeDepsWithAPIError(t)

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "create_issue", map[string]any{
		"owner": "myorg",
		"repo":  "somerepo",
		"title": "Bug report",
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err, "blocking should return nil Go error")
	assert.False(t, handler.called, "next handler should NOT be called when API errors (fail-closed)")
	assertBlocked(t, result)
}

// Test 5: create_repository with private=true passes through.
func TestWritePrivateOnlyMiddleware_CreateRepository_PrivateTrue_PassThrough(t *testing.T) {
	inv := makeInventoryWithTool(t, "create_repository", false /* not readOnly */)
	deps := BaseDeps{Client: nil} // No API call expected for create_repository

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "create_repository", map[string]any{
		"name":    "my-new-repo",
		"private": true,
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err)
	assertPassedThrough(t, handler)
	assert.Nil(t, result)
}

// Test 6: create_repository with private=false is blocked.
func TestWritePrivateOnlyMiddleware_CreateRepository_PrivateFalse_Blocked(t *testing.T) {
	inv := makeInventoryWithTool(t, "create_repository", false /* not readOnly */)
	deps := BaseDeps{Client: nil}

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "create_repository", map[string]any{
		"name":    "my-new-repo",
		"private": false,
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err, "blocking should return nil Go error")
	assert.False(t, handler.called, "next handler should NOT be called when private=false")
	assertBlocked(t, result)
}

// Test 7: create_repository with no private param is blocked (defaults to false).
func TestWritePrivateOnlyMiddleware_CreateRepository_NoPrivateParam_Blocked(t *testing.T) {
	inv := makeInventoryWithTool(t, "create_repository", false /* not readOnly */)
	deps := BaseDeps{Client: nil}

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "create_repository", map[string]any{
		"name": "my-new-repo",
		// "private" param intentionally absent
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err, "blocking should return nil Go error")
	assert.False(t, handler.called, "next handler should NOT be called when private param is absent")
	assertBlocked(t, result)
}

// Test 8: fork_repository is always blocked.
func TestWritePrivateOnlyMiddleware_ForkRepository_AlwaysBlocked(t *testing.T) {
	inv := makeInventoryWithTool(t, "fork_repository", false /* not readOnly */)
	deps := BaseDeps{Client: nil} // No API call expected

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "fork_repository", map[string]any{
		"owner": "someorg",
		"repo":  "somerepo",
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err, "blocking should return nil Go error")
	assert.False(t, handler.called, "next handler should NOT be called for fork_repository")
	assertBlocked(t, result)
}

// Test 9: Write tool without owner/repo (e.g., create_gist) passes through.
func TestWritePrivateOnlyMiddleware_WriteTool_NoOwnerRepo_PassThrough(t *testing.T) {
	inv := makeInventoryWithTool(t, "create_gist", false /* not readOnly */)
	deps := BaseDeps{Client: nil} // No API call expected — no owner/repo to check

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	req := makeWriteGuardRequest(t, "create_gist", map[string]any{
		"description": "My gist",
		"public":      false,
		"files":       map[string]any{"hello.txt": map[string]any{"content": "hello"}},
	})
	result, err := mwHandler(context.Background(), "tools/call", req)

	require.NoError(t, err)
	assertPassedThrough(t, handler)
	assert.Nil(t, result)
}

// Test 10: Non-tools/call method passes through unchanged.
func TestWritePrivateOnlyMiddleware_NonToolsCall_PassThrough(t *testing.T) {
	inv := makeEmptyInventory(t)
	deps := BaseDeps{Client: nil}

	handler := &passthroughHandler{}
	middleware := WritePrivateOnlyMiddleware(deps, inv)
	mwHandler := middleware(handler.handle)

	// Use tools/list — not a tool call, should pass through.
	result, err := mwHandler(context.Background(), "tools/list", nil)

	require.NoError(t, err)
	assertPassedThrough(t, handler)
	assert.Nil(t, result)
}

// --- checkRepoVisibility unit tests ---

// TestCheckRepoVisibility_Private verifies that a private repo returns (true, nil).
func TestCheckRepoVisibility_Private(t *testing.T) {
	mockHTTP := NewMockedHTTPClient(
		WithRequestMatch(
			EndpointPattern(GetReposByOwnerByRepo),
			&gogithub.Repository{
				Private: gogithub.Ptr(true),
			},
		),
	)
	client := gogithub.NewClient(mockHTTP)

	isPrivate, err := checkRepoVisibility(context.Background(), client, "myorg", "myrepo")
	require.NoError(t, err)
	assert.True(t, isPrivate)
}

// TestCheckRepoVisibility_Public verifies that a public repo returns (false, nil).
func TestCheckRepoVisibility_Public(t *testing.T) {
	mockHTTP := NewMockedHTTPClient(
		WithRequestMatch(
			EndpointPattern(GetReposByOwnerByRepo),
			&gogithub.Repository{
				Private: gogithub.Ptr(false),
			},
		),
	)
	client := gogithub.NewClient(mockHTTP)

	isPrivate, err := checkRepoVisibility(context.Background(), client, "myorg", "publicrepo")
	require.NoError(t, err)
	assert.False(t, isPrivate)
}

// TestCheckRepoVisibility_APIError verifies that an API error returns (false, err).
func TestCheckRepoVisibility_APIError(t *testing.T) {
	mockHTTP := NewMockedHTTPClient(
		WithRequestMatchHandler(
			EndpointPattern(GetReposByOwnerByRepo),
			mockResponse(t, http.StatusNotFound, nil),
		),
	)
	client := gogithub.NewClient(mockHTTP)

	isPrivate, err := checkRepoVisibility(context.Background(), client, "myorg", "missingrepo")
	require.Error(t, err, "expected error on API failure")
	assert.False(t, isPrivate, "should return false on error (fail-closed)")
}
