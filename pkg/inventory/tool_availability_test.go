package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
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

func TestToolCallAvailabilitySkipsFeatureChecksOnlyWhenUnavailable(t *testing.T) {
	tests := []struct {
		name                    string
		minimumProtocolVersion  string
		requiredElicitationMode ElicitationMode
		methodInfo              *ghcontext.MCPMethodInfo
		clientCapabilities      *mcp.ClientCapabilities
		legacyProtocol          bool
		wantCheckerCalls        int
		wantHandlerCalls        int
		wantError               string
	}{
		{
			name:                   "protocol unavailable",
			minimumProtocolVersion: ProtocolVersionMultiRoundTrip,
			methodInfo: &ghcontext.MCPMethodInfo{
				Method:             MCPMethodToolsCall,
				ProtocolVersion:    "2025-11-25",
				ClientCapabilities: &mcp.ClientCapabilities{},
			},
			legacyProtocol:   true,
			wantError:        `Tool "restricted" requires MCP protocol version 2026-07-28 or later.`,
			wantCheckerCalls: 0,
		},
		{
			name:                    "elicitation unavailable",
			requiredElicitationMode: ElicitationModeForm,
			methodInfo: &ghcontext.MCPMethodInfo{
				Method:          MCPMethodToolsCall,
				ProtocolVersion: ProtocolVersionMultiRoundTrip,
				ClientCapabilities: &mcp.ClientCapabilities{
					Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}},
				},
			},
			clientCapabilities: &mcp.ClientCapabilities{
				Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}},
			},
			wantError:        `Tool "restricted" requires client support for form elicitation.`,
			wantCheckerCalls: 0,
		},
		{
			name:                    "eligible",
			minimumProtocolVersion:  ProtocolVersionMultiRoundTrip,
			requiredElicitationMode: ElicitationModeForm,
			methodInfo: &ghcontext.MCPMethodInfo{
				Method:          MCPMethodToolsCall,
				ProtocolVersion: ProtocolVersionMultiRoundTrip,
				ClientCapabilities: &mcp.ClientCapabilities{
					Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}},
				},
			},
			clientCapabilities: &mcp.ClientCapabilities{
				Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}},
			},
			wantCheckerCalls: 1,
			wantHandlerCalls: 1,
		},
		{
			name:                    "unknown metadata defers",
			minimumProtocolVersion:  ProtocolVersionMultiRoundTrip,
			requiredElicitationMode: ElicitationModeForm,
			methodInfo:              &ghcontext.MCPMethodInfo{Method: MCPMethodToolsCall},
			clientCapabilities: &mcp.ClientCapabilities{
				Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}},
			},
			wantCheckerCalls: 1,
			wantHandlerCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkerCalls := 0
			handlerCalls := 0
			tool := availabilityTestTool(
				"restricted",
				tt.minimumProtocolVersion,
				tt.requiredElicitationMode,
				func() { handlerCalls++ },
			)
			if tt.wantError != "" {
				tool.Tool.Meta = map[string]any{"ui": map[string]any{"resourceUri": "ui://restricted"}}
			}
			tool.FeatureRule = NewFeatureRule([]FeatureFlag{"feature"}, func(featureAsBool FeatureResolver) bool {
				return featureAsBool("feature")
			})
			inv, err := NewBuilder().
				SetTools([]ServerTool{tool}).
				WithToolsets([]string{"all"}).
				WithFeatureChecker(func(context.Context, string) (bool, error) {
					time.Sleep(time.Millisecond)
					checkerCalls++
					return true, nil
				}).
				Build()
			require.NoError(t, err)

			ctx := ghcontext.WithMCPMethodInfo(context.Background(), tt.methodInfo)
			server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
			inv.ForMCPRequest(MCPMethodToolsCall, "restricted").RegisterTools(ctx, server, nil)
			require.Equal(t, tt.wantCheckerCalls, checkerCalls)
			if tt.legacyProtocol {
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
				Capabilities: tt.clientCapabilities,
			})
			clientSession, err := client.Connect(context.Background(), clientTransport, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = clientSession.Close() })

			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "restricted"})
			require.NoError(t, err)
			require.Equal(t, tt.wantCheckerCalls, checkerCalls)
			require.Equal(t, tt.wantHandlerCalls, handlerCalls)
			if tt.wantError == "" {
				assert.False(t, result.IsError)
				return
			}
			require.True(t, result.IsError)
			require.Len(t, result.Content, 1)
			content, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Equal(t, tt.wantError, content.Text)
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
