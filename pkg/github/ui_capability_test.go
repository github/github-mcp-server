package github

import (
	"context"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMCPRequestWithCapabilities(t *testing.T, caps *mcp.ClientCapabilities) mcp.CallToolRequest {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	st, _ := mcp.NewInMemoryTransports()
	session, err := srv.Connect(context.Background(), st, &mcp.ServerSessionOptions{
		State: &mcp.ServerSessionState{
			InitializeParams: &mcp.InitializeParams{
				ClientInfo:   &mcp.Implementation{Name: "test-client"},
				Capabilities: caps,
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return mcp.CallToolRequest{Session: session}
}

func Test_clientSupportsUI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("client with UI extension", func(t *testing.T) {
		caps := &mcp.ClientCapabilities{}
		caps.AddExtension("io.modelcontextprotocol/ui", map[string]any{
			"mimeTypes": []string{"text/html;profile=mcp-app"},
		})
		req := createMCPRequestWithCapabilities(t, caps)
		assert.True(t, clientSupportsUI(ctx, &req))
	})

	t.Run("client without UI extension", func(t *testing.T) {
		req := createMCPRequestWithCapabilities(t, &mcp.ClientCapabilities{})
		assert.False(t, clientSupportsUI(ctx, &req))
	})

	t.Run("client with nil capabilities", func(t *testing.T) {
		req := createMCPRequestWithCapabilities(t, nil)
		assert.False(t, clientSupportsUI(ctx, &req))
	})

	t.Run("nil request", func(t *testing.T) {
		assert.False(t, clientSupportsUI(ctx, nil))
	})

	t.Run("nil session", func(t *testing.T) {
		req := createMCPRequest(nil)
		assert.False(t, clientSupportsUI(ctx, &req))
	})
}

func Test_clientSupportsUI_fromContext(t *testing.T) {
	t.Parallel()

	t.Run("UI supported in context", func(t *testing.T) {
		ctx := ghcontext.WithUISupport(context.Background(), true)
		assert.True(t, clientSupportsUI(ctx, nil))
	})

	t.Run("UI not supported in context", func(t *testing.T) {
		ctx := ghcontext.WithUISupport(context.Background(), false)
		assert.False(t, clientSupportsUI(ctx, nil))
	})

	t.Run("context takes precedence over session", func(t *testing.T) {
		ctx := ghcontext.WithUISupport(context.Background(), false)
		caps := &mcp.ClientCapabilities{}
		caps.AddExtension("io.modelcontextprotocol/ui", map[string]any{})
		req := createMCPRequestWithCapabilities(t, caps)
		assert.False(t, clientSupportsUI(ctx, &req))
	})

	t.Run("no context or session", func(t *testing.T) {
		assert.False(t, clientSupportsUI(context.Background(), nil))
	})
}
