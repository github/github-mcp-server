package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type governanceWireCase struct {
	tool     inventory.ServerTool
	args     map[string]any
	status   int
	body     string
	text     string
	output   string
	contains string
	route    func(*testing.T, *http.Request) string
}

func governanceLegacyTool(name string) inventory.ServerTool {
	tr := translations.NullTranslationHelper
	switch name {
	case "get_label":
		return getLabelLegacy(tr)
	case "list_label":
		return listLabelsLegacy(tr)
	case "label_write":
		return labelWriteLegacy(tr)
	case "custom_properties_read":
		return customPropertiesReadLegacy(tr)
	case "custom_properties_write":
		return customPropertiesWriteLegacy(tr)
	case "repository_ruleset_read":
		return repositoryRulesetReadLegacy(tr)
	case "create_repository_ruleset":
		return createRepositoryRulesetLegacy(tr)
	default:
		panic("unexpected governance tool: " + name)
	}
}

func assertGovernanceWireText(t *testing.T, modern bool, legacyText string, result *mcp.CallToolResult) {
	t.Helper()
	actual := getTextResult(t, result).Text
	if modern && !result.IsError {
		require.NotNil(t, result.StructuredContent)
		assert.JSONEq(t, mustMarshalJSON(t, result.StructuredContent), actual)
		return
	}
	assert.Equal(t, legacyText, actual)
}

func assertGovernanceProjection(t *testing.T, tc governanceWireCase, output any) {
	t.Helper()
	object, ok := output.(map[string]any)
	require.True(t, ok, "modern output must have an object root")
	if tc.output != "" {
		assert.JSONEq(t, tc.output, mustMarshalJSON(t, output))
	}
	switch tc.tool.Tool.Name {
	case "get_label":
		assert.JSONEq(t, `{"name":"bug","color":"ff0000","description":"desc"}`, mustMarshalJSON(t, output))
	case "list_label":
		assert.JSONEq(t, `{"labels":[{"name":"bug","color":"ff0000","description":"desc"}],"totalCount":1}`, mustMarshalJSON(t, output))
	case "label_write":
		assert.Equal(t, tc.args["method"], object["method"])
		assert.Equal(t, tc.text, object["message"])
	case "custom_properties_read", "custom_properties_write":
		assert.Equal(t, tc.args["level"], object["level"])
		if tc.tool.Tool.Name == "custom_properties_write" {
			assert.Equal(t, "write", object["method"])
		} else {
			assert.Equal(t, "read", object["method"])
		}
		switch {
		case tc.args["level"] == "repository" && tc.tool.Tool.Name == "custom_properties_write":
			assert.Equal(t, tc.text, object["message"])
		case tc.args["level"] == "repository":
			assert.JSONEq(t, tc.text, mustMarshalJSON(t, object["values"]))
		default:
			var expected []map[string]any
			require.NoError(t, json.Unmarshal([]byte(tc.text), &expected))
			for _, definition := range expected {
				delete(definition, "custom_field")
			}
			assert.JSONEq(t, mustMarshalJSON(t, expected), mustMarshalJSON(t, object["definitions"]))
		}
	case "repository_ruleset_read":
		assert.Equal(t, tc.args["level"], object["level"])
		assert.Equal(t, tc.args["method"], object["method"])
		switch tc.args["method"] {
		case "get":
			assert.Equal(t, float64(7), object["ruleset"].(map[string]any)["id"])
		case "list":
			require.Len(t, object["rulesets"], 1)
			if tc.args["level"] == "enterprise" {
				assert.Equal(t, float64(1), object["total_count"])
			}
		case "get_rules_for_branch":
			assert.JSONEq(t, `[{"type":"creation","ruleset_id":0}]`, mustMarshalJSON(t, object["rules"]))
		case "get_rule_suite":
			assert.JSONEq(t, `{"id":11,"result":"pass"}`, mustMarshalJSON(t, object["rule_suite"]))
		}
	case "create_repository_ruleset":
		assert.JSONEq(t, tc.text, mustMarshalJSON(t, output))
	}
	for _, excluded := range []string{"url", "html_url", "_links", "node_id", "custom", "custom_field"} {
		assert.NotContains(t, mustMarshalJSON(t, output), `"`+excluded+`":`)
	}
}

func governanceTypedSession(t *testing.T, tool inventory.ServerTool, deps ToolDependencies, protocol string) (*mcp.ClientSession, *jsonschema.Resolved) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "governance", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	tool.RegisterFunc(server, deps)
	if protocol == "" || protocol == "unknown" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				switch request := req.(type) {
				case *mcp.ListToolsRequest:
					request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				case *mcp.CallToolRequest:
					request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				}
				return next(ctx, method, req)
			}
		})
	}
	version := protocol
	if version == "" || version == "unknown" {
		version = inventory.ProtocolVersionMultiRoundTrip
	}
	session := connectCommentVisibilityClient(t, server, version)
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	if protocol != inventory.ProtocolVersionMultiRoundTrip {
		assert.Nil(t, list.Tools[0].OutputSchema)
		return session, nil
	}
	require.NotNil(t, list.Tools[0].OutputSchema)
	require.NoError(t, toolsnaps.Test(tool.Tool.Name, tool.Tool))
	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, list.Tools[0].OutputSchema)), &schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	return session, resolved
}

func governanceWireDeps(t *testing.T, tc governanceWireCase) BaseDeps {
	t.Helper()
	var graphqlCalls int
	client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if tc.route != nil {
			body := tc.route(t, r)
			if body == "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			_, _ = io.WriteString(w, body)
			return
		}
		if r.URL.Path == "/repos/o/r" {
			_, _ = io.WriteString(w, `{"private":false}`)
			return
		}
		if r.URL.Path == "/graphql" {
			graphqlCalls++
			var body string
			switch tc.tool.Tool.Name {
			case "get_label":
				body = `{"data":{"repository":{"label":{"id":"L1","name":"bug","color":"ff0000","description":"desc"}}}}`
			case "list_label":
				body = `{"data":{"repository":{"labels":{"nodes":[{"id":"L1","name":"bug","color":"ff0000","description":"desc"}],"totalCount":1}}}}`
			case "label_write":
				switch tc.args["method"] {
				case "create":
					if graphqlCalls == 1 {
						body = `{"data":{"repository":{"id":"R1"}}}`
					} else {
						body = `{"data":{"createLabel":{"label":{"id":"L2","name":"feature"}}}}`
					}
				case "update":
					if graphqlCalls == 1 {
						body = `{"data":{"repository":{"label":{"id":"L1","name":"bug"}}}}`
					} else {
						body = `{"data":{"updateLabel":{"label":{"id":"L1","name":"defect"}}}}`
					}
				case "delete":
					if graphqlCalls == 1 {
						body = `{"data":{"repository":{"label":{"id":"L1","name":"bug"}}}}`
					} else {
						body = `{"data":{"deleteLabel":{"clientMutationId":"mutation-1"}}}`
					}
				}
			}
			_, _ = io.WriteString(w, body)
			return
		}
		w.WriteHeader(tc.status)
		_, _ = io.WriteString(w, tc.body)
	})}}
	deps := BaseDeps{Client: mustNewGHClient(t, client)}
	if tc.tool.Tool.Name == "get_label" || tc.tool.Tool.Name == "list_label" || tc.tool.Tool.Name == "label_write" {
		deps.GQLClient = githubv4.NewEnterpriseClient("https://api.github.com/graphql", client)
	}
	return deps
}

func governanceWireCases(t *testing.T) []governanceWireCase {
	t.Helper()
	rest := func(method, path, body string) func(*testing.T, *http.Request) string {
		return func(t *testing.T, request *http.Request) string {
			t.Helper()
			assert.Equal(t, method, request.Method)
			assert.Equal(t, path, request.URL.Path)
			if request.Body != nil {
				_, _ = io.Copy(io.Discard, request.Body)
			}
			return body
		}
	}
	tr := translations.NullTranslationHelper
	return []governanceWireCase{
		{tool: GetLabel(tr), args: map[string]any{"owner": "o", "repo": "r", "name": "bug"}, text: `{"id":"L1","name":"bug","color":"ff0000","description":"desc"}`},
		{tool: ListLabels(tr), args: map[string]any{"owner": "o", "repo": "r"}, text: `{"labels":[{"id":"L1","name":"bug","color":"ff0000","description":"desc"}],"totalCount":1}`},
		{tool: LabelWrite(tr), args: map[string]any{"method": "create", "owner": "o", "repo": "r", "name": "feature", "color": "abcdef"}, text: "label 'feature' created successfully"},
		{tool: LabelWrite(tr), args: map[string]any{"method": "update", "owner": "o", "repo": "r", "name": "bug", "new_name": "defect"}, text: "label 'defect' updated successfully"},
		{tool: LabelWrite(tr), args: map[string]any{"method": "delete", "owner": "o", "repo": "r", "name": "bug"}, text: "label 'bug' deleted successfully"},
		{tool: CustomPropertiesRead(tr), args: map[string]any{"level": "repository", "owner": "o", "repo": "r"}, status: http.StatusOK, body: `[{"property_name":"environment","value":"prod"}]`, text: `[{"property_name":"environment","value":"prod"}]`, route: rest(http.MethodGet, "/repos/o/r/properties/values", `[{"property_name":"environment","value":"prod"}]`)},
		{tool: CustomPropertiesRead(tr), args: map[string]any{"level": "organization", "org": "o"}, status: http.StatusOK, body: `[{"property_name":"environment","value_type":"string","custom_field":true}]`, text: `[{"custom_field":true,"property_name":"environment","value_type":"string"}]`, route: rest(http.MethodGet, "/orgs/o/properties/schema", `[{"property_name":"environment","value_type":"string","custom_field":true}]`)},
		{tool: CustomPropertiesRead(tr), args: map[string]any{"level": "enterprise", "enterprise": "e"}, status: http.StatusOK, body: `[{"property_name":"classification","value_type":"single_select"}]`, text: `[{"property_name":"classification","value_type":"single_select"}]`, route: rest(http.MethodGet, "/enterprises/e/properties/schema", `[{"property_name":"classification","value_type":"single_select"}]`)},
		{tool: CustomPropertiesWrite(tr), args: map[string]any{"level": "repository", "owner": "o", "repo": "r", "properties": []any{map[string]any{"property_name": "environment", "value": "prod"}}}, text: "Repository custom property values updated successfully", route: rest(http.MethodPatch, "/repos/o/r/properties/values", "")},
		{tool: CustomPropertiesWrite(tr), args: map[string]any{"level": "organization", "org": "o", "properties": []any{map[string]any{"property_name": "classification", "value_type": "string"}}}, text: `[{"property_name":"classification","value_type":"string"}]`, route: func(t *testing.T, request *http.Request) string {
			t.Helper()
			assert.Equal(t, "/orgs/o/properties/schema", request.URL.Path)
			switch request.Method {
			case http.MethodGet:
				return `[{"property_name":"classification","value_type":"string"}]`
			case http.MethodPatch:
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
				assert.Contains(t, string(body["properties"]), `"classification"`)
				return `[{"property_name":"classification","value_type":"string"}]`
			default:
				t.Fatalf("unexpected request method %s", request.Method)
				return ""
			}
		}},
		{tool: CustomPropertiesWrite(tr), args: map[string]any{"level": "enterprise", "enterprise": "e", "properties": []any{map[string]any{"property_name": "classification", "value_type": "single_select"}}}, text: `[{"property_name":"classification","value_type":"single_select"}]`, route: func(t *testing.T, request *http.Request) string {
			t.Helper()
			assert.Equal(t, "/enterprises/e/properties/schema", request.URL.Path)
			switch request.Method {
			case http.MethodGet:
				return `[]`
			case http.MethodPatch:
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
				assert.Contains(t, string(body["properties"]), `"classification"`)
				return `[{"property_name":"classification","value_type":"single_select"}]`
			default:
				t.Fatalf("unexpected request method %s", request.Method)
				return ""
			}
		}},
		{tool: CustomPropertiesWrite(tr), args: map[string]any{"level": "organization", "org": "o", "properties": []any{}}, text: `[]`, route: func(t *testing.T, request *http.Request) string {
			t.Helper()
			assert.Equal(t, "/orgs/o/properties/schema", request.URL.Path)
			switch request.Method {
			case http.MethodGet:
				return `[]`
			case http.MethodPatch:
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
				assert.JSONEq(t, `[]`, string(body["properties"]))
				return `[]`
			default:
				t.Fatalf("unexpected request method %s", request.Method)
				return ""
			}
		}},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "repository", "method": "get", "owner": "o", "repo": "r", "ruleset_id": 7}, status: http.StatusOK, body: `{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}`, text: `{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}`, route: rest(http.MethodGet, "/repos/o/r/rulesets/7", `{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "organization", "method": "get", "org": "o", "ruleset_id": 7}, status: http.StatusOK, body: `{"id":7,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[]}`, text: `{"id":7,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[]}`, output: `{"method":"get","level":"organization","ruleset":{"id":7,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[]}}`, route: rest(http.MethodGet, "/orgs/o/rulesets/7", `{"id":7,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[]}`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "enterprise", "method": "get", "enterprise": "e", "ruleset_id": 7}, status: http.StatusOK, body: `{"id":7,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[]}`, text: `{"id":7,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[]}`, output: `{"method":"get","level":"enterprise","ruleset":{"id":7,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[]}}`, route: rest(http.MethodGet, "/enterprises/e/rulesets/7", `{"id":7,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[]}`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "repository", "method": "list", "owner": "o", "repo": "r"}, status: http.StatusOK, body: `[{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}]`, text: `[{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}]`, route: rest(http.MethodGet, "/repos/o/r/rulesets", `[{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}]`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "repository", "method": "get_rules_for_branch", "owner": "o", "repo": "r", "branch": "main"}, status: http.StatusOK, body: `[{"type":"creation"}]`, contains: "Creation", route: rest(http.MethodGet, "/repos/o/r/rules/branches/main", `[{"type":"creation"}]`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "organization", "method": "get_rule_suite", "org": "o", "rule_suite_id": 11}, status: http.StatusOK, body: `{"id":11,"result":"pass","custom":{"retained":true}}`, text: `{"custom":{"retained":true},"id":11,"result":"pass"}`, route: rest(http.MethodGet, "/orgs/o/rulesets/rule-suites/11", `{"id":11,"result":"pass","custom":{"retained":true}}`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "organization", "method": "list", "org": "o"}, status: http.StatusOK, body: `[{"id":8,"name":"org rules"}]`, text: `[{"id":8,"name":"org rules","enforcement":"","source":""}]`, route: rest(http.MethodGet, "/orgs/o/rulesets", `[{"id":8,"name":"org rules"}]`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "enterprise", "method": "list", "enterprise": "e"}, status: http.StatusOK, body: `{"total_count":1,"rulesets":[{"id":9,"name":"enterprise rules"}]}`, text: `{"rulesets":[{"id":9,"name":"enterprise rules","enforcement":"","source":""}],"total_count":1}`, route: rest(http.MethodGet, "/enterprises/e/rulesets", `{"total_count":1,"rulesets":[{"id":9,"name":"enterprise rules"}]}`)},
		{tool: CreateRepositoryRuleset(tr), args: map[string]any{"level": "repository", "owner": "o", "repo": "r", "name": "main", "enforcement": "active", "rules": []any{map[string]any{"type": "creation"}}}, status: http.StatusCreated, body: `{"id":8,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[{"type":"creation"}]}`, text: `{"id":8,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[{"type":"creation"}]}`, route: rest(http.MethodPost, "/repos/o/r/rulesets", `{"id":8,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[{"type":"creation"}]}`)},
		{tool: CreateRepositoryRuleset(tr), args: map[string]any{"level": "organization", "org": "o", "name": "org-main", "enforcement": "active", "rules": []any{map[string]any{"type": "creation"}}}, status: http.StatusCreated, body: `{"id":8,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[{"type":"creation"}]}`, text: `{"id":8,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[{"type":"creation"}]}`, output: `{"id":8,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[{"type":"creation"}]}`, route: rest(http.MethodPost, "/orgs/o/rulesets", `{"id":8,"name":"org-main","target":"branch","source":"org:o","enforcement":"active","rules":[{"type":"creation"}]}`)},
		{tool: CreateRepositoryRuleset(tr), args: map[string]any{"level": "enterprise", "enterprise": "e", "name": "enterprise-main", "enforcement": "active", "rules": []any{map[string]any{"type": "creation"}}}, status: http.StatusCreated, body: `{"id":8,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[{"type":"creation"}]}`, text: `{"id":8,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[{"type":"creation"}]}`, output: `{"id":8,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[{"type":"creation"}]}`, route: rest(http.MethodPost, "/enterprises/e/rulesets", `{"id":8,"name":"enterprise-main","target":"branch","source":"enterprise:e","enforcement":"active","rules":[{"type":"creation"}]}`)},
	}
}

func TestTypedGovernanceWireOutputs(t *testing.T) {
	protocols := []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"}
	for _, protocol := range protocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			for _, tc := range governanceWireCases(t) {
				t.Run(tc.tool.Tool.Name+"/"+tc.text, func(t *testing.T) {
					session, schema := governanceTypedSession(t, tc.tool, governanceWireDeps(t, tc), protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
					require.NoError(t, err)
					require.False(t, result.IsError, mustMarshalJSON(t, result))
					require.Len(t, result.Content, 1)
					legacy := governanceLegacyTool(tc.tool.Tool.Name)
					legacyDeps := governanceWireDeps(t, tc)
					legacyResult, err := legacy.Handler(legacyDeps)(ContextWithDeps(context.Background(), legacyDeps), new(createMCPRequest(tc.args)))
					require.NoError(t, err)
					legacyText := getTextResult(t, legacyResult).Text
					assertGovernanceWireText(t, schema != nil, legacyText, result)
					switch {
					case tc.contains != "":
						assert.Contains(t, legacyText, tc.contains)
					case json.Valid([]byte(tc.text)):
						assert.JSONEq(t, tc.text, legacyText)
					default:
						assert.Equal(t, tc.text, legacyText)
					}
					if schema == nil {
						assert.Nil(t, result.StructuredContent)
						return
					}
					require.NotNil(t, result.StructuredContent)
					var output any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					require.NoError(t, schema.Validate(output))
					assertGovernanceProjection(t, tc, output)
				})
			}
		})
	}
}

func TestRuleSuiteOutputFidelity(t *testing.T) {
	for _, sourceID := range []string{"null", "0", "7"} {
		for _, method := range []string{"get_rule_suite", "list_rule_suites"} {
			for _, level := range []string{"repository", "organization"} {
				t.Run(level+"/"+method+"/"+sourceID, func(t *testing.T) {
					suite := `{"id":11,"result":"pass","evaluation_result":"fail","rule_evaluations":[{"rule_source":{"type":"protected_branch","id":` + sourceID + `},"result":"pass","rule_type":"pull_request"}]}`
					body, field := suite, "rule_suite"
					if method == "list_rule_suites" {
						body, field = "["+suite+"]", "rule_suites"
					}
					tc := governanceWireCase{
						tool: RepositoryRulesetRead(translations.NullTranslationHelper),
						args: map[string]any{"method": method, "level": level, "owner": "o", "repo": "r", "org": "o", "rule_suite_id": 11},
						route: func(t *testing.T, r *http.Request) string {
							t.Helper()
							assert.Equal(t, http.MethodGet, r.Method)
							path := "/repos/o/r/rulesets/rule-suites"
							if level == "organization" {
								path = "/orgs/o/rulesets/rule-suites"
							}
							if method == "get_rule_suite" {
								path += "/11"
							}
							assert.Equal(t, path, r.URL.Path)
							return body
						},
					}
					for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
						session, schema := governanceTypedSession(t, tc.tool, governanceWireDeps(t, tc), protocol)
						result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
						require.NoError(t, err)
						require.False(t, result.IsError)
						var legacy any
						require.NoError(t, json.Unmarshal([]byte(body), &legacy))
						assertGovernanceWireText(t, schema != nil, mustMarshalJSON(t, legacy), result)
						if schema == nil {
							assert.Nil(t, result.StructuredContent)
							continue
						}
						var output any
						require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
						require.NoError(t, schema.Validate(output))
						assert.JSONEq(t, `{"method":"`+method+`","level":"`+level+`","`+field+`":`+body+`}`, mustMarshalJSON(t, output))
					}
				})
			}
		}
	}
}

func TestTypedGovernanceLegacyInputErrors(t *testing.T) {
	cases := []struct {
		tool inventory.ServerTool
		args map[string]any
		text string
	}{
		{tool: GetLabel(translations.NullTranslationHelper), args: map[string]any{"repo": "r", "name": "bug"}, text: "missing required parameter: owner"},
		{tool: GetLabel(translations.NullTranslationHelper), args: map[string]any{"owner": false, "repo": "r", "name": "bug"}, text: "parameter owner is not of type string"},
		{tool: ListLabels(translations.NullTranslationHelper), args: map[string]any{"owner": "o"}, text: "missing required parameter: repo"},
		{tool: LabelWrite(translations.NullTranslationHelper), args: map[string]any{"method": false}, text: "parameter method is not of type string"},
		{tool: LabelWrite(translations.NullTranslationHelper), args: map[string]any{"method": false, "owner": 42}, text: "parameter method is not of type string"},
		{tool: RepositoryRulesetRead(translations.NullTranslationHelper), args: map[string]any{"level": "repository"}, text: "missing required parameter: method"},
		{tool: CustomPropertiesRead(translations.NullTranslationHelper), args: map[string]any{"level": false}, text: "parameter level is not of type string"},
		{tool: CreateRepositoryRuleset(translations.NullTranslationHelper), args: map[string]any{"level": "repository"}, text: "missing required parameter: name"},
		{tool: CreateRepositoryRuleset(translations.NullTranslationHelper), args: map[string]any{"level": "repository", "name": false, "owner": false}, text: "parameter name is not of type string"},
	}
	for _, tc := range cases {
		t.Run("legacy/"+tc.tool.Tool.Name+"/"+tc.text, func(t *testing.T) {
			deps := BaseDeps{}
			legacy := governanceLegacyTool(tc.tool.Tool.Name)
			request := createMCPRequest(tc.args)
			result, err := legacy.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Equal(t, tc.text, getTextResult(t, result).Text)
		})
	}
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.tool.Tool.Name+"/"+tc.text, func(t *testing.T) {
					session, _ := governanceTypedSession(t, tc.tool, BaseDeps{}, protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
					require.NoError(t, err)
					require.True(t, result.IsError)
					require.NotEmpty(t, getTextResult(t, result).Text)
					assert.Nil(t, result.StructuredContent)
				})
			}
		})
	}
}

func TestNormalizeGovernanceLevelRouting(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "organization level drops repository-only owner/repo",
			in:   `{"level":"organization","org":"o","owner":42,"repo":false}`,
			out:  `{"level":"organization","org":"o"}`,
		},
		{
			name: "enterprise level drops repository and organization fields",
			in:   `{"level":"enterprise","enterprise":"e","owner":42,"repo":false,"org":123}`,
			out:  `{"level":"enterprise","enterprise":"e"}`,
		},
		{
			name: "repository level keeps owner/repo and drops org/enterprise",
			in:   `{"level":"repository","owner":"o","repo":"r","org":42,"enterprise":false}`,
			out:  `{"level":"repository","owner":"o","repo":"r"}`,
		},
		{
			name: "repository level leaves a wrong-typed used field for the handler to reject",
			in:   `{"level":"repository","owner":42,"repo":"r"}`,
			out:  `{"level":"repository","owner":42,"repo":"r"}`,
		},
		{
			name: "no change when nothing unused is present",
			in:   `{"level":"organization","org":"o"}`,
			out:  `{"level":"organization","org":"o"}`,
		},
		{
			name: "missing level is passed through untouched",
			in:   `{"owner":42,"org":123}`,
			out:  `{"owner":42,"org":123}`,
		},
		{
			name: "non-string level is passed through untouched",
			in:   `{"level":false,"owner":42,"org":123}`,
			out:  `{"level":false,"owner":42,"org":123}`,
		},
		{
			name: "unrecognized level value is passed through untouched",
			in:   `{"level":"nonsense","owner":42,"org":123}`,
			out:  `{"level":"nonsense","owner":42,"org":123}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := normalizeGovernanceLevelRouting(json.RawMessage(tc.in))
			require.NoError(t, err)
			assert.JSONEq(t, tc.out, string(out))
		})
	}
}

func TestTypedGovernanceLevelRoutingNormalization(t *testing.T) {
	tr := translations.NullTranslationHelper
	rest := func(method, path, body string) func(*testing.T, *http.Request) string {
		return func(t *testing.T, request *http.Request) string {
			t.Helper()
			assert.Equal(t, method, request.Method)
			assert.Equal(t, path, request.URL.Path)
			if request.Body != nil {
				_, _ = io.Copy(io.Discard, request.Body)
			}
			return body
		}
	}
	cases := []struct {
		name string
		tc   governanceWireCase
	}{
		{
			name: "custom_properties_read organization ignores wrong-typed owner/repo",
			tc: governanceWireCase{
				tool:   CustomPropertiesRead(tr),
				args:   map[string]any{"level": "organization", "org": "o", "owner": 42, "repo": false},
				status: http.StatusOK,
				body:   `[{"property_name":"environment","value_type":"string"}]`,
				text:   `[{"property_name":"environment","value_type":"string"}]`,
				route:  rest(http.MethodGet, "/orgs/o/properties/schema", `[{"property_name":"environment","value_type":"string"}]`),
			},
		},
		{
			name: "custom_properties_read enterprise ignores wrong-typed owner/repo/org",
			tc: governanceWireCase{
				tool:   CustomPropertiesRead(tr),
				args:   map[string]any{"level": "enterprise", "enterprise": "e", "owner": 42, "repo": false, "org": 7},
				status: http.StatusOK,
				body:   `[{"property_name":"classification","value_type":"single_select"}]`,
				text:   `[{"property_name":"classification","value_type":"single_select"}]`,
				route:  rest(http.MethodGet, "/enterprises/e/properties/schema", `[{"property_name":"classification","value_type":"single_select"}]`),
			},
		},
		{
			name: "repository_ruleset_read organization ignores wrong-typed owner/repo",
			tc: governanceWireCase{
				tool:   RepositoryRulesetRead(tr),
				args:   map[string]any{"level": "organization", "method": "list", "org": "o", "owner": 42, "repo": false},
				status: http.StatusOK,
				body:   `[{"id":8,"name":"org rules"}]`,
				text:   `[{"id":8,"name":"org rules","enforcement":"","source":""}]`,
				route:  rest(http.MethodGet, "/orgs/o/rulesets", `[{"id":8,"name":"org rules"}]`),
			},
		},
		{
			name: "create_repository_ruleset organization ignores wrong-typed owner/repo",
			tc: governanceWireCase{
				tool: CreateRepositoryRuleset(tr),
				args: map[string]any{
					"level": "organization", "org": "o", "owner": 42, "repo": false,
					"name": "main", "enforcement": "active", "rules": []any{map[string]any{"type": "creation"}},
				},
				status: http.StatusCreated,
				body:   `{"id":8,"name":"main","source":"org:o","enforcement":"active","rules":[{"type":"creation"}]}`,
				text:   `{"id":8,"name":"main","source":"org:o","enforcement":"active","rules":[{"type":"creation"}]}`,
				route:  rest(http.MethodPost, "/orgs/o/rulesets", `{"id":8,"name":"main","source":"org:o","enforcement":"active","rules":[{"type":"creation"}]}`),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, _ := governanceTypedSession(t, tc.tc.tool, governanceWireDeps(t, tc.tc), inventory.ProtocolVersionMultiRoundTrip)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tc.tool.Tool.Name, Arguments: tc.tc.args})
			require.NoError(t, err)
			require.False(t, result.IsError, mustMarshalJSON(t, result))

			legacy := governanceLegacyTool(tc.tc.tool.Tool.Name)
			legacyDeps := governanceWireDeps(t, tc.tc)
			legacyResult, err := legacy.Handler(legacyDeps)(ContextWithDeps(context.Background(), legacyDeps), new(createMCPRequest(tc.tc.args)))
			require.NoError(t, err)
			require.False(t, legacyResult.IsError, mustMarshalJSON(t, legacyResult))
			assert.JSONEq(t, getTextResult(t, legacyResult).Text, tc.tc.text)
		})
	}
}

func TestTypedGovernanceLabelRetainsIFCMetadata(t *testing.T) {
	for _, tc := range governanceWireCases(t)[:2] {
		t.Run(tc.tool.Tool.Name, func(t *testing.T) {
			deps := governanceWireDeps(t, tc)
			deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
			session, _ := governanceTypedSession(t, tc.tool, deps, inventory.ProtocolVersionMultiRoundTrip)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
			require.NoError(t, err)
			require.False(t, result.IsError)
			legacyDeps := governanceWireDeps(t, tc)
			legacyDeps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
			legacy := governanceLegacyTool(tc.tool.Tool.Name)
			before, err := legacy.Handler(legacyDeps)(ContextWithDeps(context.Background(), legacyDeps), new(createMCPRequest(tc.args)))
			require.NoError(t, err)
			require.NotNil(t, result.Meta["ifc"])
			assert.JSONEq(t, mustMarshalJSON(t, before.Meta["ifc"]), mustMarshalJSON(t, result.Meta["ifc"]))
			assertGovernanceWireText(t, true, getTextResult(t, before).Text, result)
		})
	}
}

func TestTypedGetLabelToolsetVariant(t *testing.T) {
	issuesTool := GetLabel(translations.NullTranslationHelper)
	labelsTool := GetLabelForLabelsToolset(translations.NullTranslationHelper)
	assert.Equal(t, ToolsetMetadataIssues.ID, issuesTool.Toolset.ID)
	assert.Equal(t, ToolsetLabels.ID, labelsTool.Toolset.ID)
	assert.Equal(t, issuesTool.Tool.Name, labelsTool.Tool.Name)
	require.NotNil(t, issuesTool.Tool.OutputSchema)
	require.NotNil(t, labelsTool.Tool.OutputSchema)
	assert.JSONEq(t, mustMarshalJSON(t, issuesTool.Tool.OutputSchema), mustMarshalJSON(t, labelsTool.Tool.OutputSchema))
}

func TestTypedGovernanceOutputSchemasRejectMismatches(t *testing.T) {
	for _, tc := range []struct {
		name   string
		schema *jsonschema.Schema
		bad    string
	}{
		{"get label", labelOutputSchema(), `{"name":1,"color":"red","description":"desc"}`},
		{"list labels", listLabelsOutputSchema(), `{"labels":[],"totalCount":"1"}`},
		{"label write", labelWriteOutputSchema(), `{"message":1}`},
		{"custom properties read", customPropertiesReadOutputSchema(), `"not an array"`},
		{"property name", customPropertiesReadOutputSchema(), `{"method":"read","level":"repository","values":[{"property_name":123,"value":"prod"}]}`},
		{"property value", customPropertiesReadOutputSchema(), `{"method":"read","level":"repository","values":[{"property_name":"environment","value":false}]}`},
		{"property definition", customPropertiesReadOutputSchema(), `{"method":"read","level":"organization","definitions":[{"property_name":"environment","value_type":false}]}`},
		{"written definition", customPropertiesWriteOutputSchema(), `{"method":"write","level":"organization","definitions":[{"property_name":123,"value_type":"string"}]}`},
		{"custom properties write", customPropertiesWriteOutputSchema(), `1`},
		{"ruleset read", rulesetReadOutputSchema(), `true`},
		{"ruleset fields", rulesetReadOutputSchema(), `{"method":"get","level":"repository","ruleset":{"id":"invalid","rules":false}}`},
		{"ruleset list", rulesetReadOutputSchema(), `{"method":"list","level":"repository","rulesets":[{"id":"invalid","name":"main","source":"","enforcement":"active"}]}`},
		{"enterprise count", rulesetReadOutputSchema(), `{"method":"list","level":"enterprise","total_count":"invalid","rulesets":[]}`},
		{"suite fields", rulesetReadOutputSchema(), `{"method":"get_rule_suite","level":"repository","rule_suite":{"id":"invalid","result":"pass"}}`},
		{"branch fields", rulesetReadOutputSchema(), `{"method":"get_rules_for_branch","level":"repository","rules":[{"type":false}]}`},
		{"create ruleset", createdRulesetOutputSchema(), `{"rules":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := tc.schema.Resolve(nil)
			require.NoError(t, err)
			var value any
			require.NoError(t, json.Unmarshal([]byte(tc.bad), &value))
			assert.Error(t, resolved.Validate(value))
		})
	}
}

func TestTypedGovernanceEmptyNullAndValueUnions(t *testing.T) {
	tr := translations.NullTranslationHelper
	cases := []struct {
		tool     inventory.ServerTool
		args     map[string]any
		body     string
		field    string
		expected string
	}{
		{CustomPropertiesRead(tr), map[string]any{"level": "repository", "owner": "o", "repo": "r"}, "[]", "values", "[]"},
		{CustomPropertiesRead(tr), map[string]any{"level": "repository", "owner": "o", "repo": "r"}, "null", "values", "[]"},
		{CustomPropertiesRead(tr), map[string]any{"level": "repository", "owner": "o", "repo": "r"}, `[{"property_name":"cleared","value":null},{"property_name":"multi","value":["a","b"]}]`, "values", `[{"property_name":"cleared","value":null},{"property_name":"multi","value":["a","b"]}]`},
		{CustomPropertiesRead(tr), map[string]any{"level": "organization", "org": "o", "owner": false}, "[]", "definitions", "[]"},
		{CustomPropertiesRead(tr), map[string]any{"level": "enterprise", "enterprise": "e"}, "null", "definitions", "[]"},
		{CustomPropertiesRead(tr), map[string]any{"level": "organization", "org": "o"}, `[{"property_name":"classification","value_type":"multi_select","default_value":null,"allowed_values":[]}]`, "definitions", `[{"property_name":"classification","value_type":"multi_select","default_value":null,"allowed_values":[]}]`},
		{CustomPropertiesWrite(tr), map[string]any{"level": "organization", "org": "o", "properties": []any{}}, "[]", "definitions", "[]"},
		{CustomPropertiesWrite(tr), map[string]any{"level": "enterprise", "enterprise": "e", "properties": []any{}}, "null", "definitions", "[]"},
		{CustomPropertiesWrite(tr), map[string]any{"level": "enterprise", "enterprise": "e", "properties": []any{map[string]any{"property_name": "classification", "value_type": "string"}}}, `[{"property_name":"classification","value_type":"string","default_value":null,"allowed_values":[]}]`, "definitions", `[{"property_name":"classification","value_type":"string","default_value":null,"allowed_values":[]}]`},
		{RepositoryRulesetRead(tr), map[string]any{"level": "repository", "method": "LIST", "owner": "o", "repo": "r"}, "[]", "rulesets", "[]"},
		{RepositoryRulesetRead(tr), map[string]any{"level": "organization", "method": "list", "org": "o"}, "null", "rulesets", "[]"},
		{RepositoryRulesetRead(tr), map[string]any{"level": "enterprise", "method": "list", "enterprise": "e"}, `{"total_count":0,"rulesets":null}`, "rulesets", "[]"},
		{RepositoryRulesetRead(tr), map[string]any{"level": "repository", "method": "list_rule_suites", "owner": "o", "repo": "r"}, "[]", "rule_suites", "[]"},
		{RepositoryRulesetRead(tr), map[string]any{"level": "organization", "method": "list_rule_suites", "org": "o"}, "null", "rule_suites", "[]"},
		{RepositoryRulesetRead(tr), map[string]any{"level": "organization", "method": "get_rule_suite", "org": "o", "rule_suite_id": 11}, "null", "", ""},
	}
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"} {
		for _, tc := range cases {
			t.Run(protocol+"/"+tc.tool.Tool.Name+"/"+tc.body, func(t *testing.T) {
				wire := governanceWireCase{tool: tc.tool, args: tc.args, route: func(_ *testing.T, _ *http.Request) string { return tc.body }}
				session, schema := governanceTypedSession(t, tc.tool, governanceWireDeps(t, wire), protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
				require.NoError(t, err)
				require.False(t, result.IsError)
				legacyDeps := governanceWireDeps(t, wire)
				legacy := governanceLegacyTool(tc.tool.Tool.Name)
				before, err := legacy.Handler(legacyDeps)(ContextWithDeps(context.Background(), legacyDeps), new(createMCPRequest(tc.args)))
				require.NoError(t, err)
				assertGovernanceWireText(t, schema != nil, getTextResult(t, before).Text, result)
				if schema == nil {
					assert.Nil(t, result.StructuredContent)
					return
				}
				var output map[string]any
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
				require.NoError(t, schema.Validate(output))
				if tc.field != "" {
					assert.JSONEq(t, tc.expected, mustMarshalJSON(t, output[tc.field]))
				}
			})
		}
	}
}

func TestTypedGovernanceAPIErrors(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"} {
		for _, tc := range governanceWireCases(t) {
			t.Run(protocol+"/"+tc.tool.Tool.Name, func(t *testing.T) {
				tc.status = http.StatusForbidden
				tc.body = `{"message":"Forbidden"}`
				tc.route = nil
				if tc.tool.Tool.Name == "get_label" || tc.tool.Tool.Name == "list_label" || tc.tool.Tool.Name == "label_write" {
					tc.route = func(_ *testing.T, _ *http.Request) string { return `{"errors":[{"message":"Forbidden"}]}` }
				}
				session, _ := governanceTypedSession(t, tc.tool, governanceWireDeps(t, tc), protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent)
				legacyDeps := governanceWireDeps(t, tc)
				legacy := governanceLegacyTool(tc.tool.Tool.Name)
				before, err := legacy.Handler(legacyDeps)(ContextWithDeps(context.Background(), legacyDeps), new(createMCPRequest(tc.args)))
				require.NoError(t, err)
				assert.Equal(t, getTextResult(t, before).Text, getTextResult(t, result).Text)
			})
		}
	}
}

func TestTypedGovernanceCompactContracts(t *testing.T) {
	for _, tool := range []inventory.ServerTool{
		GetLabel(translations.NullTranslationHelper), GetLabelForLabelsToolset(translations.NullTranslationHelper),
		ListLabels(translations.NullTranslationHelper), LabelWrite(translations.NullTranslationHelper),
		CustomPropertiesRead(translations.NullTranslationHelper), CustomPropertiesWrite(translations.NullTranslationHelper),
		RepositoryRulesetRead(translations.NullTranslationHelper), CreateRepositoryRuleset(translations.NullTranslationHelper),
	} {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			schema := tool.Tool.OutputSchema.(*jsonschema.Schema)
			assert.Equal(t, "object", schema.Type)
			assert.Empty(t, schema.AnyOf)
			assert.Empty(t, schema.OneOf)
			if schema.Properties["method"] != nil {
				assert.Contains(t, schema.Required, "method")
				assert.NotEmpty(t, schema.Properties["method"].Enum)
			}
			_, err := schema.Resolve(nil)
			require.NoError(t, err)
		})
	}
	legacyJSON := `{"id":7,"name":"main","source":"o/r","enforcement":"active","node_id":"NODE","_links":{"self":{"href":"https://api.github.com/repos/o/r/rulesets/7"}},"rules":[{"type":"commit_message_pattern","parameters":{"operator":"regex","pattern":"^ship","custom":{"nested":[1,true,null]}}}],"conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}}}`
	output, err := decodeGovernanceJSON[RulesetOutput]([]byte(legacyJSON), nil)
	require.NoError(t, err)
	encoded := mustMarshalJSON(t, output)
	assert.NotContains(t, encoded, "https://")
	assert.NotContains(t, encoded, "node_id")
	assert.Contains(t, encoded, `"custom":{"nested":[1,true,null]}`)
	assert.Less(t, len(encoded), len(legacyJSON))
	resolved, err := createdRulesetOutputSchema().Resolve(nil)
	require.NoError(t, err)
	var value any
	require.NoError(t, json.Unmarshal([]byte(encoded), &value))
	require.NoError(t, resolved.Validate(value))
	t.Logf("representative ruleset payload: legacy=%d bytes, modern=%d bytes", len(legacyJSON), len(encoded))
}

func TestTypedGovernancePropertyValueContracts(t *testing.T) {
	for _, raw := range []string{`"prod"`, `["prod","staging"]`, `[]`, `null`} {
		t.Run(raw, func(t *testing.T) {
			var value CustomPropertyOutputValue
			require.NoError(t, json.Unmarshal([]byte(raw), &value))
			assert.JSONEq(t, raw, mustMarshalJSON(t, value))
		})
	}
	for _, raw := range []string{`false`, `42`, `{}`, `["prod",42]`} {
		t.Run("invalid/"+raw, func(t *testing.T) {
			var value CustomPropertyOutputValue
			require.Error(t, json.Unmarshal([]byte(raw), &value))
		})
	}
	for _, raw := range []string{
		`{"property_name":"classification","value_type":"multi_select","default_value":null,"allowed_values":[]}`,
		`{"property_name":"classification","value_type":"multi_select","default_value":["a"],"allowed_values":["a"]}`,
		`{"property_name":"environment","value_type":"string","required":false,"require_explicit_values":false}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var definition CustomPropertyDefinitionOutput
			require.NoError(t, json.Unmarshal([]byte(raw), &definition))
			assert.JSONEq(t, raw, mustMarshalJSON(t, definition))
			schema, err := customPropertiesReadOutputSchema().Resolve(nil)
			require.NoError(t, err)
			var value any
			require.NoError(t, json.Unmarshal([]byte(`{"method":"read","level":"organization","definitions":[`+mustMarshalJSON(t, definition)+`]}`), &value))
			require.NoError(t, schema.Validate(value))
		})
	}
}

func TestTypedGovernanceNullableProjectionContracts(t *testing.T) {
	for _, tc := range []struct {
		method   string
		level    string
		raw      string
		expected string
	}{
		{"get", "repository", "null", `{"method":"get","level":"repository"}`},
		{"list", "repository", "null", `{"method":"list","level":"repository","rulesets":[]}`},
		{"list", "enterprise", `{"total_count":0,"rulesets":null}`, `{"method":"list","level":"enterprise","total_count":0,"rulesets":[]}`},
		{"get_rules_for_branch", "repository", "null", `{"method":"get_rules_for_branch","level":"repository","rules":[]}`},
		{"list_rule_suites", "organization", "null", `{"method":"list_rule_suites","level":"organization","rule_suites":[]}`},
		{"get_rule_suite", "organization", "null", `{"method":"get_rule_suite","level":"organization"}`},
	} {
		t.Run(tc.method+"/"+tc.level, func(t *testing.T) {
			args := map[string]json.RawMessage{"method": json.RawMessage(`"` + tc.method + `"`), "level": json.RawMessage(`"` + tc.level + `"`)}
			output, err := decodeRulesetRead([]byte(tc.raw), args)
			require.NoError(t, err)
			assert.JSONEq(t, tc.expected, mustMarshalJSON(t, output))
			resolved, err := rulesetReadOutputSchema().Resolve(nil)
			require.NoError(t, err)
			var value any
			require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, output)), &value))
			require.NoError(t, resolved.Validate(value))
		})
	}
	for _, level := range []string{"repository", "organization", "enterprise"} {
		for _, raw := range []string{"null", "[]"} {
			t.Run("properties/"+level+"/"+raw, func(t *testing.T) {
				output, err := decodeCustomProperties("read")([]byte(raw), map[string]json.RawMessage{"level": json.RawMessage(`"` + level + `"`)})
				require.NoError(t, err)
				expected := `{"method":"read","level":"` + level + `","definitions":[]}`
				if level == "repository" {
					expected = `{"method":"read","level":"repository","values":[]}`
				}
				assert.JSONEq(t, expected, mustMarshalJSON(t, output))
			})
		}
	}
}
