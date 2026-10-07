package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/http/middleware"
	"github.com/github/github-mcp-server/v2/pkg/http/oauth"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
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

func typedCopilotUIClient(t *testing.T, fixture ...string) ToolDependencies {
	t.Helper()
	fixtureMode := ""
	if len(fixture) > 0 {
		fixtureMode = fixture[0]
	}
	httpClient := &http.Client{Transport: typedOutputRoundTripper(func(request *http.Request) (*http.Response, error) {
		body := `[]`
		status := http.StatusOK
		switch request.URL.Path {
		case "/graphql":
			query, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			switch {
			case strings.Contains(string(query), "timelineItems"):
				body = `{"data":{"repository":{"issue":{"timelineItems":{"nodes":[{"__typename":"CrossReferencedEvent","source":{"number":8,"url":"https://github.enterprise.example/owner/repo/pull/8","title":"Fix","state":"OPEN","createdAt":"` + time.Now().Add(time.Minute).UTC().Format(time.RFC3339) + `","author":{"login":"copilot-swe-agent"}}}]}}}}}`
			case strings.Contains(string(query), "suggestedActors"):
				body = `{"data":{"repository":{"suggestedActors":{"nodes":[{"id":"BOT","login":"copilot-swe-agent","__typename":"Bot"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
			case strings.Contains(string(query), "issue(number:"):
				body = `{"data":{"repository":{"id":"REPO","issue":{"id":"ISSUE","assignees":{"nodes":[]}}}}}`
			case strings.Contains(string(query), "updateIssue(input:"):
				body = `{"data":{"updateIssue":{"issue":{"id":"ISSUE","number":7,"url":"https://github.com/owner/repo/issues/7"}}}}`
				if fixtureMode == "linked-enterprise" {
					body = strings.ReplaceAll(body, "https://github.com/", "https://github.enterprise.example/")
				}
			case strings.Contains(string(query), "issueFields"):
				if fixtureMode == "nonempty" {
					body = `{"data":{"repository":{"issueFields":{"nodes":[{"__typename":"IssueFieldText","id":"FIELD_1","name":"Priority","description":"Issue priority","dataType":"text","visibility":"ALL"}]}}}}`
				} else {
					body = `{"data":{"repository":{"issueFields":{"nodes":[]}}}}`
				}
			case strings.Contains(string(query), "labels"):
				if fixtureMode == "nonempty" {
					body = `{"data":{"repository":{"labels":{"nodes":[{"id":"LABEL_1","name":"bug","color":"d73a4a","description":"Something isn't working"}],"totalCount":1,"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
				} else {
					body = `{"data":{"repository":{"labels":{"nodes":[],"totalCount":0,"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
				}
			default:
				return nil, assert.AnError
			}
		case "/orgs/owner/issue-types":
			switch fixtureMode {
			case "nonempty":
				body = `[{"id":12,"name":"Bug","description":"A bug","color":"red"},null]`
			case "issue-types-null":
				body = `null`
			}
		case "/repos/owner/repo/assignees":
			if fixtureMode == "nonempty" {
				body = `[{"login":"octocat","avatar_url":"https://example.com/avatar.png"}]`
			}
		case "/repos/owner/repo/branches":
			if fixtureMode == "nonempty" {
				body = `[{"name":"main","protected":true,"commit":{"sha":"abc123"}}]`
			}
		case "/repos/owner/repo/milestones":
			if fixtureMode == "nonempty" {
				body = `[{"number":4,"title":"v1","description":"First release","state":"open","open_issues":2,"due_on":"2026-12-31T00:00:00Z"}]`
			}
		case "/repos/owner/repo/collaborators":
			if fixtureMode == "nonempty" {
				body = `[{"login":"octocat","avatar_url":"https://example.com/avatar.png"}]`
			}
		case "/repos/owner/repo/teams":
			if fixtureMode == "nonempty" {
				body = `[{"slug":"docs","name":"Docs"}]`
			}
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

func typedCopilotUISession(t *testing.T, deps ToolDependencies, protocol string, pollConfigs ...PollConfig) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
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
	registry.RegisterTools(context.Background(), server, deps)
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
				pollConfig := PollConfig{}
				if len(pollConfigs) > 0 {
					pollConfig = pollConfigs[0]
				}
				ctx = ContextWithPollConfig(ctx, pollConfig)
				ctx = ghcontext.WithMCPMethodInfo(ctx, &ghcontext.MCPMethodInfo{ProtocolVersion: protocol})
			}
			return next(ctx, method, request)
		}
	})
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
		name       string
		args       map[string]any
		text       string
		structured string
	}{
		{
			name:       "assign_copilot_to_issue",
			args:       map[string]any{"owner": "owner", "repo": "repo", "issue_number": "7"},
			text:       `{"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"successfully assigned copilot to issue - pull request pending","note":"The pull request may still be in progress. Once created, the PR number can be used to check job status, or check the issue timeline for updates.","owner":"owner","repo":"repo"}`,
			structured: `{"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"successfully assigned copilot to issue - pull request pending","note":"The pull request may still be in progress. Once created, the PR number can be used to check job status, or check the issue timeline for updates.","owner":"owner","repo":"repo"}`,
		},
		{
			name:       "assign_copilot_to_issue_with_intent",
			args:       map[string]any{"owner": "owner", "repo": "repo", "issue_number": "7", "rationale": "  Clear acceptance criteria  ", "confidence": "medium", "is_suggestion": "true"},
			text:       `{"is_suggestion":true,"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"recorded pending copilot assignment suggestion","owner":"owner","repo":"repo"}`,
			structured: `{"is_suggestion":true,"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"recorded pending copilot assignment suggestion","owner":"owner","repo":"repo"}`,
		},
		{
			name:       "assign_copilot_to_issue_with_intent",
			args:       map[string]any{"owner": "owner", "repo": "repo", "issue_number": "7", "rationale": "Clear acceptance criteria", "confidence": "HIGH", "is_suggestion": false},
			text:       `{"is_suggestion":false,"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"successfully assigned copilot to issue - pull request pending","note":"The pull request may still be in progress. Once created, the PR number can be used to check job status, or check the issue timeline for updates.","owner":"owner","repo":"repo"}`,
			structured: `{"is_suggestion":false,"issue_number":7,"issue_url":"https://github.com/owner/repo/issues/7","message":"successfully assigned copilot to issue - pull request pending","note":"The pull request may still be in progress. Once created, the PR number can be used to check job status, or check the issue timeline for updates.","owner":"owner","repo":"repo"}`,
		},
		{
			name:       "request_copilot_review",
			args:       map[string]any{"owner": "owner", "repo": "repo", "pullNumber": "7"},
			text:       "",
			structured: `{"status":"requested"}`,
		},
		{
			name:       "ui_get",
			args:       map[string]any{"method": "labels", "owner": "owner", "repo": "repo"},
			text:       `{"has_more":false,"labels":[],"totalCount":0}`,
			structured: `{"method":"labels","labels":{"has_more":false,"labels":[],"totalCount":0}}`,
		},
		{
			name:       "ui_get",
			args:       map[string]any{"method": "assignees", "owner": "owner", "repo": "repo"},
			text:       `{"assignees":[],"has_more":false,"totalCount":0}`,
			structured: `{"method":"assignees","assignees":{"assignees":[],"has_more":false,"totalCount":0}}`,
		},
		{
			name:       "ui_get",
			args:       map[string]any{"method": "milestones", "owner": "owner", "repo": "repo"},
			text:       `{"has_more":false,"milestones":[],"totalCount":0}`,
			structured: `{"method":"milestones","milestones":{"has_more":false,"milestones":[],"totalCount":0}}`,
		},
		{
			name:       "ui_get",
			args:       map[string]any{"method": "issue_types", "owner": "owner"},
			text:       `[]`,
			structured: `{"method":"issue_types","issue_types":[]}`,
		},
		{
			name:       "ui_get",
			args:       map[string]any{"method": "branches", "owner": "owner", "repo": "repo"},
			text:       `{"branches":[],"has_more":false,"totalCount":0}`,
			structured: `{"method":"branches","branches":{"branches":[],"has_more":false,"totalCount":0}}`,
		},
		{
			name:       "ui_get",
			args:       map[string]any{"method": "issue_fields", "owner": "owner", "repo": "repo"},
			text:       `{"fields":[],"totalCount":0}`,
			structured: `{"method":"issue_fields","issue_fields":{"fields":[],"totalCount":0}}`,
		},
		{
			name:       "ui_get",
			args:       map[string]any{"method": "reviewers", "owner": "owner", "repo": "repo"},
			text:       `{"has_more":false,"teams":[],"totalCount":0,"users":[]}`,
			structured: `{"method":"reviewers","reviewers":{"has_more":false,"teams":[],"totalCount":0,"users":[]}}`,
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
					schema := schemas[tc.name]
					if schema == nil {
						assert.Equal(t, tc.text, getTextResult(t, result).Text)
						assert.Nil(t, result.StructuredContent)
						return
					}
					var output any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					require.NoError(t, schema.Validate(output))
					assert.JSONEq(t, tc.structured, mustMarshalJSON(t, output))
					text := getTextResult(t, result).Text
					assert.Equal(t, mustMarshalJSON(t, result.StructuredContent), text,
						"modern success text should be the compact structured output")
				})
			}
		})
	}
}

func TestTypedUIGetNonemptyWireOutputs(t *testing.T) {
	session, schemas := typedCopilotUISession(t, typedCopilotUIClient(t, "nonempty"), inventory.ProtocolVersionMultiRoundTrip)
	tests := []struct {
		method     string
		args       map[string]any
		structured string
	}{
		{method: "labels", args: map[string]any{"method": "labels", "owner": "owner", "repo": "repo"}, structured: `{"method":"labels","labels":{"labels":[{"id":"LABEL_1","name":"bug","color":"d73a4a","description":"Something isn't working"}],"totalCount":1,"has_more":false}}`},
		{method: "assignees", args: map[string]any{"method": "assignees", "owner": "owner", "repo": "repo"}, structured: `{"method":"assignees","assignees":{"assignees":[{"login":"octocat","avatar_url":"https://example.com/avatar.png"}],"totalCount":1,"has_more":false}}`},
		{method: "milestones", args: map[string]any{"method": "milestones", "owner": "owner", "repo": "repo"}, structured: `{"method":"milestones","milestones":{"milestones":[{"number":4,"title":"v1","description":"First release","state":"open","open_issues":2,"due_on":"2026-12-31"}],"totalCount":1,"has_more":false}}`},
		{method: "issue_types", args: map[string]any{"method": "issue_types", "owner": "owner"}, structured: `{"method":"issue_types","issue_types":[{"id":12,"name":"Bug","description":"A bug","color":"red"},null]}`},
		{method: "branches", args: map[string]any{"method": "branches", "owner": "owner", "repo": "repo"}, structured: `{"method":"branches","branches":{"branches":[{"name":"main","sha":"abc123","protected":true}],"totalCount":1,"has_more":false}}`},
		{method: "issue_fields", args: map[string]any{"method": "issue_fields", "owner": "owner", "repo": "repo"}, structured: `{"method":"issue_fields","issue_fields":{"fields":[{"id":"FIELD_1","name":"Priority","data_type":"text","description":"Issue priority"}],"totalCount":1}}`},
		{method: "reviewers", args: map[string]any{"method": "reviewers", "owner": "owner", "repo": "repo"}, structured: `{"method":"reviewers","reviewers":{"users":[{"login":"octocat","avatar_url":"https://example.com/avatar.png"}],"teams":[{"slug":"docs","name":"Docs","org":"owner"}],"totalCount":2,"has_more":false}}`},
	}
	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "ui_get", Arguments: tc.args})
			require.NoError(t, err)
			require.False(t, result.IsError, mustMarshalJSON(t, result))
			schema := schemas["ui_get"]
			require.NotNil(t, result.StructuredContent)
			var output any
			require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
			require.NoError(t, schema.Validate(output))
			assert.JSONEq(t, tc.structured, mustMarshalJSON(t, output))
			assert.Equal(t, mustMarshalJSON(t, result.StructuredContent), getTextResult(t, result).Text)
		})
	}
}

func TestTypedCopilotAssignmentCanonicalURLs(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25"} {
		t.Run(protocol, func(t *testing.T) {
			session, schemas := typedCopilotUISession(t, typedCopilotUIClient(t, "linked-enterprise"), protocol, PollConfig{MaxAttempts: 1})
			for _, tc := range []struct {
				name       string
				suggestion bool
			}{
				{name: "assign_copilot_to_issue"},
				{name: "assign_copilot_to_issue_with_intent"},
				{name: "assign_copilot_to_issue_with_intent", suggestion: true},
			} {
				t.Run(tc.name+"/"+mustMarshalJSON(t, tc.suggestion), func(t *testing.T) {
					args := map[string]any{"owner": "owner", "repo": "repo", "issue_number": 7}
					if tc.name == "assign_copilot_to_issue_with_intent" {
						args["is_suggestion"] = tc.suggestion
						args["rationale"] = "Clear acceptance criteria"
						args["confidence"] = "HIGH"
					}
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
					require.NoError(t, err)
					require.False(t, result.IsError, mustMarshalJSON(t, result))
					var output map[string]any
					require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
					assert.Equal(t, "https://github.enterprise.example/owner/repo/issues/7", output["issue_url"])
					if tc.suggestion {
						assert.NotContains(t, output, "pull_request")
					} else {
						pr, ok := output["pull_request"].(map[string]any)
						require.True(t, ok, "%v", output)
						assert.Equal(t, "https://github.enterprise.example/owner/repo/pull/8", pr["url"])
					}
					if schema := schemas[tc.name]; schema != nil {
						require.NoError(t, schema.Validate(output))
						assert.JSONEq(t, mustMarshalJSON(t, output), mustMarshalJSON(t, result.StructuredContent))
					} else {
						assert.Nil(t, result.StructuredContent)
					}
				})
			}
		})
	}
}

func TestTypedUIGetNullIssueTypesWireOutput(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25"} {
		t.Run(protocol, func(t *testing.T) {
			session, schemas := typedCopilotUISession(t, typedCopilotUIClient(t, "issue-types-null"), protocol)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "ui_get",
				Arguments: map[string]any{"method": "issue_types", "owner": "owner"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError)
			if protocol == inventory.ProtocolVersionMultiRoundTrip {
				require.NotNil(t, result.StructuredContent)
				var output any
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
				require.NoError(t, schemas["ui_get"].Validate(output))
				assert.JSONEq(t, `{"method":"issue_types","issue_types":null}`, mustMarshalJSON(t, output))
				assert.Equal(t, mustMarshalJSON(t, result.StructuredContent), getTextResult(t, result).Text)
			} else {
				assert.Equal(t, "null", getTextResult(t, result).Text)
				assert.Nil(t, result.StructuredContent)
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
		{name: "ui_get", args: map[string]any{"method": "unknown", "owner": "owner", "repo": true}, text: "unknown method: unknown"},
		{name: "ui_get", args: map[string]any{"method": "unknown"}, text: "missing required parameter: owner"},
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

func TestTypedUIGetHTTPScopesAndDispatch(t *testing.T) {
	registry, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{
		UIGet(translations.NullTranslationHelper),
	}).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	scopes.SetToolScopeMapFromInventory(registry)
	t.Cleanup(func() { scopes.SetGlobalToolScopeMap(nil) })

	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, _ := typedCopilotUISession(t, typedCopilotUIClient(t), protocol)
			for _, tc := range []struct {
				name       string
				args       map[string]any
				scopes     []string
				wantStatus int
				wantError  string
			}{
				{name: "unknown method does not challenge", args: map[string]any{"method": "unknown", "owner": "owner", "repo": "repo"}, scopes: []string{"gist"}, wantStatus: http.StatusOK, wantError: "unknown method: unknown"},
				{name: "unknown method validates owner first", args: map[string]any{"method": "unknown", "repo": "repo"}, scopes: []string{"gist"}, wantStatus: http.StatusOK, wantError: "missing required parameter: owner"},
				{name: "labels cannot be overridden to issue types", args: map[string]any{"method": "labels", "owner": "owner", "repo": "repo", "_compat_method": "issue_types"}, scopes: []string{"repo"}, wantStatus: http.StatusOK},
				{name: "labels cannot be overridden to reviewers", args: map[string]any{"method": "labels", "owner": "owner", "repo": "repo", "_compat_method": "reviewers"}, scopes: []string{"repo"}, wantStatus: http.StatusOK},
				{name: "issue types still require read org", args: map[string]any{"method": "issue_types", "owner": "owner", "_compat_method": "labels"}, scopes: []string{"repo"}, wantStatus: http.StatusForbidden},
				{name: "labels still require repo", args: map[string]any{"method": "labels", "owner": "owner", "repo": "repo"}, scopes: []string{"gist"}, wantStatus: http.StatusForbidden},
			} {
				t.Run(tc.name, func(t *testing.T) {
					raw, err := json.Marshal(tc.args)
					require.NoError(t, err)
					called := false
					handler := middleware.WithScopeChallenge(&oauth.Config{}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						called = true
						result, err := session.CallTool(r.Context(), &mcp.CallToolParams{Name: "ui_get", Arguments: tc.args})
						require.NoError(t, err)
						if tc.wantError != "" {
							require.True(t, result.IsError)
							assert.Equal(t, tc.wantError, getTextResult(t, result).Text)
							assert.Nil(t, result.StructuredContent)
						} else {
							require.False(t, result.IsError)
							if protocol == inventory.ProtocolVersionMultiRoundTrip {
								assert.JSONEq(t, `{"method":"labels","labels":{"labels":[],"totalCount":0,"has_more":false}}`, getTextResult(t, result).Text)
							} else {
								assert.Equal(t, `{"has_more":false,"labels":[],"totalCount":0}`, getTextResult(t, result).Text)
							}
						}
						require.NoError(t, json.NewEncoder(w).Encode(result))
					}))
					request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
					ctx := ghcontext.WithTokenInfo(request.Context(), &ghcontext.TokenInfo{TokenType: utils.TokenTypeOAuthAccessToken})
					ctx = ghcontext.WithTokenScopes(ctx, tc.scopes)
					ctx = ghcontext.WithMCPMethodInfo(ctx, &ghcontext.MCPMethodInfo{
						Method: "tools/call", ItemName: "ui_get", RawArguments: raw, ProtocolVersion: protocol,
					})
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, request.WithContext(ctx))
					assert.Equal(t, tc.wantStatus, response.Code)
					assert.Equal(t, tc.wantStatus == http.StatusOK, called)
					if tc.wantStatus == http.StatusOK {
						assert.Empty(t, response.Header().Get("WWW-Authenticate"))
					} else {
						assert.Contains(t, response.Header().Get("WWW-Authenticate"), "insufficient_scope")
					}
				})
			}
		})
	}
}

func TestTypedUIGetUnionOutputs(t *testing.T) {
	cases := []struct {
		method string
		raw    string
		typed  string
	}{
		{method: "labels", raw: `{"labels":[{"id":"L","name":"bug","color":"red","description":""}],"totalCount":1,"has_more":false}`, typed: `{"method":"labels","labels":{"labels":[{"id":"L","name":"bug","color":"red","description":""}],"totalCount":1,"has_more":false}}`},
		{method: "assignees", raw: `{"assignees":[{"login":"octocat","avatar_url":"https://example.com/avatar.png"}],"totalCount":1,"has_more":false}`, typed: `{"method":"assignees","assignees":{"assignees":[{"login":"octocat","avatar_url":"https://example.com/avatar.png"}],"totalCount":1,"has_more":false}}`},
		{method: "milestones", raw: `{"milestones":[{"number":4,"title":"v1","description":"","state":"open","open_issues":2,"due_on":""}],"totalCount":1,"has_more":false}`, typed: `{"method":"milestones","milestones":{"milestones":[{"number":4,"title":"v1","description":"","state":"open","open_issues":2,"due_on":""}],"totalCount":1,"has_more":false}}`},
		{method: "issue_types", raw: `[{"id":1,"name":"Bug"}]`, typed: `{"method":"issue_types","issue_types":[{"id":1,"name":"Bug"}]}`},
		{method: "issue_types", raw: `null`, typed: `{"method":"issue_types","issue_types":null}`},
		{method: "branches", raw: `{"branches":[{"name":"main","sha":"abc","protected":true}],"totalCount":1,"has_more":false}`, typed: `{"method":"branches","branches":{"branches":[{"name":"main","sha":"abc","protected":true}],"totalCount":1,"has_more":false}}`},
		{method: "issue_fields", raw: `{"fields":[{"id":"F","name":"Priority","data_type":"single_select","description":"","options":[]}],"totalCount":1}`, typed: `{"method":"issue_fields","issue_fields":{"fields":[{"id":"F","name":"Priority","data_type":"single_select","description":"","options":[]}],"totalCount":1}}`},
		{method: "reviewers", raw: `{"users":[{"login":"octocat","avatar_url":""}],"teams":[{"slug":"docs","name":"Docs","org":"owner"}],"totalCount":2,"has_more":false}`, typed: `{"method":"reviewers","reviewers":{"users":[{"login":"octocat","avatar_url":""}],"teams":[{"slug":"docs","name":"Docs","org":"owner"}],"totalCount":2,"has_more":false}}`},
	}
	schema, err := uiGetOutputSchema().Resolve(nil)
	require.NoError(t, err)
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			output, err := decodeUIGetOutput(tc.method, []byte(tc.raw))
			require.NoError(t, err)
			encoded, err := json.Marshal(output)
			require.NoError(t, err)
			assert.JSONEq(t, tc.typed, string(encoded))
			var value any
			require.NoError(t, json.Unmarshal(encoded, &value))
			require.NoError(t, schema.Validate(value))
		})
	}
}

func TestToolDefinitionsUseConcreteOutputTypes(t *testing.T) {
	for _, tool := range AllTools(translations.NullTranslationHelper) {
		registerTyped := reflect.ValueOf(tool).FieldByName("registerTyped")
		require.True(t, registerTyped.IsValid(), "tool %q has no output registration metadata", tool.Tool.Name)
		assert.False(t, registerTyped.IsNil(), "tool %q uses an untyped output registration", tool.Tool.Name)
	}
}

func TestTypedCopilotOutputSchemas(t *testing.T) {
	cases := []struct {
		name   string
		schema *jsonschema.Schema
		raw    string
	}{
		{
			name:   "assign_copilot_to_issue",
			schema: assignCopilotToIssueOutputSchema(),
			raw:    `{"issue_number":7,"issue_url":"https://github.enterprise.example/owner/repo/issues/7","message":"successfully assigned copilot to issue - pull request pending","note":"pending","owner":"owner","pull_request":{"number":8,"state":"OPEN","title":"Fix","url":"https://github.enterprise.example/owner/repo/pull/8"},"repo":"repo"}`,
		},
		{
			name:   "assign_copilot_to_issue_with_intent",
			schema: assignCopilotToIssueWithIntentOutputSchema(),
			raw:    `{"is_suggestion":true,"issue_number":7,"issue_url":"https://github.enterprise.example/owner/repo/issues/7","message":"suggested","owner":"owner","repo":"repo"}`,
		},
		{
			name:   "request_copilot_review",
			schema: copilotReviewOutputSchema(),
			raw:    `{"status":"requested"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, "object", tc.schema.Type)
			require.NotEmpty(t, tc.schema.Properties)
			assert.Nil(t, tc.schema.AnyOf)
			assert.Nil(t, tc.schema.OneOf)
			resolved, err := tc.schema.Resolve(nil)
			require.NoError(t, err)
			var value any
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &value))
			require.NoError(t, resolved.Validate(value))
		})
	}
	assert.Contains(t, mustMarshalJSON(t, assignCopilotToIssueOutputSchema()), `"url"`)
	assert.Contains(t, mustMarshalJSON(t, assignCopilotToIssueWithIntentOutputSchema()), `"url"`)
	reviewSchema, err := copilotReviewOutputSchema().Resolve(nil)
	require.NoError(t, err)
	require.NoError(t, reviewSchema.Validate(map[string]any{"status": "requested"}))
	require.NoError(t, reviewSchema.Validate(map[string]any{"status": ""}))
	assert.Error(t, reviewSchema.Validate(map[string]any{}))
}

func TestTypedUIGetOutputSchemaDiscriminator(t *testing.T) {
	schema := uiGetOutputSchema()
	require.Equal(t, "object", schema.Type)
	require.Equal(t, []string{"method"}, schema.Required)
	assert.Nil(t, schema.AnyOf)
	assert.Len(t, schema.OneOf, len(schema.Properties["method"].Enum))
	require.Contains(t, schema.Properties, "method")
	assert.Equal(t, []any{"labels", "assignees", "milestones", "issue_types", "branches", "issue_fields", "reviewers"}, schema.Properties["method"].Enum)
	for _, property := range []string{"labels", "assignees", "milestones", "issue_types", "branches", "issue_fields", "reviewers"} {
		require.Contains(t, schema.Properties, property)
		assert.NotEqual(t, `{}`, mustMarshalJSON(t, schema.Properties[property]), property)
	}
	assert.NotContains(t, mustMarshalJSON(t, schema), `"url"`)
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	for _, raw := range []string{
		`{"method":"labels","labels":{"labels":[],"totalCount":0,"has_more":false}}`,
		`{"method":"issue_types","issue_types":[]}`,
	} {
		var value any
		require.NoError(t, json.Unmarshal([]byte(raw), &value))
		require.NoError(t, resolved.Validate(value))
	}
	for _, raw := range []string{`{}`, `{"method":"unsupported"}`, `{"method":"labels","labels":true}`} {
		var value any
		require.NoError(t, json.Unmarshal([]byte(raw), &value))
		assert.Error(t, resolved.Validate(value), raw)
	}
}
