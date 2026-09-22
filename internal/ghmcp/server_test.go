package ghmcp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGitHubClientsGraphQLUserAgent(t *testing.T) {
	t.Parallel()
	for _, insiders := range []bool{false, true} {
		t.Run(map[bool]string{false: "release", true: "insiders"}[insiders], func(t *testing.T) {
			headers := make(chan http.Header, 1)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
			}))
			defer api.Close()
			cfg := github.MCPServerConfig{Version: "v1.2.3", Token: "fixture-token", InsidersMode: insiders}
			clients, err := createGitHubClients(cfg, newStaticAPIHostResolver(t, api.URL))
			require.NoError(t, err)
			var query struct {
				Viewer struct{ Login string }
			}
			require.NoError(t, clients.gql.Query(t.Context(), &query, nil))
			want := "github-mcp-server/v1.2.3"
			if insiders {
				want += " (insiders)"
			}
			header := <-headers
			assert.Equal(t, want, header.Get("User-Agent"))
			assert.Equal(t, "Bearer fixture-token", header.Get("Authorization"))
		})
	}
}

func TestStdioGraphQLUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handshake  string
		clientInfo *mcp.Implementation
		version    string
		insiders   bool
		want       string
	}{
		{
			name: "SDK discovery", handshake: "sdk", version: "v1.2.3",
			clientInfo: &mcp.Implementation{Name: "test-client", Version: "4.5.6"},
			want:       "github-mcp-server/v1.2.3 (test-client/4.5.6)",
		},
		{
			name: "direct modern call", version: "v1.2.3",
			clientInfo: &mcp.Implementation{Name: "test-client", Version: "4.5.6"},
			want:       "github-mcp-server/v1.2.3 (test-client/4.5.6)",
		},
		{
			name: "modern call without optional client info", version: "v1.2.3",
			want: "github-mcp-server/v1.2.3",
		},
		{
			name: "legacy initialize", handshake: "legacy", version: "v1.2.3",
			clientInfo: &mcp.Implementation{Name: "test-client", Version: "4.5.6"},
			want:       "github-mcp-server/v1.2.3 (test-client/4.5.6)",
		},
		{
			name: "insiders", version: "v1.2.3", insiders: true,
			clientInfo: &mcp.Implementation{Name: "test-client", Version: "4.5.6"},
			want:       "github-mcp-server/v1.2.3 (test-client/4.5.6) (insiders)",
		},
		{
			name: "different server build", version: "v1.2.4",
			clientInfo: &mcp.Implementation{Name: "test-client", Version: "4.5.6"},
			want:       "github-mcp-server/v1.2.4 (test-client/4.5.6)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			headers := make(chan http.Header, 2)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"repository":{"isPrivate":false,"issues":{"nodes":[],"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`))
			}))
			defer api.Close()

			cfg := github.MCPServerConfig{
				Version: tt.version, Token: "fixture-token", InsidersMode: tt.insiders,
				Translator: translations.NullTranslationHelper, Logger: discardLogger(),
			}
			clients, err := createGitHubClients(cfg, newStaticAPIHostResolver(t, api.URL))
			require.NoError(t, err)
			obs, err := observability.NewExporters(cfg.Logger, metrics.NewNoopMetrics())
			require.NoError(t, err)
			deps := github.NewBaseDeps(
				clients.rest, clients.gql, clients.raw, nil, cfg.Translator,
				github.FeatureFlags{}, 5000, createFeatureChecker(nil, tt.insiders), obs,
			)
			inv, err := github.NewInventory(cfg.Translator).
				WithToolsets([]string{}).WithTools([]string{"list_issues"}).Build()
			require.NoError(t, err)
			server, err := github.NewMCPServer(ctx, &cfg, deps, inv)
			require.NoError(t, err)
			server.AddReceivingMiddleware(addUserAgentsMiddleware(cfg))
			var initialized atomic.Bool
			server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == "initialize" {
						initialized.Store(true)
					}
					return next(ctx, method, req)
				}
			})

			serverConn, clientConn := net.Pipe()
			defer clientConn.Close()
			require.NoError(t, clientConn.SetDeadline(time.Now().Add(10*time.Second)))
			session, err := server.Connect(ctx, &mcp.IOTransport{Reader: serverConn, Writer: serverConn}, nil)
			require.NoError(t, err)
			defer session.Close()
			params := &mcp.CallToolParams{
				Name: "list_issues", Arguments: map[string]any{"owner": "owner", "repo": "repo"},
			}
			var changedClient bool
			if tt.handshake == "sdk" {
				client := mcp.NewClient(tt.clientInfo, nil)
				cs, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientConn, Writer: clientConn}, nil)
				require.NoError(t, err)
				defer cs.Close()
				assert.Equal(t, "2026-07-28", cs.InitializeResult().ProtocolVersion)
				result, err := cs.CallTool(ctx, params)
				require.NoError(t, err)
				require.False(t, result.IsError, "%+v", result.Content)
			} else {
				encoder, decoder := json.NewEncoder(clientConn), json.NewDecoder(clientConn)
				id := 0
				call := func(method string, params any) json.RawMessage {
					t.Helper()
					id++
					require.NoError(t, encoder.Encode(map[string]any{
						"jsonrpc": "2.0", "id": id, "method": method, "params": params,
					}))
					var response struct {
						Result json.RawMessage `json:"result"`
						Error  json.RawMessage `json:"error"`
					}
					require.NoError(t, decoder.Decode(&response))
					require.Empty(t, response.Error)
					return response.Result
				}
				if tt.handshake == "legacy" {
					call("initialize", &mcp.InitializeParams{
						ProtocolVersion: "2025-11-25", Capabilities: &mcp.ClientCapabilities{},
						ClientInfo: tt.clientInfo,
					})
					require.NoError(t, encoder.Encode(map[string]any{
						"jsonrpc": "2.0", "method": "notifications/initialized",
					}))
				} else {
					params.Meta = mcp.Meta{
						mcp.MetaKeyProtocolVersion: "2026-07-28", mcp.MetaKeyClientCapabilities: map[string]any{},
					}
					if tt.clientInfo != nil {
						params.Meta[mcp.MetaKeyClientInfo] = tt.clientInfo
					}
				}
				var result mcp.CallToolResult
				require.NoError(t, json.Unmarshal(call("tools/call", params), &result))
				require.False(t, result.IsError, "%+v", result.Content)
				if tt.handshake != "legacy" {
					params.Meta[mcp.MetaKeyClientInfo] = &mcp.Implementation{Name: "other-client", Version: "7.8.9"}
					require.NoError(t, json.Unmarshal(call("tools/call", params), &result))
					require.False(t, result.IsError, "%+v", result.Content)
					changedClient = true
				}
			}

			assert.Equal(t, tt.handshake == "legacy", initialized.Load())
			select {
			case header := <-headers:
				assert.Equal(t, tt.want, header.Get("User-Agent"))
				assert.Equal(t, "Bearer fixture-token", header.Get("Authorization"))
				assert.Equal(t, "issue_fields, repo_issue_fields", header.Get("GraphQL-Features"))
			case <-ctx.Done():
				t.Fatal("GraphQL request did not reach the local HTTP server")
			}
			if changedClient {
				want := "github-mcp-server/" + tt.version + " (other-client/7.8.9)"
				if tt.insiders {
					want += " (insiders)"
				}
				assert.Equal(t, want, (<-headers).Get("User-Agent"))
			}
		})
	}
}
