package binding

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syntheticTool builds a small ServerTool whose handler records the arguments
// it ultimately receives, so tests can assert exactly what the binding wrapper
// injected, rejected, or passed through.
func syntheticTool(captured *map[string]any) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner":  {Type: "string"},
			"repo":   {Type: "string"},
			"query":  {Type: "string"},
			"body":   {Type: "string"},
			"method": {Type: "string", Enum: []any{"get", "create", "delete"}},
		},
		Required: []string{"owner", "repo", "method"},
	}
	return inventory.ServerTool{
		Tool: mcp.Tool{Name: "synthetic", Description: "original", InputSchema: schema},
		HandlerFunc: func(_ any) mcp.ToolHandler {
			return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				m := map[string]any{}
				if len(req.Params.Arguments) > 0 {
					if err := json.Unmarshal(req.Params.Arguments, &m); err != nil {
						return nil, err
					}
				}
				*captured = m
				return &mcp.CallToolResult{}, nil
			}
		},
	}
}

func repoCtx(t *testing.T) Context {
	t.Helper()
	c, err := NewRepoContext("octocat", "hello-world")
	require.NoError(t, err)
	return c
}

func callBound(t *testing.T, bound inventory.ServerTool, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	res, err := bound.HandlerFunc(nil)(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: raw},
	})
	require.NoError(t, err)
	return res
}

func TestBindToolPrunesSchemaAndKeepsSingletonIntact(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	orig := st.Tool.InputSchema.(*jsonschema.Schema)

	tb := ToolBinding{
		Bind:        bindRepo,
		MethodAllow: []string{"get", "create"},
		Description: "bespoke",
	}
	bound, err := bindTool(st, tb, repoCtx(t))
	require.NoError(t, err)

	got := bound.Tool.InputSchema.(*jsonschema.Schema)
	assert.NotContains(t, got.Properties, "owner", "bound owner must be removed from advertised schema")
	assert.NotContains(t, got.Properties, "repo", "bound repo must be removed from advertised schema")
	assert.NotContains(t, got.Required, "owner")
	assert.NotContains(t, got.Required, "repo")
	assert.Equal(t, []any{"get", "create"}, got.Properties["method"].Enum)
	assert.Equal(t, "bespoke", bound.Tool.Description)

	// The package-level singleton must be untouched: properties and the nested
	// method enum on the original schema are unchanged.
	assert.Contains(t, orig.Properties, "owner")
	assert.Contains(t, orig.Properties, "repo")
	assert.Equal(t, []any{"get", "create", "delete"}, orig.Properties["method"].Enum)
	assert.Equal(t, "original", st.Tool.Description)
}

func TestWrapHandlerInjectsBoundValues(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	bound, err := bindTool(st, ToolBinding{Bind: bindRepo, Description: "d"}, repoCtx(t))
	require.NoError(t, err)

	res := callBound(t, bound, map[string]any{"method": "get"})
	require.False(t, res.IsError)
	assert.Equal(t, "octocat", captured["owner"])
	assert.Equal(t, "hello-world", captured["repo"])
	assert.Equal(t, "get", captured["method"])
}

func TestWrapHandlerRejectsSuppliedBoundParam(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	bound, err := bindTool(st, ToolBinding{Bind: bindRepo, Description: "d"}, repoCtx(t))
	require.NoError(t, err)

	res := callBound(t, bound, map[string]any{"owner": "attacker", "method": "get"})
	require.True(t, res.IsError, "supplying a fixed parameter must be rejected")
	assert.Nil(t, captured, "handler must not run when a fixed parameter is supplied")
}

func TestWrapHandlerRejectsRejectedParam(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	bound, err := bindTool(st, ToolBinding{Bind: bindRepo, ParamReject: []string{"body"}, Description: "d"}, repoCtx(t))
	require.NoError(t, err)

	res := callBound(t, bound, map[string]any{"body": "x", "method": "get"})
	require.True(t, res.IsError, "supplying a rejected parameter must be rejected")
	assert.Nil(t, captured)
}

func TestWrapHandlerEnforcesMethodAllow(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	bound, err := bindTool(st, ToolBinding{Bind: bindRepo, MethodAllow: []string{"get"}, Description: "d"}, repoCtx(t))
	require.NoError(t, err)

	res := callBound(t, bound, map[string]any{"method": "create"})
	require.True(t, res.IsError, "a method outside the allow list must be rejected at runtime")
	assert.Nil(t, captured)

	captured = nil
	res = callBound(t, bound, map[string]any{"method": "get"})
	require.False(t, res.IsError)
	assert.Equal(t, "get", captured["method"])
}

func TestWrapHandlerEnforcesMethodDeny(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	bound, err := bindTool(st, ToolBinding{Bind: bindRepo, MethodDeny: []string{"delete"}, Description: "d"}, repoCtx(t))
	require.NoError(t, err)

	res := callBound(t, bound, map[string]any{"method": "delete"})
	require.True(t, res.IsError, "a denied method must be rejected at runtime")
	assert.Nil(t, captured)
}

func TestWrapHandlerQueryGuard(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	bound, err := bindTool(st, ToolBinding{Bind: bindRepo, QueryGuard: true, Description: "d"}, repoCtx(t))
	require.NoError(t, err)

	for _, q := range []string{"repo:other/x foo", "is:open ORG:evil", "-user:someone", "bug OR is:issue label:x", "(is:open)", "a AND b"} {
		captured = nil
		res := callBound(t, bound, map[string]any{"method": "get", "query": q})
		require.Truef(t, res.IsError, "query %q that can escape the bound scope must be rejected", q)
		assert.Nil(t, captured)
	}

	captured = nil
	res := callBound(t, bound, map[string]any{"method": "get", "query": "is:open label:bug"})
	require.False(t, res.IsError, "a query without cross-context qualifiers must pass")
	assert.Equal(t, "octocat", captured["owner"])
}

func TestWrapHandlerRequiresMethodWhenRestricted(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	bound, err := bindTool(st, ToolBinding{Bind: bindRepo, MethodAllow: []string{"get"}, Description: "d"}, repoCtx(t))
	require.NoError(t, err)

	// An omitted method must not fall through to the handler when the method is
	// restricted, even for a deny-only configuration where the empty string is
	// not explicitly denied.
	res := callBound(t, bound, map[string]any{})
	require.True(t, res.IsError, "an omitted method must be rejected when the method is restricted")
	assert.Nil(t, captured)
}

func TestBindToolErrorsOnUnknownRejectedParam(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	_, err := bindTool(st, ToolBinding{Bind: bindRepo, ParamReject: []string{"nonexistent"}, Description: "d"}, repoCtx(t))
	require.Error(t, err, "rejecting a parameter that is not in the schema must fail loudly")
}

func TestNarrowEnum(t *testing.T) {
	enum := []any{"get", "create", "delete"}

	got, err := narrowEnum(enum, []string{"create", "get"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"get", "create"}, got, "order follows the original enum, not the allow list")

	got, err = narrowEnum(enum, nil, []string{"delete"})
	require.NoError(t, err)
	assert.Equal(t, []any{"get", "create"}, got)

	got, err = narrowEnum(enum, []string{"get", "create"}, []string{"create"})
	require.NoError(t, err)
	assert.Equal(t, []any{"get"}, got)

	_, err = narrowEnum(enum, []string{"bogus"}, nil)
	require.Error(t, err, "an allow value absent from the enum must fail loudly")

	_, err = narrowEnum(enum, nil, []string{"bogus"})
	require.Error(t, err, "a deny value absent from the enum must fail loudly")

	_, err = narrowEnum(enum, []string{"get"}, []string{"get"})
	require.Error(t, err, "narrowing to an empty enum must fail")
}

func TestBindToolErrorsOnUnknownBoundParam(t *testing.T) {
	var captured map[string]any
	st := syntheticTool(&captured)
	_, err := bindTool(st, ToolBinding{Bind: map[string]ctxKey{"nonexistent": keyOwner}, Description: "d"}, repoCtx(t))
	require.Error(t, err, "binding a parameter that is not in the schema must fail")
}
