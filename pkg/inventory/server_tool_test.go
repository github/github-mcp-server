package inventory

import (
	"context"
	"encoding/json"
	"sync"
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

func TestServerToolRegisterFuncAppliesMiddleware(t *testing.T) {
	tool := NewServerTool(
		mcp.Tool{
			Name:        "wrapped_tool",
			InputSchema: &jsonschema.Schema{Type: "object"},
		},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "handler"}},
			}, nil
		},
	)

	middlewareCalled := make(chan struct{}, 1)
	middleware := func(next mcp.ToolHandler) mcp.ToolHandler {
		return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			middlewareCalled <- struct{}{}
			return next(ctx, req)
		}
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil, middleware)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "wrapped_tool"})
	require.NoError(t, err)
	select {
	case <-middlewareCalled:
	default:
		t.Fatal("tool middleware was not called")
	}
	require.Len(t, result.Content, 1)
	assert.Equal(t, "handler", result.Content[0].(*mcp.TextContent).Text)
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
	AnnotateHeaderParams(tool)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Equal(t, "owner", schema.Properties["owner"].Extra["x-mcp-header"])
	assert.Equal(t, "repo", schema.Properties["repo"].Extra["x-mcp-header"])
	assert.Nil(t, schema.Properties["detail"].Extra)

	// No-op for tools without owner/repo and when InputSchema is not a *jsonschema.Schema
	AnnotateHeaderParams(&mcp.Tool{InputSchema: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"x": {}}}})
	AnnotateHeaderParams(&mcp.Tool{InputSchema: json.RawMessage(`{}`)})
}

func TestAnnotateHeaderParams_DoesNotMutateOriginal(t *testing.T) {
	orig := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{"owner": {Type: "string"}, "repo": {Type: "string"}},
	}
	tool := &mcp.Tool{InputSchema: orig}
	AnnotateHeaderParams(tool)

	// Original schema and its property Extra maps must be untouched.
	require.Nil(t, orig.Properties["owner"].Extra, "must not mutate original owner schema")
	require.Nil(t, orig.Properties["repo"].Extra, "must not mutate original repo schema")
	// Returned copy carries the annotation.
	got := tool.InputSchema.(*jsonschema.Schema)
	require.NotSame(t, orig, got, "must replace InputSchema with a copy")
	require.Equal(t, "owner", got.Properties["owner"].Extra["x-mcp-header"])
}

func TestAnnotateHeaderParams_ConcurrentRegistrationIsRaceFree(t *testing.T) {
	// Shared base schema, as ServerTool.Tool is shallow-copied per registration.
	base := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{"owner": {Type: "string"}, "repo": {Type: "string"}},
	}
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			tool := mcp.Tool{InputSchema: base} // shallow copy shares *Schema
			AnnotateHeaderParams(&tool)
			got := tool.InputSchema.(*jsonschema.Schema)
			require.Equal(t, "repo", got.Properties["repo"].Extra["x-mcp-header"])
		})
	}
	wg.Wait()
	require.Nil(t, base.Properties["owner"].Extra, "shared base must remain unmutated")
}
