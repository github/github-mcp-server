package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test helpers ---

// makeDenylistToolRequest builds a *mcp.CallToolRequest with the given tool name and JSON arguments.
func makeDenylistToolRequest(t *testing.T, toolName string, args map[string]any) *mcp.CallToolRequest {
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

// denylistPassthroughHandler is a MethodHandler that records whether it was called.
type denylistPassthroughHandler struct {
	called bool
}

func (h *denylistPassthroughHandler) handle(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
	h.called = true
	return nil, nil
}

// isBlockedResult returns true if the result is a CallToolResult with IsError=true.
func isBlockedResult(t *testing.T, result mcp.Result) bool {
	t.Helper()
	if result == nil {
		return false
	}
	toolResult, ok := result.(*mcp.CallToolResult)
	if !ok {
		return false
	}
	return toolResult.IsError
}

// --- RepoDenylistMiddleware tests ---

// Test 1: Tool with denied owner/repo → blocked
func TestRepoDenylistMiddleware_DeniedRepo_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/denied-repo"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "get_file_contents", map[string]any{
		"owner": "owner",
		"repo":  "denied-repo",
		"path":  "README.md",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result), "expected blocked result for denied repo")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test 2: Tool with allowed owner/repo → passes through
func TestRepoDenylistMiddleware_AllowedRepo_PassesThrough(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/denied-repo"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "get_file_contents", map[string]any{
		"owner": "owner",
		"repo":  "allowed-repo",
		"path":  "README.md",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.False(t, isBlockedResult(t, result), "expected pass-through for allowed repo")
	assert.True(t, handler.called, "handler should have been called")
}

// Test 3: Tool with no owner/repo → passes through
func TestRepoDenylistMiddleware_NoOwnerRepo_PassesThrough(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/denied-repo"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "list_notifications", map[string]any{
		"all": true,
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.False(t, isBlockedResult(t, result), "expected pass-through when no owner/repo")
	assert.True(t, handler.called, "handler should have been called")
}

// Test 9: Nil denylist → passes through
func TestRepoDenylistMiddleware_NilDenylist_PassesThrough(t *testing.T) {
	mw := RepoDenylistMiddleware(nil)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "get_file_contents", map[string]any{
		"owner": "owner",
		"repo":  "any-repo",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.False(t, isBlockedResult(t, result), "expected pass-through for nil denylist")
	assert.True(t, handler.called, "handler should have been called")
}

// Test 10: Non-tools/call method → passes through
func TestRepoDenylistMiddleware_NonToolsCall_PassesThrough(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/denied-repo"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	// Use a tools/list request (not tools/call) — no CallToolRequest to parse.
	req := &mcp.ListToolsRequest{}

	result, err := wrapped(context.Background(), "tools/list", req)
	require.NoError(t, err)
	assert.False(t, isBlockedResult(t, result), "expected pass-through for non-tools/call method")
	assert.True(t, handler.called, "handler should have been called")
}

// --- SearchDenylistMiddleware tests ---

// Test 4: Search with denied repo: qualifier → blocked
func TestSearchDenylistMiddleware_DeniedRepoQualifier_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/secret-repo"})
	mw := SearchDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "search_code", map[string]any{
		"query": "repo:owner/secret-repo some function",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result), "expected blocked result for denied repo: qualifier")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test 5: Search with denied org: qualifier → blocked
func TestSearchDenylistMiddleware_DeniedOrgQualifier_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"secret-org/*"})
	mw := SearchDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "search_repositories", map[string]any{
		"query": "org:secret-org topic:golang",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result), "expected blocked result for denied org: qualifier")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test 6: Search with allowed query → passes through
func TestSearchDenylistMiddleware_AllowedQuery_PassesThrough(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/secret-repo"})
	mw := SearchDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "search_code", map[string]any{
		"query": "repo:owner/public-repo some function",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.False(t, isBlockedResult(t, result), "expected pass-through for allowed query")
	assert.True(t, handler.called, "handler should have been called")
}

// Test 7: Search with denied owner+repo args (not in query) → blocked
func TestSearchDenylistMiddleware_DeniedOwnerRepoArgs_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/secret-repo"})
	mw := SearchDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "search_issues", map[string]any{
		"owner": "owner",
		"repo":  "secret-repo",
		"query": "is:open label:bug",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result), "expected blocked result for denied owner/repo args")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test 8: Multi-qualifier bypass: "repo:allowed/repo repo:denied/repo" → blocked
func TestSearchDenylistMiddleware_MultiQualifierBypass_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/denied-repo"})
	mw := SearchDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "search_code", map[string]any{
		"query": "repo:owner/allowed-repo repo:owner/denied-repo some function",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result), "expected blocked result when any repo: qualifier is denied")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test: Non-search tool is not intercepted by SearchDenylistMiddleware
func TestSearchDenylistMiddleware_NonSearchTool_PassesThrough(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/secret-repo"})
	mw := SearchDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	// get_file_contents is not a search tool — should pass through even with a denied repo
	req := makeDenylistToolRequest(t, "get_file_contents", map[string]any{
		"owner": "owner",
		"repo":  "secret-repo",
		"path":  "README.md",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.False(t, isBlockedResult(t, result), "SearchDenylistMiddleware should not intercept non-search tools")
	assert.True(t, handler.called, "handler should have been called")
}

// --- DenylistResourceHandler tests ---

// Test 11: Resource handler with denied URI → blocked
func TestDenylistResourceHandler_DeniedURI_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/secret-repo"})

	innerCalled := false
	inner := mcp.ResourceHandler(func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		innerCalled = true
		return &mcp.ReadResourceResult{}, nil
	})

	wrapped := DenylistResourceHandler(denylist, inner)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "repo://owner/secret-repo/contents/README.md",
		},
	}

	result, err := wrapped(context.Background(), req)
	assert.Error(t, err, "expected error for denied URI")
	assert.Nil(t, result, "expected nil result for denied URI")
	assert.False(t, innerCalled, "inner handler should not have been called")
}

// Test 12: Resource handler with allowed URI → passes through
func TestDenylistResourceHandler_AllowedURI_PassesThrough(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/secret-repo"})

	innerCalled := false
	inner := mcp.ResourceHandler(func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		innerCalled = true
		return &mcp.ReadResourceResult{}, nil
	})

	wrapped := DenylistResourceHandler(denylist, inner)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "repo://owner/public-repo/contents/README.md",
		},
	}

	result, err := wrapped(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, result, "expected result for allowed URI")
	assert.True(t, innerCalled, "inner handler should have been called")
}

// Test: Nil denylist resource handler → passes through
func TestDenylistResourceHandler_NilDenylist_PassesThrough(t *testing.T) {
	innerCalled := false
	inner := mcp.ResourceHandler(func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		innerCalled = true
		return &mcp.ReadResourceResult{}, nil
	})

	wrapped := DenylistResourceHandler(nil, inner)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "repo://owner/any-repo/contents/README.md",
		},
	}

	result, err := wrapped(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, innerCalled, "inner handler should have been called for nil denylist")
}

// Test: Resource handler with unparseable URI → passes through
func TestDenylistResourceHandler_UnparseableURI_PassesThrough(t *testing.T) {
	denylist := NewRepoDenylist([]string{"owner/secret-repo"})

	innerCalled := false
	inner := mcp.ResourceHandler(func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		innerCalled = true
		return &mcp.ReadResourceResult{}, nil
	})

	wrapped := DenylistResourceHandler(denylist, inner)

	req := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "https://example.com/not-a-repo-uri",
		},
	}

	result, err := wrapped(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, innerCalled, "inner handler should have been called for unparseable URI")
}

// Test: Org wildcard denylist blocks all repos in org
func TestRepoDenylistMiddleware_OrgWildcard_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"secret-org/*"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "get_file_contents", map[string]any{
		"owner": "secret-org",
		"repo":  "any-repo-in-org",
		"path":  "README.md",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result), "expected blocked result for org wildcard denylist")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test: create_repository uses "organization"+"name" instead of "owner"+"repo"
func TestRepoDenylistMiddleware_CreateRepository_OrgNameBlocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"denied-org/*"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	// create_repository uses "organization" and "name", not "owner" and "repo"
	req := makeDenylistToolRequest(t, "create_repository", map[string]any{
		"organization": "denied-org",
		"name":         "new-repo",
		"private":      true,
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result),
		"create_repository with denied organization should be blocked")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test: create_repository with allowed org passes through
func TestRepoDenylistMiddleware_CreateRepository_AllowedOrg(t *testing.T) {
	denylist := NewRepoDenylist([]string{"denied-org/*"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "create_repository", map[string]any{
		"organization": "allowed-org",
		"name":         "new-repo",
		"private":      true,
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.False(t, isBlockedResult(t, result),
		"create_repository with allowed organization should pass through")
	assert.True(t, handler.called, "handler should have been called")
}

// Test: create_repository exact repo match (organization + name)
func TestRepoDenylistMiddleware_CreateRepository_ExactRepoBlocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"my-org/forbidden-repo"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "create_repository", map[string]any{
		"organization": "my-org",
		"name":         "forbidden-repo",
		"private":      true,
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result),
		"create_repository with exact denied org/name should be blocked")
	assert.False(t, handler.called, "handler should not have been called")
}

// Test: Case-insensitive denylist matching
func TestRepoDenylistMiddleware_CaseInsensitive_Blocked(t *testing.T) {
	denylist := NewRepoDenylist([]string{"Owner/Secret-Repo"})
	mw := RepoDenylistMiddleware(denylist)

	handler := &denylistPassthroughHandler{}
	wrapped := mw(handler.handle)

	req := makeDenylistToolRequest(t, "get_file_contents", map[string]any{
		"owner": "OWNER",
		"repo":  "SECRET-REPO",
		"path":  "README.md",
	})

	result, err := wrapped(context.Background(), "tools/call", req)
	require.NoError(t, err)
	assert.True(t, isBlockedResult(t, result), "expected blocked result for case-insensitive match")
	assert.False(t, handler.called, "handler should not have been called")
}
