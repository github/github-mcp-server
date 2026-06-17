package utils //nolint:revive // package name matches the package under test

import (
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestNewToolResultText(t *testing.T) {
	res := NewToolResultText("hello world")

	require.NotNil(t, res)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 1)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected content to be *mcp.TextContent")
	require.Equal(t, "hello world", tc.Text)
}

func TestNewToolResultError(t *testing.T) {
	res := NewToolResultError("something went wrong")

	require.NotNil(t, res)
	require.True(t, res.IsError)
	require.Len(t, res.Content, 1)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected content to be *mcp.TextContent")
	require.Equal(t, "something went wrong", tc.Text)
}

func TestNewToolResultErrorFromErr(t *testing.T) {
	res := NewToolResultErrorFromErr("failed to do the thing", errors.New("boom"))

	require.NotNil(t, res)
	require.True(t, res.IsError)
	require.Len(t, res.Content, 1)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected content to be *mcp.TextContent")
	require.Equal(t, "failed to do the thing: boom", tc.Text)
}

func TestNewToolResultResource(t *testing.T) {
	contents := &mcp.ResourceContents{
		URI:      "repo://owner/repo/contents/file.txt",
		Text:     "file body",
		MIMEType: "text/plain",
	}

	res := NewToolResultResource("downloaded file", contents)

	require.NotNil(t, res)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 2)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected first content to be *mcp.TextContent")
	require.Equal(t, "downloaded file", tc.Text)

	embedded, ok := res.Content[1].(*mcp.EmbeddedResource)
	require.True(t, ok, "expected second content to be *mcp.EmbeddedResource")
	require.Same(t, contents, embedded.Resource)
}
