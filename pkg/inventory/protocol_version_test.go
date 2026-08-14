package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolMinimumProtocolVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		protocolVersion   string
		wantVersionedTool bool
	}{
		{
			name:              "current protocol lists and calls versioned tool",
			protocolVersion:   ProtocolVersionMultiRoundTrip,
			wantVersionedTool: true,
		},
		{
			name:              "legacy protocol hides and refuses versioned tool",
			protocolVersion:   "2025-11-25",
			wantVersionedTool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var versionedToolCalls int
			tools := []ServerTool{
				protocolTestTool("always_available", "", nil),
				protocolTestTool("versioned", ProtocolVersionMultiRoundTrip, func() {
					versionedToolCalls++
				}),
			}
			inv, err := NewBuilder().
				SetTools(tools).
				WithToolsets([]string{"all"}).
				Build()
			require.NoError(t, err)

			server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
			inv.RegisterTools(context.Background(), server, nil)
			if tt.protocolVersion < ProtocolVersionMultiRoundTrip {
				server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
					return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
						if method == "server/discover" {
							return nil, errors.New("legacy server does not support discovery")
						}
						return next(ctx, method, request)
					}
				})
			}

			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(context.Background(), serverTransport, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = serverSession.Close() })

			client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
			clientSession, err := client.Connect(context.Background(), clientTransport, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = clientSession.Close() })

			listResult, err := clientSession.ListTools(context.Background(), nil)
			require.NoError(t, err)
			toolNames := make([]string, 0, len(listResult.Tools))
			for _, tool := range listResult.Tools {
				toolNames = append(toolNames, tool.Name)
			}
			assert.Contains(t, toolNames, "always_available")
			if tt.wantVersionedTool {
				assert.Contains(t, toolNames, "versioned")
			} else {
				assert.NotContains(t, toolNames, "versioned")
			}

			callResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "versioned"})
			require.NoError(t, err)
			if tt.wantVersionedTool {
				assert.False(t, callResult.IsError)
				assert.Equal(t, 1, versionedToolCalls)
			} else {
				assert.True(t, callResult.IsError)
				assert.Zero(t, versionedToolCalls)
			}
		})
	}
}

func protocolTestTool(name, minimumProtocolVersion string, onCall func()) ServerTool {
	return ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			InputSchema: &jsonschema.Schema{Type: "object"},
		},
		Toolset: ToolsetMetadata{ID: "test"},
		HandlerFunc: func(any) mcp.ToolHandler {
			return func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				if onCall != nil {
					onCall()
				}
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "called"}},
				}, nil
			}
		},
		MinimumProtocolVersion: minimumProtocolVersion,
	}
}
