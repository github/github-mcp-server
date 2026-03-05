package ghmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Local test helpers (cannot use unexported helpers from pkg/github) ---

// makeTestCallToolRequest builds a *mcp.CallToolRequest with the given tool name and JSON arguments.
func makeTestCallToolRequest(t *testing.T, toolName string, args map[string]any) *mcp.CallToolRequest {
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

// makeTestInventoryWithTool creates a minimal inventory with one tool for testing.
// readOnly controls whether the tool has ReadOnlyHint=true.
func makeTestInventoryWithTool(t *testing.T, toolName string, readOnly bool) *inventory.Inventory {
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

// repoVisibilityRoundTripper is a minimal http.RoundTripper that responds to
// any request with a fixed repository visibility (private or public).
// Used to mock the GitHub API for write guard tests without importing
// the unexported test helpers from pkg/github.
type repoVisibilityRoundTripper struct {
	isPrivate bool
}

func (r *repoVisibilityRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	repo := &gogithub.Repository{
		Private: gogithub.Ptr(r.isPrivate),
	}
	body, _ := json.Marshal(repo)
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(http.StatusOK)
	_, _ = rec.Write(body)
	return rec.Result(), nil
}

// panicOnGetClientDeps is a ToolDependencies that panics if GetClient is called.
// Used in denylist-ordering tests to prove the denylist blocked before the write
// guard ever attempted to fetch a GitHub client.
type panicOnGetClientDeps struct {
	github.BaseDeps
}

func (d *panicOnGetClientDeps) GetClient(_ context.Context) (*gogithub.Client, error) {
	panic("GetClient must NOT be called when denylist blocks first — middleware ordering is broken")
}

// makeTestDepsWithVisibility returns a github.BaseDeps whose REST client will
// report the given repo visibility for any GET /repos/{owner}/{repo} request.
func makeTestDepsWithVisibility(isPrivate bool) github.BaseDeps {
	httpClient := &http.Client{Transport: &repoVisibilityRoundTripper{isPrivate: isPrivate}}
	client := gogithub.NewClient(httpClient)
	return github.BaseDeps{Client: client}
}

// buildMiddlewareChain composes the guard middleware in the same order as
// NewStdioMCPServer (server.go):
//
//	RepoDenylist → SearchDenylist → OwnerExtract → WriteGuard → handler
//
// Middleware is applied in reverse (innermost first) so the first entry in the
// slice becomes outermost (runs first per request), matching the semantics of
// AddReceivingMiddleware(m1, m2, m3).
func buildMiddlewareChain(
	denylist *github.RepoDenylist,
	deps github.ToolDependencies,
	inv *inventory.Inventory,
	writePrivateOnly bool,
	appAuthActive bool,
	handler mcp.MethodHandler,
) mcp.MethodHandler {
	// Collect middleware in outermost-first order (same as server.go).
	var middlewares []mcp.Middleware

	if denylist != nil && !denylist.IsEmpty() {
		middlewares = append(middlewares,
			github.RepoDenylistMiddleware(denylist),
			github.SearchDenylistMiddleware(denylist),
		)
	}

	if appAuthActive {
		middlewares = append(middlewares, github.OwnerExtractMiddleware())
	}

	if writePrivateOnly {
		middlewares = append(middlewares, github.WritePrivateOnlyMiddleware(deps, inv))
	}

	// Apply in reverse order so the first middleware in the slice is outermost.
	// Each middleware wraps the current chain, so applying in reverse gives
	// outermost-first execution.
	chain := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		chain = middlewares[i](chain)
	}
	return chain
}

// --- Integration tests ---

// TestMiddlewareChain_DenylistBlocksBeforeWriteGuard verifies that the denylist
// middleware runs BEFORE the write guard. If ordering is wrong, the write guard
// would call deps.GetClient for a denied repo — the panicOnGetClientDeps will
// catch that and fail the test loudly.
func TestMiddlewareChain_DenylistBlocksBeforeWriteGuard(t *testing.T) {
	denylist := github.NewRepoDenylist([]string{"denied-org/*"})
	inv := makeTestInventoryWithTool(t, "create_issue", false /* not readOnly */)

	// Deps that panics if GetClient is called — proves denylist ran first.
	deps := &panicOnGetClientDeps{}

	passthrough := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		t.Fatal("handler must NOT be called for a denied repo")
		return nil, nil
	}

	chain := buildMiddlewareChain(
		denylist,
		deps,
		inv,
		true, /* writePrivateOnly */
		true, /* appAuthActive */
		passthrough,
	)

	req := makeTestCallToolRequest(t, "create_issue", map[string]any{
		"owner": "denied-org",
		"repo":  "secret",
		"title": "Should be blocked",
	})

	result, err := chain(context.Background(), "tools/call", req)

	require.NoError(t, err, "denylist block should return nil Go error")
	toolResult, ok := result.(*mcp.CallToolResult)
	require.True(t, ok, "expected *mcp.CallToolResult, got %T", result)
	assert.True(t, toolResult.IsError, "denied repo should produce an error result")
	require.NotEmpty(t, toolResult.Content, "error result must have content")
	text, ok := toolResult.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent")
	assert.Contains(t, text.Text, "denied-org",
		"error message should identify the denied org")
}

// TestMiddlewareChain_WriteGuardBlocksPublicRepo verifies that a tool call
// targeting an allowed (non-denied) but public repository is blocked by the
// write guard when WritePrivateOnly=true.
func TestMiddlewareChain_WriteGuardBlocksPublicRepo(t *testing.T) {
	// No denylist — repo is allowed.
	denylist := github.NewRepoDenylist(nil)
	inv := makeTestInventoryWithTool(t, "create_issue", false /* not readOnly */)

	// Mock deps: repo is public (isPrivate=false).
	deps := makeTestDepsWithVisibility(false /* public */)

	handlerCalled := false
	passthrough := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		handlerCalled = true
		return nil, nil
	}

	chain := buildMiddlewareChain(
		denylist,
		deps,
		inv,
		true,  /* writePrivateOnly */
		false, /* appAuthActive — PAT mode, no owner extract needed */
		passthrough,
	)

	req := makeTestCallToolRequest(t, "create_issue", map[string]any{
		"owner": "allowed-org",
		"repo":  "public-repo",
		"title": "Bug report",
	})

	result, err := chain(context.Background(), "tools/call", req)

	require.NoError(t, err, "write guard block should return nil Go error")
	assert.False(t, handlerCalled, "handler must NOT be called for a public repo write")
	toolResult, ok := result.(*mcp.CallToolResult)
	require.True(t, ok, "expected *mcp.CallToolResult, got %T", result)
	assert.True(t, toolResult.IsError, "public repo write should produce an error result")
	require.NotEmpty(t, toolResult.Content)
	text, ok := toolResult.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent")
	assert.Contains(t, text.Text, "public",
		"error message should mention the public repo restriction")
}

// TestMiddlewareChain_AllowedPrivateRepoPassesThrough verifies that a tool call
// targeting an allowed, private repository passes through the full middleware
// chain (denylist + write guard) and reaches the handler.
func TestMiddlewareChain_AllowedPrivateRepoPassesThrough(t *testing.T) {
	// Denylist blocks "denied-org/*" but NOT "allowed-org".
	denylist := github.NewRepoDenylist([]string{"denied-org/*"})
	inv := makeTestInventoryWithTool(t, "create_issue", false /* not readOnly */)

	// Mock deps: repo is private.
	deps := makeTestDepsWithVisibility(true /* private */)

	handlerCalled := false
	passthrough := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		handlerCalled = true
		return nil, nil
	}

	chain := buildMiddlewareChain(
		denylist,
		deps,
		inv,
		true,  /* writePrivateOnly */
		false, /* appAuthActive — PAT mode */
		passthrough,
	)

	req := makeTestCallToolRequest(t, "create_issue", map[string]any{
		"owner": "allowed-org",
		"repo":  "private-repo",
		"title": "Feature request",
	})

	result, err := chain(context.Background(), "tools/call", req)

	require.NoError(t, err)
	assert.True(t, handlerCalled, "handler MUST be called for an allowed private repo")
	assert.Nil(t, result, "passthrough handler returns nil result")
}

// TestMiddlewareChain_OwnerExtractPopulatesContext verifies that OwnerExtractMiddleware
// stores the owner in context so downstream middleware (e.g., MultiOrgDeps.GetClient)
// can route to the correct org's GitHub App installation.
func TestMiddlewareChain_OwnerExtractPopulatesContext(t *testing.T) {
	denylist := github.NewRepoDenylist(nil) // no denylist
	inv := makeTestInventoryWithTool(t, "create_issue", false)

	// Deps with nil client — write guard is disabled so this won't be called.
	deps := github.BaseDeps{Client: nil}

	var capturedOwner string
	passthrough := func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		capturedOwner = github.OwnerFromContext(ctx)
		return nil, nil
	}

	// Build chain with appAuthActive=true so OwnerExtractMiddleware is included.
	// WritePrivateOnly=false so write guard doesn't block (no client available).
	chain := buildMiddlewareChain(
		denylist,
		deps,
		inv,
		false, /* writePrivateOnly — off so nil client doesn't block */
		true,  /* appAuthActive — enables OwnerExtractMiddleware */
		passthrough,
	)

	req := makeTestCallToolRequest(t, "create_issue", map[string]any{
		"owner": "my-org",
		"repo":  "my-repo",
		"title": "Test",
	})

	_, err := chain(context.Background(), "tools/call", req)

	require.NoError(t, err)
	assert.Equal(t, "my-org", capturedOwner,
		"OwnerExtractMiddleware should store the owner in context for multi-org routing")
}
