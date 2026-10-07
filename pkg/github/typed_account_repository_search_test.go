package github

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typedAccountRepositorySearchTools() []inventory.ServerTool {
	return []inventory.ServerTool{
		SearchRepositories(translations.NullTranslationHelper),
		SearchUsers(translations.NullTranslationHelper),
		SearchOrgs(translations.NullTranslationHelper),
		SearchCommits(translations.NullTranslationHelper),
	}
}

func connectTypedSearchClient(t *testing.T, tool inventory.ServerTool, deps ToolDependencies, protocol string) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "typed-search-test", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	tool.RegisterFunc(server, deps)
	version := protocol
	if protocol == "" {
		version = inventory.ProtocolVersionMultiRoundTrip
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				switch request := req.(type) {
				case *mcp.ListToolsRequest:
					request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
				case *mcp.CallToolRequest:
					request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
				}
				return next(ctx, method, req)
			}
		})
	}
	return connectCommentVisibilityClient(t, server, version)
}

func TestTypedAccountRepositorySearchOutputs(t *testing.T) {
	fixtures := map[string]struct {
		path string
		body string
		text string
	}{
		"search_repositories": {
			GetSearchRepositories,
			`{"total_count":1,"incomplete_results":false,"items":[{"id":7,"name":"repo","full_name":"owner/repo","description":"<b>description</b>","html_url":"repo-url","stargazers_count":0,"private":false,"fork":false,"archived":false,"created_at":"2026-01-02T03:04:05Z","topics":["go"],"owner":{"login":"owner"},"node_id":"R_7","homepage":"https://example.com","visibility":"public","pushed_at":"2026-01-03T03:04:05Z","watchers_count":4,"size":0,"disabled":false,"has_issues":true,"license":{"key":"mit","name":"MIT License","spdx_id":"MIT","url":"license-api-url"},"permissions":{"admin":false,"pull":true},"issues_url":"issues-api-url","parent":{"id":8},"custom_properties":{"text":"value"}}]}`,
			`{"total_count":1,"incomplete_results":false,"items":[{"id":7,"name":"repo","full_name":"owner/repo","description":"\u003cb\u003edescription\u003c/b\u003e","html_url":"repo-url","stargazers_count":0,"forks_count":0,"open_issues_count":0,"created_at":"2026-01-02T03:04:05Z","topics":["go"],"private":false,"fork":false,"archived":false}]}`,
		},
		"search_users": {
			GetSearchUsers,
			`{"total_count":3,"incomplete_results":true,"items":[{"id":7,"login":"octo","html_url":"profile","avatar_url":"avatar"},{"id":8},{"id":9,"login":""}]}`,
			`{"total_count":3,"incomplete_results":true,"items":[{"login":"octo","id":7,"profile_url":"profile","avatar_url":"avatar"},{"login":"","id":9}]}`,
		},
		"search_orgs": {
			GetSearchUsers,
			`{"total_count":3,"incomplete_results":true,"items":[{"id":7,"login":"octo","html_url":"profile","avatar_url":"avatar"},{"id":8},{"id":9,"login":""}]}`,
			`{"total_count":3,"incomplete_results":true,"items":[{"login":"octo","id":7,"profile_url":"profile","avatar_url":"avatar"},{"login":"","id":9}]}`,
		},
		"search_commits": {
			GetSearchCommits,
			`{"total_count":2,"incomplete_results":false,"items":[{"sha":"abc","html_url":"commit-url","commit":{"message":"<b>fix</b>","author":{"name":"Author","email":"a@example.com","date":"2026-01-02T03:04:05Z"}},"author":{"login":"octo","id":7,"html_url":"profile","avatar_url":"avatar"},"repository":{"full_name":"owner/repo","html_url":"repo-url","private":true}},{"sha":"def","html_url":""}]}`,
			`{"total_count":2,"incomplete_results":false,"items":[{"sha":"abc","html_url":"commit-url","commit":{"message":"\u003cb\u003efix\u003c/b\u003e","author":{"name":"Author","email":"a@example.com","date":"2026-01-02T03:04:05Z"}},"author":{"login":"octo","id":7,"profile_url":"profile","avatar_url":"avatar"},"repository":{"full_name":"owner/repo","html_url":"repo-url","private":true}},{"sha":"def","html_url":""}]}`,
		},
	}
	for _, tool := range typedAccountRepositorySearchTools() {
		fixture := fixtures[tool.Tool.Name]
		var apiErrorText string
		for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip, ""} {
			t.Run(tool.Tool.Name+"/"+protocol, func(t *testing.T) {
				body := fixture.body
				query := "repo:owner/repo fix"
				page, perPage, sort, order := "1", "30", "", ""
				calls := 0
				handler := func(w http.ResponseWriter, r *http.Request) {
					calls++
					expectedQuery := query
					if !hasTypeFilter(query) {
						switch tool.Tool.Name {
						case "search_users":
							expectedQuery = "type:user " + query
						case "search_orgs":
							expectedQuery = "type:org " + query
						}
					}
					assert.Equal(t, expectedQuery, r.URL.Query().Get("q"))
					assert.Equal(t, page, r.URL.Query().Get("page"))
					assert.Equal(t, perPage, r.URL.Query().Get("per_page"))
					assert.Equal(t, sort, r.URL.Query().Get("sort"))
					assert.Equal(t, order, r.URL.Query().Get("order"))
					if body == "forbidden" {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
						return
					}
					_, _ = w.Write([]byte(body))
				}
				deps := BaseDeps{
					Client:         mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{fixture.path: handler})),
					featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
				}
				session := connectTypedSearchClient(t, tool, deps, protocol)
				list, err := session.ListTools(context.Background(), nil)
				require.NoError(t, err)
				require.Len(t, list.Tools, 1)
				listed := list.Tools[0]
				assert.Equal(t, tool.Tool.Name, listed.Name)
				var resolved *jsonschema.Resolved
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					require.NotNil(t, listed.OutputSchema)
					canonical := *listed
					canonical.Icons = nil // Registration adds toolset icons absent from canonical snapshots.
					require.NoError(t, toolsnaps.Test(tool.Tool.Name, canonical))
					var schema jsonschema.Schema
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, listed.OutputSchema)), &schema))
					resolved, err = schema.Resolve(nil)
					require.NoError(t, err)
				} else {
					assert.Nil(t, listed.OutputSchema)
				}
				checkStructured := func(args map[string]any, expected, structured string) {
					t.Helper()
					args = maps.Clone(args)
					args["query"] = query
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: args})
					require.NoError(t, err)
					require.False(t, result.IsError, "%+v", result)
					require.Len(t, result.Content, 1)
					if protocol == inventory.ProtocolVersionMultiRoundTrip {
						assert.JSONEq(t, structured, getTextResult(t, result).Text)
					} else {
						assert.Equal(t, expected, getTextResult(t, result).Text)
					}
					require.NotNil(t, result.Meta)
					label := unmarshalIFC(t, result.Meta["ifc"])
					if tool.Tool.Name == "search_commits" && body == fixture.body {
						assert.Equal(t, "trusted", label["integrity"])
						assert.Equal(t, "private", label["confidentiality"])
					} else {
						assert.Equal(t, "untrusted", label["integrity"])
						assert.Equal(t, "public", label["confidentiality"])
					}
					if resolved == nil {
						assert.Nil(t, result.StructuredContent)
					} else {
						require.NotNil(t, result.StructuredContent)
						encoded := mustMarshalJSON(t, result.StructuredContent)
						assert.JSONEq(t, structured, encoded)
						assert.JSONEq(t, encoded, getTextResult(t, result).Text)
						var output any
						require.NoError(t, json.Unmarshal([]byte(encoded), &output))
						require.NoError(t, resolved.Validate(output))
					}
					request := createMCPRequest(args)
					direct, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
					require.NoError(t, err)
					require.False(t, direct.IsError)
					assert.Equal(t, expected, getTextResult(t, direct).Text)
				}
				check := func(args map[string]any, expected string) {
					t.Helper()
					checkStructured(args, expected, expected)
				}
				for _, args := range []map[string]any{
					{}, {"page": 0, "perPage": 0}, {"page": "0", "perPage": "0"},
					{"page": "0e0", "perPage": "-0"}, {"fields": []string{"id"}},
				} {
					check(args, fixture.text)
				}
				page, perPage, order = "2", "7", "asc"
				switch tool.Tool.Name {
				case "search_repositories":
					sort = "stars"
				case "search_commits":
					sort = "author-date"
				default:
					sort = "followers"
				}
				check(map[string]any{"page": "2e0", "perPage": "7.0", "sort": sort, "order": order}, fixture.text)
				page, perPage, sort, order = "1", "30", "", ""
				query = "type:org (location:seattle OR location:california)"
				check(map[string]any{}, fixture.text)
				if tool.Tool.Name == "search_repositories" {
					var full github.RepositoriesSearchResult
					require.NoError(t, json.Unmarshal([]byte(body), &full))
					checkStructured(map[string]any{"minimal_output": false}, mustMarshalJSON(t, full),
						`{"total_count":1,"incomplete_results":false,"items":[{"id":7,"name":"repo","full_name":"owner/repo","description":"<b>description</b>","html_url":"repo-url","stargazers_count":0,"forks_count":0,"open_issues_count":0,"created_at":"2026-01-02T03:04:05Z","topics":["go"],"private":false,"fork":false,"archived":false,"homepage":"https://example.com","visibility":"public","pushed_at":"2026-01-03T03:04:05Z","watchers_count":4,"size":0,"disabled":false,"has_issues":true,"license":{"key":"mit","name":"MIT License","spdx_id":"MIT"},"permissions":{"admin":false,"pull":true}}]}`)
					check(map[string]any{"minimal_output": true}, fixture.text)
				}
				for _, emptyBody := range []string{`{"total_count":0,"incomplete_results":false,"items":[]}`, `{"items":null}`, `{}`} {
					body = emptyBody
					check(map[string]any{}, `{"total_count":0,"incomplete_results":false,"items":[]}`)
					if tool.Tool.Name == "search_repositories" {
						expected := `{"total_count":0,"incomplete_results":false}`
						if emptyBody != `{"total_count":0,"incomplete_results":false,"items":[]}` {
							expected = "{}"
						}
						checkStructured(map[string]any{"minimal_output": false}, expected, `{"total_count":0,"incomplete_results":false,"items":[]}`)
					}
				}
				before := calls
				invalid := []struct {
					args map[string]any
					text string
				}{
					{map[string]any{}, "missing required parameter: query"},
					{map[string]any{"query": ""}, "missing required parameter: query"},
					{map[string]any{"query": nil}, "parameter query is not of type string"},
					{map[string]any{"query": 1}, "parameter query is not of type string"},
					{map[string]any{"query": query, "sort": nil}, "parameter sort is not of type string"},
					{map[string]any{"query": query, "order": false}, "parameter order is not of type string"},
					{map[string]any{"query": query, "page": "1.5"}, "non-integer numeric value"},
					{map[string]any{"query": query, "page": 1.5}, "non-integer numeric value"},
					{map[string]any{"query": query, "perPage": false}, "expected number"},
					{map[string]any{"query": query, "perPage": nil}, "expected number"},
					{map[string]any{"query": query, "page": "NaN"}, "non-finite numeric value"},
					{map[string]any{"query": query, "page": "1e30"}, "numeric value out of int range"},
					{map[string]any{"query": query, "after": nil}, "parameter after is not of type string"},
				}
				if tool.Tool.Name == "search_repositories" {
					invalid = append(invalid, struct {
						args map[string]any
						text string
					}{map[string]any{"query": query, "minimal_output": nil}, "parameter minimal_output is not of type bool"})
				}
				for _, tc := range invalid {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: tc.args})
					require.NoError(t, err)
					require.True(t, result.IsError)
					assert.Contains(t, getErrorResult(t, result).Text, tc.text)
					assert.Nil(t, result.StructuredContent)
					request := createMCPRequest(tc.args)
					direct, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
					require.NoError(t, err)
					require.True(t, direct.IsError)
					assert.Contains(t, getErrorResult(t, direct).Text, tc.text)
				}
				assert.Equal(t, before, calls, "invalid input must never reach GitHub")
				body = "forbidden"
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: tool.Tool.Name, Arguments: map[string]any{"query": query},
				})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, "failed to search")
				assert.Contains(t, getErrorResult(t, result).Text, query)
				assert.Nil(t, result.StructuredContent)
				if apiErrorText == "" {
					apiErrorText = getErrorResult(t, result).Text
				}
				assert.Equal(t, apiErrorText, getErrorResult(t, result).Text, "API errors are identical across protocols")
			})
		}
	}
}

func TestSearchRepositoryOutputSchemaIsCompact(t *testing.T) {
	schema := searchRepositoriesOutputSchema()
	assert.Equal(t, searchRepositoryVisibilityEnum, schema.Properties["items"].Items.Properties["visibility"].Enum)
	encoded := mustMarshalJSON(t, schema)
	for _, field := range []string{"node_id", "custom_properties", "owner", "parent", "issues_url", "clone_url", "url\""} {
		assert.NotContains(t, encoded, `"`+field, field)
	}
	assert.Contains(t, encoded, `"html_url"`)
	for _, tool := range typedAccountRepositorySearchTools() {
		assert.True(t, tool.IsReadOnly())
	}
}

type failingSearchClientDeps struct {
	BaseDeps
}

func (failingSearchClientDeps) GetClient(context.Context) (*github.Client, error) {
	return nil, errors.New("client unavailable")
}

func TestTypedAccountRepositorySearchClientErrors(t *testing.T) {
	for _, tool := range typedAccountRepositorySearchTools() {
		for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip, ""} {
			t.Run(tool.Tool.Name+"/"+protocol, func(t *testing.T) {
				session := connectTypedSearchClient(t, tool, failingSearchClientDeps{}, protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: tool.Tool.Name, Arguments: map[string]any{"query": "query"},
				})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Equal(t, "failed to get GitHub client: client unavailable", getErrorResult(t, result).Text)
				assert.Nil(t, result.StructuredContent)
			})
		}
	}
}

func TestSearchOutputsHaveNoUntypedFields(t *testing.T) {
	seen := make(map[reflect.Type]bool)
	var check func(reflect.Type)
	check = func(typ reflect.Type) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Interface:
			t.Errorf("untyped output field: %s", typ)
		case reflect.Pointer, reflect.Slice, reflect.Array:
			check(typ.Elem())
		case reflect.Map:
			check(typ.Elem())
		case reflect.Struct:
			for field := range typ.Fields() {
				if field.IsExported() && !strings.HasPrefix(field.Tag.Get("json"), "-") {
					check(field.Type)
				}
			}
		}
	}
	check(reflect.TypeFor[SearchRepositoriesOutput]())
	check(reflect.TypeFor[MinimalSearchUsersResult]())
	check(reflect.TypeFor[MinimalSearchCommitsResult]())
}
