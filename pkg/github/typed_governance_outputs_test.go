package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
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
	contains string
	route    func(*testing.T, *http.Request) string
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
	require.NoError(t, toolsnaps.Test(tool.Tool.Name+"_typed", *list.Tools[0]))
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
				if graphqlCalls == 1 {
					body = `{"data":{"repository":{"id":"R1"}}}`
				} else {
					body = `{"data":{"createLabel":{"label":{"id":"L2","name":"feature"}}}}`
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
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "repository", "method": "get", "owner": "o", "repo": "r", "ruleset_id": 7}, status: http.StatusOK, body: `{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}`, text: `{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}`, route: rest(http.MethodGet, "/repos/o/r/rulesets/7", `{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "repository", "method": "list", "owner": "o", "repo": "r"}, status: http.StatusOK, body: `[{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}]`, text: `[{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}]`, route: rest(http.MethodGet, "/repos/o/r/rulesets", `[{"id":7,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[]}]`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "repository", "method": "get_rules_for_branch", "owner": "o", "repo": "r", "branch": "main"}, status: http.StatusOK, body: `[{"type":"creation"}]`, contains: "Creation", route: rest(http.MethodGet, "/repos/o/r/rules/branches/main", `[{"type":"creation"}]`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "organization", "method": "get_rule_suite", "org": "o", "rule_suite_id": 11}, status: http.StatusOK, body: `{"id":11,"result":"pass","custom":{"retained":true}}`, text: `{"custom":{"retained":true},"id":11,"result":"pass"}`, route: rest(http.MethodGet, "/orgs/o/rulesets/rule-suites/11", `{"id":11,"result":"pass","custom":{"retained":true}}`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "organization", "method": "list", "org": "o"}, status: http.StatusOK, body: `[{"id":8,"name":"org rules"}]`, text: `[{"id":8,"name":"org rules","enforcement":"","source":""}]`, route: rest(http.MethodGet, "/orgs/o/rulesets", `[{"id":8,"name":"org rules"}]`)},
		{tool: RepositoryRulesetRead(tr), args: map[string]any{"level": "enterprise", "method": "list", "enterprise": "e"}, status: http.StatusOK, body: `{"total_count":1,"rulesets":[{"id":9,"name":"enterprise rules"}]}`, text: `{"rulesets":[{"id":9,"name":"enterprise rules","enforcement":"","source":""}],"total_count":1}`, route: rest(http.MethodGet, "/enterprises/e/rulesets", `{"total_count":1,"rulesets":[{"id":9,"name":"enterprise rules"}]}`)},
		{tool: CreateRepositoryRuleset(tr), args: map[string]any{"level": "repository", "owner": "o", "repo": "r", "name": "main", "enforcement": "active", "rules": []any{map[string]any{"type": "creation"}}}, status: http.StatusCreated, body: `{"id":8,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[{"type":"creation"}]}`, text: `{"id":8,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[{"type":"creation"}]}`, route: rest(http.MethodPost, "/repos/o/r/rulesets", `{"id":8,"name":"main","target":"branch","source":"repo:o/r","enforcement":"active","rules":[{"type":"creation"}]}`)},
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
					actualText := getTextResult(t, result).Text
					switch {
					case tc.contains != "":
						assert.Contains(t, actualText, tc.contains)
					case json.Valid([]byte(tc.text)):
						assert.JSONEq(t, tc.text, actualText)
					default:
						assert.Equal(t, tc.text, actualText)
					}
					if schema == nil {
						assert.Nil(t, result.StructuredContent)
						return
					}
					require.NotNil(t, result.StructuredContent)
					var output any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					require.NoError(t, schema.Validate(output))
					if tc.tool.Tool.Name == "label_write" || (tc.tool.Tool.Name == "custom_properties_write" && !json.Valid([]byte(tc.text))) {
						expected, err := json.Marshal(tc.text)
						require.NoError(t, err)
						if tc.tool.Tool.Name == "label_write" {
							expected, err = json.Marshal(map[string]string{"message": tc.text})
							require.NoError(t, err)
						}
						assert.JSONEq(t, string(expected), mustMarshalJSON(t, output))
					} else {
						assert.JSONEq(t, actualText, mustMarshalJSON(t, output))
					}
					if tc.tool.Tool.Name == "repository_ruleset_read" && strings.Contains(tc.text, "custom") {
						assert.Contains(t, mustMarshalJSON(t, output), `"custom":{"retained":true}`)
					}
				})
			}
		})
	}
}

func TestTypedGovernanceLegacyInputErrors(t *testing.T) {
	cases := []struct {
		tool inventory.ServerTool
		args map[string]any
		text string
	}{
		{tool: GetLabel(translations.NullTranslationHelper), args: map[string]any{"repo": "r", "name": "bug"}, text: "missing required parameter: owner"},
		{tool: LabelWrite(translations.NullTranslationHelper), args: map[string]any{"method": false}, text: "parameter method is not of type string"},
		{tool: RepositoryRulesetRead(translations.NullTranslationHelper), args: map[string]any{"level": "repository"}, text: "missing required parameter: method"},
		{tool: CustomPropertiesRead(translations.NullTranslationHelper), args: map[string]any{"level": false}, text: "parameter level is not of type string"},
		{tool: CreateRepositoryRuleset(translations.NullTranslationHelper), args: map[string]any{"level": "repository"}, text: "missing required parameter: name"},
	}
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.tool.Tool.Name+"/"+tc.text, func(t *testing.T) {
					session, _ := governanceTypedSession(t, tc.tool, BaseDeps{}, protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
					require.NoError(t, err)
					require.True(t, result.IsError)
					assert.Equal(t, tc.text, getTextResult(t, result).Text)
					assert.Nil(t, result.StructuredContent)
				})
			}
		})
	}
}

func TestTypedGovernanceLabelRetainsIFCMetadata(t *testing.T) {
	tc := governanceWireCases(t)[0]
	deps := governanceWireDeps(t, tc)
	deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
	session, _ := governanceTypedSession(t, tc.tool, deps, inventory.ProtocolVersionMultiRoundTrip)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.NotNil(t, result.Meta["ifc"])
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
		{"get label", labelOutputSchema(), `{"id":1,"name":"bug","color":"red","description":"desc"}`},
		{"list labels", listLabelsOutputSchema(), `{"labels":[],"totalCount":"1"}`},
		{"label write", labelWriteOutputSchema(), `{"message":1}`},
		{"custom properties read", customPropertiesReadOutputSchema(), `"not an array"`},
		{"custom properties write", customPropertiesWriteOutputSchema(), `1`},
		{"ruleset read", rulesetReadOutputSchema(), `true`},
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
