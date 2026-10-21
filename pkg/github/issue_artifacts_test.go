package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func artifactTestTools() []inventory.ServerTool {
	t := translations.NullTranslationHelper
	return []inventory.ServerTool{CreateIssueArtifact(t), UpdateIssueArtifact(t), DeleteIssueArtifact(t), ListIssueArtifacts(t), GetIssueArtifact(t)}
}

func TestIssueArtifactTools(t *testing.T) {
	for _, tool := range artifactTestTools() {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			require.NoError(t, toolsnaps.Test(tool.Tool.Name+"_ff_"+FeatureFlagIssueArtifacts, tool.Tool))
			assert.Equal(t, ToolsetMetadataIssues, tool.Toolset)
			assert.Equal(t, []inventory.FeatureFlag{FeatureFlagIssueArtifacts}, tool.FeatureRule.Features())
			schema := tool.Tool.InputSchema.(*jsonschema.Schema)
			assert.Subset(t, schema.Required, []string{"owner", "repo", "issue_number"})
			assert.NotNil(t, tool.Tool.OutputSchema)
			assert.Equal(t, strings.HasPrefix(tool.Tool.Name, "get_") || strings.HasPrefix(tool.Tool.Name, "list_"), tool.IsReadOnly())
		})
	}
	assert.False(t, ResolveFeatureFlags(nil, true)[FeatureFlagIssueArtifacts])
	assert.True(t, ResolveFeatureFlags([]string{FeatureFlagIssueArtifacts}, false)[FeatureFlagIssueArtifacts])
	for _, tc := range []struct {
		name     string
		enabled  bool
		readOnly bool
		count    int
	}{
		{"default", false, false, 0},
		{"enabled", true, false, 5},
		{"read only", true, true, 2},
		{"read only default", false, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var flags []inventory.FeatureFlag
			if tc.enabled {
				flags = append(flags, FeatureFlagIssueArtifacts)
			}
			inv, err := inventory.NewBuilder().SetTools(AllTools(translations.NullTranslationHelper)).
				WithReadOnly(tc.readOnly).WithFeatureChecker(featureCheckerFor(flags...)).Build()
			require.NoError(t, err)
			count := 0
			for _, tool := range inv.AvailableTools(context.Background()) {
				if strings.HasSuffix(tool.Tool.Name, "_issue_artifact") || tool.Tool.Name == "list_issue_artifacts" {
					count++
					if tc.readOnly {
						assert.True(t, tool.IsReadOnly())
					}
				}
			}
			assert.Equal(t, tc.count, count)
		})
	}
}

func callArtifactTool(t *testing.T, tool inventory.ServerTool, deps BaseDeps, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	request := createMCPRequest(args)
	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestIssueArtifactRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		index  int
		method string
		path   string
		args   map[string]any
		body   string
		status int
		result string
	}{
		{"create empty body", 0, "POST", "/repos/o/r/issues/12/artifacts", map[string]any{"type": "plan", "name": "Plan", "body": ""}, `{"type":"plan","name":"Plan","body":""}`, 201, `{"id":999,"number":3,"type":"plan","name":"Plan","body":""}`},
		{"update body only", 1, "PATCH", "/repos/o/r/issues/12/artifacts/3", map[string]any{"artifact_number": 3, "body": ""}, `{"body":""}`, 200, `{"id":999,"number":3,"type":"plan","name":"Plan","body":""}`},
		{"update name only", 1, "PATCH", "/repos/o/r/issues/12/artifacts/3", map[string]any{"artifact_number": 3, "name": "Renamed"}, `{"name":"Renamed"}`, 200, `{"id":999,"number":3,"type":"plan","name":"Renamed","body":"unchanged"}`},
		{"update both", 1, "PATCH", "/repos/o/r/issues/12/artifacts/3", map[string]any{"artifact_number": 3, "name": "Renamed", "body": "new"}, `{"name":"Renamed","body":"new"}`, 200, `{"id":999,"number":3,"type":"plan","name":"Renamed","body":"new"}`},
		{"delete", 2, "DELETE", "/repos/o/r/issues/12/artifacts/3", map[string]any{"artifact_number": 3}, "", 204, ""},
		{"list default", 3, "GET", "/repos/o/r/issues/12/artifacts?page=1&per_page=30", nil, "", 200, `[{"id":999,"number":3,"type":"plan","name":"Plan","body":"full body"}]`},
		{"list explicit page", 3, "GET", "/repos/o/r/issues/12/artifacts?page=2&per_page=1&type=plan", map[string]any{"page": 2, "perPage": 1, "type": "plan"}, "", 200, `[{"id":999,"number":3,"type":"plan","name":"Plan","body":"full body"}]`},
		{"list empty", 3, "GET", "/repos/o/r/issues/12/artifacts?page=1&per_page=30", nil, "", 200, `[]`},
		{"get envelope and future type", 4, "GET", "/repos/o/r/issues/12/artifacts/3", map[string]any{"artifact_number": 3}, "", 200, `{"artifact":{"id":999,"number":3,"type":"future_type","name":"Plan","body":""}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, tc.method, r.Method)
				assert.Equal(t, "/api/v3"+tc.path, r.URL.RequestURI())
				if tc.body != "" {
					var body json.RawMessage
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.JSONEq(t, tc.body, string(body))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.result))
			}))
			defer server.Close()
			client := mustNewGHClient(t, server.Client())
			baseURL, err := url.Parse(server.URL + "/api/v3/")
			require.NoError(t, err)
			client.BaseURL = baseURL
			args := map[string]any{"owner": "o", "repo": "r", "issue_number": 12}
			for key, value := range tc.args {
				args[key] = value
			}
			tool := artifactTestTools()[tc.index]
			result := callArtifactTool(t, tool, BaseDeps{Client: client}, args)
			require.False(t, result.IsError, "%s", getTextResult(t, result).Text)
			assert.Equal(t, 1, calls)
			text := getTextResult(t, result).Text
			switch tc.index {
			case 2:
				assert.JSONEq(t, `{"issue_number":12,"artifact_number":3,"message":"artifact deleted"}`, text)
			case 3:
				var output ListIssueArtifactsOutput
				require.NoError(t, json.Unmarshal([]byte(text), &output))
				if tc.name == "list empty" {
					require.NotNil(t, output.Artifacts)
					assert.Empty(t, output.Artifacts)
				} else {
					require.Len(t, output.Artifacts, 1)
					assert.Equal(t, "full body", output.Artifacts[0].Body)
				}
				if tc.name == "list explicit page" {
					assert.Equal(t, 2, output.Page)
					assert.Equal(t, 1, output.PerPage)
				} else {
					assert.Equal(t, 1, output.Page)
					assert.Equal(t, 30, output.PerPage)
				}
			default:
				var output IssueArtifact
				require.NoError(t, json.Unmarshal([]byte(text), &output))
				assert.Equal(t, int64(999), output.ID)
				assert.Equal(t, 3, output.Number)
			}
		})
	}
}

func TestIssueArtifactInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index int
		args  map[string]any
	}{
		{"missing body", 0, map[string]any{"type": "plan", "name": "Plan"}},
		{"empty name", 0, map[string]any{"type": "plan", "name": "  ", "body": ""}},
		{"unsupported type", 0, map[string]any{"type": "unknown", "name": "Plan", "body": ""}},
		{"empty update", 1, map[string]any{"artifact_number": 3}},
		{"null update", 1, map[string]any{"artifact_number": 3, "body": nil}},
		{"immutable type", 1, map[string]any{"artifact_number": 3, "type": "plan", "body": ""}},
		{"blank replacement", 1, map[string]any{"artifact_number": 3, "name": " "}},
		{"global id not number", 4, map[string]any{"id": 999}},
		{"negative number", 4, map[string]any{"artifact_number": -1}},
		{"zero number", 2, map[string]any{"artifact_number": 0}},
		{"negative issue", 3, map[string]any{"issue_number": -1}},
		{"empty owner", 3, map[string]any{"owner": " "}},
		{"path traversal", 3, map[string]any{"repo": ".."}},
		{"slash in owner", 3, map[string]any{"owner": "o/r"}},
		{"negative page", 3, map[string]any{"page": -1}},
		{"zero page", 3, map[string]any{"page": 0}},
		{"too large page size", 3, map[string]any{"perPage": 101}},
		{"zero page size", 3, map[string]any{"perPage": 0}},
		{"unsupported filter", 3, map[string]any{"type": "unknown"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{"owner": "o", "repo": "r", "issue_number": 12}
			for key, value := range tc.args {
				args[key] = value
			}
			result := callArtifactTool(t, artifactTestTools()[tc.index], BaseDeps{}, args)
			assert.True(t, result.IsError)
		})
	}
}

func TestIssueArtifactFailures(t *testing.T) {
	for _, tool := range artifactTestTools() {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			args := map[string]any{"owner": "o", "repo": "r", "issue_number": 12}
			switch tool.Tool.Name {
			case "create_issue_artifact":
				args["name"], args["type"], args["body"] = "Plan", "plan", ""
			case "update_issue_artifact":
				args["artifact_number"], args["body"] = 3, ""
			case "get_issue_artifact", "delete_issue_artifact":
				args["artifact_number"] = 3
			}
			t.Run("lockdown", func(t *testing.T) {
				result := callArtifactTool(t, tool, BaseDeps{Flags: FeatureFlags{LockdownMode: true}}, args)
				assert.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "lockdown")
			})
			for _, status := range []int{401, 403, 404, 422, 500} {
				t.Run(http.StatusText(status), func(t *testing.T) {
					calls := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						calls++
						w.WriteHeader(status)
						_, _ = w.Write([]byte(`{"message":"artifact access failed"}`))
					}))
					defer server.Close()
					client := mustNewGHClient(t, server.Client())
					baseURL, err := url.Parse(server.URL + "/")
					require.NoError(t, err)
					client.BaseURL = baseURL
					result := callArtifactTool(t, tool, BaseDeps{Client: client}, args)
					assert.True(t, result.IsError)
					assert.Contains(t, getTextResult(t, result).Text, "artifact")
					assert.Equal(t, 1, calls, "requests must not be retried")
				})
			}
		})
	}
}
