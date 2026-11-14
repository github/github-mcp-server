package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
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

func TestProtocolVersionAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		protocolVersion string
		minimum         string
		want            bool
	}{
		{
			name:            "unrestricted tool",
			protocolVersion: "2024-11-05",
			want:            true,
		},
		{
			name:            "known older version cannot reach newer minimum",
			protocolVersion: "2025-06-18",
			minimum:         latestLegacyProtocolVersion,
			want:            false,
		},
		{
			name:            "known minimum reached",
			protocolVersion: ProtocolVersionMultiRoundTrip,
			minimum:         latestLegacyProtocolVersion,
			want:            true,
		},
		{
			name:            "unknown version is not treated as newer",
			protocolVersion: "zzz",
			minimum:         ProtocolVersionMultiRoundTrip,
			want:            false,
		},
		{
			name:            "unknown version still reaches legacy minimum",
			protocolVersion: "zzz",
			minimum:         "2025-06-18",
			want:            true,
		},
		{
			name:            "date-shaped unknown version is not treated as newer",
			protocolVersion: "2026-07-28-unknown",
			minimum:         ProtocolVersionMultiRoundTrip,
			want:            false,
		},
		{
			name:            "future-looking unknown version is not treated as newer",
			protocolVersion: "9999-12-31",
			minimum:         ProtocolVersionMultiRoundTrip,
			want:            false,
		},
		{
			name:            "unset version cannot reach a gated minimum",
			protocolVersion: "",
			minimum:         ProtocolVersionMultiRoundTrip,
			want:            false,
		},
		{
			name:            "unknown minimum keeps lexical comparison",
			protocolVersion: "2026-07-28",
			minimum:         "2026-01-01",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, protocolVersionAllowed(tt.protocolVersion, tt.minimum))
		})
	}
}

func TestProtocolVersionForTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		metaVersion    string
		sessionVersion string
		want           string
	}{
		{
			name:           "per-request version wins",
			metaVersion:    ProtocolVersionMultiRoundTrip,
			sessionVersion: latestLegacyProtocolVersion,
			want:           ProtocolVersionMultiRoundTrip,
		},
		{
			name:           "legacy session cannot negotiate multi-round-trip",
			sessionVersion: ProtocolVersionMultiRoundTrip,
			want:           latestLegacyProtocolVersion,
		},
		{
			name:           "unknown session version negotiates down to legacy",
			sessionVersion: "zzz",
			want:           latestLegacyProtocolVersion,
		},
		{
			name:           "known legacy session version is preserved",
			sessionVersion: "2025-06-18",
			want:           "2025-06-18",
		},
		{
			name: "unset versions stay unset",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, protocolVersionForTools(tt.metaVersion, tt.sessionVersion))
		})
	}
}

// TestToolAvailabilityUsesNegotiatedProtocolVersion emulates a client that
// requests a version the server cannot negotiate during initialize and sends no
// per-request protocolVersion metadata, as a raw JSON-RPC client does.
func TestToolAvailabilityUsesNegotiatedProtocolVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
	}{
		{
			name:      "multi-round-trip request is capped to the negotiated legacy version",
			requested: ProtocolVersionMultiRoundTrip,
		},
		{
			name:      "unknown request version is not treated as newer",
			requested: "zzz",
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
			server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
					if method == "server/discover" {
						return nil, errors.New("legacy server does not support discovery")
					}
					return next(ctx, method, request)
				}
			})

			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(context.Background(), serverTransport, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = serverSession.Close() })

			client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, &mcp.ClientOptions{
				Capabilities: &mcp.ClientCapabilities{
					Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}},
				},
			})
			clientSession, err := client.Connect(context.Background(), negotiatedVersionTransport{Transport: clientTransport, requested: tt.requested}, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = clientSession.Close() })
			assert.Equal(t, latestLegacyProtocolVersion, clientSession.InitializeResult().ProtocolVersion)

			listResult, err := clientSession.ListTools(context.Background(), nil)
			require.NoError(t, err)
			toolNames := make([]string, 0, len(listResult.Tools))
			for _, tool := range listResult.Tools {
				toolNames = append(toolNames, tool.Name)
			}
			assert.Contains(t, toolNames, "always_available")
			assert.NotContains(t, toolNames, "restricted")

			callResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "restricted"})
			require.NoError(t, err)
			assert.True(t, callResult.IsError)
			assert.Zero(t, restrictedToolCalls)
		})
	}
}

// negotiatedVersionTransport rewrites the initialize request to the requested
// version and strips per-request protocolVersion metadata, matching clients that
// never send _meta.
type negotiatedVersionTransport struct {
	mcp.Transport
	requested string
}

func (t negotiatedVersionTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	connection, err := t.Transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return negotiatedVersionConnection{Connection: connection, requested: t.requested}, nil
}

type negotiatedVersionConnection struct {
	mcp.Connection
	requested string
}

func (c negotiatedVersionConnection) Write(ctx context.Context, message jsonrpc.Message) error {
	req, ok := message.(*jsonrpc.Request)
	if !ok {
		return c.Connection.Write(ctx, message)
	}
	if len(req.Params) == 0 {
		return c.Connection.Write(ctx, message)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return err
	}
	if req.Method == "initialize" {
		version, err := json.Marshal(c.requested)
		if err != nil {
			return err
		}
		params["protocolVersion"] = version
	} else if rawMeta, ok := params["_meta"]; ok {
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(rawMeta, &meta); err != nil {
			return err
		}
		delete(meta, mcp.MetaKeyProtocolVersion)
		encodedMeta, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		params["_meta"] = encodedMeta
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	requestCopy := *req
	requestCopy.Params = encoded
	return c.Connection.Write(ctx, &requestCopy)
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
