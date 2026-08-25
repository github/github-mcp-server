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

func TestNewServerToolWithContextHandler_Arguments(t *testing.T) {
	tests := []struct {
		name              string
		arguments         json.RawMessage
		requireQuery      bool
		wantHandlerCalled bool
		wantIsError       bool
		wantText          string
	}{
		{
			name:              "omitted arguments",
			arguments:         nil,
			wantHandlerCalled: true,
			wantText:          "success",
		},
		{
			name:              "empty argument bytes",
			arguments:         json.RawMessage{},
			wantHandlerCalled: true,
			wantText:          "success",
		},
		{
			name:              "explicit empty object",
			arguments:         json.RawMessage(`{}`),
			wantHandlerCalled: true,
			wantText:          "success",
		},
		{
			name:        "explicit null",
			arguments:   json.RawMessage(`null`),
			wantIsError: true,
			wantText:    "arguments must be a JSON object",
		},
		{
			name:        "malformed JSON",
			arguments:   json.RawMessage(`{not valid json`),
			wantIsError: true,
			wantText:    "invalid arguments",
		},
		{
			name:              "omitted arguments reach required parameter validation",
			arguments:         nil,
			requireQuery:      true,
			wantHandlerCalled: true,
			wantIsError:       true,
			wantText:          "missing required parameter: query",
		},
		{
			name:              "required parameter is decoded",
			arguments:         json.RawMessage(`{"query":"is:open"}`),
			requireQuery:      true,
			wantHandlerCalled: true,
			wantText:          "success: is:open",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			tool := NewServerToolWithContextHandler(
				mcp.Tool{Name: "test_context_tool"},
				testToolsetMetadata("test"),
				func(_ context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
					handlerCalled = true
					query, _ := args["query"].(string)
					if tc.requireQuery && query == "" {
						return &mcp.CallToolResult{
							Content: []mcp.Content{
								&mcp.TextContent{Text: "missing required parameter: query"},
							},
							IsError: true,
						}, nil, nil
					}
					text := "success"
					if query != "" {
						text += ": " + query
					}
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: text},
						},
					}, nil, nil
				},
			)

			result, err := tool.HandlerFunc(nil)(context.Background(), &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name:      "test_context_tool",
					Arguments: tc.arguments,
				},
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.wantHandlerCalled, handlerCalled)
			assert.Equal(t, tc.wantIsError, result.IsError)
			require.Len(t, result.Content, 1)
			textContent, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Contains(t, textContent.Text, tc.wantText)
		})
	}
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
