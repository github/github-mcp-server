package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func userEventsTool(t *testing.T) inventory.ServerTool {
	t.Helper()
	for _, tool := range AllTools(translations.NullTranslationHelper) {
		if tool.Tool.Name == "list_user_events" {
			return tool
		}
	}
	t.Fatal("list_user_events is missing from the tool inventory")
	return inventory.ServerTool{}
}

func Test_ListUserEvents(t *testing.T) {
	t.Parallel()
	tool := userEventsTool(t)
	require.Equal(t, ToolsetMetadataUsers.ID, tool.Toolset.ID)
	require.True(t, tool.IsReadOnly())
	require.NoError(t, toolsnaps.Test(tool.Tool.Name, tool.Tool))

	tests := []struct {
		name     string
		args     map[string]any
		page     string
		perPage  string
		status   int
		response string
		want     string
		wantErr  string
	}{
		{
			name: "public events retain payloads and exact IDs with minimal metadata",
			args: map[string]any{"username": "octocat"}, page: "1", perPage: "30", status: http.StatusOK,
			response: `[{"id":"1","type":"PushEvent","public":true,"actor":{"login":"octocat","avatar_url":"https://example.com/avatar"},"repo":{"name":"octo-org/demo","url":"https://api.github.com/repos/octo-org/demo"},"created_at":"2026-10-01T12:00:00Z","payload":{"ref":"refs/heads/main","head":"abc123","push_id":9007199254740993}}, {"id":"2","type":"FutureEvent","public":true,"actor":{"login":"octocat"},"repo":{"name":"octo-org/demo"},"payload":{"comment":{"body":"Hello\u202e world"},"items":[{"text":"a\u200bb"}]}}]`,
			want:     `[{"id":"1","type":"PushEvent","public":true,"actor":"octocat","repo":"octo-org/demo","created_at":"2026-10-01T12:00:00Z","payload":{"ref":"refs/heads/main","head":"abc123","push_id":9007199254740993}}, {"id":"2","type":"FutureEvent","public":true,"actor":"octocat","repo":"octo-org/demo","payload":{"comment":{"body":"Hello world"},"items":[{"text":"ab"}]}}]`,
		},
		{
			name: "private events returned by GitHub are retained",
			args: map[string]any{"username": "octocat"}, page: "1", perPage: "30", status: http.StatusOK,
			response: `[{"id":"4","type":"PushEvent","public":false,"actor":{"login":"octocat"},"repo":{"name":"octo-org/private"},"payload":{"ref":"refs/heads/main"}}]`,
			want:     `[{"id":"4","type":"PushEvent","public":false,"actor":"octocat","repo":"octo-org/private","payload":{"ref":"refs/heads/main"}}]`,
		},
		{
			name: "explicit pagination reaches GitHub",
			args: map[string]any{"username": "octocat", "page": 3, "perPage": 100}, page: "3", perPage: "100", status: http.StatusOK,
			response: `[]`, want: `[]`,
		},
		{
			name: "empty feed returns an array",
			args: map[string]any{"username": "octocat"}, page: "1", perPage: "30", status: http.StatusOK,
			response: `[]`, want: `[]`,
		},
		{
			name: "optional event fields may be absent",
			args: map[string]any{"username": "octocat"}, page: "1", perPage: "30", status: http.StatusOK,
			response: `[{"id":"3","type":"WatchEvent","payload":null}]`, want: `[{"id":"3","type":"WatchEvent"}]`,
		},
		{
			name: "unknown user surfaces API error",
			args: map[string]any{"username": "octocat"}, page: "1", perPage: "30", status: http.StatusNotFound,
			response: `{"message":"Not Found"}`, wantErr: "Not Found",
		},
		{
			name: "rate limit surfaces API error",
			args: map[string]any{"username": "octocat"}, page: "1", perPage: "30", status: http.StatusForbidden,
			response: `{"message":"API rate limit exceeded"}`, wantErr: "API rate limit exceeded",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /users/octocat/events": func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, tc.page, r.URL.Query().Get("page"))
					assert.Equal(t, tc.perPage, r.URL.Query().Get("per_page"))
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.response))
				},
			})
			deps := BaseDeps{Client: mustNewGHClient(t, client), Obsv: stubExporters()}
			request := createMCPRequest(tc.args)
			result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &request)
			require.NoError(t, err)
			if tc.wantErr != "" {
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.wantErr)
				return
			}
			require.False(t, result.IsError)
			text := getTextResult(t, result).Text
			assert.JSONEq(t, tc.want, text)
			if tc.name == "public events retain payloads and exact IDs with minimal metadata" {
				// JSONEq decodes numbers as float64, so also check the large ID literally.
				assert.Contains(t, text, `9007199254740993`)
			}
		})
	}
}

func Test_ListUserEvents_InvalidArguments(t *testing.T) {
	t.Parallel()
	tool := userEventsTool(t)
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing username", map[string]any{}, "username"},
		{"empty username", map[string]any{"username": ""}, "username"},
		{"wrong username type", map[string]any{"username": 42}, "username"},
		{"page below minimum", map[string]any{"username": "octocat", "page": 0}, "page"},
		{"perPage below minimum", map[string]any{"username": "octocat", "perPage": 0}, "perPage"},
		{"perPage above maximum", map[string]any{"username": "octocat", "perPage": 101}, "perPage"},
		{"fractional page", map[string]any{"username": "octocat", "page": 1.5}, "page"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := MockHTTPClientWithHandler(func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("invalid arguments must not send an API request")
			})
			deps := BaseDeps{Client: mustNewGHClient(t, client), Obsv: stubExporters()}
			request := createMCPRequest(tc.args)
			result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, tc.want)
		})
	}
}

func Test_ListUserEvents_ClientError(t *testing.T) {
	t.Parallel()
	tool := userEventsTool(t)
	deps := stubDeps{clientFn: stubClientFnErr("client unavailable"), obsv: stubExporters()}
	request := createMCPRequest(map[string]any{"username": "octocat"})
	result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, getErrorResult(t, result).Text, "client unavailable")
}

func Test_ListUserEvents_AdvertisedPagination(t *testing.T) {
	t.Parallel()
	tool := userEventsTool(t)
	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.Tool.InputSchema)), &schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	require.NoError(t, resolved.Validate(map[string]any{"username": "octocat", "page": 2, "perPage": 5}))
	for _, args := range []map[string]any{
		{"username": "octocat", "page": 1.5},
		{"username": "octocat", "perPage": 2.5},
	} {
		require.Error(t, resolved.Validate(args), "advertised schema must reject fractional pagination")
	}
}

func Test_ListUserEvents_IFC(t *testing.T) {
	t.Parallel()
	tool := userEventsTool(t)
	for _, tc := range []struct{ name, response, confidentiality string }{
		{"empty feed", `[]`, "public"},
		{"public feed", `[{"id":"1","type":"WatchEvent","public":true}]`, "public"},
		{"private feed", `[{"id":"2","type":"PushEvent","public":false}]`, "private"},
		{"mixed feed", `[{"id":"1","type":"WatchEvent","public":true},{"id":"2","type":"PushEvent","public":false}]`, "private"},
		{"missing visibility", `[{"id":"3","type":"FutureEvent"}]`, "private"},
		{"null visibility", `[{"id":"3","type":"FutureEvent","public":null}]`, "private"},
		{"null event", `[null,{"id":"1","type":"WatchEvent","public":true}]`, "public"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /users/octocat/events": func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte(tc.response))
				},
			})
			deps := NewBaseDeps(mustNewGHClient(t, client), nil, nil, nil,
				translations.NullTranslationHelper, FeatureFlags{}, 0,
				func(_ context.Context, flag string) (bool, error) {
					return flag == FeatureFlagIFCLabels, nil
				}, stubExporters())
			request := createMCPRequest(map[string]any{"username": "octocat"})
			result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			label, err := json.Marshal(result.Meta["ifc"])
			require.NoError(t, err)
			assert.JSONEq(t, `{"integrity":"untrusted","confidentiality":"`+tc.confidentiality+`"}`, string(label))
		})
	}
}

func Test_ListUserEvents_Protocols(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			tool := userEventsTool(t)
			response := `[{"id":"1","type":"FutureEvent","public":true,"actor":{"login":"octocat"},"repo":{"name":"octo-org/demo"},"created_at":"2026-10-01T12:00:00Z","payload":{"id":9007199254740993,"text":"a\u200bb"}}]`
			status := http.StatusOK
			calls := 0
			client := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /users/octocat/events": func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.Equal(t, "1", r.URL.Query().Get("page"))
					assert.Equal(t, "30", r.URL.Query().Get("per_page"))
					w.WriteHeader(status)
					_, _ = w.Write([]byte(response))
				},
			})
			deps := BaseDeps{Client: mustNewGHClient(t, client), Obsv: stubExporters()}
			session := connectTypedSearchClient(t, tool, deps, protocol)
			listed, err := session.ListTools(t.Context(), nil)
			require.NoError(t, err)
			require.Len(t, listed.Tools, 1)
			var outputSchema *jsonschema.Resolved
			if protocol == inventory.ProtocolVersionMultiRoundTrip {
				require.NotNil(t, listed.Tools[0].OutputSchema, "modern clients need a typed output schema")
				require.NoError(t, toolsnaps.Test(tool.Tool.Name+"_output", mcp.Tool{
					Name: tool.Tool.Name, OutputSchema: listed.Tools[0].OutputSchema,
				}))
				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, listed.Tools[0].OutputSchema)), &schema))
				outputSchema, err = schema.Resolve(nil)
				require.NoError(t, err)
			} else {
				assert.Nil(t, listed.Tools[0].OutputSchema)
			}
			args := &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: map[string]any{"username": "octocat"}}
			result, err := session.CallTool(t.Context(), args)
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			text := getTextResult(t, result).Text
			assert.JSONEq(t, `[{"id":"1","type":"FutureEvent","public":true,"actor":"octocat","repo":"octo-org/demo","created_at":"2026-10-01T12:00:00Z","payload":{"id":9007199254740993,"text":"ab"}}]`, text)
			assert.Contains(t, text, `9007199254740993`)
			if outputSchema != nil {
				require.NotNil(t, result.StructuredContent)
				assert.JSONEq(t, text, mustMarshalJSON(t, result.StructuredContent))
				require.NoError(t, outputSchema.Validate(result.StructuredContent))
			} else {
				assert.Nil(t, result.StructuredContent)
			}
			assert.Equal(t, 1, calls)

			response = `[{"id":"2","type":"PushEvent","public":false,"repo":{"name":"octo-org/private"},"payload":{"ref":"refs/heads/main"}}]`
			private, err := session.CallTool(t.Context(), args)
			require.NoError(t, err)
			require.False(t, private.IsError)
			privateText := getTextResult(t, private).Text
			assert.JSONEq(t, `[{"id":"2","type":"PushEvent","public":false,"repo":"octo-org/private","payload":{"ref":"refs/heads/main"}}]`, privateText)
			if outputSchema != nil {
				assert.JSONEq(t, privateText, mustMarshalJSON(t, private.StructuredContent))
				require.NoError(t, outputSchema.Validate(private.StructuredContent))
			}

			response = `[]`
			empty, err := session.CallTool(t.Context(), args)
			require.NoError(t, err)
			require.False(t, empty.IsError)
			assert.Equal(t, `[]`, getTextResult(t, empty).Text)
			if outputSchema != nil {
				assert.JSONEq(t, `[]`, mustMarshalJSON(t, empty.StructuredContent))
			}

			status, response = http.StatusNotFound, `{"message":"Not Found"}`
			failed, err := session.CallTool(t.Context(), args)
			require.NoError(t, err)
			require.True(t, failed.IsError)
			assert.Contains(t, getErrorResult(t, failed).Text, "Not Found")
			assert.Nil(t, failed.StructuredContent)

			invalid, err := session.CallTool(t.Context(), &mcp.CallToolParams{
				Name: tool.Tool.Name, Arguments: map[string]any{"username": "octocat", "perPage": 0},
			})
			require.NoError(t, err)
			require.True(t, invalid.IsError)
			assert.Equal(t, 4, calls, "invalid pagination must not reach GitHub")
		})
	}
}
