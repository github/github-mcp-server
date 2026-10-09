package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedHeaderInputSchema(t *testing.T) {
	original := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type: "string", MinLength: new(1), Enum: []any{"octo"},
				Default: json.RawMessage(`"octo"`),
				Extra:   map[string]any{"x-mcp-header": "owner", "unrelated": "metadata"},
			},
			"query": {Type: "string", Default: json.RawMessage(`"default"`)},
			"nested": {Type: "object", Required: []string{"count"}, Properties: map[string]*jsonschema.Schema{
				"count": {Type: "integer", Minimum: new(float64(1)), Extra: map[string]any{"x-mcp-header": "count"}},
			}},
		},
		Required:             []string{"owner", "query", "nested"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
	before := mustMarshalJSON(t, original)
	raw := json.RawMessage(before)
	var mapped map[string]any
	require.NoError(t, json.Unmarshal(raw, &mapped))
	var first *jsonschema.Schema
	for _, value := range []any{original, CloneSchema(original), raw, mapped} {
		projected, err := cachedHeaderInputSchema(value)
		require.NoError(t, err)
		if first == nil {
			first = projected
		} else {
			assert.Same(t, first, projected)
		}
		assert.JSONEq(t, `{
				"type":"object",
				"properties":{
					"owner":{"type":"string","x-mcp-header":"owner"},
					"nested":{"properties":{"count":{"type":"integer","x-mcp-header":"count"}}}
				}
			}`, mustMarshalJSON(t, projected))
		resolved, err := projected.Resolve(nil)
		require.NoError(t, err)
		arguments := map[string]any{"query": 42}
		require.NoError(t, resolved.ApplyDefaults(&arguments))
		require.NoError(t, resolved.Validate(arguments))
		assert.Equal(t, map[string]any{"query": 42}, arguments, "no required fields, defaults, or unannotated body constraints")
	}
	assert.JSONEq(t, before, mustMarshalJSON(t, original), "projection must not mutate the advertised schema")
	empty, err := cachedHeaderInputSchema(&jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{"query": {Type: "string"}}})
	require.NoError(t, err)
	object, err := cachedObjectInputSchema()
	require.NoError(t, err)
	assert.Same(t, object, empty)
}

func TestTypedHeaderParamsHTTP(t *testing.T) {
	type input struct {
		Owner  string `json:"owner"`
		Repo   string `json:"repo"`
		Query  string `json:"query,omitempty"`
		Active bool   `json:"active,omitempty"`
		Count  int    `json:"count,omitempty"`
		Nested struct {
			Route string `json:"route"`
		} `json:"nested,omitzero"`
	}
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {Type: "string", MinLength: new(1)},
			"repo":  {Type: "string"},
			"query": {Type: "string", Default: json.RawMessage(`"default-query"`), Enum: []any{"default-query"}},
			"active": {Type: "boolean", Extra: map[string]any{
				"x-mcp-header": "Active",
			}},
			"count": {Type: "integer", Extra: map[string]any{
				"x-mcp-header": "count",
			}},
			"nested": {Type: "object", Properties: map[string]*jsonschema.Schema{
				"route": {Type: "string", Extra: map[string]any{"x-mcp-header": "route"}},
			}},
		},
		Required: []string{"owner", "repo"},
	}
	for _, typed := range []bool{false, true} {
		for _, registration := range []string{"single", "inventory", "modern", "legacy", "validation-override", "inferred"} {
			t.Run(fmt.Sprintf("typed=%t/%s", typed, registration), func(t *testing.T) {
				for _, protocol := range []string{ProtocolVersionMultiRoundTrip, "2025-11-25"} {
					t.Run(protocol, func(t *testing.T) {
						toolSchema := schema
						if registration == "inferred" {
							var err error
							toolSchema, err = CachedInputSchemaFor[input](nil)
							require.NoError(t, err)
						}
						options := TypedSchemaOptions{}
						if registration == "validation-override" {
							options.ValidationInputSchema = CloneSchema(toolSchema)
							// Runtime validation need not carry the advertised bindings.
							for _, property := range options.ValidationInputSchema.Properties {
								property.Extra = nil
							}
							options.ValidationInputSchema.Properties["nested"].Properties["route"].Extra = nil
						}
						handlerCalls, receivingCalls, guardCalls := 0, 0, 0
						gotQuery := ""
						definition := mcp.Tool{Name: "header_probe", InputSchema: toolSchema}
						tool := NewServerToolWithContextHandlerAndSchemaOptions(
							definition, testToolsetMetadata("test"),
							func(_ context.Context, _ *mcp.CallToolRequest, args input) (*mcp.CallToolResult, typedTestOutput, error) {
								handlerCalls++
								gotQuery = args.Query
								return nil, typedTestOutput{Query: args.Query}, nil
							}, options,
						)
						if !typed {
							tool = NewServerToolWithContextHandler(
								definition, testToolsetMetadata("test"),
								func(context.Context, *mcp.CallToolRequest, input) (*mcp.CallToolResult, any, error) {
									handlerCalls++
									return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
								},
							)
						} else if registration == "inferred" {
							tool.Tool.InputSchema = nil
						}
						server := mcp.NewServer(&mcp.Implementation{Name: "header-test", Version: "test"}, nil)
						guard := func(next mcp.ToolHandler) mcp.ToolHandler {
							return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
								guardCalls++
								return next(ctx, req)
							}
						}
						switch registration {
						case "inventory":
							inv, err := NewBuilder().SetTools([]ServerTool{tool}).WithToolsets([]string{"test"}).Build()
							require.NoError(t, err)
							inv.RegisterTools(context.Background(), server, nil, guard)
						case "modern":
							tool.RegisterFuncForProtocolEra(server, nil, ProtocolEraModern, guard)
						case "legacy":
							tool.RegisterFuncForProtocolEra(server, nil, ProtocolEraLegacy, guard)
						default:
							tool.RegisterFunc(server, nil, guard)
						}
						// Model a scanner installed as receiving middleware. Header failures
						// must be rejected by the HTTP transport before reaching it.
						server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
							return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
								if method == MCPMethodToolsCall {
									receivingCalls++
								}
								return next(ctx, method, req)
							}
						})
						handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
							return server
						}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
						request := func(method, arguments string, headers map[string]string) *httptest.ResponseRecorder {
							t.Helper()
							params := fmt.Sprintf(`{"name":"header_probe","arguments":%s,"_meta":{"io.modelcontextprotocol/protocolVersion":%q,"io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}`, arguments, protocol)
							req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(fmt.Sprintf(
								`{"jsonrpc":"2.0","id":1,"method":%q,"params":%s}`, method, params)))
							req.Header.Set("Content-Type", "application/json")
							req.Header.Set("Accept", "application/json, text/event-stream")
							req.Header.Set("Mcp-Protocol-Version", protocol)
							req.Header.Set("Mcp-Method", method)
							req.Header.Set("Mcp-Name", "header_probe")
							for key, value := range headers {
								req.Header.Set(key, value)
							}
							rec := httptest.NewRecorder()
							handler.ServeHTTP(rec, req)
							return rec
						}
						list := request(MCPMethodToolsList, `{}`, nil)
						require.Equal(t, http.StatusOK, list.Code, list.Body.String())
						var listed struct {
							Result mcp.ListToolsResult `json:"result"`
						}
						require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listed))
						require.Len(t, listed.Result.Tools, 1)
						want := definition
						want.InputSchema = toolSchema
						AnnotateHeaderParams(&want)
						assert.JSONEq(t, mustMarshalJSON(t, want.InputSchema), mustMarshalJSON(t, listed.Result.Tools[0].InputSchema))

						type headerCase struct {
							name      string
							arguments string
							headers   map[string]string
							reject    bool
						}
						cases := []headerCase{
							{"matching", `{"owner":"octo","repo":"hello"}`, map[string]string{"Mcp-Param-owner": "octo", "Mcp-Param-repo": "hello"}, false},
							{"owner-mismatch", `{"owner":"octo","repo":"hello"}`, map[string]string{"Mcp-Param-owner": "other", "Mcp-Param-repo": "hello"}, true},
							{"repo-mismatch", `{"owner":"octo","repo":"hello"}`, map[string]string{"Mcp-Param-owner": "octo", "Mcp-Param-repo": "other"}, true},
							{"missing-headers", `{"owner":"octo","repo":"hello"}`, nil, true},
							{"absent-body", `{}`, map[string]string{"Mcp-Param-owner": "octo"}, true},
							{"null-body", `{"owner":null}`, map[string]string{"Mcp-Param-owner": "octo"}, true},
							{"case-insensitive-header", `{"owner":"octo","repo":"hello"}`, map[string]string{"mcp-param-OWNER": "octo", "MCP-PARAM-REPO": "hello"}, false},
							{"case-sensitive-value", `{"owner":"Octo","repo":"hello"}`, map[string]string{"Mcp-Param-owner": "octo", "Mcp-Param-repo": "hello"}, true},
							{"case-sensitive-property", `{"Owner":"octo"}`, map[string]string{"Mcp-Param-owner": "octo"}, true},
							{"base64", `{"owner":"octo","repo":"hello"}`, map[string]string{"Mcp-Param-owner": "=?base64?b2N0bw==?=", "Mcp-Param-repo": "hello"}, false},
							{"invalid-base64", `{"owner":"octo","repo":"hello"}`, map[string]string{"Mcp-Param-owner": "=?base64?!!!?=", "Mcp-Param-repo": "hello"}, true},
						}
						if registration != "inferred" {
							cases = append(cases,
								headerCase{"optional-and-nested", `{"owner":"octo","repo":"hello","active":false,"count":42,"nested":{"route":"here"}}`,
									map[string]string{"Mcp-Param-owner": "octo", "Mcp-Param-repo": "hello", "Mcp-Param-active": "false", "Mcp-Param-count": "42", "Mcp-Param-route": "here"}, false},
								headerCase{"nested-mismatch", `{"owner":"octo","repo":"hello","nested":{"route":"here"}}`,
									map[string]string{"Mcp-Param-owner": "octo", "Mcp-Param-repo": "hello", "Mcp-Param-route": "elsewhere"}, true},
								headerCase{"optional-absent", `{"owner":"octo","repo":"hello"}`,
									map[string]string{"Mcp-Param-owner": "octo", "Mcp-Param-repo": "hello", "Mcp-Param-active": "false"}, true},
							)
						}
						for _, tc := range cases {
							t.Run(tc.name, func(t *testing.T) {
								handlerCalls, receivingCalls, guardCalls = 0, 0, 0
								rec := request(MCPMethodToolsCall, tc.arguments, tc.headers)
								if tc.reject && protocol == ProtocolVersionMultiRoundTrip {
									require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
									var wire struct {
										Error *jsonrpc.Error `json:"error"`
									}
									require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &wire))
									require.NotNil(t, wire.Error)
									assert.EqualValues(t, mcp.CodeHeaderMismatch, wire.Error.Code)
									assert.Contains(t, wire.Error.Message, "header mismatch")
									assert.Zero(t, handlerCalls)
									assert.Zero(t, receivingCalls)
									assert.Zero(t, guardCalls)
								} else {
									require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
									assert.Equal(t, 1, receivingCalls)
									assert.Equal(t, 1, guardCalls)
									if !tc.reject {
										assert.Equal(t, 1, handlerCalls, rec.Body.String())
										if typed && registration != "inferred" {
											assert.Equal(t, "default-query", gotQuery)
										}
									}
								}
							})
						}
					})
				}
			})
		}
	}
}
