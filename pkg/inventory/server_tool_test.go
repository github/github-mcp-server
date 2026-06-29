package inventory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerToolWithContextHandler_InvalidArguments_ReturnsIsError(t *testing.T) {
	type expectedArgs struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}

	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "test_context_tool"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, _ expectedArgs) (*mcp.CallToolResult, any, error) {
			t.Fatal("handler should not be called with invalid arguments")
			return nil, nil, nil
		},
	)

	handler := tool.HandlerFunc(nil)

	result, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "test_context_tool",
			Arguments: json.RawMessage(`{not valid json`),
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "invalid arguments")
}

func TestNewServerToolWithContextHandler_ValidArguments_Succeeds(t *testing.T) {
	type expectedArgs struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	}

	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "test_tool"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, args expectedArgs) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "success: " + args.Owner + "/" + args.Repo},
				},
			}, nil, nil
		},
	)

	handler := tool.HandlerFunc(nil)

	goodArgs, _ := json.Marshal(map[string]any{"owner": "octocat", "repo": "hello-world"})
	result, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "test_tool",
			Arguments: goodArgs,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "success: octocat/hello-world", textContent.Text)
}

func TestAnnotateHeaderParams(t *testing.T) {
	tool := &mcp.Tool{InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner":  {Type: "string"},
			"repo":   {Type: "string"},
			"detail": {Type: "string"},
		},
	}}
	annotateHeaderParams(tool)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Equal(t, "owner", schema.Properties["owner"].Extra["x-mcp-header"])
	assert.Equal(t, "repo", schema.Properties["repo"].Extra["x-mcp-header"])
	assert.Nil(t, schema.Properties["detail"].Extra)

	// No-op for tools without owner/repo and when InputSchema is not a *jsonschema.Schema
	annotateHeaderParams(&mcp.Tool{InputSchema: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"x": {}}}})
	annotateHeaderParams(&mcp.Tool{InputSchema: json.RawMessage(`{}`)})
}
