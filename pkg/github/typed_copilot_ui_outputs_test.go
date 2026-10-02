package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type typedOutputRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper typedOutputRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

func typedCopilotUIClient(t *testing.T) ToolDependencies {
	t.Helper()
	httpClient := &http.Client{Transport: typedOutputRoundTripper(func(request *http.Request) (*http.Response, error) {
		body := `[]`
		status := http.StatusOK
		switch request.URL.Path {
		case "/graphql":
			query, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			switch {
			case strings.Contains(string(query), "suggestedActors"):
				body = `{"data":{"repository":{"suggestedActors":{"nodes":[{"id":"BOT","login":"copilot-swe-agent","__typename":"Bot"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
			case strings.Contains(string(query), "issue(number:"):
				body = `{"data":{"repository":{"id":"REPO","issue":{"id":"ISSUE","assignees":{"nodes":[]}}}}}`
			case strings.Contains(string(query), "updateIssue(input:"):
				body = `{"data":{"updateIssue":{"issue":{"id":"ISSUE","number":7,"url":"https://github.com/owner/repo/issues/7"}}}}`
			case strings.Contains(string(query), "issueFields"):
				body = `{"data":{"repository":{"issueFields":{"nodes":[]}}}}`
			case strings.Contains(string(query), "labels"):
				body = `{"data":{"repository":{"labels":{"nodes":[],"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
			default:
				return nil, assert.AnError
			}
		case "/orgs/owner/issue-types":
			body = `[]`
		case "/repos/owner/repo/pulls/7/requested_reviewers":
			status = http.StatusCreated
			body = `{}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
	client := mustNewGHClient(t, httpClient)
	return BaseDeps{
		Client:    client,
		GQLClient: githubv4.NewEnterpriseClient("https://api.github.com/graphql", httpClient),
	}
}

func typedCopilotUISession(t *testing.T, deps ToolDependencies, protocol string) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	translation := translations.NullTranslationHelper
	tools := []inventory.ServerTool{
		AssignCopilotToIssue(translation),
		AssignCopilotToIssueWithIntent(translation),
		RequestCopilotReview(translation),
		UIGet(translation),
	}
	registry, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "copilot-ui", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			switch request := request.(type) {
			case *mcp.ListToolsRequest:
				if request.Params == nil {
					request.Params = &mcp.ListToolsParams{}
				}
				request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
			case *mcp.CallToolRequest:
				if request.Params == nil {
					request.Params = &mcp.CallToolParamsRaw{}
				}
				request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				ctx = ContextWithPollConfig(ctx, PollConfig{})
			}
			return next(ctx, method, request)
		}
	})
	registry.RegisterTools(context.Background(), server, deps)
	version := protocol
	if version == "" || version == "unknown" {
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
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	return session, schemas
}

func TestTypedCopilotAndUIWireOutputs(t *testing.T) {
	protocols := []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"}
	tests := []struct {
		name string
		args map[string]any
		text string
	}{
		{
			name: "assign_copilot_to_issue",
			args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "7"},
			text: `{"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"successfully assigned copilot to issue - pull request pending","note":"The pull request may still be in progress. Once created, the PR number can be used to check job status, or check the issue timeline for updates.","owner":"owner","repo":"repo"}`,
		},
		{
			name: "assign_copilot_to_issue_with_intent",
			args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "7", "rationale": "  Clear acceptance criteria  ", "confidence": "medium", "is_suggestion": "true"},
			text: `{"is_suggestion":true,"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"recorded pending copilot assignment suggestion","owner":"owner","repo":"repo"}`,
		},
		{
			name: "request_copilot_review",
			args: map[string]any{"owner": "owner", "repo": "repo", "pullNumber": "7"},
			text: "",
		},
		{
			name: "ui_get",
			args: map[string]any{"method": "labels", "owner": "owner", "repo": "repo"},
			text: `{"has_more":false,"labels":[],"totalCount":0}`,
		},
		{
			name: "ui_get",
			args: map[string]any{"method": "assignees", "owner": "owner", "repo": "repo"},
			text: `{"assignees":[],"has_more":false,"totalCount":0}`,
		},
		{
			name: "ui_get",
			args: map[string]any{"method": "milestones", "owner": "owner", "repo": "repo"},
			text: `{"has_more":false,"milestones":[],"totalCount":0}`,
		},
		{
			name: "ui_get",
			args: map[string]any{"method": "issue_types", "owner": "owner"},
			text: `[]`,
		},
		{
			name: "ui_get",
			args: map[string]any{"method": "branches", "owner": "owner", "repo": "repo"},
			text: `{"branches":[],"has_more":false,"totalCount":0}`,
		},
		{
			name: "ui_get",
			args: map[string]any{"method": "issue_fields", "owner": "owner", "repo": "repo"},
			text: `{"fields":[],"totalCount":0}`,
		},
		{
			name: "ui_get",
			args: map[string]any{"method": "reviewers", "owner": "owner", "repo": "repo"},
			text: `{"has_more":false,"teams":[],"totalCount":0,"users":[]}`,
		},
	}

	for _, protocol := range protocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, schemas := typedCopilotUISession(t, typedCopilotUIClient(t), protocol)
			for _, tc := range tests {
				t.Run(tc.name+"/"+mustMarshalJSON(t, tc.args), func(t *testing.T) {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
					require.NoError(t, err)
					require.False(t, result.IsError, mustMarshalJSON(t, result))
					require.Len(t, result.Content, 1)
					assert.Equal(t, tc.text, getTextResult(t, result).Text)
					schema := schemas[tc.name]
					if schema == nil {
						assert.Nil(t, result.StructuredContent)
						return
					}
					if tc.name == "request_copilot_review" {
						assert.Nil(t, result.StructuredContent)
						require.NoError(t, schema.Validate(nil))
						return
					}
					var output any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					require.NoError(t, schema.Validate(output))
					assert.JSONEq(t, tc.text, mustMarshalJSON(t, output))
				})
			}
		})
	}
}

func TestTypedCopilotUIErrorsPreserveLegacyText(t *testing.T) {
	protocols := []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"}
	tests := []struct {
		name string
		args map[string]any
		text string
	}{
		{name: "assign_copilot_to_issue", args: map[string]any{"repo": "repo", "issue_number": 7}, text: "missing required parameter: owner"},
		{name: "assign_copilot_to_issue_with_intent", args: map[string]any{"owner": "owner", "repo": "repo"}, text: "is_suggestion is required"},
		{name: "assign_copilot_to_issue_with_intent", args: map[string]any{"repo": "repo", "issue_number": 7, "rationale": strings.Repeat("x", 281), "confidence": "LOW", "is_suggestion": false}, text: "missing required parameter: owner"},
		{name: "assign_copilot_to_issue_with_intent", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": 7, "rationale": strings.Repeat("x", 281), "confidence": "LOW", "is_suggestion": false}, text: "rationale must be 280 characters or less"},
		{name: "assign_copilot_to_issue_with_intent", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": 7, "rationale": "valid", "confidence": "BAD", "is_suggestion": false}, text: "confidence must be one of: LOW, MEDIUM, HIGH"},
		{name: "request_copilot_review", args: map[string]any{"repo": "repo", "pullNumber": 7}, text: "missing required parameter: owner"},
		{name: "ui_get", args: map[string]any{"method": "labels", "owner": "owner"}, text: "missing required parameter: repo"},
		{name: "ui_get", args: map[string]any{"method": "unknown", "owner": "owner"}, text: "unknown method: unknown"},
		{name: "ui_get", args: map[string]any{"method": true, "owner": "owner"}, text: "parameter method is not of type string"},
	}
	for _, protocol := range protocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, _ := typedCopilotUISession(t, typedCopilotUIClient(t), protocol)
			for _, tc := range tests {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Len(t, result.Content, 1)
				assert.Equal(t, tc.text, getTextResult(t, result).Text)
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestTypedUIGetUnionOutputs(t *testing.T) {
	cases := []struct {
		method string
		raw    string
	}{
		{method: "labels", raw: `{"labels":[{"id":"L","name":"bug","color":"red","description":""}],"totalCount":1,"has_more":false}`},
		{method: "assignees", raw: `{"assignees":[{"login":"octocat","avatar_url":"https://example.com/avatar.png"}],"totalCount":1,"has_more":false}`},
		{method: "milestones", raw: `{"milestones":[{"number":4,"title":"v1","description":"","state":"open","open_issues":2,"due_on":""}],"totalCount":1,"has_more":false}`},
		{method: "issue_types", raw: `[{"id":1,"name":"Bug"}]`},
		{method: "branches", raw: `{"branches":[{"name":"main","sha":"abc","protected":true}],"totalCount":1,"has_more":false}`},
		{method: "issue_fields", raw: `{"fields":[{"id":"F","name":"Priority","data_type":"single_select","description":"","options":[]}],"totalCount":1}`},
		{method: "reviewers", raw: `{"users":[{"login":"octocat","avatar_url":""}],"teams":[{"slug":"docs","name":"Docs","org":"owner"}],"totalCount":2,"has_more":false}`},
	}
	schema, err := uiGetOutputSchema().Resolve(nil)
	require.NoError(t, err)
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			output, err := decodeUIGetOutput(tc.method, []byte(tc.raw))
			require.NoError(t, err)
			encoded, err := json.Marshal(output)
			require.NoError(t, err)
			assert.JSONEq(t, tc.raw, string(encoded))
			var value any
			require.NoError(t, json.Unmarshal(encoded, &value))
			require.NoError(t, schema.Validate(value))
		})
	}
}

func TestTypedCopilotOutputSchemas(t *testing.T) {
	cases := []struct {
		schema *jsonschema.Schema
		raw    string
	}{
		{
			schema: assignCopilotToIssueOutputSchema(),
			raw:    `{"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"assigned","note":"pending","owner":"owner","pull_request":{"number":8,"state":"OPEN","title":"Fix","url":"https://github.com/owner/repo/pull/8"},"repo":"repo"}`,
		},
		{
			schema: assignCopilotToIssueWithIntentOutputSchema(),
			raw:    `{"is_suggestion":true,"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"suggested","owner":"owner","repo":"repo"}`,
		},
	}
	for _, tc := range cases {
		resolved, err := tc.schema.Resolve(nil)
		require.NoError(t, err)
		var value any
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &value))
		require.NoError(t, resolved.Validate(value))
	}
}
