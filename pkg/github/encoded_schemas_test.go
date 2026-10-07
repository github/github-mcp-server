package github

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The baseline uses the public single-tool registration path, which does not
// install the tools/list encoder. Both paths select the same inventory first.
func schemaListServer(ctx context.Context, inv *inventory.Inventory, optimized bool, pageSize int, schemaCache *mcp.SchemaCache) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "schema-list"}, &mcp.ServerOptions{PageSize: pageSize, SchemaCache: schemaCache})
	if optimized {
		inv.RegisterTools(ctx, server, nil)
	} else {
		for _, tool := range inv.ToolsForRegistration(ctx) {
			tool.RegisterFunc(server, nil)
		}
	}
	return server
}

func schemaListContext(protocol string, ui bool) context.Context {
	return ghcontext.WithUISupport(ghcontext.WithMCPMethodInfo(context.Background(), &ghcontext.MCPMethodInfo{
		Method:             inventory.MCPMethodToolsList,
		ProtocolVersion:    protocol,
		ClientCapabilities: &mcp.ClientCapabilities{},
	}), ui)
}

type schemaListWire struct {
	Result *mcp.ListToolsResult `json:"result"`
	Error  json.RawMessage      `json:"error"`
}

func schemaListPages(ctx context.Context, t *testing.T, inv *inventory.Inventory, optimized bool, transport, protocol string, pageSize int, schemaCache *mcp.SchemaCache) ([][]byte, []string) {
	t.Helper()
	var request func(string) []byte
	switch transport {
	case "stdio":
		server := schemaListServer(ctx, inv, optimized, pageSize, schemaCache)
		serverConn, clientConn := net.Pipe()
		ss, err := server.Connect(ctx, &mcp.IOTransport{Reader: serverConn, Writer: serverConn}, nil)
		require.NoError(t, err)
		defer func() { _ = ss.Close() }()
		defer func() { _ = clientConn.Close() }()
		reader := bufio.NewReader(clientConn)
		exchange := func(body string) []byte {
			_, err := io.WriteString(clientConn, body+"\n")
			require.NoError(t, err)
			response, err := reader.ReadBytes('\n')
			require.NoError(t, err)
			return response
		}
		init := exchange(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, protocol))
		require.NotContains(t, string(init), `"error"`)
		_, err = io.WriteString(clientConn, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
		require.NoError(t, err)
		request = exchange
	case "http":
		// Reuse the fixed inventory's server across pages instead of repeating
		// SDK annotation validation for every stateless request.
		server := schemaListServer(ctx, inv, optimized, pageSize, schemaCache)
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
		request = func(body string) []byte {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("Mcp-Protocol-Version", protocol)
			req.Header.Set("Mcp-Method", "tools/list")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, rec.Code, rec.Body.String())
			return rec.Body.Bytes()
		}
	default:
		t.Fatalf("unknown transport: %s", transport)
	}
	var pages [][]byte
	names := make([]string, 0)
	listRequest := func(id int, cursor string) string {
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/list","params":{"cursor":%q,"_meta":{"io.modelcontextprotocol/protocolVersion":%q,"io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`, id, cursor, protocol)
	}
	cursor := ""
	for {
		response := request(listRequest(2, cursor))
		pages = append(pages, response)
		var wire schemaListWire
		require.NoError(t, json.Unmarshal(response, &wire))
		require.Empty(t, wire.Error)
		require.NotNil(t, wire.Result)
		for _, tool := range wire.Result.Tools {
			names = append(names, tool.Name)
		}
		if wire.Result.NextCursor == "" {
			break
		}
		require.NotEqual(t, cursor, wire.Result.NextCursor)
		cursor = wire.Result.NextCursor
	}
	bad := request(listRequest(3, "not-a-valid-cursor"))
	var wire schemaListWire
	require.NoError(t, json.Unmarshal(bad, &wire))
	require.NotEmpty(t, wire.Error)
	pages = append(pages, bad)
	return pages, names
}

func TestEncodedSchemasWireParity(t *testing.T) {
	// Share immutable resolved schemas without changing either registration
	// path or the wire comparison.
	schemaCache := mcp.NewSchemaCache()
	type selection struct {
		name     string
		toolsets []string
		features []string
		exclude  []string
		explicit []string
		scopes   []string
		protocol string
		ui       bool
	}
	prototype, err := NewInventory(translations.NullTranslationHelper).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	var selections []selection
	for _, toolset := range append([]inventory.ToolsetID{"all", "default"}, prototype.ToolsetIDs()...) {
		for _, enabled := range []bool{false, true} {
			features := []string(nil)
			if enabled {
				features = AllowedFeatureFlags
			}
			selections = append(selections, selection{
				name: fmt.Sprintf("%s/all-flags=%v", toolset, enabled), toolsets: []string{string(toolset)}, features: features,
			})
		}
	}
	for _, feature := range AllowedFeatureFlags {
		selections = append(selections, selection{name: "flag/" + feature, toolsets: []string{"all"}, features: []string{feature}})
	}
	selections = append(selections,
		selection{name: "insiders", toolsets: []string{"all"}, features: InsidersFeatureFlags},
		selection{name: "none", toolsets: []string{}},
		selection{name: "explicit-and-excluded", toolsets: []string{"repos"}, explicit: []string{"get_me"}, exclude: []string{"get_commit"}},
		selection{name: "scopes", toolsets: []string{"all"}, scopes: []string{"read:user"}},
		selection{name: "legacy-no-ui", toolsets: []string{"all"}, features: AllowedFeatureFlags, protocol: "2025-11-25"},
		selection{name: "apps", toolsets: []string{"all"}, features: AllowedFeatureFlags, ui: true},
	)
	for _, host := range []utils.HostType{utils.HostTypeDotcom, utils.HostTypeGHEC, utils.HostTypeGHES} {
		// Reuse the same static definitions across request-time variants, just as
		// both the local HTTP and remote inventory factories do.
		definitions := AllTools(translations.NullTranslationHelper, WithHost(host))
		for _, readOnly := range []bool{false, true} {
			for _, sel := range selections {
				t.Run(fmt.Sprintf("host=%d/readonly=%v/%s", host, readOnly, sel.name), func(t *testing.T) {
					protocol := sel.protocol
					if protocol == "" {
						protocol = inventory.ProtocolVersionMultiRoundTrip
					}
					ctx := schemaListContext(protocol, sel.ui)
					flags := ResolveFeatureFlags(sel.features, false)
					builder := inventory.NewBuilder().SetTools(definitions).
						WithToolsets(sel.toolsets).WithReadOnly(readOnly).
						WithTools(sel.explicit).WithExcludeTools(sel.exclude).
						WithFeatureChecker(func(_ context.Context, stringFlag string) (bool, error) {
							return flags[stringFlag], nil
						})
					if sel.scopes != nil {
						builder.WithFilter(CreateToolScopeFilter(sel.scopes))
					}
					inv, err := builder.Build()
					require.NoError(t, err)
					wantNames := make([]string, 0)
					for _, tool := range inv.ToolsForRegistration(ctx) {
						wantNames = append(wantNames, tool.Tool.Name)
					}
					slices.Sort(wantNames)
					wantNames = slices.Compact(wantNames)
					for _, transport := range []string{"stdio", "http"} {
						t.Run(transport, func(t *testing.T) {
							before, baselineNames := schemaListPages(ctx, t, inv, false, transport, protocol, 7, schemaCache)
							after, encodedNames := schemaListPages(ctx, t, inv, true, transport, protocol, 7, schemaCache)
							assert.Equal(t, before, after, "complete JSON-RPC wire responses, including cursors and errors")
							assert.Equal(t, wantNames, baselineNames)
							assert.Equal(t, wantNames, encodedNames)
						})
					}
				})
			}
		}
	}
}

func schemaListClient(tb testing.TB, server *mcp.Server) *mcp.ClientSession {
	tb.Helper()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	require.NoError(tb, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "benchmark"}, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(tb, err)
	tb.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Close()
	})
	return cs
}

func BenchmarkEncodedSchemas(b *testing.B) {
	ctx := schemaListContext(inventory.ProtocolVersionMultiRoundTrip, false)
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(func(context.Context, string) (bool, error) { return true, nil }).
		Build()
	require.NoError(b, err)
	for _, optimized := range []bool{false, true} {
		name := map[bool]string{false: "baseline", true: "encoded"}[optimized]
		b.Run(name, func(b *testing.B) {
			b.Run("ListTools", func(b *testing.B) {
				cs := schemaListClient(b, schemaListServer(ctx, inv, optimized, 1000, nil))
				result, err := cs.ListTools(ctx, nil)
				require.NoError(b, err)
				payload, err := json.Marshal(result)
				require.NoError(b, err)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if _, err := cs.ListTools(ctx, nil); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(len(payload)), "payload-B")
				b.ReportMetric(float64(len(result.Tools)), "tools")
			})
			b.Run("ConstructionAndList", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					st, ct := mcp.NewInMemoryTransports()
					server := schemaListServer(ctx, inv, optimized, 1000, nil)
					ss, err := server.Connect(ctx, st, nil)
					if err != nil {
						b.Fatal(err)
					}
					cs, err := mcp.NewClient(&mcp.Implementation{Name: "benchmark"}, nil).Connect(ctx, ct, nil)
					if err != nil {
						b.Fatal(err)
					}
					_, err = cs.ListTools(ctx, nil)
					_ = cs.Close()
					_ = ss.Close()
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
