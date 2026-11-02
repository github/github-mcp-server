package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	gogithub "github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func artifactTestTools() []inventory.ServerTool {
	t := translations.NullTranslationHelper
	return []inventory.ServerTool{IssueArtifactRead(t), IssueArtifactWrite(t)}
}

func artifactTestOperation(index int) (inventory.ServerTool, string) {
	method := []string{"create", "update", "delete", "list", "get"}[index]
	tool := IssueArtifactWrite(translations.NullTranslationHelper)
	if method == "list" || method == "get" {
		tool = IssueArtifactRead(translations.NullTranslationHelper)
	}
	return tool, method
}

func TestIssueArtifactTools(t *testing.T) {
	session := artifactTestSession(t, artifactTestTools(), BaseDeps{})
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 2)
	registered := make(map[string]*mcp.Tool, len(list.Tools))
	for _, tool := range list.Tools {
		registered[tool.Name] = tool
	}
	for _, tool := range artifactTestTools() {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			wireTool := registered[tool.Tool.Name]
			require.NotNil(t, wireTool)
			require.NoError(t, toolsnaps.Test(tool.Tool.Name+"_ff_"+FeatureFlagIssueArtifacts, wireTool))
			assert.Equal(t, ToolsetMetadataIssues.ID, tool.Toolset.ID)
			assert.Equal(t, []inventory.FeatureFlag{FeatureFlagIssueArtifacts}, tool.FeatureRule.Features())
			schema := tool.Tool.InputSchema.(*jsonschema.Schema)
			assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "issue_number"})
			assert.NotNil(t, wireTool.OutputSchema)
			assert.Equal(t, tool.Tool.Name == "issue_artifact_read", tool.IsReadOnly())
			assert.Equal(t, tool.Tool.Name == "issue_artifact_write", *tool.Tool.Annotations.DestructiveHint)
			methods := []string{"list", "get"}
			if tool.Tool.Name == "issue_artifact_write" {
				methods = []string{"create", "update", "delete"}
			}
			var actualMethods []string
			for _, method := range schema.Properties["method"].Enum {
				actualMethods = append(actualMethods, method.(string))
			}
			assert.ElementsMatch(t, methods, actualMethods)
			assert.ElementsMatch(t, []any{"generic", "link", "plan", "embedded_html"}, schema.Properties["type"].Enum)
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
		{"enabled", true, false, 2},
		{"read only", true, true, 1},
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
				if strings.HasPrefix(tool.Tool.Name, "issue_artifact_") {
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

func artifactTestSession(t *testing.T, tools []inventory.ServerTool, deps BaseDeps) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "issue-artifacts-test", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	for _, tool := range tools {
		tool.RegisterFunc(server, deps)
	}
	return connectCommentVisibilityClient(t, server, inventory.ProtocolVersionMultiRoundTrip)
}

func callArtifactTool(t *testing.T, tool inventory.ServerTool, deps BaseDeps, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	session := artifactTestSession(t, []inventory.ServerTool{tool}, deps)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: args})
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
			client, err := gogithub.NewClient(
				gogithub.WithHTTPClient(server.Client()),
				gogithub.WithEnterpriseURLs(server.URL+"/api/v3/", server.URL+"/api/uploads/"),
			)
			require.NoError(t, err)
			args := map[string]any{"owner": "o", "repo": "r", "issue_number": 12}
			maps.Copy(args, tc.args)
			tool, method := artifactTestOperation(tc.index)
			args["method"] = method
			result := callArtifactTool(t, tool, BaseDeps{Client: client}, args)
			require.False(t, result.IsError, "%s", getTextResult(t, result).Text)
			assert.Equal(t, 1, calls)
			text := getTextResult(t, result).Text
			var structured any
			require.NoError(t, json.Unmarshal([]byte(text), &structured))
			assert.Equal(t, structured, result.StructuredContent)
			switch tc.index {
			case 2:
				assert.JSONEq(t, `{"deleted":{"issue_number":12,"artifact_number":3,"message":"artifact deleted"}}`, text)
			case 3:
				var readOutput IssueArtifactReadOutput
				require.NoError(t, json.Unmarshal([]byte(text), &readOutput))
				require.Nil(t, readOutput.Artifact)
				output := readOutput.List
				require.NotNil(t, output)
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
				var output struct {
					Artifact *IssueArtifact `json:"artifact"`
				}
				require.NoError(t, json.Unmarshal([]byte(text), &output))
				require.NotNil(t, output.Artifact)
				assert.Equal(t, int64(999), output.Artifact.ID)
				assert.Equal(t, 3, output.Artifact.Number)
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
		{"instruction create", 0, map[string]any{"type": "instruction", "name": "Instructions", "body": "content"}},
		{"skill create", 0, map[string]any{"type": "skill", "name": "Skill", "body": "content"}},
		{"mcp config create", 0, map[string]any{"type": "mcp_config", "name": "Config", "body": "content"}},
		{"reference create", 0, map[string]any{"type": "reference", "name": "Reference", "body": "content"}},
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
		{"instruction filter", 3, map[string]any{"type": "instruction"}},
		{"skill filter", 3, map[string]any{"type": "skill"}},
		{"mcp config filter", 3, map[string]any{"type": "mcp_config"}},
		{"reference filter", 3, map[string]any{"type": "reference"}},
		{"missing read method", 3, map[string]any{"method": nil}},
		{"missing write method", 0, map[string]any{"method": nil}},
		{"unknown read method", 3, map[string]any{"method": "create"}},
		{"unknown write method", 0, map[string]any{"method": "get"}},
		{"get without number", 4, nil},
		{"update without number", 1, map[string]any{"body": ""}},
		{"delete without number", 2, nil},
		{"create without type", 0, map[string]any{"name": "Plan", "body": ""}},
		{"create without name", 0, map[string]any{"type": "plan", "body": ""}},
		{"create with null body", 0, map[string]any{"type": "plan", "name": "Plan", "body": nil}},
		{"create with number", 0, map[string]any{"artifact_number": 3, "type": "plan", "name": "Plan", "body": ""}},
		{"delete with body", 2, map[string]any{"artifact_number": 3, "body": ""}},
		{"delete with name", 2, map[string]any{"artifact_number": 3, "name": "Plan"}},
		{"delete with type", 2, map[string]any{"artifact_number": 3, "type": "plan"}},
		{"get with page", 4, map[string]any{"artifact_number": 3, "page": 1}},
		{"get with page size", 4, map[string]any{"artifact_number": 3, "perPage": 1}},
		{"get with filter", 4, map[string]any{"artifact_number": 3, "type": "plan"}},
		{"list with number", 3, map[string]any{"artifact_number": 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool, method := artifactTestOperation(tc.index)
			args := map[string]any{"owner": "o", "repo": "r", "issue_number": 12, "method": method}
			maps.Copy(args, tc.args)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			client, err := gogithub.NewClient(
				gogithub.WithHTTPClient(server.Client()),
				gogithub.WithEnterpriseURLs(server.URL+"/", server.URL+"/"),
			)
			require.NoError(t, err)
			result := callArtifactTool(t, tool, BaseDeps{Client: client}, args)
			assert.True(t, result.IsError)
			assert.Zero(t, calls, "invalid arguments must not make API requests")
		})
	}
}

func TestIssueArtifactTypes(t *testing.T) {
	for _, artifactType := range []string{"generic", "link", "plan", "embedded_html"} {
		for _, method := range []string{"create", "list"} {
			t.Run(artifactType+"/"+method, func(t *testing.T) {
				artifact := IssueArtifact{ID: 999, Number: 3, Type: artifactType, Name: "Artifact", Body: "content"}
				inputBody := "content"
				if artifactType == "embedded_html" {
					inputBody = "<p>content</p>"
				}
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.Equal(t, "/api/v3/repos/o/r/issues/12/artifacts", r.URL.Path)
					w.Header().Set("Content-Type", "application/json")
					if method == "create" {
						assert.Equal(t, http.MethodPost, r.Method)
						var body map[string]string
						require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
						assert.Equal(t, map[string]string{"type": artifactType, "name": "Artifact", "body": inputBody}, body)
						w.WriteHeader(http.StatusCreated)
						require.NoError(t, json.NewEncoder(w).Encode(artifact))
					} else {
						assert.Equal(t, http.MethodGet, r.Method)
						assert.Equal(t, artifactType, r.URL.Query().Get("type"))
						require.NoError(t, json.NewEncoder(w).Encode([]IssueArtifact{artifact}))
					}
				}))
				defer server.Close()
				client, err := gogithub.NewClient(
					gogithub.WithHTTPClient(server.Client()),
					gogithub.WithEnterpriseURLs(server.URL+"/", server.URL+"/"),
				)
				require.NoError(t, err)
				tool := IssueArtifactRead(translations.NullTranslationHelper)
				args := map[string]any{"owner": "o", "repo": "r", "issue_number": 12, "method": method, "type": artifactType}
				if method == "create" {
					tool = IssueArtifactWrite(translations.NullTranslationHelper)
					args["name"], args["body"] = "Artifact", inputBody
				}
				result := callArtifactTool(t, tool, BaseDeps{Client: client}, args)
				require.False(t, result.IsError, "%s", getTextResult(t, result).Text)
				assert.Equal(t, 1, calls)
				var output IssueArtifactReadOutput
				require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
				if method == "create" {
					require.NotNil(t, output.Artifact)
					assert.Equal(t, artifact, *output.Artifact)
				} else {
					require.NotNil(t, output.List)
					assert.Equal(t, []IssueArtifact{artifact}, output.List.Artifacts)
				}
			})
		}
	}
}

func TestIssueArtifactListPagination(t *testing.T) {
	for _, withLink := range []bool{false, true} {
		name := "without Link headers"
		if withLink {
			name = "with Link headers"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v3/repos/o/r/issues/12/artifacts", r.URL.Path)
				assert.Equal(t, "2", r.URL.Query().Get("per_page"))
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Query().Get("page") {
				case "1":
					if withLink {
						w.Header().Set("Link", `</api/v3/repos/o/r/issues/12/artifacts?page=2&per_page=2>; rel="next"`)
					}
					_, _ = w.Write([]byte(`[{"id":999,"number":3,"type":"plan","name":"First","body":"first"},{"id":1000,"number":4,"type":"generic","name":"Second","body":"second"}]`))
				case "2":
					if withLink {
						w.Header().Set("Link", `</api/v3/repos/o/r/issues/12/artifacts?page=1&per_page=2>; rel="prev"`)
					}
					_, _ = w.Write([]byte(`[{"id":1001,"number":5,"type":"link","name":"Third","body":"third"}]`))
				default:
					t.Errorf("unexpected page: %s", r.URL.Query().Get("page"))
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			client, err := gogithub.NewClient(
				gogithub.WithHTTPClient(server.Client()),
				gogithub.WithEnterpriseURLs(server.URL+"/", server.URL+"/"),
			)
			require.NoError(t, err)
			for page := 1; page <= 2; page++ {
				result := callArtifactTool(t, IssueArtifactRead(translations.NullTranslationHelper), BaseDeps{Client: client},
					map[string]any{"owner": "o", "repo": "r", "issue_number": 12, "method": "list", "page": page, "perPage": 2})
				require.False(t, result.IsError, "%s", getTextResult(t, result).Text)
				var output IssueArtifactReadOutput
				require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
				require.NotNil(t, output.List)
				assert.Equal(t, page, output.List.Page)
				assert.Equal(t, 2, output.List.PerPage)
				assert.Len(t, output.List.Artifacts, 3-page)
				assert.Equal(t, page, calls, "each tool call must fetch only the requested page")
			}
		})
	}
}

func TestIssueArtifactFailures(t *testing.T) {
	for index := range 5 {
		tool, method := artifactTestOperation(index)
		t.Run(method, func(t *testing.T) {
			args := map[string]any{"owner": "o", "repo": "r", "issue_number": 12, "method": method}
			switch method {
			case "create":
				args["name"], args["type"], args["body"] = "Plan", "plan", ""
			case "update":
				args["artifact_number"], args["body"] = 3, ""
			case "get", "delete":
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
					client, err := gogithub.NewClient(
						gogithub.WithHTTPClient(server.Client()),
						gogithub.WithEnterpriseURLs(server.URL+"/", server.URL+"/"),
					)
					require.NoError(t, err)
					result := callArtifactTool(t, tool, BaseDeps{Client: client}, args)
					assert.True(t, result.IsError)
					assert.Contains(t, getTextResult(t, result).Text, "artifact")
					assert.Equal(t, 1, calls, "requests must not be retried")
				})
			}
		})
	}
}
