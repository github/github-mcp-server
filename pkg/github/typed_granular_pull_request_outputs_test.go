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

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	gogithub "github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func granularPullRequestTools() []inventory.ServerTool {
	t := translations.NullTranslationHelper
	return []inventory.ServerTool{
		GranularUpdatePullRequestTitle(t), GranularUpdatePullRequestBody(t),
		GranularUpdatePullRequestState(t), GranularUpdatePullRequestDraftState(t),
		GranularRequestPullRequestReviewers(t), GranularCreatePullRequestReview(t),
		GranularSubmitPendingPullRequestReview(t), GranularDeletePendingPullRequestReview(t),
		GranularAddPullRequestReviewComment(t), GranularResolveReviewThread(t),
		GranularUnresolveReviewThread(t), GranularAddPullRequestReviewCommentReaction(t),
		GranularRemovePullRequestReviewCommentReaction(t),
		GranularResolveReviewThreadWithResolutionReason(t),
	}
}

type granularPRCase struct {
	name    string
	args    map[string]any
	text    string
	payload string
}

const granularPRReference = `{"id":"123","url":"https://github.com/owner/repo/pull/1"}`

func granularPRCases() []granularPRCase {
	return []granularPRCase{
		{"update_pull_request_title", map[string]any{"title": "Changed"}, granularPRReference, `{"title":"Changed"}`},
		{"update_pull_request_body", map[string]any{"body": "Body"}, granularPRReference, `{"body":"Body"}`},
		{"update_pull_request_state", map[string]any{"state": "closed"}, granularPRReference, `{"state":"closed"}`},
		{"update_pull_request_state", map[string]any{"state": "other"}, granularPRReference, `{"state":"other"}`},
		{"update_pull_request_draft_state", map[string]any{"draft": true}, "pull request converted to draft", `"pullRequestId":"PR_1"`},
		{"update_pull_request_draft_state", map[string]any{"draft": false}, "pull request marked as ready for review", `"pullRequestId":"PR_1"`},
		{"request_pull_request_reviewers", map[string]any{"reviewers": []string{"octocat", "org/team", "/bad", "org/a/b"}}, granularPRReference, `{"reviewers":["octocat","/bad","org/a/b"],"team_reviewers":["team"]}`},
		{"create_pull_request_review", map[string]any{}, "pending pull request created", `"pullRequestId":"PR_1"`},
		{"create_pull_request_review", map[string]any{"event": "APPROVE", "body": "LGTM", "commitID": "abc"}, "pull request review submitted successfully", `"event":"APPROVE"`},
		{"create_pull_request_review", map[string]any{"event": "COMMENT", "body": "Note"}, "pull request review submitted successfully", `"body":"Note"`},
		{"create_pull_request_review", map[string]any{"event": "REQUEST_CHANGES"}, "pull request review submitted successfully", `"event":"REQUEST_CHANGES"`},
		{"create_pull_request_review", map[string]any{"event": 3, "body": false, "commitID": []any{}}, "pending pull request created", `"pullRequestId":"PR_1"`},
		{"create_pull_request_review", map[string]any{"event": "", "body": "ignored"}, "pending pull request created", `"pullRequestId":"PR_1"`},
		{"submit_pending_pull_request_review", map[string]any{"event": "APPROVE", "body": "LGTM"}, "pending pull request review successfully submitted", `"pullRequestReviewId":"R_1"`},
		{"submit_pending_pull_request_review", map[string]any{"event": "COMMENT", "body": false}, "pending pull request review successfully submitted", `"body":""`},
		{"submit_pending_pull_request_review", map[string]any{"event": "REQUEST_CHANGES"}, "pending pull request review successfully submitted", `"event":"REQUEST_CHANGES"`},
		{"delete_pending_pull_request_review", map[string]any{}, "pending pull request review successfully deleted", `"pullRequestReviewId":"R_1"`},
		{"add_pull_request_review_comment", map[string]any{"path": "file", "body": "nit", "subjectType": "LINE", "line": "3e0", "side": "RIGHT", "startLine": "2.0", "startSide": "LEFT"}, "pull request review comment successfully added to pending review", `"line":3`},
		{"add_pull_request_review_comment", map[string]any{"path": "file", "body": "nit", "subjectType": "FILE", "line": 0, "startLine": 0, "side": 3, "startSide": false}, "pull request review comment successfully added to pending review", `"subjectType":"FILE"`},
		{"resolve_review_thread", map[string]any{"threadID": "T_1"}, "review thread resolved successfully", `"threadId":"T_1"`},
		{"resolve_review_thread", map[string]any{"threadID": "T_1", "resolutionReason": ""}, "review thread resolved successfully", `"threadId":"T_1"`},
		{"unresolve_review_thread", map[string]any{"threadID": "T_1"}, "review thread unresolved successfully", `"threadId":"T_1"`},
		{"add_pull_request_review_comment_reaction", map[string]any{"comment_id": "42.0", "content": "heart"}, `{"id":"77","url":"https://api.github.com/repos/owner/repo/pulls/comments/42/reactions/77"}`, `{"content":"heart"}`},
		{"remove_pull_request_review_comment_reaction", map[string]any{"comment_id": "42", "reaction_id": "77e0"}, "reaction successfully removed from pull request review comment", ""},
	}
}

// These expectations were run against the untyped handlers at the exact
// parent commit before migrating them. Keep equality, not substring checks.
func granularPRValidationCases() []granularPRCase {
	return []granularPRCase{
		{"update_pull_request_title", map[string]any{"title": ""}, "missing required parameter: title", ""},
		{"update_pull_request_body", map[string]any{"body": nil}, "parameter body is not of type string", ""},
		{"update_pull_request_state", map[string]any{}, "missing required parameter: state", ""},
		{"update_pull_request_draft_state", map[string]any{}, "missing required parameter: draft", ""},
		{"update_pull_request_draft_state", map[string]any{"draft": nil}, "parameter draft is not of type bool, is <nil>", ""},
		{"request_pull_request_reviewers", map[string]any{"reviewers": nil}, "missing required parameter: reviewers", ""},
		{"request_pull_request_reviewers", map[string]any{"reviewers": []any{"octocat", false}}, "parameter reviewers is not of type string, is bool", ""},
		{"create_pull_request_review", map[string]any{"pullNumber": 1.5}, "parameter pullNumber is not a valid number: non-integer numeric value: 1.5", ""},
		{"submit_pending_pull_request_review", map[string]any{"event": ""}, "missing required parameter: event", ""},
		{"submit_pending_pull_request_review", map[string]any{"event": false}, "parameter event is not of type string", ""},
		{"delete_pending_pull_request_review", map[string]any{"owner": ""}, "missing required parameter: owner", ""},
		{"add_pull_request_review_comment", map[string]any{"path": "file", "body": "", "subjectType": false}, "missing required parameter: body", ""},
		{"add_pull_request_review_comment", map[string]any{"path": "file", "body": "nit", "subjectType": "LINE", "line": false, "startLine": 1.5}, "parameter line is not a valid number: expected number, got bool", ""},
		{"add_pull_request_review_comment", map[string]any{"path": "file", "body": "nit", "subjectType": "LINE", "startLine": nil}, "parameter startLine is not a valid number: expected number, got <nil>", ""},
		{"resolve_review_thread", map[string]any{"threadID": ""}, "missing required parameter: threadID", ""},
		{"unresolve_review_thread", map[string]any{"threadID": 3}, "parameter threadID is not of type string", ""},
		{"add_pull_request_review_comment_reaction", map[string]any{"comment_id": 0, "content": nil}, "missing required parameter: comment_id", ""},
		{"add_pull_request_review_comment_reaction", map[string]any{"comment_id": "42", "content": false}, "parameter content is not of type string", ""},
		{"remove_pull_request_review_comment_reaction", map[string]any{"comment_id": "42", "reaction_id": "bad"}, "parameter reaction_id is not a valid number: invalid numeric value: bad", ""},
	}
}

func granularPRArgs(tc granularPRCase) map[string]any {
	args := map[string]any{"owner": "owner", "repo": "repo", "pullNumber": "1.0"}
	maps.Copy(args, tc.args)
	return args
}

func granularPRDeps(t *testing.T, current **granularPRCase, outcome string) BaseDeps {
	t.Helper()
	rest := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if outcome == "error" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			return
		}
		if current != nil && *current != nil && (*current).payload != "" {
			var payload any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.JSONEq(t, (*current).payload, mustMarshalJSON(t, payload))
		}
		path := r.URL.Path
		switch {
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case outcome == "empty":
			_, _ = w.Write([]byte(`null`))
		case strings.Contains(path, "/reactions"):
			_, _ = w.Write([]byte(`{"id":77,"content":"heart"}`))
		default:
			_, _ = w.Write([]byte(`{"id":123,"html_url":"https://github.com/owner/repo/pull/1"}`))
		}
	})}}
	gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string          `json:"query"`
			Variables json.RawMessage `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if outcome == "error" || outcome == "mutation_error" && strings.HasPrefix(req.Query, "mutation") {
			_, _ = w.Write([]byte(`{"errors":[{"message":"Forbidden"}]}`))
			return
		}
		if strings.HasPrefix(req.Query, "mutation") && current != nil && *current != nil {
			assert.Contains(t, string(req.Variables), (*current).payload)
		}
		var body string
		switch q := req.Query; {
		case strings.Contains(q, "convertPullRequestToDraft("):
			body = `{"data":{"convertPullRequestToDraft":{"pullRequest":{"id":"PR_1","isDraft":true}}}}`
		case strings.Contains(q, "markPullRequestReadyForReview("):
			body = `{"data":{"markPullRequestReadyForReview":{"pullRequest":{"id":"PR_1","isDraft":false}}}}`
		case strings.Contains(q, "addPullRequestReviewThread("):
			body = `{"data":{"addPullRequestReviewThread":{"thread":{"id":"T_2"}}}}`
		case strings.Contains(q, "addPullRequestReview("):
			body = `{"data":{"addPullRequestReview":{"pullRequestReview":{"id":"R_1"}}}}`
		case strings.Contains(q, "submitPullRequestReview("):
			body = `{"data":{"submitPullRequestReview":{"pullRequestReview":{"id":"R_1"}}}}`
		case strings.Contains(q, "deletePullRequestReview("):
			body = `{"data":{"deletePullRequestReview":{"pullRequestReview":{"id":"R_1"}}}}`
		case strings.Contains(q, "unresolveReviewThread("):
			body = `{"data":{"unresolveReviewThread":{"thread":{"id":"T_1","isResolved":false}}}}`
		case strings.Contains(q, "resolveReviewThread("):
			body = `{"data":{"resolveReviewThread":{"thread":{"id":"T_1","isResolved":true}}}}`
		case strings.Contains(q, "reviews(first: 100"):
			body = `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"id":"R_1","author":{"userId":"U_1"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}}`
			if outcome == "no_pending" {
				body = `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}}`
			}
		case strings.Contains(q, "viewer{"):
			body = `{"data":{"viewer":{"id":"U_1"}}}`
		default:
			require.Contains(t, q, "pullRequest(number:")
			body = `{"data":{"repository":{"pullRequest":{"id":"PR_1"}}}}`
		}
		if outcome == "empty" {
			body = `{"data":{}}`
		}
		_, _ = w.Write([]byte(body))
	})}}
	return BaseDeps{
		Client: mustNewGHClient(t, rest), GQLClient: githubv4.NewClient(gql),
		RepoAccessCache: stubRepoAccessCache(nil, time.Minute),
	}
}

func granularPRSession(t *testing.T, deps ToolDependencies, protocol string, reason bool) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	tools := granularPullRequestTools()
	flags := []inventory.FeatureFlag{inventory.FeatureFlag(FeatureFlagPullRequestsGranular)}
	if reason {
		flags = append(flags, FeatureFlagThreadResolutionReason)
	}
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).
		WithFeatureChecker(featureCheckerFor(flags...)).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "granular-pr", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	inv.RegisterTools(context.Background(), server, deps)
	if protocol == "" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				switch req := req.(type) {
				case *mcp.ListToolsRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
				case *mcp.CallToolRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
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
	require.Len(t, list.Tools, 13)
	schemas := make(map[string]*jsonschema.Resolved)
	for _, tool := range list.Tools {
		name := tool.Name
		if name == "resolve_review_thread" && reason {
			name += "_resolution_reason"
		}
		// Resolve the advertised input schema as well as every modern output.
		var inputSchema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.InputSchema)), &inputSchema))
		_, err := inputSchema.Resolve(nil)
		require.NoError(t, err, name)
		if protocol != inventory.ProtocolVersionMultiRoundTrip {
			assert.Nil(t, tool.OutputSchema, name)
			continue
		}
		require.NotNil(t, tool.OutputSchema, name)
		require.NoError(t, toolsnaps.Test(name, *tool))
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err, name)
		schemas[tool.Name] = resolved
		if tool.Name == "add_pull_request_review_comment_reaction" {
			require.NoError(t, resolved.Validate(map[string]any{"id": "77", "url": "https://api.github.com/reaction"}))
			require.Error(t, resolved.Validate(map[string]any{"id": float64(77), "content": "heart"}))
		}
	}
	return session, schemas
}

func TestGranularPullRequestCanonicalSnapshots(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		for _, reason := range []bool{false, true} {
			granularPRSession(t, BaseDeps{}, protocol, reason)
		}
	}
}

func assertGranularPRResult(t *testing.T, result *mcp.CallToolResult, schema *jsonschema.Resolved, text string, isError bool) {
	t.Helper()
	require.Equal(t, isError, result.IsError, mustMarshalJSON(t, result))
	require.Len(t, result.Content, 1)
	wireText := result.Content[0].(*mcp.TextContent).Text
	if schema == nil || isError {
		require.Equal(t, text, wireText)
		require.Nil(t, result.StructuredContent)
		return
	}
	require.NotNil(t, result.StructuredContent)
	var output any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
	require.NoError(t, schema.Validate(output))
	switch {
	case strings.Contains(text, "/reactions/"):
		assert.JSONEq(t, text, mustMarshalJSON(t, output))
		assert.JSONEq(t, mustMarshalJSON(t, output), wireText)
	case strings.HasPrefix(text, "{"):
		assert.JSONEq(t, mustMarshalJSON(t, output), wireText)
		assert.JSONEq(t, text, mustMarshalJSON(t, output))
	default:
		assert.JSONEq(t, mustMarshalJSON(t, RepositoryMessageOutput{Message: text}), mustMarshalJSON(t, output))
		assert.JSONEq(t, mustMarshalJSON(t, output), wireText)
	}
	for _, raw := range []string{`{}`, `[]`, `{"id":false}`, `{"message":1}`} {
		var invalid any
		require.NoError(t, json.Unmarshal([]byte(raw), &invalid))
		require.Error(t, schema.Validate(invalid), raw)
	}
}

func TestTypedGranularPullRequestWireOutputs(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		for _, reason := range []bool{false, true} {
			t.Run("protocol="+protocol+"/reason="+mustMarshalJSON(t, reason), func(t *testing.T) {
				var current *granularPRCase
				deps := granularPRDeps(t, &current, "")
				session, schemas := granularPRSession(t, deps, protocol, reason)
				for _, tc := range granularPRCases() {
					t.Run(tc.name+"/"+tc.text+"/"+mustMarshalJSON(t, tc.args), func(t *testing.T) {
						current = &tc
						result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: granularPRArgs(tc)})
						require.NoError(t, err)
						assertGranularPRResult(t, result, schemas[tc.name], tc.text, false)
					})
				}
				for _, value := range []any{"addressed", "wont-fix", "invalid", "", nil, false} {
					tc := granularPRCase{name: "resolve_review_thread", args: map[string]any{"threadID": "T_1", "resolutionReason": value}, text: "review thread resolved successfully"}
					current = &tc
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
					require.NoError(t, err)
					if reason && (value == nil || value == false) {
						typeName := "<nil>"
						if value == false {
							typeName = "bool"
						}
						assertGranularPRResult(t, result, schemas[tc.name], "parameter resolutionReason is not of type string, is "+typeName, true)
					} else {
						assertGranularPRResult(t, result, schemas[tc.name], tc.text, false)
					}
				}
				// Legacy handlers sent negative numbers and non-enum strings to GitHub.
				for _, tc := range []granularPRCase{
					{"update_pull_request_title", map[string]any{"pullNumber": -1, "title": "Title"}, granularPRReference, `{"title":"Title"}`},
					{"create_pull_request_review", map[string]any{"event": "other"}, "pull request review submitted successfully", `"event":"other"`},
					{"add_pull_request_review_comment", map[string]any{"path": "f", "body": "b", "subjectType": "other", "side": "other", "line": -1}, "pull request review comment successfully added to pending review", `"line":-1`},
				} {
					current = &tc
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: granularPRArgs(tc)})
					require.NoError(t, err)
					assertGranularPRResult(t, result, schemas[tc.name], tc.text, false)
				}
			})
		}
	}
}

func TestTypedGranularPullRequestErrorsAndEmptyResults(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		for _, reason := range []bool{false, true} {
			t.Run("protocol="+protocol+"/reason="+mustMarshalJSON(t, reason), func(t *testing.T) {
				session, schemas := granularPRSession(t, granularPRDeps(t, nil, "error"), protocol, reason)
				for _, tc := range granularPRValidationCases() {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: granularPRArgs(tc)})
					require.NoError(t, err)
					assertGranularPRResult(t, result, schemas[tc.name], tc.text, true)
				}
				for _, tc := range granularPRCases() {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: granularPRArgs(tc)})
					require.NoError(t, err)
					require.True(t, result.IsError, tc.name)
					require.Len(t, result.Content, 1)
					assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "Forbidden")
					assert.Nil(t, result.StructuredContent)
				}
				session, _ = granularPRSession(t, granularPRDeps(t, nil, "mutation_error"), protocol, reason)
				for _, tc := range granularPRCases() {
					if strings.HasPrefix(tc.text, "{") || tc.name == "remove_pull_request_review_comment_reaction" {
						continue
					}
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: granularPRArgs(tc)})
					require.NoError(t, err)
					require.True(t, result.IsError, tc.name)
					require.Len(t, result.Content, 1)
					assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "Forbidden")
					assert.Nil(t, result.StructuredContent)
				}
				session, schemas = granularPRSession(t, granularPRDeps(t, nil, "empty"), protocol, reason)
				for _, tc := range granularPRCases() {
					text := tc.text
					isError := false
					switch tc.name {
					case "update_pull_request_title", "update_pull_request_body", "update_pull_request_state", "request_pull_request_reviewers":
						text = `{"id":"0","url":""}`
					case "add_pull_request_review_comment_reaction":
						text = `{"id":"0","url":"https://api.github.com/repos/owner/repo/pulls/comments/42/reactions/0"}`
					case "submit_pending_pull_request_review", "delete_pending_pull_request_review", "add_pull_request_review_comment":
						text, isError = "No pending review found for the viewer", true
					}
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: granularPRArgs(tc)})
					require.NoError(t, err)
					assertGranularPRResult(t, result, schemas[tc.name], text, isError)
				}
			})
		}
	}
}

func TestGranularPullRequestFeatureVariants(t *testing.T) {
	for _, tool := range granularPullRequestTools() {
		assert.False(t, tool.IsReadOnly(), tool.Tool.Name)
		assert.Equal(t, []string{"repo"}, tool.ScopeAccess.Scopes, tool.Tool.Name)
		assert.False(t, tool.FeatureRule.Enabled(func(inventory.FeatureFlag) bool { return false }), tool.Tool.Name)
	}
	variant := GranularResolveReviewThreadWithResolutionReason(translations.NullTranslationHelper, WithHost(utils.HostTypeGHES))
	require.NotNil(t, variant.Enabled)
	enabled, err := variant.Enabled(context.Background())
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestGranularPullRequestAdvertisedInputConstraints(t *testing.T) {
	enumFields := map[string]map[string][]any{
		"update_pull_request_state":          {"state": {"open", "closed"}},
		"create_pull_request_review":         {"event": {"APPROVE", "REQUEST_CHANGES", "COMMENT"}},
		"submit_pending_pull_request_review": {"event": {"APPROVE", "REQUEST_CHANGES", "COMMENT"}},
		"add_pull_request_review_comment": {
			"subjectType": {"FILE", "LINE"}, "side": {"LEFT", "RIGHT"}, "startSide": {"LEFT", "RIGHT"},
		},
		"add_pull_request_review_comment_reaction": {"content": {"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"}},
	}
	for _, tool := range granularPullRequestTools() {
		schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
		require.True(t, ok, tool.Tool.Name)
		validation := granularPullRequestValidationSchema(schema)
		for _, field := range []string{"pullNumber", "comment_id", "reaction_id"} {
			if property := schema.Properties[field]; property != nil {
				require.NotNil(t, property.Minimum, tool.Tool.Name+"/"+field)
				assert.Equal(t, 1.0, *property.Minimum, tool.Tool.Name+"/"+field)
				assert.Nil(t, validation.Properties[field].Minimum)
			}
		}
		for field, values := range enumFields[tool.Tool.Name] {
			require.NotNil(t, schema.Properties[field], tool.Tool.Name+"/"+field)
			assert.ElementsMatch(t, values, schema.Properties[field].Enum, tool.Tool.Name+"/"+field)
			assert.Empty(t, validation.Properties[field].Enum)
		}
		assert.ElementsMatch(t, schema.Required, validation.Required)
	}
}

func TestTypedInputDescriptionsMatchMain(t *testing.T) {
	descriptions := map[string]map[string]string{
		"add_pull_request_review_comment": {
			"side":        "The side of the diff to comment on (optional)",
			"startSide":   "The start side of a multi-line comment (optional)",
			"subjectType": "The subject type of the comment",
		},
		"add_pull_request_review_comment_reaction": {
			"content": "The emoji reaction type",
		},
		"create_pull_request_review": {
			"event": "The review action to perform. If omitted, creates a pending review.",
		},
		"submit_pending_pull_request_review": {
			"event": "The review action to perform",
		},
		"update_pull_request_state": {
			"state": "The new state for the pull request",
		},
		"find_duplicate": {
			"page":    "Page number for pagination (min 0). Zero is forwarded for the GitHub API default.",
			"perPage": "Results per page for pagination (min 0, max 100). Zero is forwarded for the GitHub API default.",
		},
	}
	tools := append(granularPullRequestTools(), FindDuplicate(translations.NullTranslationHelper))
	for _, tool := range tools {
		for field, description := range descriptions[tool.Tool.Name] {
			schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
			require.True(t, ok, tool.Tool.Name)
			property := schema.Properties[field]
			require.NotNil(t, property, tool.Tool.Name+"/"+field)
			assert.Equal(t, description, property.Description, tool.Tool.Name+"/"+field)
		}
	}

	findDuplicate, ok := FindDuplicate(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.Equal(t, 0.0, *findDuplicate.Properties["page"].Minimum)
	assert.Equal(t, 0.0, *findDuplicate.Properties["perPage"].Minimum)
	assert.Equal(t, 100.0, *findDuplicate.Properties["perPage"].Maximum)
}

func TestGranularPullRequestReactionSchema(t *testing.T) {
	schema, err := minimalPullRequestCommentReactionSchema().Resolve(nil)
	require.NoError(t, err)
	require.NoError(t, schema.Validate(map[string]any{"id": "77", "url": "https://api.github.com/reaction"}))
	require.NoError(t, schema.Validate(map[string]any{"id": "0", "url": ""}))
	for _, raw := range []string{
		`null`, `[]`, `{}`, `{"id":"77"}`, `{"id":77,"content":"other"}`,
		`{"id":77,"content":null}`, `{"id":77,"url":"https://api.github.com/reaction"}`,
		`{"id":"77","url":null}`, `{"id":"77","url":"https://api.github.com/reaction","content":"heart"}`,
	} {
		var output any
		require.NoError(t, json.Unmarshal([]byte(raw), &output))
		require.Error(t, schema.Validate(output), raw)
	}
}

func TestGranularPullRequestParentBehavior(t *testing.T) {
	tools := granularPullRequestTools()
	byName := make(map[string]inventory.ServerTool)
	for _, tool := range tools[:len(tools)-1] {
		byName[tool.Tool.Name] = tool
	}
	var current *granularPRCase
	deps := granularPRDeps(t, &current, "")
	ctx := ContextWithDeps(context.Background(), deps)
	for _, tc := range append(granularPRCases(), granularPRValidationCases()...) {
		t.Run(tc.name+"/"+tc.text, func(t *testing.T) {
			current = &tc
			tool := byName[tc.name]
			result, err := tool.Handler(deps)(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
				Name: tc.name, Arguments: json.RawMessage(mustMarshalJSON(t, granularPRArgs(tc))),
			}})
			require.NoError(t, err)
			require.Len(t, result.Content, 1)
			assert.Equal(t, tc.text, result.Content[0].(*mcp.TextContent).Text)
		})
	}
}

type granularPRClientErrorDeps struct{ BaseDeps }

func (granularPRClientErrorDeps) GetClient(context.Context) (*gogithub.Client, error) {
	return nil, fmt.Errorf("client unavailable")
}

func (granularPRClientErrorDeps) GetGQLClient(context.Context) (*githubv4.Client, error) {
	return nil, fmt.Errorf("client unavailable")
}

func TestTypedGranularPullRequestClientErrors(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		for _, reason := range []bool{false, true} {
			session, schemas := granularPRSession(t, granularPRClientErrorDeps{}, protocol, reason)
			for _, tc := range granularPRCases() {
				message := "failed to get GitHub GraphQL client: client unavailable"
				if strings.HasPrefix(tc.text, "{") || tc.name == "remove_pull_request_review_comment_reaction" {
					message = "failed to get GitHub client: client unavailable"
				}
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: granularPRArgs(tc)})
				require.NoError(t, err)
				assertGranularPRResult(t, result, schemas[tc.name], message, true)
			}
		}
	}
}

func TestTypedGranularPullRequestResolutionReasonPayload(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		for _, reason := range []bool{false, true} {
			for _, value := range []string{"addressed", "wont-fix", "invalid", ""} {
				client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var request struct {
						Variables struct {
							Input struct {
								ThreadID         string  `json:"threadId"`
								ResolutionReason *string `json:"resolutionReason"`
							} `json:"input"`
						} `json:"variables"`
					}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
					assert.Equal(t, "T_1", request.Variables.Input.ThreadID)
					if reason {
						require.NotNil(t, request.Variables.Input.ResolutionReason)
						assert.Equal(t, value, *request.Variables.Input.ResolutionReason)
					} else {
						assert.Nil(t, request.Variables.Input.ResolutionReason)
					}
					_, _ = w.Write([]byte(`{"data":{"resolveReviewThread":{"thread":{"id":"T_1","isResolved":true}}}}`))
				})}}
				session, schemas := granularPRSession(t, BaseDeps{GQLClient: githubv4.NewClient(client)}, protocol, reason)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: "resolve_review_thread", Arguments: map[string]any{"threadID": "T_1", "resolutionReason": value},
				})
				require.NoError(t, err)
				assertGranularPRResult(t, result, schemas["resolve_review_thread"], "review thread resolved successfully", false)
			}
		}
	}
}
