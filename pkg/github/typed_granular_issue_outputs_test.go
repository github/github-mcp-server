package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/http/headers"
	transportpkg "github.com/github/github-mcp-server/v2/pkg/http/transport"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func granularIssueSession(t *testing.T, deps BaseDeps, protocol string) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	tools := granularToolsForToolset(ToolsetMetadataIssues.ID, FeatureFlagIssuesGranular)
	require.Len(t, tools, 18)
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).
		WithFeatureChecker(featureCheckerFor(inventory.FeatureFlag(FeatureFlagIssuesGranular))).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "granular-issues", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	inv.RegisterTools(context.Background(), server, deps)
	if protocol == "" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
				switch req := request.(type) {
				case *mcp.ListToolsRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
				case *mcp.CallToolRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
				}
				return next(ctx, method, request)
			}
		})
	}
	version := protocol
	if version == "" {
		version = inventory.ProtocolVersionMultiRoundTrip
	}
	session := connectCommentVisibilityClient(t, server, version)
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, len(tools))
	schemas := make(map[string]*jsonschema.Resolved)
	for _, tool := range list.Tools {
		if protocol != inventory.ProtocolVersionMultiRoundTrip {
			assert.Nil(t, tool.OutputSchema, tool.Name)
			continue
		}
		require.NotNil(t, tool.OutputSchema, tool.Name)
		require.NoError(t, toolsnaps.Test(tool.Name, *tool))
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err, tool.Name)
		schemas[tool.Name] = resolved
		if tool.Name == "add_sub_issue" {
			schemaJSON := mustMarshalJSON(t, tool.OutputSchema)
			for _, excluded := range []string{`"url"`, `"comments_url"`, `"events_url"`, `"labels_url"`, `"repository_url"`, `"profile_url"`, `"avatar_url"`, `"pinned_comment"`, `"performed_via_github_app"`} {
				assert.NotContains(t, schemaJSON, excluded)
			}
			assert.Less(t, len(schemaJSON), 12000)
		}
	}
	return session, schemas
}

type granularIssueWireCase struct {
	name       string
	args       map[string]any
	text       string
	structured string
	payload    string
	path       string
	method     string
}

const granularMutationText = `{"id":"123","url":"https://github.com/owner/repo/issues/1"}`
const granularSubIssueAPIResponse = `{"id":7,"number":7,"title":"Child\u202e","body":"body\u202e","url":"https://api.github.com/repos/owner/repo/issues/7","html_url":"https://github.com/owner/repo/issues/7","comments_url":"https://api.github.com/repos/owner/repo/issues/7/comments","node_id":"I_7","issue_field_values":[{"issue_field_id":11,"node_id":"IF_11","data_type":"number","value":0},{"issue_field_id":12,"node_id":"IF_12","data_type":"text","value":""},{"issue_field_id":13,"node_id":"IF_13","data_type":"text","value":false},{"issue_field_id":14,"node_id":"IF_14","data_type":"text","value":null}]}`
const granularSubIssueText = `{"id":7,"number":7,"title":"Child","body":"body","url":"https://api.github.com/repos/owner/repo/issues/7","html_url":"https://github.com/owner/repo/issues/7","comments_url":"https://api.github.com/repos/owner/repo/issues/7/comments","node_id":"I_7","issue_field_values":[{"issue_field_id":11,"node_id":"IF_11","data_type":"number","value":0},{"issue_field_id":12,"node_id":"IF_12","data_type":"text","value":""},{"issue_field_id":13,"node_id":"IF_13","data_type":"text","value":false},{"issue_field_id":14,"node_id":"IF_14","data_type":"text","value":null}]}`

const granularSubIssueStructured = `{"method":"add","issue":{"id":7,"number":7,"title":"Child","body":"body","html_url":"https://github.com/owner/repo/issues/7","assignees":[],"field_values":[{"issue_field_id":11,"data_type":"number","value":0},{"issue_field_id":12,"data_type":"text","value":""},{"issue_field_id":13,"data_type":"text","value":false},{"issue_field_id":14,"data_type":"text","value":null}]}}`

func granularIssueWireCases() []granularIssueWireCase {
	cases := []granularIssueWireCase{
		{"create_issue", map[string]any{"title": "Subject"}, granularMutationText, "", `{"title":"Subject"}`, "/repos/owner/repo/issues", "POST"},
		{"create_issue", map[string]any{"title": "Subject", "body": "Body"}, granularMutationText, "", `{"title":"Subject","body":"Body"}`, "/repos/owner/repo/issues", "POST"},
		{"create_issue", map[string]any{"title": "Subject", "body": ""}, granularMutationText, "", `{"title":"Subject"}`, "/repos/owner/repo/issues", "POST"},
		{"create_issue", map[string]any{"title": "Subject", "parent_issue_number": "7.0"}, granularMutationText, "", "", "", ""},
		{"create_issue", map[string]any{"title": "Subject", "body": "Body", "parent_issue_number": "7", "parent_owner": "other", "parent_repo": "parent"}, granularMutationText, "", "", "", ""},
		{"update_issue_title", map[string]any{"title": "Changed"}, granularMutationText, "", `{"title":"Changed"}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_body", map[string]any{"body": "Changed"}, granularMutationText, "", `{"body":"Changed"}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_assignees", map[string]any{"assignees": []string{}}, granularMutationText, "", `{"assignees":[]}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_assignees", map[string]any{"assignees": []any{"octocat", map[string]any{"login": "hubot"}}}, granularMutationText, "", `{"assignees":["octocat","hubot"]}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_assignees", map[string]any{"assignees": []any{"hubot", map[string]any{"login": "octocat", "rationale": "  Authored file  ", "confidence": " high\t", "is_suggestion": true}}}, granularMutationText, "", `{"assignees":["hubot",{"login":"octocat","rationale":"Authored file","confidence":"HIGH","suggest":true}]}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_labels", map[string]any{"labels": []string{}}, granularMutationText, "", `{"labels":[]}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_labels", map[string]any{"labels": []any{"bug", map[string]any{"name": "triage"}}}, granularMutationText, "", `{"labels":["bug","triage"]}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_labels", map[string]any{"labels": []any{"triage", map[string]any{"name": "bug", "rationale": "  Crash  ", "confidence": " medium ", "is_suggestion": true}}}, granularMutationText, "", `{"labels":["triage",{"name":"bug","rationale":"Crash","confidence":"MEDIUM","suggest":true}]}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_milestone", map[string]any{"milestone": "3e0"}, granularMutationText, "", `{"milestone":3}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_type", map[string]any{"issue_type": nil}, granularMutationText, "", `{"type":null}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_type", map[string]any{"issue_type": "Bug"}, granularMutationText, "", `{"type":"Bug"}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_type", map[string]any{"issue_type": "Bug", "rationale": " Crash ", "confidence": " low ", "is_suggestion": true}, granularMutationText, "", `{"type":{"value":"Bug","rationale":"Crash","confidence":"LOW","suggest":true}}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_state", map[string]any{"state": "open"}, granularMutationText, "", `{"state":"open"}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_state", map[string]any{"state": "closed", "state_reason": "completed"}, granularMutationText, "", `{"state":"closed","state_reason":"completed"}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_state", map[string]any{"state": "closed", "state_reason": "not_planned", "confidence": " high "}, granularMutationText, "", `{"state":{"value":"closed","confidence":"HIGH"},"state_reason":"not_planned"}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"update_issue_state", map[string]any{"state": "closed", "state_reason": "duplicate", "duplicate_of": "2.0", "is_suggestion": true}, granularMutationText, "", `{"state":{"value":"closed","suggest":true},"state_reason":"duplicate","duplicate_issue_id":456}`, "/repos/owner/repo/issues/1", "PATCH"},
		{"add_sub_issue", map[string]any{"sub_issue_id": "7", "replace_parent": true}, granularSubIssueText, granularSubIssueStructured, `{"sub_issue_id":7,"replace_parent":true}`, "/repos/owner/repo/issues/1/sub_issues", "POST"},
		{"remove_sub_issue", map[string]any{"sub_issue_id": "7"}, granularSubIssueText, granularSubIssueStructured, `{"sub_issue_id":7}`, "/repos/owner/repo/issues/1/sub_issue", "DELETE"},
		{"reprioritize_sub_issue", map[string]any{"sub_issue_id": "7", "after_id": "8.0"}, granularSubIssueText, granularSubIssueStructured, `{"sub_issue_id":7,"after_id":8}`, "/repos/owner/repo/issues/1/sub_issues/priority", "PATCH"},
		{"reprioritize_sub_issue", map[string]any{"sub_issue_id": "7", "before_id": "8"}, granularSubIssueText, granularSubIssueStructured, `{"sub_issue_id":7,"before_id":8}`, "/repos/owner/repo/issues/1/sub_issues/priority", "PATCH"},
		{"set_issue_fields", map[string]any{"fields": []any{map[string]any{"field_id": "IF_text", "text_value": "Value", "rationale": " Signal ", "confidence": " high ", "is_suggestion": true}, map[string]any{"field_id": "IF_number", "number_value": 0}, map[string]any{"field_id": "IF_date", "date_value": "2026-10-02"}, map[string]any{"field_id": "IF_select", "single_select_option_id": "OPT_1"}, map[string]any{"field_id": "IF_delete", "delete": true}}}, `{"id":"I_1","url":"https://github.com/owner/repo/issues/1"}`, "", "", "", ""},
		{"add_issue_reaction", map[string]any{"content": "+1"}, `{"id":"9","url":"https://api.github.com/repos/owner/repo/issues/1/reactions/9"}`, "", `{"content":"+1"}`, "/repos/owner/repo/issues/1/reactions", "POST"},
		{"remove_issue_reaction", map[string]any{"reaction_id": "9e0"}, "reaction successfully removed from issue", `{"message":"reaction successfully removed from issue"}`, "", "/repos/owner/repo/issues/1/reactions/9", "DELETE"},
		{"add_issue_comment_reaction", map[string]any{"comment_id": "42.0", "content": "-1"}, `{"id":"9","url":"https://api.github.com/repos/owner/repo/issues/comments/42/reactions/9"}`, "", `{"content":"-1"}`, "/repos/owner/repo/issues/comments/42/reactions", "POST"},
		{"remove_issue_comment_reaction", map[string]any{"comment_id": "42", "reaction_id": "9"}, "reaction successfully removed from issue comment", `{"message":"reaction successfully removed from issue comment"}`, "", "/repos/owner/repo/issues/comments/42/reactions/9", "DELETE"},
		{"hide_issue_comment", map[string]any{"comment_id": "42", "classifier": "oFf_ToPiC"}, `{"node_id":"NODE_1","is_minimized":true,"minimized_reason":"off-topic"}`, "", "", "", ""},
		{"unhide_issue_comment", map[string]any{"comment_id": "42"}, `{"node_id":"NODE_1","is_minimized":false}`, "", "", "", ""},
	}
	for _, reaction := range []string{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"} {
		cases = append(cases,
			granularIssueWireCase{
				name: "add_issue_reaction", args: map[string]any{"content": reaction},
				text:    `{"id":"9","url":"https://api.github.com/repos/owner/repo/issues/1/reactions/9"}`,
				payload: fmt.Sprintf(`{"content":%q}`, reaction), path: "/repos/owner/repo/issues/1/reactions", method: "POST",
			},
			granularIssueWireCase{
				name: "add_issue_comment_reaction", args: map[string]any{"comment_id": "42", "content": reaction},
				text:    `{"id":"9","url":"https://api.github.com/repos/owner/repo/issues/comments/42/reactions/9"}`,
				payload: fmt.Sprintf(`{"content":%q}`, reaction), path: "/repos/owner/repo/issues/comments/42/reactions", method: "POST",
			})
	}
	return cases
}

func granularWireDeps(t *testing.T, current **granularIssueWireCase, fail bool) BaseDeps {
	t.Helper()
	rest := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc := *current
		if fail {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/repos/owner/repo/issues/2" {
			_, _ = w.Write([]byte(`{"id":456,"number":2}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/comments/42") {
			assert.Equal(t, "GET", r.Method)
			_, _ = w.Write([]byte(`{"node_id":"NODE_1"}`))
			return
		}
		require.Equal(t, tc.path, r.URL.Path)
		require.Equal(t, tc.method, r.Method)
		if tc.payload != "" {
			var body json.RawMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.JSONEq(t, tc.payload, string(body))
		}
		if strings.Contains(tc.name, "remove_issue") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		}
		switch {
		case strings.Contains(tc.name, "sub_issue"):
			_, _ = w.Write([]byte(granularSubIssueAPIResponse))
		case strings.Contains(tc.name, "reaction"):
			_, _ = w.Write([]byte(`{"id":9}`))
		default:
			_, _ = w.Write([]byte(`{"id":123,"html_url":"https://github.com/owner/repo/issues/1"}`))
		}
	})}}
	gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			_, _ = w.Write([]byte(`{"errors":[{"message":"Forbidden"}]}`))
			return
		}
		tc := *current
		var request struct {
			Query     string                     `json:"query"`
			Variables map[string]json.RawMessage `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		switch {
		case strings.Contains(request.Query, "parentRepository:"):
			parentOwner, parentRepo := "owner", "repo"
			if tc.args["parent_owner"] != nil {
				parentOwner, parentRepo = tc.args["parent_owner"].(string), tc.args["parent_repo"].(string)
			}
			assert.JSONEq(t, fmt.Sprintf(`"%s"`, parentOwner), string(request.Variables["parentOwner"]))
			assert.JSONEq(t, fmt.Sprintf(`"%s"`, parentRepo), string(request.Variables["parentRepo"]))
			assert.JSONEq(t, `7`, string(request.Variables["parentIssueNumber"]))
			_, _ = w.Write([]byte(`{"data":{"childRepository":{"id":"R_1","nameWithOwner":"owner/repo"},"parentRepository":{"issue":{"id":"I_parent","number":7}}}}`))
		case strings.Contains(request.Query, "createIssue("):
			var input map[string]any
			require.NoError(t, json.Unmarshal(request.Variables["input"], &input))
			assert.Equal(t, "R_1", input["repositoryId"])
			assert.Equal(t, "I_parent", input["parentIssueId"])
			assert.Equal(t, "Subject", input["title"])
			if tc.args["body"] != nil {
				assert.Equal(t, tc.args["body"], input["body"])
			}
			_, _ = w.Write([]byte(`{"data":{"createIssue":{"issue":{"fullDatabaseId":"123","url":"https://github.com/owner/repo/issues/1"}}}}`))
		case strings.Contains(request.Query, "setIssueFieldValue("):
			assert.Contains(t, r.Header.Get(headers.GraphQLFeaturesHeader), "update_issue_suggestions")
			assert.JSONEq(t, `{"issueId":"I_1","issueFields":[{"fieldId":"IF_text","textValue":"Value","rationale":"Signal","confidence":"HIGH","suggest":true},{"fieldId":"IF_number","numberValue":0},{"fieldId":"IF_date","dateValue":"2026-10-02"},{"fieldId":"IF_select","singleSelectOptionId":"OPT_1"},{"fieldId":"IF_delete","delete":true}]}`, string(request.Variables["input"]))
			_, _ = w.Write([]byte(`{"data":{"setIssueFieldValue":{"issue":{"id":"I_1","url":"https://github.com/owner/repo/issues/1"}}}}`))
		case strings.Contains(request.Query, "unminimizeComment("):
			_, _ = w.Write([]byte(`{"data":{"unminimizeComment":{"unminimizedComment":{"isMinimized":false}}}}`))
		case strings.Contains(request.Query, "minimizeComment("):
			_, _ = w.Write([]byte(`{"data":{"minimizeComment":{"minimizedComment":{"isMinimized":true,"minimizedReason":"off-topic"}}}}`))
		case strings.Contains(request.Query, "issue(number:"):
			_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"id":"I_1"}}}}`))
		default:
			t.Errorf("unexpected GraphQL query: %s", request.Query)
		}
	})}}
	gql.Transport = &transportpkg.GraphQLFeaturesTransport{Transport: gql.Transport}
	return BaseDeps{Client: mustNewGHClient(t, rest), GQLClient: githubv4.NewClient(gql)}
}

func TestTypedGranularIssueWireOutputs(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var current *granularIssueWireCase
			session, schemas := granularIssueSession(t, granularWireDeps(t, &current, false), protocol)
			for i, tc := range granularIssueWireCases() {
				t.Run(fmt.Sprintf("%s/%d", tc.name, i), func(t *testing.T) {
					current = &tc
					args := map[string]any{"owner": "owner", "repo": "repo", "issue_number": "1.0"}
					maps.Copy(args, tc.args)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
					require.NoError(t, err)
					require.False(t, result.IsError, mustMarshalJSON(t, result))
					require.Len(t, result.Content, 1)
					expected := tc.text
					if schema := schemas[tc.name]; schema != nil {
						if tc.structured != "" {
							expected = tc.structured
						} else {
							expected = tc.text
						}
						if strings.Contains(tc.name, "sub_issue") {
							method := map[string]string{"add_sub_issue": "add", "remove_sub_issue": "remove", "reprioritize_sub_issue": "reprioritize"}[tc.name]
							expected = strings.Replace(expected, `"method":"add"`, fmt.Sprintf(`"method":%q`, method), 1)
						}
						require.NotNil(t, result.StructuredContent)
						assert.JSONEq(t, expected, mustMarshalJSON(t, result.StructuredContent))
						assert.JSONEq(t, expected, getTextResult(t, result).Text)
						assert.JSONEq(t, getTextResult(t, result).Text, mustMarshalJSON(t, result.StructuredContent))
						require.NoError(t, schema.Validate(result.StructuredContent))
						if strings.Contains(tc.name, "sub_issue") {
							var output map[string]json.RawMessage
							require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
							for _, excluded := range []string{"url", "comments_url", "events_url", "labels_url", "repository_url", "node_id"} {
								assert.NotContains(t, output, excluded)
							}
						}
					} else {
						assert.Equal(t, expected, getTextResult(t, result).Text)
						assert.Nil(t, result.StructuredContent)
					}
				})
			}
		})
	}
}

func TestGranularSubIssueStructuredSchema(t *testing.T) {
	resolved, err := subIssueWriteOutputSchema().Resolve(nil)
	require.NoError(t, err)
	for _, raw := range []string{
		`{"method":"add","issue":null}`,
		`{"method":"add","issue":{"id":7,"number":7,"state":"open","title":"Child","html_url":"https://github.com/owner/repo/issues/7","assignees":[],"field_values":[]}}`,
		`{"method":"reprioritize","issue":{"id":7,"assignees":[],"field_values":[{"issue_field_id":11,"data_type":"number","value":0},{"issue_field_id":12,"data_type":"text","value":null}]}}`,
	} {
		var output any
		require.NoError(t, json.Unmarshal([]byte(raw), &output))
		require.NoError(t, resolved.Validate(output), raw)
	}
	for _, raw := range []string{
		`{"method":"other","issue":null}`,
		`{"method":"add","issue":{"state":"queued"}}`,
		`{"method":"add","issue":{"url":"https://api.github.com/repos/owner/repo/issues/7"}}`,
		`{"method":"add","issue":{"comments_url":"https://api.github.com/repos/owner/repo/issues/7/comments"}}`,
	} {
		var output any
		require.NoError(t, json.Unmarshal([]byte(raw), &output))
		require.Error(t, resolved.Validate(output), raw)
	}
}

func TestTypedGranularIssueVisibilityGates(t *testing.T) {
	tools := granularToolsForToolset(ToolsetMetadataIssues.ID, FeatureFlagIssuesGranular)
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25"} {
		for _, gate := range []string{"feature disabled", "read only", "missing repo scope"} {
			t.Run(protocol+"/"+gate, func(t *testing.T) {
				builder := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).
					WithFeatureChecker(func(context.Context, string) (bool, error) { return gate != "feature disabled", nil })
				switch gate {
				case "read only":
					builder.WithReadOnly(true)
				case "missing repo scope":
					builder.WithFilter(CreateToolScopeFilter([]string{"public_repo"}))
				}
				inv, err := builder.Build()
				require.NoError(t, err)
				server := mcp.NewServer(&mcp.Implementation{Name: "granular-gates", Version: "v1"}, nil)
				inv.RegisterTools(context.Background(), server, nil)
				session := connectCommentVisibilityClient(t, server, protocol)
				list, err := session.ListTools(context.Background(), nil)
				require.NoError(t, err)
				assert.Empty(t, list.Tools)
				for _, tool := range tools {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name})
					if err == nil {
						require.NotNil(t, result)
						assert.True(t, result.IsError, tool.Tool.Name)
						assert.Nil(t, result.StructuredContent, tool.Tool.Name)
					}
				}
			})
		}
	}
}

func TestTypedGranularIssueWireErrors(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		text string
	}{
		{"create_issue", map[string]any{}, "missing required parameter: title"},
		{"create_issue", map[string]any{"title": "Subject", "parent_issue_number": 0}, "parent_issue_number must be greater than 0"},
		{"create_issue", map[string]any{"title": "Subject", "parent_issue_number": nil}, "not a valid number"},
		{"create_issue", map[string]any{"title": "Subject", "parent_owner": "other"}, "can only be used"},
		{"create_issue", map[string]any{"title": "Subject", "parent_issue_number": 7, "parent_repo": "parent"}, "must be provided together"},
		{"update_issue_title", map[string]any{"title": ""}, "missing required parameter: title"},
		{"update_issue_body", map[string]any{"body": ""}, "missing required parameter: body"},
		{"update_issue_milestone", map[string]any{"milestone": "1.5"}, "non-integer"},
		{"update_issue_assignees", map[string]any{"assignees": []any{map[string]any{"login": "octo", "confidence": "maybe"}}}, "confidence must be one of"},
		{"update_issue_labels", map[string]any{"labels": []any{map[string]any{"name": "bug", "rationale": strings.Repeat("x", 281)}}}, "280 characters"},
		{"update_issue_type", map[string]any{}, "missing required parameter: issue_type"},
		{"update_issue_type", map[string]any{"issue_type": nil, "is_suggestion": true}, "suggestion metadata is not supported"},
		{"update_issue_type", map[string]any{"issue_type": ""}, "must not be empty"},
		{"update_issue_state", map[string]any{"state": "CLOSED"}, "closed"},
		{"update_issue_state", map[string]any{"state": "open", "state_reason": "completed"}, "state_reason can only"},
		{"update_issue_state", map[string]any{"state": "closed", "state_reason": "duplicate", "is_suggestion": true}, "duplicate_of is required"},
		{"update_issue_state", map[string]any{"state": "closed", "duplicate_of": 2}, "duplicate_of can only"},
		{"add_sub_issue", map[string]any{}, "missing required parameter: sub_issue_id"},
		{"remove_sub_issue", map[string]any{"sub_issue_id": "7.5"}, "non-integer"},
		{"reprioritize_sub_issue", map[string]any{"sub_issue_id": 7}, "either after_id or before_id"},
		{"reprioritize_sub_issue", map[string]any{"sub_issue_id": 7, "after_id": 8, "before_id": 9}, "only one"},
		{"set_issue_fields", map[string]any{"fields": []any{}}, "fields array must not be empty"},
		{"set_issue_fields", map[string]any{"fields": []any{map[string]any{"field_id": "IF_1", "text_value": ""}}}, "each field must have a value"},
		{"set_issue_fields", map[string]any{"fields": []any{map[string]any{"field_id": "IF_1", "text_value": "one", "number_value": 0}}}, "exactly one value"},
		{"set_issue_fields", map[string]any{"fields": []any{map[string]any{"field_id": "IF_1", "number_value": nil}}}, "each field must have a value"},
		{"add_issue_reaction", map[string]any{"content": "HEART"}, "heart"},
		{"remove_issue_reaction", map[string]any{"reaction_id": "1.5"}, "non-integer"},
		{"add_issue_comment_reaction", map[string]any{"comment_id": 42, "content": "invalid"}, "heart"},
		// Float-to-int overflow diagnostics differ across architectures.
		{"remove_issue_comment_reaction", map[string]any{"comment_id": "9223372036854775808", "reaction_id": 9}, "comment_id"},
	}

	testGranularNullResponses := func(t *testing.T) {
		for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
			for _, payload := range []string{`null`, `{}`} {
				t.Run("protocol="+protocol+"/payload="+payload, func(t *testing.T) {
					client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method == "POST" {
							w.WriteHeader(http.StatusCreated)
						}
						_, _ = w.Write([]byte(payload))
					})}}
					session, schemas := granularIssueSession(t, BaseDeps{Client: mustNewGHClient(t, client)}, protocol)
					for _, tc := range []struct {
						name string
						args map[string]any
						text string
					}{
						{"create_issue", map[string]any{"title": "Subject"}, `{"id":"0","url":""}`},
						{"update_issue_title", map[string]any{"title": "Subject"}, `{"id":"0","url":""}`},
						{"update_issue_body", map[string]any{"body": "Body"}, `{"id":"0","url":""}`},
						{"update_issue_assignees", map[string]any{"assignees": []string{}}, `{"id":"0","url":""}`},
						{"update_issue_labels", map[string]any{"labels": []string{}}, `{"id":"0","url":""}`},
						{"update_issue_milestone", map[string]any{"milestone": 3}, `{"id":"0","url":""}`},
						{"update_issue_type", map[string]any{"issue_type": nil}, `{"id":"0","url":""}`},
						{"update_issue_state", map[string]any{"state": "open"}, `{"id":"0","url":""}`},
						{"add_sub_issue", map[string]any{"sub_issue_id": 7}, payload},
						{"remove_sub_issue", map[string]any{"sub_issue_id": 7}, payload},
						{"reprioritize_sub_issue", map[string]any{"sub_issue_id": 7, "after_id": 8}, payload},
						{"add_issue_reaction", map[string]any{"content": "heart"}, `{"id":"0","url":"https://api.github.com/repos/owner/repo/issues/1/reactions/0"}`},
						{"add_issue_comment_reaction", map[string]any{"comment_id": 42, "content": "heart"}, `{"id":"0","url":"https://api.github.com/repos/owner/repo/issues/comments/42/reactions/0"}`},
					} {
						args := map[string]any{"owner": "owner", "repo": "repo", "issue_number": 1}
						maps.Copy(args, tc.args)
						result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
						require.NoError(t, err)
						require.False(t, result.IsError, tc.name)
						require.Len(t, result.Content, 1)
						expectedText := tc.text
						if strings.Contains(tc.name, "sub_issue") && schemas[tc.name] != nil {
							method := map[string]string{"add_sub_issue": "add", "remove_sub_issue": "remove", "reprioritize_sub_issue": "reprioritize"}[tc.name]
							expectedText = fmt.Sprintf(`{"method":%q,"issue":null}`, method)
							if payload == `{}` {
								expectedText = fmt.Sprintf(`{"method":%q,"issue":{"assignees":[]}}`, method)
							}
						}
						if schemas[tc.name] != nil {
							assert.JSONEq(t, expectedText, getTextResult(t, result).Text)
						} else {
							assert.Equal(t, expectedText, getTextResult(t, result).Text)
						}
						if schema := schemas[tc.name]; schema != nil {
							assert.JSONEq(t, expectedText, mustMarshalJSON(t, result.StructuredContent))
							require.NoError(t, schema.Validate(result.StructuredContent))
						} else {
							assert.Nil(t, result.StructuredContent)
						}
					}

				})
			}
		}
	}
	t.Run("null-responses", testGranularNullResponses)
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var current *granularIssueWireCase
			session, _ := granularIssueSession(t, granularWireDeps(t, &current, true), protocol)
			for _, tc := range cases {
				t.Run(tc.name+"/"+tc.text, func(t *testing.T) {
					args := map[string]any{"owner": "owner", "repo": "repo", "issue_number": "1"}
					maps.Copy(args, tc.args)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
					require.NoError(t, err)
					require.True(t, result.IsError, mustMarshalJSON(t, result))
					assert.Nil(t, result.StructuredContent)
					assert.Contains(t, getErrorResult(t, result).Text, tc.text)
				})
			}
			for _, tc := range granularIssueWireCases() {
				current = &tc
				args := map[string]any{"owner": "owner", "repo": "repo", "issue_number": "1"}
				maps.Copy(args, tc.args)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
				require.NoError(t, err)
				require.True(t, result.IsError, tc.name)
				assert.Nil(t, result.StructuredContent, tc.name)
				assert.Contains(t, getErrorResult(t, result).Text, "Forbidden", tc.name)
			}
		})
	}
}
