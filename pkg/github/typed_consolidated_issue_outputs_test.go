package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func consolidatedIssueSession(t *testing.T, deps BaseDeps, protocol string, ui bool) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	tools := []inventory.ServerTool{
		IssueRead(translations.NullTranslationHelper), IssueWrite(translations.NullTranslationHelper),
		SubIssueWrite(translations.NullTranslationHelper),
	}
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).
		WithFeatureChecker(featureCheckerFor()).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "consolidated-issues", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	inv.RegisterTools(context.Background(), server, deps)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			if protocol == "" {
				switch req := request.(type) {
				case *mcp.ListToolsRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
				case *mcp.CallToolRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
				}
			}
			return next(ghcontext.WithUISupport(ctx, ui), method, request)
		}
	})
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
		require.NotNil(t, tool.OutputSchema)
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		assert.Equal(t, "object", schema.Type, tool.Name)
		assert.NotEmpty(t, schema.Properties["method"], tool.Name)
		assertCompactIssueSchema(t, &schema)
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	return session, schemas
}

func assertCompactIssueSchema(t *testing.T, schema *jsonschema.Schema) {
	t.Helper()
	if schema == nil {
		return
	}
	for field, property := range schema.Properties {
		if strings.HasSuffix(field, "_url") && field != "html_url" {
			t.Errorf("hypermedia URL %q in compact issue output", field)
		}
		assert.NotEqual(t, "node_id", field)
		assert.NotEqual(t, "issue_field_values", field)
		assertCompactIssueSchema(t, property)
	}
	for _, definition := range schema.Definitions {
		assertCompactIssueSchema(t, definition)
	}
	for _, definition := range schema.Defs {
		assertCompactIssueSchema(t, definition)
	}
	assertCompactIssueSchema(t, schema.Items)
	for _, variants := range [][]*jsonschema.Schema{schema.OneOf, schema.AnyOf, schema.AllOf} {
		for _, variant := range variants {
			assertCompactIssueSchema(t, variant)
		}
	}
}

func TestCompactIssueOutputSchemasAreCached(t *testing.T) {
	assert.Same(t, issueReadOutputSchema(), issueReadOutputSchema())
	assert.Same(t, issueWriteOutputSchema(), issueWriteOutputSchema())
	assert.Same(t, subIssueWriteOutputSchema(), subIssueWriteOutputSchema())
}

func TestForbiddenIssueVariantPropertySchema(t *testing.T) {
	schema := forbiddenIssueVariantProperty()
	assert.JSONEq(t, `{"not":{"description":"Any value."}}`, mustMarshalJSON(t, schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	for _, value := range []any{nil, "", float64(0), false, []any{}, map[string]any{}} {
		assert.Error(t, resolved.Validate(value), "%v", value)
	}
}

func assertConsolidatedResult(t *testing.T, session *mcp.ClientSession, schemas map[string]*jsonschema.Resolved, name string, args map[string]any, text string) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.False(t, result.IsError, "%s: %s", name, mustMarshalJSON(t, result))
	require.Len(t, result.Content, 1)
	if schema := schemas[name]; schema != nil {
		// Nullable method data must still have a present object envelope.
		require.NotNil(t, result.StructuredContent)
		assert.JSONEq(t, mustMarshalJSON(t, result.StructuredContent), getTextResult(t, result).Text,
			"modern text must serialize the same compact DTO as structuredContent")
		var value any
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &value))
		require.NoError(t, schema.Validate(value))
	} else {
		assert.Equal(t, text, getTextResult(t, result).Text, "legacy text must preserve the original formatter output")
		assert.Nil(t, result.StructuredContent)
	}
	return result
}

func consolidatedIssueDeps(t *testing.T) BaseDeps {
	t.Helper()
	rest := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/comments") {
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			assert.Equal(t, "30", r.URL.Query().Get("per_page"))
			switch {
			case strings.Contains(r.URL.Path, "/2/"):
				_, _ = w.Write([]byte(`[]`))
			case strings.Contains(r.URL.Path, "/3/"):
				_, _ = w.Write([]byte(`null`))
			default:
				_, _ = w.Write([]byte(`[{"id":42,"body":"hello","html_url":"https://github.com/owner/repo/issues/1#issuecomment-42"}]`))
			}
			return
		}
		if strings.Contains(r.URL.Path, "/sub_issue") {
			if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/1/") {
				assert.Equal(t, "2", r.URL.Query().Get("page"))
				assert.Equal(t, "5", r.URL.Query().Get("per_page"))
			}
			switch {
			case strings.Contains(r.URL.Path, "/2/"):
				_, _ = w.Write([]byte(`[]`))
				return
			case strings.Contains(r.URL.Path, "/3/"):
				_, _ = w.Write([]byte(`null`))
				return
			case strings.Contains(r.URL.Path, "/4/"):
				_, _ = w.Write([]byte(`[null]`))
				return
			}
			if r.Method == http.MethodPost && !strings.HasSuffix(r.URL.Path, "/priority") {
				w.WriteHeader(http.StatusCreated)
			}
			text := `{"id":7,"number":7,"title":"Child","body":"body","created_at":"2026-01-01T00:00:00Z","issue_field_values":[{"issue_field_id":11,"node_id":"IF_11","data_type":"number","value":0}]}`
			if r.Method == http.MethodGet {
				text = "[" + text + "]"
			}
			_, _ = w.Write([]byte(text))
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte(`{"id":123,"number":1,"title":"Subject","state":"open","html_url":"https://github.com/owner/repo/issues/1","assignees":[],"labels":[]}`))
	})}}
	gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string `json:"query"`
			Variables struct {
				IssueNumber int `json:"issueNumber"`
				Input       struct {
					IssueID          string `json:"issueId"`
					StateReason      string `json:"stateReason"`
					DuplicateIssueID string `json:"duplicateIssueId"`
				} `json:"input"`
			} `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch {
		case strings.Contains(req.Query, "createIssue("):
			_, _ = w.Write([]byte(`{"data":{"createIssue":{"issue":{"fullDatabaseId":"123","url":"https://github.com/owner/repo/issues/1"}}}}`))
		case strings.Contains(req.Query, "parentRepository:"):
			_, _ = w.Write([]byte(`{"data":{"childRepository":{"id":"R_1","nameWithOwner":"owner/repo"},"parentRepository":{"issue":{"id":"I_parent","number":7}}}}`))
		case strings.Contains(req.Query, "parent{"):
			parent := `{"number":7,"title":"Parent","state":"OPEN","url":"https://github.com/owner/repo/issues/7","repository":{"nameWithOwner":"owner/repo"},"author":{"login":"octocat"}}`
			if req.Variables.IssueNumber == 2 {
				parent = "null"
			}
			_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"parent":` + parent + `}}}}`))
		case strings.Contains(req.Query, "labels("):
			labels := `{"nodes":[{"id":"L_1","name":"bug","color":"red","description":"Bug"}],"totalCount":1}`
			if req.Variables.IssueNumber == 2 {
				labels = `{"nodes":[],"totalCount":0}`
			}
			_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"labels":` + labels + `}}}}`))
		case strings.Contains(req.Query, "reopenIssue("):
			assert.Equal(t, "I_1", req.Variables.Input.IssueID)
			_, _ = w.Write([]byte(`{"data":{"reopenIssue":{"issue":{"id":"I_1","number":1,"url":"issue","state":"OPEN"}}}}`))
		case strings.Contains(req.Query, "closeIssue("):
			assert.Equal(t, "I_1", req.Variables.Input.IssueID)
			assert.Contains(t, []string{"COMPLETED", "NOT_PLANNED", "DUPLICATE"}, req.Variables.Input.StateReason)
			if req.Variables.Input.StateReason == "DUPLICATE" {
				assert.Equal(t, "I_2", req.Variables.Input.DuplicateIssueID)
			}
			_, _ = w.Write([]byte(`{"data":{"closeIssue":{"issue":{"id":"I_1","number":1,"url":"issue","state":"CLOSED"}}}}`))
		case strings.Contains(req.Query, "issue(number: $issueNumber)"):
			if strings.Contains(req.Query, "duplicateIssue:") {
				_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"id":"I_1"},"duplicateIssue":{"id":"I_2"}}}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"id":"I_1"}}}}`))
			}
		default:
			t.Errorf("unexpected GraphQL query: %s", req.Query)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})}}
	return BaseDeps{
		Client: mustNewGHClient(t, rest), GQLClient: githubv4.NewClient(gql),
		RepoAccessCache: stubRepoAccessCache(nil, time.Minute),
	}
}

func TestTypedConsolidatedIssueOutputs(t *testing.T) {
	const mutationText = `{"id":"123","url":"https://github.com/owner/repo/issues/1"}`
	const subText = `{"id":7,"number":7,"title":"Child","body":"body","created_at":"2026-01-01T00:00:00Z","issue_field_values":[{"issue_field_id":11,"node_id":"IF_11","data_type":"number","value":0}]}`
	const subStructured = `{"id":7,"number":7,"title":"Child","body":"body","created_at":"2026-01-01T00:00:00Z","assignees":[],"field_values":[{"issue_field_id":11,"data_type":"number","value":0}]}`
	cases := []struct {
		tool string
		args map[string]any
		text string
	}{
		{"issue_read", map[string]any{"method": "get", "issue_number": "1.0"}, `{"number":1,"title":"Subject","state":"open","html_url":"https://github.com/owner/repo/issues/1","assignees":[]}`},
		{"issue_read", map[string]any{"method": "get_comments", "issue_number": "1", "page": "0", "perPage": "0"}, `[{"id":42,"body":"hello","html_url":"https://github.com/owner/repo/issues/1#issuecomment-42"}]`},
		{"issue_read", map[string]any{"method": "get_comments", "issue_number": 2}, `[]`},
		{"issue_read", map[string]any{"method": "get_comments", "issue_number": 3}, `[]`},
		{"issue_read", map[string]any{"method": "get_sub_issues", "issue_number": "1", "page": "2", "perPage": "5"}, `[` + subText + `]`},
		{"issue_read", map[string]any{"method": "get_sub_issues", "issue_number": 2}, `[]`},
		{"issue_read", map[string]any{"method": "get_sub_issues", "issue_number": 3}, `null`},
		{"issue_read", map[string]any{"method": "get_sub_issues", "issue_number": 4}, `[null]`},
		{"issue_read", map[string]any{"method": "get_parent", "issue_number": "1"}, `{"parent":{"number":7,"repository":"owner/repo","state":"OPEN","title":"Parent","url":"https://github.com/owner/repo/issues/7"}}`},
		{"issue_read", map[string]any{"method": "get_parent", "issue_number": 2}, `{"parent":null}`},
		{"issue_read", map[string]any{"method": "get_labels", "issue_number": 1}, `{"labels":[{"color":"red","description":"Bug","id":"L_1","name":"bug"}],"totalCount":1}`},
		{"issue_read", map[string]any{"method": "get_labels", "issue_number": 2}, `{"labels":[],"totalCount":0}`},
		{"issue_write", map[string]any{"method": "create", "title": "Subject"}, mutationText},
		{"issue_write", map[string]any{"method": "update", "issue_number": "1.0"}, mutationText},
		{"issue_write", map[string]any{"method": "update", "issue_number": "1", "state": "open"}, mutationText},
		{"issue_write", map[string]any{"method": "update", "issue_number": 1, "state": "closed", "state_reason": "completed"}, mutationText},
		{"issue_write", map[string]any{"method": "update", "issue_number": 1, "state": "closed", "state_reason": "not_planned"}, mutationText},
		{"issue_write", map[string]any{"method": "update", "issue_number": 1, "state": "closed", "state_reason": "duplicate", "duplicate_of": "2"}, mutationText},
		{"issue_write", map[string]any{"method": "create", "title": "Subject", "parent_issue_number": "7"}, mutationText},
		{"sub_issue_write", map[string]any{"method": "ADD", "issue_number": "1", "sub_issue_id": "7", "replace_parent": true}, subText},
		{"sub_issue_write", map[string]any{"method": "remove", "issue_number": 1, "sub_issue_id": 7}, subText},
		{"sub_issue_write", map[string]any{"method": "reprioritize", "issue_number": 1, "sub_issue_id": 7, "after_id": "8.0"}, subText},
		{"sub_issue_write", map[string]any{"method": "reprioritize", "issue_number": 1, "sub_issue_id": 7, "before_id": "8"}, subText},
	}
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, schemas := consolidatedIssueSession(t, consolidatedIssueDeps(t), protocol, false)
			for i, tc := range cases {
				t.Run(tc.tool+"/"+tc.args["method"].(string)+"/"+string(rune('A'+i)), func(t *testing.T) {
					args := map[string]any{"owner": "owner", "repo": "repo"}
					maps.Copy(args, tc.args)
					result := assertConsolidatedResult(t, session, schemas, tc.tool, args, tc.text)
					if protocol == inventory.ProtocolVersionMultiRoundTrip {
						want := tc.text
						switch want {
						case subText:
							want = subStructured
						case "[" + subText + "]":
							want = "[" + subStructured + "]"
						}
						method := strings.ToLower(tc.args["method"].(string))
						field := "issue"
						switch method {
						case "get_comments":
							field = "comments"
						case "get_sub_issues":
							field = "sub_issues"
						case "get_parent":
							field = "parent"
						case "get_labels":
							field = "labels"
						}
						if method == "get_parent" || method == "get_labels" {
							want = `{"method":"` + method + `",` + strings.TrimPrefix(want, "{")
						} else {
							want = `{"method":"` + method + `","` + field + `":` + want + `}`
						}
						assert.JSONEq(t, want, mustMarshalJSON(t, result.StructuredContent))
					}
				})
			}
			for _, tc := range []struct {
				tool string
				args map[string]any
				text string
			}{
				{"issue_read", map[string]any{}, "missing required parameter: method"},
				{"issue_read", map[string]any{"method": "get", "issue_number": 0}, "missing required parameter: issue_number"},
				{"issue_read", map[string]any{"method": "get", "issue_number": "1.5"}, "non-integer numeric value"},
				{"issue_read", map[string]any{"method": "GET", "issue_number": 1}, "get"},
				{"issue_read", map[string]any{"method": "get", "issue_number": 1, "page": nil}, "not a valid number"},
				{"issue_write", map[string]any{"method": "create"}, "missing required parameter: title"},
				{"issue_write", map[string]any{"method": "update"}, "missing required parameter: issue_number"},
				{"issue_write", map[string]any{"method": "update", "issue_number": 1, "type": ""}, "must not be empty"},
				{"issue_write", map[string]any{"method": "update", "issue_number": 1, "labels": 42}, "[]string"},
				{"issue_write", map[string]any{"method": "create", "parent_issue_number": 0}, "greater than 0"},
				{"issue_write", map[string]any{"method": "update", "issue_number": 1, "duplicate_of": 2}, "duplicate_of can only"},
				{"issue_write", map[string]any{"method": "create", "issue_fields": []any{map[string]any{"field_name": "Priority", "value": nil}}}, "value cannot be null"},
				{"sub_issue_write", map[string]any{"method": "add", "issue_number": 1}, "missing required parameter: sub_issue_id"},
				{"sub_issue_write", map[string]any{"method": "reprioritize", "issue_number": 1, "sub_issue_id": 7}, "either after_id or before_id"},
				{"sub_issue_write", map[string]any{"method": "reprioritize", "issue_number": 1, "sub_issue_id": 7, "after_id": 8, "before_id": 9}, "only one"},
			} {
				args := map[string]any{"owner": "owner", "repo": "repo"}
				maps.Copy(args, tc.args)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				require.NoError(t, err)
				require.True(t, result.IsError, "%s: %s", tc.tool, mustMarshalJSON(t, result))
				assert.Contains(t, getErrorResult(t, result).Text, tc.text)
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestTypedIssueWriteAwaitingForm(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, schemas := consolidatedIssueSession(t, consolidatedIssueDeps(t), protocol, true)
			for _, method := range []string{"create", "update"} {
				args := map[string]any{"method": method, "owner": "owner", "repo": "repo", "issue_number": "1", "title": "Subject"}
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "issue_write", Arguments: args})
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Len(t, result.Content, 1)
				assert.Contains(t, getErrorResult(t, result).Text, "interactive form has been shown")
				assert.Contains(t, getErrorResult(t, result).Text, "NOT been")
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					require.NotNil(t, result.StructuredContent)
					assert.JSONEq(t, `{"status":"awaiting_user_submission","reason":"An interactive form is being shown to the user. The operation has not been performed."}`, mustMarshalJSON(t, result.StructuredContent))
					require.NoError(t, schemas["issue_write"].Validate(result.StructuredContent))
				} else {
					assert.Nil(t, result.StructuredContent)
				}
				args["_ui_submitted"] = true
				assertConsolidatedResult(t, session, schemas, "issue_write", args, `{"id":"123","url":"https://github.com/owner/repo/issues/1"}`)
			}
		})
	}
}

func TestTypedConsolidatedIssueAPIErrors(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			})}}
			deps := BaseDeps{Client: mustNewGHClient(t, client), GQLClient: githubv4.NewClient(client)}
			session, _ := consolidatedIssueSession(t, deps, protocol, false)
			for tool, methods := range map[string][]string{
				"issue_read":  {"get", "get_comments", "get_sub_issues", "get_parent", "get_labels"},
				"issue_write": {"create", "update"}, "sub_issue_write": {"add", "remove", "reprioritize"},
			} {
				for _, method := range methods {
					args := map[string]any{
						"method": method, "owner": "owner", "repo": "repo", "issue_number": 1,
						"title": "Subject", "sub_issue_id": 7, "after_id": 8,
					}
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
					require.NoError(t, err)
					require.True(t, result.IsError)
					assert.Nil(t, result.StructuredContent, tool+"/"+method)
					assert.Contains(t, getErrorResult(t, result).Text, "Forbidden")
				}
			}
		})
	}
}

func TestTypedConsolidatedNullMutationResponses(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		for _, payload := range []string{`{}`, `null`} {
			t.Run("protocol="+protocol+"/payload="+payload, func(t *testing.T) {
				client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost && !strings.HasSuffix(r.URL.Path, "/priority") {
						w.WriteHeader(http.StatusCreated)
					}
					_, _ = w.Write([]byte(payload))
				})}}
				deps := BaseDeps{Client: mustNewGHClient(t, client), GQLClient: githubv4.NewClient(client)}
				session, schemas := consolidatedIssueSession(t, deps, protocol, false)
				for _, method := range []string{"create", "update"} {
					assertConsolidatedResult(t, session, schemas, "issue_write", map[string]any{
						"method": method, "owner": "owner", "repo": "repo", "issue_number": 1, "title": "Subject",
					}, `{"id":"0","url":""}`)
				}
				for _, method := range []string{"add", "remove", "reprioritize"} {
					result := assertConsolidatedResult(t, session, schemas, "sub_issue_write", map[string]any{
						"method": method, "owner": "owner", "repo": "repo", "issue_number": 1, "sub_issue_id": 7, "after_id": 8,
					}, payload)
					if protocol == inventory.ProtocolVersionMultiRoundTrip {
						data := `{"assignees":[]}`
						if payload == "null" {
							data = "null"
						}
						assert.JSONEq(t, `{"method":"`+method+`","issue":`+data+`}`, mustMarshalJSON(t, result.StructuredContent))
					}
				}
			})
		}
	}
}

func TestConsolidatedIssueStrictSchemasAndScalars(t *testing.T) {
	fieldSchema := *IssueWrite(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema).Properties["issue_fields"].Items
	fieldSchema.OneOf = issueWriteFieldVariants()
	for _, tc := range []struct {
		schema  *jsonschema.Schema
		valid   []string
		invalid []string
	}{
		{
			issueReadOutputSchema(),
			[]string{`{"method":"get","issue":null}`, `{"method":"get_comments","comments":[]}`, `{"method":"get_sub_issues","sub_issues":[null]}`, `{"method":"get_parent","parent":null}`, `{"method":"get","issue":{"number":0,"title":"","state":"","assignees":[]}}`, `{"method":"get_labels","labels":[],"totalCount":0}`},
			[]string{`null`, `[]`, `{}`, `{"method":"get"}`, `{"method":"get","comments":[]}`, `{"method":"get","issue":{"number":1}}`, `{"method":"get_parent","parent":{"number":1}}`, `{"method":"get_labels","labels":[]}`, `{"method":"get","issue":null,"comments":[]}`},
		},
		{
			issueWriteOutputSchema(),
			[]string{`{"method":"create","issue":{"id":"","url":""}}`, `{"status":"awaiting_user_submission","reason":"wait"}`},
			[]string{`{}`, `{"method":"create","issue":{"id":"1"}}`, `{"status":"created","reason":"wait"}`, `{"status":"awaiting_user_submission"}`, `{"method":"create","issue":{"id":"1","url":"url"},"status":"awaiting_user_submission","reason":"wait"}`},
		},
		{
			subIssueWriteOutputSchema(),
			[]string{
				`{"method":"add","issue":null}`,
				`{"method":"remove","issue":{"assignees":[]}}`,
				`{"method":"reprioritize","issue":{"state":"closed","state_reason":"duplicate","assignees":[]}}`,
				`{"method":"add","issue":{"assignees":[],"field_values":[null,{"issue_field_id":1,"data_type":"date","value":"2026-01-01"}]}}`,
			},
			[]string{
				`null`, `{}`, `{"method":"add"}`, `{"method":"get","issue":null}`,
				`{"method":"add","issue":{"assignees":[],"state":"OPEN"}}`,
				`{"method":"add","issue":{"assignees":[],"state_reason":"unknown"}}`,
				`{"method":"add","issue":{"assignees":[],"url":"api"}}`,
				`{"method":"add","issue":{"assignees":[],"field_values":[{"issue_field_id":1,"data_type":"unknown","value":1}]}}`,
				`{"method":"add","issue":{"assignees":[],"field_values":[{"issue_field_id":1,"data_type":"number","value":{}}]}}`,
			},
		},
		{
			&fieldSchema,
			[]string{`{"field_name":"Text","value":""}`, `{"field_name":"Number","value":0,"delete":false}`, `{"field_name":"Date","value":"2026-01-01"}`, `{"field_name":"Priority","field_option_name":"High"}`, `{"field_name":"Text","delete":true}`},
			[]string{`{"field_name":"Text"}`, `{"field_name":"Text","value":null}`, `{"field_name":"Text","value":1,"delete":true}`, `{"field_name":"Text","value":1,"field_option_name":"High"}`, `{"field_name":"Text","delete":false}`, `{"value":1}`},
		},
	} {
		resolved, err := tc.schema.Resolve(nil)
		require.NoError(t, err)
		for _, raw := range tc.valid {
			var value any
			require.NoError(t, json.Unmarshal([]byte(raw), &value))
			require.NoError(t, resolved.Validate(value), raw)
		}
		for _, raw := range tc.invalid {
			var value any
			require.NoError(t, json.Unmarshal([]byte(raw), &value))
			require.Error(t, resolved.Validate(value), raw)
		}
	}
	for _, raw := range []string{`""`, `"2026-01-01"`, `"High"`, `0`, `1.5`, `false`, `null`} {
		var value IssueFieldValue
		require.NoError(t, json.Unmarshal([]byte(raw), &value))
		assert.JSONEq(t, raw, mustMarshalJSON(t, value))
	}
	var value IssueFieldValue
	require.Error(t, json.Unmarshal([]byte(`{}`), &value))
	require.Error(t, json.Unmarshal([]byte(`[]`), &value))
}

func TestSubIssueOutputProjectsCompleteAPIResponse(t *testing.T) {
	// Every exported API field is populated, including false/zero pointers,
	// nullable collection elements and all custom-field scalar variants.
	var issue github.SubIssue
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":1,"number":1,"state":"open","state_reason":"reopened","locked":false,
		"title":"Child\u202e","body":"body\u202e","author_association":"OWNER",
		"user":{"login":"octo","id":1},"labels":[null,{"name":"bug","id":2}],
		"assignee":{"login":"octo"},"comments":0,"closed_at":"2026-01-01T00:00:00Z",
		"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z",
		"closed_by":{"login":"octo"},"url":"api","html_url":"html","comments_url":"comments",
		"events_url":"events","labels_url":"labels","repository_url":"repo","parent_issue_url":"parent",
		"milestone":{"title":"v1","number":1},"pull_request":{"url":"pr"},
		"repository":{"id":1,"name":"repo","owner":{"login":"octo"},"topics":["topic"]},
		"reactions":{"total_count":0},"assignees":[null,{"login":"octo"}],
		"node_id":"I_1","draft":false,"type":{"id":1,"name":"Bug"},
		"pinned_comment":{"id":1,"body":"comment"},"performed_via_github_app":{"id":1,"permissions":{"issues":"write"}},
		"issue_dependencies_summary":{"blocked_by":0,"blocking":1},
		"sub_issues_summary":{"total":1,"completed":0,"percent_completed":0},
		"issue_field_values":[null,
			{"issue_field_id":1,"node_id":"IF_1","data_type":"text","value":""},
			{"issue_field_id":2,"node_id":"IF_2","data_type":"number","value":0},
			{"issue_field_id":3,"node_id":"IF_3","data_type":"date","value":"2026-01-01"},
			{"issue_field_id":4,"node_id":"IF_4","data_type":"single_select","value":"High","single_select_option":{"id":1,"name":"High","color":"red"}},
			{"issue_field_id":5,"node_id":"IF_5","data_type":"text","value":null}
		],
		"text_matches":[{"fragment":"match","matches":[{"text":"match","indices":[0,5]}]}],
		"active_lock_reason":"resolved"
	}`), &issue))
	sanitizeSubIssueTitleAndBody(&issue)
	out, err := subIssueOutput(&issue)
	require.NoError(t, err)
	assert.Equal(t, sanitize.PlainText("Child\u202e"), *out.Title)
	assert.JSONEq(t, `{
		"id":1,"number":1,"title":"Child","body":"body","state":"open","state_reason":"reopened",
		"html_url":"html","user":{"login":"octo","id":1},"labels":["bug"],"assignees":["octo"],
		"milestone":"v1","comments":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","closed_at":"2026-01-01T00:00:00Z",
		"issue_type":"Bug","sub_issues_summary":{"total":1,"completed":0,"percent_completed":0},
		"field_values":[null,{"issue_field_id":1,"data_type":"text","value":""},{"issue_field_id":2,"data_type":"number","value":0},
		{"issue_field_id":3,"data_type":"date","value":"2026-01-01"},{"issue_field_id":4,"data_type":"single_select","value":"High"},
		{"issue_field_id":5,"data_type":"text","value":null}]
	}`, mustMarshalJSON(t, out))
	resolved, err := subIssueSchema().Resolve(nil)
	require.NoError(t, err)
	var value any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, out)), &value))
	require.NoError(t, resolved.Validate(value))
}

func TestTypedIssueCommentsCompactUsersAndLockdown(t *testing.T) {
	const comment = `{"id":42,"body":"hello\u202e","html_url":"comment","url":"api-comment","node_id":"IC_42","user":{"login":"author","id":7,"html_url":"profile","avatar_url":"avatar"}}`
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		for _, lockdown := range []bool{false, true} {
			t.Run(fmt.Sprintf("protocol=%s/lockdown=%t", protocol, lockdown), func(t *testing.T) {
				client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte(`[` + comment + `,{"id":43,"body":"restricted","html_url":"unsafe","user":{"login":"unsafe"}}]`))
				})}}
				deps := BaseDeps{
					Client: mustNewGHClient(t, client), GQLClient: githubv4.NewClient(client),
					RepoAccessCache: stubRepoAccessCache(mockRESTPermissionServer(t, "read", map[string]string{"author": "write"}), time.Minute),
					Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": lockdown}),
				}
				session, schemas := consolidatedIssueSession(t, deps, protocol, false)
				text := `[{"id":42,"body":"hello","html_url":"comment","user":{"login":"author","id":7,"profile_url":"profile","avatar_url":"avatar"}}`
				projected := `{"method":"get_comments","comments":[{"id":42,"body":"hello","html_url":"comment","user":{"login":"author","id":7}}`
				if !lockdown {
					unsafe := `,{"id":43,"body":"restricted","html_url":"unsafe","user":{"login":"unsafe"}}`
					text += unsafe
					projected += unsafe
				}
				result := assertConsolidatedResult(t, session, schemas, "issue_read", map[string]any{
					"method": "get_comments", "owner": "owner", "repo": "repo", "issue_number": 1,
				}, text+`]`)
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					assert.JSONEq(t, projected+`]}`, mustMarshalJSON(t, result.StructuredContent))
				}
			})
		}
	}
}

func TestTypedIssueWriteMutationOptionalityAndFields(t *testing.T) {
	const resultText = `{"id":"123","url":"https://github.com/owner/repo/issues/1"}`
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var requestBody string
			var deleted []string
			rest := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					deleted = append(deleted, r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
					return
				}

				var body json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				requestBody = string(body)
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusCreated)
				}
				var sent struct {
					Labels []string `json:"labels"`
				}
				require.NoError(t, json.Unmarshal(body, &sent))
				response := &github.Issue{ID: new(int64(123)), HTMLURL: new("https://github.com/owner/repo/issues/1")}
				for _, label := range sent.Labels {
					response.Labels = append(response.Labels, &github.Label{Name: label})
				}
				require.NoError(t, json.NewEncoder(w).Encode(response))
			})}}
			existing := `[{"__typename":"IssueFieldTextValue","field":{"fullDatabaseId":"1","name":"Text"},"value":"old"},{"__typename":"IssueFieldDateValue","field":{"fullDatabaseId":"3","name":"Date"},"value":"2026-01-01"}]`
			gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Query string `json:"query"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
				switch {
				case strings.Contains(request.Query, "issueFields("):
					_, _ = w.Write([]byte(`{"data":{"repository":{"issueFields":{"nodes":[
						{"__typename":"IssueFieldText","fullDatabaseId":"1","name":"Text","dataType":"TEXT"},
						{"__typename":"IssueFieldNumber","fullDatabaseId":"2","name":"Number","dataType":"NUMBER"},
						{"__typename":"IssueFieldDate","fullDatabaseId":"3","name":"Date","dataType":"DATE"},
						{"__typename":"IssueFieldSingleSelect","fullDatabaseId":"4","name":"Priority","dataType":"SINGLE_SELECT","options":[{"fullDatabaseId":"40","name":"High"}]}
					]}}}}`))
				case strings.Contains(request.Query, "issueFieldValues("):
					_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"issueFieldValues":{"nodes":` + existing + `}}}}}`))
				default:
					t.Errorf("unexpected GraphQL mutation/query: %s", request.Query)
				}
			})}}
			deps := BaseDeps{Client: mustNewGHClient(t, rest), GQLClient: githubv4.NewClient(gql)}
			session, schemas := consolidatedIssueSession(t, deps, protocol, false)
			cases := []struct {
				args map[string]any
				body string
			}{
				{map[string]any{}, `{}`},
				{map[string]any{"labels": []string{}, "assignees": []string{}, "type": nil, "milestone": "0"}, `{"labels":[],"assignees":[],"type":null}`},
				{map[string]any{"labels": nil, "assignees": nil}, `{}`},
				{map[string]any{"labels": []string{"bug"}, "assignees": []string{"octo"}, "type": "Bug", "milestone": "2.0"}, `{"labels":["bug"],"assignees":["octo"],"type":"Bug","milestone":2}`},
				{map[string]any{"issue_fields": []any{
					map[string]any{"field_name": "Number", "value": 0},
					map[string]any{"field_name": "Text", "delete": true},
				}}, `{"issue_field_values":[{"field_id":2,"value":0},{"field_id":3,"value":"2026-01-01"}]}`},
				{map[string]any{"method": "create", "title": "Subject", "body": "", "issue_fields": []any{
					map[string]any{"field_name": " text ", "value": ""},
					map[string]any{"field_name": "Number", "value": 0},
					map[string]any{"field_name": "Date", "value": "2026-01-01"},
					map[string]any{"field_name": "Priority", "field_option_name": " high "},
				}}, `{"title":"Subject","body":"","issue_field_values":[{"field_id":1,"value":""},{"field_id":2,"value":0},{"field_id":3,"value":"2026-01-01"},{"field_id":4,"value":"High"}]}`},
				{map[string]any{"method": "create", "title": "Subject", "issue_fields": []any{
					map[string]any{"field_name": "Text", "delete": true},
				}}, `{"title":"Subject","body":""}`},
			}
			for _, tc := range cases {
				args := map[string]any{"method": "update", "owner": "owner", "repo": "repo", "issue_number": "1"}
				maps.Copy(args, tc.args)
				assertConsolidatedResult(t, session, schemas, "issue_write", args, resultText)
				assert.JSONEq(t, tc.body, requestBody)
			}
			existing = `[{"__typename":"IssueFieldTextValue","field":{"fullDatabaseId":"1","name":"Text"},"value":"old"}]`
			assertConsolidatedResult(t, session, schemas, "issue_write", map[string]any{
				"method": "update", "owner": "owner", "repo": "repo", "issue_number": 1,
				"issue_fields": []any{map[string]any{"field_name": "Text", "delete": true}},
			}, resultText)
			assert.JSONEq(t, `{}`, requestBody)
			assert.Equal(t, []string{"/repos/owner/repo/issues/1/issue-field-values/1"}, deleted)
			for _, fields := range []any{
				[]any{map[string]any{"field_name": "Missing", "value": "value"}},
				[]any{map[string]any{"field_name": "Text", "field_option_name": "High"}},
				[]any{map[string]any{"field_name": "Priority", "field_option_name": "Missing"}},
			} {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "issue_write", Arguments: map[string]any{
					"method": "create", "owner": "owner", "repo": "repo", "title": "Subject", "issue_fields": fields,
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent)
				assert.Contains(t, getErrorResult(t, result).Text, "failed to resolve issue_fields")
			}
		})
	}
}
