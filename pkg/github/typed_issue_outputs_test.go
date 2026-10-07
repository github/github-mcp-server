package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typedIssueDeps(t *testing.T) BaseDeps {
	t.Helper()
	relatedIssue := &github.Issue{
		ID: new(int64(7)), Number: new(7), Title: new("<script>alert(1)</script>Blocker"),
		State: new("open"), HTMLURL: new("https://github.com/owner/repo/issues/7"),
		RepositoryURL: new("https://api.github.com/repos/owner/repo"),
	}
	subject := &github.Issue{
		Number: new(123), Title: new("Subject"), State: new("open"),
		HTMLURL: new("https://github.com/owner/repo/issues/123"),
	}
	types := []*github.IssueType{{ID: new(int64(1)), Name: new("Bug"), IsEnabled: new(false)}}
	comment := &github.IssueComment{
		ID: new(int64(42)), HTMLURL: new("https://github.com/owner/repo/issues/123#issuecomment-42"),
		IssueURL: new("https://api.github.com/repos/owner/repo/issues/123"),
	}
	handlers := map[string]http.HandlerFunc{
		"GET /orgs/{owner}/issue-types":                                     mockResponse(t, http.StatusOK, types),
		"GET /repos/{owner}/{repo}/issue-types":                             mockResponse(t, http.StatusOK, types),
		string(endpointGetIssue):                                            mockResponse(t, http.StatusOK, relatedIssue),
		string(endpointAddBlock):                                            mockResponse(t, http.StatusCreated, subject),
		string(endpointRemoveBlk):                                           mockResponse(t, http.StatusOK, subject),
		string(endpointBlockedBy):                                           mockResponse(t, http.StatusOK, []*github.Issue{relatedIssue}),
		string(endpointBlocking):                                            mockResponse(t, http.StatusOK, []*github.Issue{}),
		"POST /repos/{owner}/{repo}/issues/{issue_number}/comments":         mockResponse(t, http.StatusCreated, comment),
		"PATCH /repos/{owner}/{repo}/issues/comments/{comment_id}":          mockResponse(t, http.StatusOK, comment),
		"GET /repos/{owner}/{repo}/issues/comments/{comment_id}":            mockResponse(t, http.StatusOK, comment),
		GetReposByOwnerByRepo:                                               mockResponse(t, http.StatusOK, map[string]any{"name": "repo", "private": false}),
		"POST /repos/{owner}/{repo}/issues/{issue_number}/reactions":        mockResponse(t, http.StatusCreated, &github.Reaction{ID: new(int64(9))}),
		"POST /repos/{owner}/{repo}/issues/comments/{comment_id}/reactions": mockResponse(t, http.StatusCreated, &github.Reaction{ID: new(int64(9))}),
		"POST /repos/{owner}/{repo}/issues/{issue_number}/sub_issues": mockResponse(t, http.StatusCreated, &github.SubIssue{
			Number: new(23), Title: new("Child"), State: new("open"),
			HTMLURL: new("https://github.com/owner/repo/issues/23"),
		}),
		"DELETE /repos/{owner}/{repo}/issues/{issue_number}/sub_issue": mockResponse(t, http.StatusOK, &github.SubIssue{
			Number: new(23), Title: new("Child"), State: new("open"),
			HTMLURL: new("https://github.com/owner/repo/issues/23"),
		}),
		"PATCH /repos/{owner}/{repo}/issues/{issue_number}/sub_issues/priority": mockResponse(t, http.StatusOK, &github.SubIssue{
			Number: new(23), Title: new("Child"), State: new("open"),
			HTMLURL: new("https://github.com/owner/repo/issues/23"),
		}),
		"GET /repos/{owner}/{repo}/issues/{issue_number}/semantically_similar": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("threshold") {
				assert.Equal(t, "page=0&per_page=0&threshold=0", r.URL.RawQuery)
			} else {
				assert.Empty(t, r.URL.RawQuery, "omitted threshold and pagination must stay omitted")
			}
			_, _ = w.Write([]byte(`[{"issue":{"number":7,"title":"Candidate<script>alert(1)</script>","state":"open","html_url":"https://github.com/owner/repo/issues/7"},"score":null,"confidence":"high","likely_duplicate":true}]`))
		},
	}
	gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		container := "repository"
		if strings.Contains(request.Query, "organization(") {
			container = "organization"
		}
		_, _ = w.Write([]byte(`{"data":{"` + container + `":{"issueFields":{"nodes":[{"__typename":"IssueFieldSingleSelect","id":"IF_1","fullDatabaseId":"99","name":"Priority","description":"Importance","dataType":"SINGLE_SELECT","visibility":"ALL","options":[{"id":"OPT_1","name":"High","description":"","color":"red","priority":0}]}]}}}}`))
	})}}
	return BaseDeps{
		Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(handlers)),
		GQLClient: githubv4.NewClient(gql),
	}
}

func TestTypedIssueOutputs(t *testing.T) {
	translate := translations.NullTranslationHelper
	tools := []inventory.ServerTool{
		IssueRead(translate), ListIssueTypes(translate), ListIssueFields(translate), AddIssueComment(translate),
		UpdateIssueComment(translate), IssueDependencyRead(translate), IssueDependencyWrite(translate), FindDuplicate(translate),
	}
	// These tools remain feature-gated in production. Enable their existing
	// feature rules in the contract harness rather than clearing the rules.
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).
		WithFeatureChecker(func(context.Context, string) (bool, error) { return true, nil }).Build()
	require.NoError(t, err)

	const commentText = `{"id":"42","url":"https://github.com/owner/repo/issues/123#issuecomment-42"}`
	const reactionText = `{"id":"9","url":"https://api.github.com/repos/owner/repo/issues/123/reactions/9"}`
	const blockedText = `{"issues":[{"number":7,"title":"Blocker","state":"OPEN","url":"https://github.com/owner/repo/issues/7","repository":"owner/repo"}],"pageInfo":{"hasNextPage":false,"nextPage":0}}`
	const dependencyRefs = `"blocked_issue":{"number":123,"title":"Subject","state":"OPEN","url":"https://github.com/owner/repo/issues/123","repository":"owner/repo"},"blocking_issue":{"number":7,"title":"Blocker","state":"OPEN","url":"https://github.com/owner/repo/issues/7","repository":"owner/repo"}`
	calls := []struct {
		name       string
		args       map[string]any
		text       string
		structured string
	}{
		{name: "issue_read", args: map[string]any{"method": "get", "owner": "owner", "repo": "repo", "issue_number": "7.0", "page": "0", "perPage": "0"}, text: `{"number":7,"title":"Blocker","state":"open","html_url":"https://github.com/owner/repo/issues/7","assignees":[]}`, structured: `{"method":"get","issue":{"number":7,"title":"Blocker","state":"open","html_url":"https://github.com/owner/repo/issues/7","assignees":[]}}`},
		{name: "list_issue_types", args: map[string]any{"owner": "owner"}, text: `[{"id":1,"name":"Bug","is_enabled":false}]`, structured: `[{"id":1,"name":"Bug","is_enabled":false}]`},
		{name: "list_issue_types", args: map[string]any{"owner": "owner", "repo": "repo"}, text: `[{"id":1,"name":"Bug","is_enabled":false}]`, structured: `[{"id":1,"name":"Bug","is_enabled":false}]`},
		{name: "list_issue_fields", args: map[string]any{"owner": "owner"}, text: `[{"id":"IF_1","full_database_id":99,"name":"Priority","description":"Importance","data_type":"SINGLE_SELECT","visibility":"ALL","options":[{"id":"OPT_1","name":"High","color":"red","priority":0}]}]`, structured: `[{"id":"IF_1","full_database_id":99,"name":"Priority","description":"Importance","data_type":"SINGLE_SELECT","visibility":"ALL","options":[{"id":"OPT_1","name":"High","color":"red","priority":0}]}]`},
		{name: "list_issue_fields", args: map[string]any{"owner": "owner", "repo": "repo"}, text: `[{"id":"IF_1","full_database_id":99,"name":"Priority","description":"Importance","data_type":"SINGLE_SELECT","visibility":"ALL","options":[{"id":"OPT_1","name":"High","color":"red","priority":0}]}]`, structured: `[{"id":"IF_1","full_database_id":99,"name":"Priority","description":"Importance","data_type":"SINGLE_SELECT","visibility":"ALL","options":[{"id":"OPT_1","name":"High","color":"red","priority":0}]}]`},
		{name: "add_issue_comment", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "123.0", "body": "Hello"}, text: commentText, structured: `{"id":"42","html_url":"https://github.com/owner/repo/issues/123#issuecomment-42"}`},
		{name: "add_issue_comment", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "123", "reaction": "heart"}, text: reactionText, structured: `{"id":"9"}`},
		{name: "add_issue_comment", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "123", "body": "Hello", "reaction": "heart"}, text: `{"comment":` + commentText + `,"reaction":` + reactionText + `}`, structured: `{"comment":{"id":"42","html_url":"https://github.com/owner/repo/issues/123#issuecomment-42"},"reaction":{"id":"9"}}`},
		{name: "add_issue_comment", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "123", "comment_id": "42.0", "reaction": "heart"}, text: `{"id":"9","url":"https://api.github.com/repos/owner/repo/issues/comments/42/reactions/9"}`, structured: `{"id":"9"}`},
		{name: "update_issue_comment", args: map[string]any{"owner": "owner", "repo": "repo", "comment_id": "42.0", "body": "Changed"}, text: commentText, structured: `{"id":"42","html_url":"https://github.com/owner/repo/issues/123#issuecomment-42"}`},
		{name: "issue_dependency_read", args: map[string]any{"method": "get_blocked_by", "owner": "owner", "repo": "repo", "issue_number": "123", "page": "0", "perPage": "0"}, text: blockedText, structured: `{"issues":[{"number":7,"title":"Blocker","state":"OPEN","html_url":"https://github.com/owner/repo/issues/7","repository":"owner/repo"}],"pageInfo":{"hasNextPage":false,"nextPage":0}}`},
		{name: "issue_dependency_read", args: map[string]any{"method": "get_blocking", "owner": "owner", "repo": "repo", "issue_number": "123"}, text: `{"issues":[],"pageInfo":{"hasNextPage":false,"nextPage":0}}`, structured: `{"issues":[],"pageInfo":{"hasNextPage":false,"nextPage":0}}`},
		{name: "issue_dependency_write", args: map[string]any{"method": "ADD", "type": "BLOCKED_BY", "owner": "owner", "repo": "repo", "issue_number": "123", "related_issue_number": "7"}, text: `{` + dependencyRefs + `,"message":"dependency added"}`, structured: `{"blocked_issue":{"number":123,"title":"Subject","state":"OPEN","html_url":"https://github.com/owner/repo/issues/123","repository":"owner/repo"},"blocking_issue":{"number":7,"title":"Blocker","state":"OPEN","html_url":"https://github.com/owner/repo/issues/7","repository":"owner/repo"},"message":"dependency added"}`},
		{name: "issue_dependency_write", args: map[string]any{"method": "REMOVE", "type": "BLOCKED_BY", "owner": "owner", "repo": "repo", "issue_number": "123", "related_issue_number": "7"}, text: `{` + dependencyRefs + `,"message":"dependency removed"}`, structured: `{"blocked_issue":{"number":123,"title":"Subject","state":"OPEN","html_url":"https://github.com/owner/repo/issues/123","repository":"owner/repo"},"blocking_issue":{"number":7,"title":"Blocker","state":"OPEN","html_url":"https://github.com/owner/repo/issues/7","repository":"owner/repo"},"message":"dependency removed"}`},
		{name: "find_duplicate", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "123", "confidence_threshold": 0, "page": "0", "perPage": "0"}, text: `[{"issue":{"number":7,"title":"Candidate","state":"open","url":"https://github.com/owner/repo/issues/7"},"score":null,"confidence":"high","likely_duplicate":true}]`, structured: `[{"issue":{"number":7,"title":"Candidate","state":"open","html_url":"https://github.com/owner/repo/issues/7"},"score":null,"confidence":"high","likely_duplicate":true}]`},
		{name: "find_duplicate", args: map[string]any{"owner": "owner", "repo": "repo", "issue_number": "123"}, text: `[{"issue":{"number":7,"title":"Candidate","state":"open","url":"https://github.com/owner/repo/issues/7"},"score":null,"confidence":"high","likely_duplicate":true}]`, structured: `[{"issue":{"number":7,"title":"Candidate","state":"open","html_url":"https://github.com/owner/repo/issues/7"},"score":null,"confidence":"high","likely_duplicate":true}]`},
	}
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip, ""} {
		name := protocol
		if name == "" {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			deps := typedIssueDeps(t)
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-issue-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			inv.RegisterTools(context.Background(), server, deps)
			if protocol == "" {
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
				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
				resolved, err := schema.Resolve(nil)
				require.NoError(t, err)
				schemas[tool.Name] = resolved
				if tool.Name == "add_issue_comment" {
					for _, invalid := range []string{
						`{}`,
						`{"id":"1","html_url":"url","reaction":{"id":"2"}}`,
						`{"comment":{"id":"1","html_url":"url"}}`,
						`{"comment":{"id":"1","html_url":"url"},"reaction":{"id":"2","html_url":"url"}}`,
					} {
						var output any
						require.NoError(t, json.Unmarshal([]byte(invalid), &output))
						require.Error(t, resolved.Validate(output), "partial comment unions must be rejected")
					}
				}
			}

			for _, call := range calls {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
				require.NoError(t, err, call.name)
				require.False(t, result.IsError, "%s: %s", call.name, result)
				require.Len(t, result.Content, 1, call.name)
				if protocol != inventory.ProtocolVersionMultiRoundTrip {
					assert.Equal(t, call.text, getTextResult(t, result).Text, call.name)
					assert.Nil(t, result.StructuredContent, call.name)
					continue
				}
				require.NotNil(t, result.StructuredContent, call.name)
				structured := mustMarshalJSON(t, result.StructuredContent)
				assert.JSONEq(t, call.structured, structured)
				if call.name == "issue_read" || call.name == "add_issue_comment" {
					assert.JSONEq(t, structured, getTextResult(t, result).Text, call.name)
				}
				var output any
				require.NoError(t, json.Unmarshal([]byte(structured), &output))
				require.NoError(t, schemas[call.name].Validate(output), call.name)
			}
			errors := []struct {
				name string
				args map[string]any
				text string
			}{
				{"list_issue_types", map[string]any{}, "missing required parameter: owner"},
				{"list_issue_fields", map[string]any{"owner": "owner", "repo": nil}, "parameter repo is not of type string"},
				{"add_issue_comment", map[string]any{"owner": "owner", "repo": "repo", "issue_number": 123}, "at least one of body or reaction is required"},
				{"add_issue_comment", map[string]any{"owner": "owner", "repo": "repo", "issue_number": 123, "body": "Hello", "comment_id": 42}, "comment_id cannot be combined with body"},
				{"update_issue_comment", map[string]any{"owner": "owner", "repo": "repo", "comment_id": "1.5", "body": "Hello"}, "non-integer numeric value"},
				{"issue_dependency_read", map[string]any{"method": "GET_BLOCKING", "owner": "owner", "repo": "repo", "issue_number": 123}, "get_blocking"},
				{"issue_dependency_write", map[string]any{"method": "add", "type": "blocked_by", "owner": "owner", "repo": "repo", "issue_number": 123, "related_issue_number": 123}, "an issue cannot block or depend on itself"},
				{"find_duplicate", map[string]any{"owner": "owner", "repo": "repo", "issue_number": "1.5"}, "not a valid number"},
				{"find_duplicate", map[string]any{"owner": "owner", "repo": "repo", "issue_number": 123, "confidence_threshold": nil}, "parameter confidence_threshold is not of type float64"},
			}
			for _, call := range errors {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
				require.NoError(t, err, call.name)
				require.True(t, result.IsError, call.name)
				assert.Contains(t, getErrorResult(t, result).Text, call.text)
				assert.Nil(t, result.StructuredContent, call.name)
			}
		})
	}
}

func TestTypedIssueNullOutput(t *testing.T) {
	tool := ListIssueTypes(translations.NullTranslationHelper)
	inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{tool}).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		"GET /orgs/{owner}/issue-types": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("null"))
		},
	}))}

	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-issue-null", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			inv.RegisterTools(context.Background(), server, deps)
			session := connectCommentVisibilityClient(t, server, protocol)
			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, 1)
			if protocol == inventory.ProtocolVersionMultiRoundTrip {
				require.NotNil(t, list.Tools[0].OutputSchema)
			} else {
				assert.Nil(t, list.Tools[0].OutputSchema)
			}

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "list_issue_types", Arguments: map[string]any{"owner": "owner"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.Equal(t, "null", getTextResult(t, result).Text)
			if protocol == inventory.ProtocolVersionMultiRoundTrip {
				assert.Nil(t, result.StructuredContent, "the SDK omits structuredContent when the typed output is null")
			} else {
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestIssuePaginationValidationSchemaKeepsAdvertisedSchema(t *testing.T) {
	advertised := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"page":    {Type: "number", Description: "Page number (min 1)", Minimum: new(1.0), Default: json.RawMessage("1")},
			"perPage": {Type: "number", Description: "Items per page (min 1, max 100)", Minimum: new(1.0), Maximum: new(100.0), Default: json.RawMessage("30")},
		},
	}
	advertisedBefore := mustMarshalJSON(t, advertised)

	validation := issuePaginationValidationSchema(advertised)
	require.Equal(t, advertisedBefore, mustMarshalJSON(t, advertised), "runtime validation must not mutate the advertised schema")
	assert.Equal(t, "Page number (min 1)", validation.Properties["page"].Description)
	assert.Equal(t, "Items per page (min 1, max 100)", validation.Properties["perPage"].Description)
	assert.Equal(t, 1.0, *advertised.Properties["page"].Minimum)
	assert.Equal(t, 1.0, *advertised.Properties["perPage"].Minimum)
	assert.Equal(t, 0.0, *validation.Properties["page"].Minimum)
	assert.Equal(t, 0.0, *validation.Properties["perPage"].Minimum)
	assert.Equal(t, 100.0, *validation.Properties["perPage"].Maximum)
	assert.Nil(t, validation.Properties["page"].Default, "validation must not inject defaults absent from the original call")
	assert.Nil(t, validation.Properties["perPage"].Default, "validation must not inject defaults absent from the original call")
	resolved, err := validation.Resolve(nil)
	require.NoError(t, err)
	require.NoError(t, resolved.Validate(map[string]any{"page": float64(0), "perPage": float64(0)}))
	assert.Error(t, resolved.Validate(map[string]any{"page": float64(-1)}))
}

func TestTypedIssueAPIErrors(t *testing.T) {
	translate := translations.NullTranslationHelper
	cases := []struct {
		tool inventory.ServerTool
		args map[string]any
		text string
	}{
		{ListIssueTypes(translate), map[string]any{"owner": "owner"}, "failed to list issue types"},
		{ListIssueFields(translate), map[string]any{"owner": "owner"}, "failed to list issue fields"},
		{AddIssueComment(translate), map[string]any{"owner": "owner", "repo": "repo", "issue_number": 123, "body": "Hello"}, "failed to create comment"},
		{UpdateIssueComment(translate), map[string]any{"owner": "owner", "repo": "repo", "comment_id": 42, "body": "Hello"}, "failed to update issue comment"},
		{IssueDependencyRead(translate), map[string]any{"method": "get_blocked_by", "owner": "owner", "repo": "repo", "issue_number": 123}, "failed to list blocked-by issues"},
		{IssueDependencyWrite(translate), map[string]any{"method": "add", "type": "blocked_by", "owner": "owner", "repo": "repo", "issue_number": 123, "related_issue_number": 7}, "failed to resolve blocking issue"},
		{FindDuplicate(translate), map[string]any{"owner": "owner", "repo": "repo", "issue_number": 123}, "failed to find duplicate issues"},
	}
	for _, tc := range cases {
		t.Run(tc.tool.Tool.Name, func(t *testing.T) {
			client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			})}}
			deps := BaseDeps{Client: mustNewGHClient(t, client), GQLClient: githubv4.NewClient(client)}
			inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{tc.tool}).WithToolsets([]string{"all"}).
				WithFeatureChecker(func(context.Context, string) (bool, error) { return true, nil }).Build()
			require.NoError(t, err)
			var legacyText string
			for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
				server := mcp.NewServer(&mcp.Implementation{Name: "typed-issue-errors", Version: "v1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				inv.RegisterTools(context.Background(), server, deps)
				session := connectCommentVisibilityClient(t, server, protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Len(t, result.Content, 1)
				text := getErrorResult(t, result).Text
				assert.Contains(t, text, tc.text)
				assert.Nil(t, result.StructuredContent)
				if protocol == "2025-11-25" {
					legacyText = text
				} else {
					assert.Equal(t, legacyText, text, "API errors must be byte-exact across protocols")
				}
			}
		})
	}
}

func TestTypedSubIssueWriteProtocols(t *testing.T) {
	translate := translations.NullTranslationHelper
	tool := SubIssueWrite(translate)
	inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{tool}).WithToolsets([]string{"all"}).
		WithFeatureChecker(func(context.Context, string) (bool, error) { return false, nil }).Build()
	require.NoError(t, err)

	expectedIssue := &github.SubIssue{
		Number: new(23), Title: new("Child"), State: new("open"),
		HTMLURL: new("https://github.com/owner/repo/issues/23"),
	}
	legacy, err := json.Marshal(expectedIssue)
	require.NoError(t, err)
	subIssue, err := subIssueOutput(expectedIssue)
	require.NoError(t, err)
	structured, err := json.Marshal(SubIssueWriteOutput{Method: "add", Issue: subIssue})
	require.NoError(t, err)

	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			deps := typedIssueDeps(t)
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-sub-issue-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			inv.RegisterTools(context.Background(), server, deps)
			session := connectCommentVisibilityClient(t, server, protocol)

			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, 1)
			if protocol == inventory.ProtocolVersionMultiRoundTrip {
				require.NotNil(t, list.Tools[0].OutputSchema)
				var outputSchema jsonschema.Schema
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, list.Tools[0].OutputSchema)), &outputSchema))
				resolvedSchema, err := outputSchema.Resolve(nil)
				require.NoError(t, err)
				var output any
				require.NoError(t, json.Unmarshal(structured, &output))
				require.NoError(t, resolvedSchema.Validate(output))
			} else {
				assert.Nil(t, list.Tools[0].OutputSchema)
			}

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "sub_issue_write",
				Arguments: map[string]any{
					"method": "ADD", "owner": "owner", "repo": "repo",
					"issue_number": "123.0", "sub_issue_id": "23.0", "replace_parent": true,
				},
			})
			require.NoError(t, err)
			require.False(t, result.IsError)
			if protocol != inventory.ProtocolVersionMultiRoundTrip {
				assert.Equal(t, string(legacy), getTextResult(t, result).Text)
				assert.Nil(t, result.StructuredContent)
				return
			}
			require.NotNil(t, result.StructuredContent)
			assert.JSONEq(t, string(structured), mustMarshalJSON(t, result.StructuredContent))
			assert.JSONEq(t, string(structured), getTextResult(t, result).Text)

			invalid, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "sub_issue_write",
				Arguments: map[string]any{
					"method": "add", "owner": "owner", "repo": "repo",
					"issue_number": 123, "sub_issue_id": 23, "replace_parent": nil,
				},
			})
			require.NoError(t, err)
			require.True(t, invalid.IsError)
			assert.Contains(t, getErrorResult(t, invalid).Text, "parameter replace_parent is not of type bool")
			assert.Nil(t, invalid.StructuredContent)
		})
	}
}
