package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	gogithub "github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pullRequestArtifactTestTools() []inventory.ServerTool {
	t := translations.NullTranslationHelper
	return []inventory.ServerTool{PullRequestArtifactRead(t), PullRequestArtifactWrite(t)}
}

func pullRequestArtifactTestTool(method string) inventory.ServerTool {
	if method == "list" {
		return PullRequestArtifactRead(translations.NullTranslationHelper)
	}
	return PullRequestArtifactWrite(translations.NullTranslationHelper)
}

func pullRequestArtifactTestClient(t *testing.T, handler http.HandlerFunc) *gogithub.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := gogithub.NewClient(
		gogithub.WithHTTPClient(server.Client()),
		gogithub.WithEnterpriseURLs(server.URL+"/api/v3/", server.URL+"/api/uploads/"),
	)
	require.NoError(t, err)
	return client
}

func pullRequestArtifactTestSession(t *testing.T, deps BaseDeps, protocol string) *mcp.ClientSession {
	t.Helper()
	inv, err := inventory.NewBuilder().SetTools(pullRequestArtifactTestTools()).
		WithFeatureChecker(featureCheckerFor(FeatureFlagPullRequestArtifacts)).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "pull-request-artifacts", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	inv.RegisterTools(context.Background(), server, deps)
	return connectCommentVisibilityClient(t, server, protocol)
}

func TestPullRequestArtifactTools(t *testing.T) {
	session := pullRequestArtifactTestSession(t, BaseDeps{}, inventory.ProtocolVersionMultiRoundTrip)
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 2)
	for _, wireTool := range list.Tools {
		require.NoError(t, toolsnaps.Test(wireTool.Name+"_ff_"+FeatureFlagPullRequestArtifacts, wireTool))
		assert.NotNil(t, wireTool.OutputSchema)
	}
	for _, tool := range pullRequestArtifactTestTools() {
		assert.Equal(t, ToolsetMetadataPullRequests.ID, tool.Toolset.ID)
		assert.Equal(t, []inventory.FeatureFlag{FeatureFlagPullRequestArtifacts}, tool.FeatureRule.Features())
		readOnly := tool.Tool.Name == "pull_request_artifact_read"
		assert.Equal(t, readOnly, tool.IsReadOnly())
		assert.Equal(t, !readOnly, *tool.Tool.Annotations.DestructiveHint)
		schema := tool.Tool.InputSchema.(*jsonschema.Schema)
		required := []string{"owner", "repo", "pull_number", "method"}
		if !readOnly {
			required = append(required, "name")
		}
		assert.ElementsMatch(t, required, schema.Required)
	}
	for _, tc := range []struct {
		name     string
		flags    []inventory.FeatureFlag
		toolsets []string
		readOnly bool
		count    int
	}{
		{name: "default"},
		{name: "explicit", flags: []inventory.FeatureFlag{FeatureFlagPullRequestArtifacts}, count: 2},
		{name: "read only", flags: []inventory.FeatureFlag{FeatureFlagPullRequestArtifacts}, readOnly: true, count: 1},
		{name: "read only default", readOnly: true},
		{name: "granular", flags: []inventory.FeatureFlag{FeatureFlagPullRequestArtifacts, inventory.FeatureFlag(FeatureFlagPullRequestsGranular)}, count: 2},
		{name: "pull request toolset", flags: []inventory.FeatureFlag{FeatureFlagPullRequestArtifacts}, toolsets: []string{"pull_requests"}, count: 2},
		{name: "other toolset", flags: []inventory.FeatureFlag{FeatureFlagPullRequestArtifacts}, toolsets: []string{"issues"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := inventory.NewBuilder().SetTools(AllTools(translations.NullTranslationHelper)).
				WithToolsets(tc.toolsets).WithReadOnly(tc.readOnly).
				WithFeatureChecker(featureCheckerFor(tc.flags...)).Build()
			require.NoError(t, err)
			count := 0
			for _, tool := range inv.AvailableTools(context.Background()) {
				if strings.HasPrefix(tool.Tool.Name, "pull_request_artifact_") {
					count++
					if tc.readOnly {
						assert.True(t, tool.IsReadOnly())
					}
				}
			}
			assert.Equal(t, tc.count, count)
		})
	}
	assert.False(t, ResolveFeatureFlags(nil, false)[FeatureFlagPullRequestArtifacts])
	assert.False(t, ResolveFeatureFlags(nil, true)[FeatureFlagPullRequestArtifacts])
	assert.True(t, ResolveFeatureFlags([]string{"pull_request_artifacts"}, false)[FeatureFlagPullRequestArtifacts])
	assert.NotContains(t, ResolveFeatureFlags([]string{"pull_request_artifacts_api"}, false), "pull_request_artifacts_api")
}

func TestPullRequestArtifactRequests(t *testing.T) {
	artifact := `{"id":"artifact-id","number":3,"name":"report","body":"full body","commit_oid":null,"creator":null,"created_at":null,"updated_at":null}`
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25"} {
		for _, tc := range []struct {
			name, method, verb, query, body string
			args                            map[string]any
			status                          int
			response                        string
			next                            bool
		}{
			{name: "upsert create empty body", method: "upsert", verb: "PUT", args: map[string]any{"name": "report", "body": ""}, body: `{"body":""}`, status: 201, response: artifact},
			{name: "upsert update with commit", method: "upsert", verb: "PUT", args: map[string]any{"name": "report", "body": "new", "commit_oid": "abc123"}, body: `{"body":"new","commit_oid":"abc123"}`, status: 200, response: artifact},
			{name: "delete", method: "delete", verb: "DELETE", args: map[string]any{"name": "report"}, status: 204},
			{name: "list default", method: "list", verb: "GET", query: "page=1&per_page=30", status: 200, response: "[" + artifact + "]"},
			{name: "list current", method: "list", verb: "GET", args: map[string]any{"commit_oid": "current"}, query: "commit_oid=current&page=1&per_page=30", status: 200, response: "[" + artifact + "]"},
			{name: "list all", method: "list", verb: "GET", args: map[string]any{"commit_oid": "all"}, query: "commit_oid=all&page=1&per_page=30", status: 200, response: "[" + artifact + "]"},
			{name: "list explicit commit and page", method: "list", verb: "GET", args: map[string]any{"commit_oid": "abc123", "page": 2, "perPage": 1}, query: "commit_oid=abc123&page=2&per_page=1", status: 200, response: "[" + artifact + "]"},
			{name: "list empty", method: "list", verb: "GET", query: "page=1&per_page=30", status: 200, response: "[]"},
			{name: "short page with next link", method: "list", verb: "GET", query: "page=1&per_page=30", status: 200, response: "[" + artifact + "]", next: true},
			{name: "full page without next link", method: "list", verb: "GET", args: map[string]any{"perPage": 1}, query: "page=1&per_page=1", status: 200, response: "[" + artifact + "]"},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				calls := 0
				client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					if calls == 1 {
						assert.Equal(t, http.MethodGet, r.Method)
						assert.Equal(t, "/api/v3/repos/o/r", r.URL.Path)
						_, _ = w.Write([]byte(`{"id":42,"private":true}`))
						return
					}
					assert.Equal(t, tc.verb, r.Method)
					path := "/api/v3/repositories/42/pulls/12/artifacts"
					if tc.method != "list" {
						path += "/report"
					}
					assert.Equal(t, path, r.URL.Path)
					assert.Equal(t, tc.query, r.URL.RawQuery)
					if tc.body != "" {
						var body json.RawMessage
						require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
						assert.JSONEq(t, tc.body, string(body))
					}
					if tc.next {
						w.Header().Set("Link", `<https://api.example/repositories/42/pulls/12/artifacts?page=2&per_page=30>; rel="next"`)
					}
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.response))
				})
				deps := BaseDeps{Client: client, featureChecker: featureCheckerFor(FeatureFlagIFCLabels)}
				session := pullRequestArtifactTestSession(t, deps, protocol)
				args := map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": tc.method}
				maps.Copy(args, tc.args)
				tool := pullRequestArtifactTestTool(tc.method)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError, mustMarshalJSON(t, result))
				assert.Equal(t, 2, calls, "one repo lookup and one artifact request, without pagination or IFC refetches")
				text := getTextResult(t, result).Text
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					assert.JSONEq(t, text, mustMarshalJSON(t, result.StructuredContent))
				} else {
					assert.Nil(t, result.StructuredContent)
				}
				if tc.method == "delete" {
					assert.JSONEq(t, `{"deleted":{"pull_number":12,"name":"report","message":"artifact deleted"}}`, text)
					assert.NotContains(t, result.Meta, "ifc")
				} else {
					assert.JSONEq(t, mustMarshalJSON(t, ifc.PrivateTrusted()), mustMarshalJSON(t, result.Meta["ifc"]))
				}
				switch tc.method {
				case "list":
					var output PullRequestArtifactReadOutput
					require.NoError(t, json.Unmarshal([]byte(text), &output))
					require.NotNil(t, output.List)
					require.NotNil(t, output.List.Artifacts)
					nextPage := 0
					if tc.next {
						nextPage = 2
					}
					assert.Equal(t, nextPage, output.List.NextPage)
					if tc.name == "list explicit commit and page" {
						assert.Equal(t, 2, output.List.Page)
						assert.Equal(t, 1, output.List.PerPage)
					}
				case "upsert":
					assert.JSONEq(t, `{"artifact":`+artifact+`}`, text)
				}
			})
		}
	}
}

func TestPullRequestArtifactEscapedNames(t *testing.T) {
	for _, method := range []string{"upsert", "delete"} {
		t.Run(method, func(t *testing.T) {
			name := "report #1?%draft"
			calls := 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					_, _ = w.Write([]byte(`{"id":42}`))
					return
				}
				assert.Equal(t, "/api/v3/repositories/42/pulls/12/artifacts/"+url.PathEscape(name), r.URL.EscapedPath())
				assert.Empty(t, r.URL.RawQuery)
				if method == "delete" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				_, _ = fmt.Fprintf(w, `{"id":9007199254740993,"number":3,"name":%q,"body":""}`, name)
			})
			args := map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": method, "name": name}
			if method == "upsert" {
				args["body"] = ""
			}
			deps := BaseDeps{Client: client}
			tool := pullRequestArtifactTestTool(method)
			request := createMCPRequest(args)
			result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, mustMarshalJSON(t, result))
			assert.Equal(t, 2, calls)
			if method == "upsert" {
				assert.Contains(t, getTextResult(t, result).Text, `"id":9007199254740993`)
			}
		})
	}
}

func TestPullRequestArtifactRepositoryEscaping(t *testing.T) {
	calls := 0
	client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			assert.Equal(t, "/api/v3/repos/o%3Fquery/r%23fragment%252F", r.URL.EscapedPath())
			assert.Empty(t, r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"id":42}`))
			return
		}
		assert.Equal(t, "/api/v3/repositories/42/pulls/12/artifacts", r.URL.Path)
		_, _ = w.Write([]byte(`[]`))
	})
	deps := BaseDeps{Client: client}
	tool := PullRequestArtifactRead(translations.NullTranslationHelper)
	request := createMCPRequest(map[string]any{"owner": "o?query", "repo": "r#fragment%2F", "pull_number": 12, "method": "list"})
	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, mustMarshalJSON(t, result))
	assert.Equal(t, 2, calls)
}

func TestPullRequestArtifactInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		args         map[string]any
	}{
		{"missing body", "upsert", nil},
		{"missing name", "upsert", map[string]any{"name": "", "body": ""}},
		{"null body", "upsert", map[string]any{"body": nil}},
		{"blank name", "delete", map[string]any{"name": " "}},
		{"dot name", "delete", map[string]any{"name": "."}},
		{"traversal name", "delete", map[string]any{"name": ".."}},
		{"slash name", "delete", map[string]any{"name": "../x"}},
		{"backslash name", "delete", map[string]any{"name": `a\b`}},
		{"delete body", "delete", map[string]any{"body": ""}},
		{"delete commit", "delete", map[string]any{"commit_oid": "abc"}},
		{"negative pull", "list", map[string]any{"pull_number": -1}},
		{"missing pull", "list", map[string]any{"pull_number": 0}},
		{"empty owner", "list", map[string]any{"owner": " "}},
		{"traversal repo", "list", map[string]any{"repo": ".."}},
		{"slash owner", "list", map[string]any{"owner": "a/b"}},
		{"negative page", "list", map[string]any{"page": -1}},
		{"negative size", "list", map[string]any{"perPage": -1}},
		{"large size", "list", map[string]any{"perPage": 101}},
		{"unknown read method", "list", map[string]any{"method": "get"}},
		{"unknown write method", "upsert", map[string]any{"method": "create", "body": ""}},
		{"missing method", "list", map[string]any{"method": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusInternalServerError)
			})
			args := map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": tc.method}
			if tc.method != "list" {
				args["name"] = "report"
			}
			maps.Copy(args, tc.args)
			deps := BaseDeps{Client: client}
			tool := pullRequestArtifactTestTool(tc.method)
			session := pullRequestArtifactTestSession(t, deps, inventory.ProtocolVersionMultiRoundTrip)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: args})
			require.NoError(t, err)
			assert.True(t, result.IsError, mustMarshalJSON(t, result))
			request := createMCPRequest(args)
			direct, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.True(t, direct.IsError, mustMarshalJSON(t, direct))
			assert.Zero(t, calls)
		})
	}
}

func TestPullRequestArtifactLockdown(t *testing.T) {
	for _, method := range []string{"list", "upsert", "delete"} {
		t.Run(method, func(t *testing.T) {
			deps := BaseDeps{Flags: FeatureFlags{LockdownMode: true}}
			tool := pullRequestArtifactTestTool(method)
			args := map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": method}
			if method != "list" {
				args["name"] = "report"
			}
			if method == "upsert" {
				args["body"] = ""
			}
			request := createMCPRequest(args)
			result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getTextResult(t, result).Text, "lockdown")
		})
	}
}

func TestPullRequestArtifactFailures(t *testing.T) {
	for _, method := range []string{"list", "upsert", "delete"} {
		for _, status := range []int{400, 401, 403, 404, 409, 422, 429, 500, 503} {
			t.Run(method+"/"+http.StatusText(status), func(t *testing.T) {
				calls := 0
				client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					if calls == 1 {
						_, _ = w.Write([]byte(`{"id":42}`))
						return
					}
					if status == http.StatusTooManyRequests {
						w.Header().Set("Retry-After", "60")
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"message":"artifact access failed"}`))
				})
				deps := BaseDeps{Client: client}
				tool := pullRequestArtifactTestTool(method)
				args := map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": method}
				if method != "list" {
					args["name"] = "report"
				}
				if method == "upsert" {
					args["body"] = ""
				}
				request := createMCPRequest(args)
				result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "artifact")
				if status == http.StatusTooManyRequests {
					assert.Contains(t, getTextResult(t, result).Text, "Retry after")
				}
				assert.Equal(t, 2, calls, "errors must not cause retries or fallback requests")
			})
		}
	}
}

func TestPullRequestArtifactRepositoryFailures(t *testing.T) {
	for _, response := range []struct {
		status int
		body   string
	}{
		{404, `{"message":"Not Found"}`},
		{200, `{}`},
		{200, `null`},
	} {
		t.Run(response.body, func(t *testing.T) {
			calls := 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, "/api/v3/repos/o/r", r.URL.Path)
				w.WriteHeader(response.status)
				_, _ = w.Write([]byte(response.body))
			})
			deps := BaseDeps{Client: client}
			tool := PullRequestArtifactRead(translations.NullTranslationHelper)
			request := createMCPRequest(map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": "list"})
			result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Equal(t, 1, calls, "lookup failure must prevent artifact access")
		})
	}
}

func TestPullRequestArtifactRepositoryGate(t *testing.T) {
	for _, method := range []string{"list", "upsert", "delete"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch r.URL.Path {
				case "/api/v3/repos/o/enabled":
					_, _ = w.Write([]byte(`{"id":42}`))
				case "/api/v3/repos/o/disabled":
					_, _ = w.Write([]byte(`{"id":43}`))
				default:
					if strings.HasPrefix(r.URL.Path, "/api/v3/repositories/43/") {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message":"Not Found"}`))
						return
					}
					require.True(t, strings.HasPrefix(r.URL.Path, "/api/v3/repositories/42/"))
					artifact := `{"id":"a","number":1,"name":"report","body":""}`
					switch method {
					case "list":
						_, _ = w.Write([]byte("[" + artifact + "]"))
					case "upsert":
						_, _ = w.Write([]byte(artifact))
					case "delete":
						w.WriteHeader(http.StatusNoContent)
					}
				}
			})
			deps := BaseDeps{Client: client}
			ctx := inventory.WithFeatureState(ContextWithDeps(context.Background(), deps), featureCheckerFor(FeatureFlagPullRequestArtifacts))
			tool := pullRequestArtifactTestTool(method)
			for _, repo := range []string{"enabled", "disabled", "enabled"} {
				args := map[string]any{"owner": "o", "repo": repo, "pull_number": 12, "method": method}
				if method != "list" {
					args["name"] = "report"
				}
				if method == "upsert" {
					args["body"] = ""
				}
				request := createMCPRequest(args)
				result, err := tool.Handler(deps)(ctx, &request)
				require.NoError(t, err)
				assert.Equal(t, repo == "disabled", result.IsError, mustMarshalJSON(t, result))
			}
			assert.Equal(t, 6, calls)
		})
	}
}

func TestPullRequestArtifactResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		name, method, response string
		status                 int
	}{
		{"null list", "list", "null", 200},
		{"missing ID", "list", `[{"number":1,"name":"report","body":""}]`, 200},
		{"invalid ID", "list", `[{"id":{},"number":1,"name":"report","body":""}]`, 200},
		{"missing number", "upsert", `{"id":"a","name":"report","body":""}`, 200},
		{"mismatched name", "upsert", `{"id":"a","number":1,"name":"other","body":""}`, 200},
		{"unexpected success", "upsert", `{}`, 202},
		{"unexpected delete", "delete", `{}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				calls++
				if calls == 1 {
					_, _ = w.Write([]byte(`{"id":42}`))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			})
			deps := BaseDeps{Client: client}
			tool := pullRequestArtifactTestTool(tc.method)
			args := map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": tc.method}
			if tc.method != "list" {
				args["name"] = "report"
			}
			if tc.method == "upsert" {
				args["body"] = ""
			}
			request := createMCPRequest(args)
			result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Equal(t, 2, calls)
		})
	}
}

func TestPullRequestArtifactSanitization(t *testing.T) {
	calls := 0
	client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"id":42,"private":false}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":123,"number":1,"name":"<b>report</b>","body":"<p>sa\u200bfe</p>","commit_oid":"abc123","creator":{"login":"<b>octocat</b>","id":7},"created_at":"2026-11-13T00:00:00Z","updated_at":"2026-11-13T00:00:00Z"}]`))
	})
	deps := BaseDeps{Client: client, featureChecker: featureCheckerFor(FeatureFlagIFCLabels)}
	session := pullRequestArtifactTestSession(t, deps, inventory.ProtocolVersionMultiRoundTrip)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "pull_request_artifact_read", Arguments: map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": "list"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, mustMarshalJSON(t, result))
	var output PullRequestArtifactReadOutput
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
	require.Len(t, output.List.Artifacts, 1)
	artifact := output.List.Artifacts[0]
	assert.Equal(t, "report", artifact.Name)
	assert.Equal(t, "<p>safe</p>", artifact.Body)
	require.NotNil(t, artifact.Creator)
	assert.Equal(t, "octocat", artifact.Creator.Login)
	assert.Equal(t, int64(7), artifact.Creator.ID)
	assert.Equal(t, "abc123", *artifact.CommitOID)
	assert.Equal(t, "2026-11-13T00:00:00Z", *artifact.CreatedAt)
	assert.Equal(t, "2026-11-13T00:00:00Z", *artifact.UpdatedAt)
	assert.JSONEq(t, mustMarshalJSON(t, ifc.PublicUntrusted()), mustMarshalJSON(t, result.Meta["ifc"]))
	assert.Equal(t, 2, calls)
}
