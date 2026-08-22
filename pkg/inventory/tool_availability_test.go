package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolAvailability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		protocolVersion string
		capabilities    *mcp.ClientCapabilities
		wantTool        bool
		wantError       string
	}{
		{
			name:            "current protocol and form elicitation list and call tool",
			protocolVersion: ProtocolVersionMultiRoundTrip,
			capabilities: &mcp.ClientCapabilities{
				Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}},
			},
			wantTool: true,
		},
		{
			name:            "empty elicitation capability implies form support",
			protocolVersion: ProtocolVersionMultiRoundTrip,
			capabilities: &mcp.ClientCapabilities{
				Elicitation: &mcp.ElicitationCapabilities{},
			},
			wantTool: true,
		},
		{
			name:            "URL-only elicitation hides and refuses form tool",
			protocolVersion: ProtocolVersionMultiRoundTrip,
			capabilities: &mcp.ClientCapabilities{
				Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}},
			},
			wantError: "requires client support for form elicitation",
		},
		{
			name:            "missing elicitation capability hides and refuses form tool",
			protocolVersion: ProtocolVersionMultiRoundTrip,
			capabilities:    &mcp.ClientCapabilities{},
			wantError:       "requires client support for form elicitation",
		},
		{
			name:            "legacy protocol hides and refuses versioned tool",
			protocolVersion: "2025-11-25",
			capabilities: &mcp.ClientCapabilities{
				Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}},
			},
			wantTool:  false,
			wantError: "requires MCP protocol version 2026-07-28 or later",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var restrictedToolCalls int
			tools := []ServerTool{
				availabilityTestTool("always_available", "", "", nil),
				availabilityTestTool("restricted", ProtocolVersionMultiRoundTrip, ElicitationModeForm, func() {
					restrictedToolCalls++
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

			client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, &mcp.ClientOptions{
				Capabilities: tt.capabilities,
			})
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
			if tt.wantTool {
				assert.Contains(t, toolNames, "restricted")
			} else {
				assert.NotContains(t, toolNames, "restricted")
			}

			callResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "restricted"})
			require.NoError(t, err)
			if tt.wantTool {
				assert.False(t, callResult.IsError)
				assert.Equal(t, 1, restrictedToolCalls)
			} else {
				assert.True(t, callResult.IsError)
				assert.Zero(t, restrictedToolCalls)
				require.Len(t, callResult.Content, 1)
				text, ok := callResult.Content[0].(*mcp.TextContent)
				require.True(t, ok)
				assert.Contains(t, text.Text, tt.wantError)
				if tt.protocolVersion >= ProtocolVersionMultiRoundTrip {
					encoded, err := json.Marshal(callResult)
					require.NoError(t, err)
					assert.Contains(t, string(encoded), `"resultType":"complete"`)
				}
			}
		})
	}
}

func availabilityTestTool(name, minimumProtocolVersion string, requiredElicitationMode ElicitationMode, onCall func()) ServerTool {
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
		MinimumProtocolVersion:  minimumProtocolVersion,
		RequiredElicitationMode: requiredElicitationMode,
	}
}
