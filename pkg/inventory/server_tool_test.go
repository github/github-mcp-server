package inventory

import (
	"context"
	"encoding/json"
	"testing"

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

func sampleTool(name string) mcp.Tool {
	return mcp.Tool{Name: name, InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)}
}

// TestNewServerTool covers the raw (non-generic) constructor, which stores the
// handler directly. NewServerToolWithContextHandler is already covered above.
func TestNewServerTool(t *testing.T) {
	ts := testToolsetMetadata("repos")
	st := NewServerTool(
		sampleTool("raw_tool"), ts,
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	)
	assert.Equal(t, "raw_tool", st.Tool.Name)
	assert.Equal(t, ts, st.Toolset)
	require.True(t, st.HasHandler())

	handler := st.Handler(nil)
	require.NotNil(t, handler)
	_, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)},
	})
	require.NoError(t, err)
}

func TestToolsetMetadataIcons(t *testing.T) {
	assert.Nil(t, ToolsetMetadata{Icon: ""}.Icons())

	icons := ToolsetMetadata{Icon: "repo"}.Icons()
	require.Len(t, icons, 2)
	for _, ic := range icons {
		assert.NotEmpty(t, ic.Source)
	}
}

func TestServerToolRegisterFunc(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	// A toolset with an icon exercises the icon-application path in RegisterFunc.
	toolset := ToolsetMetadata{ID: "repos", Description: "repos", Icon: "repo"}
	st := NewServerTool(
		sampleTool("registered_tool"), toolset,
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	)
	assert.NotPanics(t, func() { st.RegisterFunc(server, nil) })

	// A tool with no handler must panic on registration.
	nilHandlerTool := ServerTool{Tool: sampleTool("broken"), Toolset: toolset}
	assert.Panics(t, func() { nilHandlerTool.RegisterFunc(server, nil) })
}
