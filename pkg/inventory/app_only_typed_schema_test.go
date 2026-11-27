package inventory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTypedAppOnlyToolOmitsInferredHeaderAnnotations pins the header contract
// for app-only tools whose input schema is INFERRED from the handler's input
// type, rather than declared explicitly as ui_get does. The two tools below
// share one input type: the inferred schema is cached per type, so the
// annotation has to be selected per tool visibility when it is read out of the
// cache. Annotating the app-only one makes every view call fail with -32020 in
// hosts that do not project headers for proxied calls (issue #3834).
func TestTypedAppOnlyToolOmitsInferredHeaderAnnotations(t *testing.T) {
	handler := func(_ context.Context, _ *mcp.CallToolRequest, input typedTestInput) (*mcp.CallToolResult, typedTestOutput, error) {
		return nil, typedTestOutput{Query: input.Query}, nil
	}

	appOnlyTool := NewServerToolWithContextHandler(
		mcp.Tool{
			Name: "typed_app_only_tool",
			Meta: mcp.Meta{
				"ui": map[string]any{
					"visibility": []string{"app"},
				},
			},
		},
		testToolsetMetadata("test"),
		handler,
	)
	require.Nil(t, appOnlyTool.Tool.InputSchema, "the schema under test must be inferred")
	require.True(t, IsAppOnlyTool(&appOnlyTool.Tool))

	modelTool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_model_tool"},
		testToolsetMetadata("test"),
		handler,
	)

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	modelTool.RegisterFunc(server, nil)
	appOnlyTool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 2)

	schemas := make(map[string]map[string]any, len(list.Tools))
	for _, tool := range list.Tools {
		var schema map[string]any
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.InputSchema)), &schema))
		schemas[tool.Name] = schema
	}

	appOnlyProperties := schemas["typed_app_only_tool"]["properties"].(map[string]any)
	assert.NotContains(t, mustMarshalJSON(t, appOnlyProperties["owner"]), "x-mcp-header",
		"an app-only tool must not project owner onto Mcp-Param-owner")
	assert.NotContains(t, mustMarshalJSON(t, appOnlyProperties["repo"]), "x-mcp-header",
		"an app-only tool must not project repo onto Mcp-Param-repo")

	modelProperties := schemas["typed_model_tool"]["properties"].(map[string]any)
	assert.Equal(t, "owner", modelProperties["owner"].(map[string]any)["x-mcp-header"],
		"the model-visible tool that shares the input type keeps its routing metadata")
	assert.Equal(t, "repo", modelProperties["repo"].(map[string]any)["x-mcp-header"])

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "typed_app_only_tool",
		Arguments: map[string]any{"owner": "octo", "repo": "hello", "query": "is:open"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"query":"is:open"}`, mustMarshalJSON(t, result.StructuredContent),
		"owner/repo stay in the call arguments even without the header projection")
}
