package github

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	getIssueCommentRoute  = "GET /repos/{owner}/{repo}/issues/comments/{comment_id}"
	getReviewCommentRoute = "GET /repos/{owner}/{repo}/pulls/comments/{comment_id}"
	getReviewRoute        = "GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}"
)

// minimizedReason is the lowercase, hyphenated form GitHub returns (e.g. "off-topic"), not the classifier enum.
func minimizeCommentMatcher(nodeID, classifier, minimizedReason string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			MinimizeComment struct {
				MinimizedComment struct {
					IsMinimized     githubv4.Boolean
					MinimizedReason githubv4.String
				}
			} `graphql:"minimizeComment(input: $input)"`
		}{},
		githubv4.MinimizeCommentInput{
			SubjectID:  githubv4.ID(nodeID),
			Classifier: githubv4.ReportedContentClassifiers(classifier),
		},
		nil,
		githubv4mock.DataResponse(map[string]any{
			"minimizeComment": map[string]any{
				"minimizedComment": map[string]any{
					"isMinimized":     true,
					"minimizedReason": minimizedReason,
				},
			},
		}),
	)
}

func unminimizeCommentMatcher(nodeID string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			UnminimizeComment struct {
				UnminimizedComment struct {
					IsMinimized githubv4.Boolean
				}
			} `graphql:"unminimizeComment(input: $input)"`
		}{},
		githubv4.UnminimizeCommentInput{SubjectID: githubv4.ID(nodeID)},
		nil,
		githubv4mock.DataResponse(map[string]any{
			"unminimizeComment": map[string]any{
				"unminimizedComment": map[string]any{"isMinimized": false},
			},
		}),
	)
}

func minimizeCommentErrorMatcher(nodeID, classifier string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			MinimizeComment struct {
				MinimizedComment struct {
					IsMinimized     githubv4.Boolean
					MinimizedReason githubv4.String
				}
			} `graphql:"minimizeComment(input: $input)"`
		}{},
		githubv4.MinimizeCommentInput{
			SubjectID:  githubv4.ID(nodeID),
			Classifier: githubv4.ReportedContentClassifiers(classifier),
		},
		nil,
		githubv4mock.ErrorResponse("Resource not accessible by integration"),
	)
}

func unminimizeCommentErrorMatcher(nodeID string) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			UnminimizeComment struct {
				UnminimizedComment struct {
					IsMinimized githubv4.Boolean
				}
			} `graphql:"unminimizeComment(input: $input)"`
		}{},
		githubv4.UnminimizeCommentInput{SubjectID: githubv4.ID(nodeID)},
		nil,
		githubv4mock.ErrorResponse("Resource not accessible by integration"),
	)
}

func Test_CommentVisibilityToolSchemas(t *testing.T) {
	tests := []struct {
		tool            inventory.ServerTool
		name            string
		toolset         inventory.ToolsetID
		featureFlag     string
		expectedRequire []string
	}{
		{
			tool:            GranularHideIssueComment(translations.NullTranslationHelper),
			name:            "hide_issue_comment",
			toolset:         ToolsetMetadataIssues.ID,
			featureFlag:     FeatureFlagIssuesGranular,
			expectedRequire: []string{"owner", "repo", "comment_id", "classifier"},
		},
		{
			tool:            GranularUnhideIssueComment(translations.NullTranslationHelper),
			name:            "unhide_issue_comment",
			toolset:         ToolsetMetadataIssues.ID,
			featureFlag:     FeatureFlagIssuesGranular,
			expectedRequire: []string{"owner", "repo", "comment_id"},
		},
		{
			tool:            GranularHidePullRequestReviewComment(translations.NullTranslationHelper),
			name:            "hide_pull_request_review_comment",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "comment_id", "classifier"},
		},
		{
			tool:            GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			name:            "unhide_pull_request_review_comment",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "comment_id"},
		},
		{
			tool:            GranularHidePullRequestReview(translations.NullTranslationHelper),
			name:            "hide_pull_request_review",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "pullNumber", "review_id", "classifier"},
		},
		{
			tool:            GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			name:            "unhide_pull_request_review",
			toolset:         ToolsetMetadataPullRequests.ID,
			featureFlag:     FeatureFlagPullRequestsGranular,
			expectedRequire: []string{"owner", "repo", "pullNumber", "review_id"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.tool.Tool
			if tc.toolset != ToolsetMetadataIssues.ID {
				require.NoError(t, toolsnaps.Test(tool.Name, tool))
			}

			assert.Equal(t, tc.name, tool.Name)
			assert.NotEmpty(t, tool.Description)
			assert.False(t, tool.Annotations.ReadOnlyHint)
			assert.Equal(t, tc.toolset, tc.tool.Toolset.ID)
			assert.Equal(t, []inventory.FeatureFlag{inventory.FeatureFlag(tc.featureFlag)}, tc.tool.FeatureRule.Features())
			assert.Equal(t, []string{"repo"}, tc.tool.ScopeAccess.Scopes)
			assert.True(t, tc.tool.ScopeAccess.Visible([]string{"repo"}))
			assert.False(t, tc.tool.ScopeAccess.Visible(nil))
			assert.Equal(t, []string{"repo"}, tc.tool.ScopeAccess.Challenge(nil, nil))
			assert.Empty(t, tc.tool.ScopeAccess.Challenge(nil, []string{"repo"}))

			schema := tool.InputSchema.(*jsonschema.Schema)
			assert.ElementsMatch(t, tc.expectedRequire, schema.Required)
			assert.Len(t, schema.Properties, len(tc.expectedRequire), "every property should be required")
		})
	}
}

func TestCommentVisibilityTypedInputSchemaMatchesLegacy(t *testing.T) {
	for _, target := range []commentVisibilityTarget{issueCommentVisibilityTarget, pullRequestReviewCommentVisibilityTarget, pullRequestReviewVisibilityTarget} {
		for _, hide := range []bool{true, false} {
			tool := commentVisibilityTool(translations.NullTranslationHelper, target, hide)
			t.Run(tool.Tool.Name, func(t *testing.T) {
				legacyProperties := map[string]*jsonschema.Schema{
					"owner": {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":  {Type: "string", Description: "Repository name"},
				}
				maps.Copy(legacyProperties, target.properties())
				legacyRequired := append([]string{"owner", "repo"}, target.required...)
				if hide {
					legacyProperties["classifier"] = &jsonschema.Schema{
						Type:        "string",
						Description: "The reason for hiding the comment",
						Enum:        commentClassifiers,
					}
					legacyRequired = append(legacyRequired, "classifier")
				}
				legacy := &jsonschema.Schema{
					Type:       "object",
					Properties: legacyProperties,
					Required:   legacyRequired,
				}
				got, err := json.Marshal(tool.Tool.InputSchema)
				require.NoError(t, err)
				want, err := json.Marshal(legacy)
				require.NoError(t, err)
				assert.JSONEq(t, string(want), string(got))
			})
		}
	}
}

func connectCommentVisibilityClient(t *testing.T, server *mcp.Server, version string) *mcp.ClientSession {
	t.Helper()
	if version == "2025-11-25" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				if method == "server/discover" {
					return nil, errors.New("use initialize for legacy protocol compatibility test")
				}
				return next(ctx, method, req)
			}
		})
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "comment-visibility-test", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), commentVisibilityProtocolTransport{clientTransport, version}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	require.Equal(t, version, session.InitializeResult().ProtocolVersion)
	return session
}

type commentVisibilityProtocolTransport struct {
	mcp.Transport
	version string
}

func (t commentVisibilityProtocolTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	connection, err := t.Transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return commentVisibilityProtocolConnection{connection, t.version}, nil
}

type commentVisibilityProtocolConnection struct {
	mcp.Connection
	version string
}

func (c commentVisibilityProtocolConnection) Write(ctx context.Context, message jsonrpc.Message) error {
	req, ok := message.(*jsonrpc.Request)
	if !ok || req.Method != "initialize" {
		return c.Connection.Write(ctx, message)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return err
	}
	version, err := json.Marshal(c.version)
	if err != nil {
		return err
	}
	params["protocolVersion"] = version
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	requestCopy := *req
	requestCopy.Params = encoded
	return c.Connection.Write(ctx, &requestCopy)
}

func TestCommentVisibilityProtocols(t *testing.T) {
	// Typed output requires a supported negotiated 2026-07-28+ version;
	// unknown versions remain legacy even when their date is later.
	for _, version := range []string{"2025-11-25", "2026-07-28"} {
		for _, target := range []commentVisibilityTarget{issueCommentVisibilityTarget, pullRequestReviewCommentVisibilityTarget, pullRequestReviewVisibilityTarget} {
			for _, hide := range []bool{true, false} {
				tool := commentVisibilityTool(translations.NullTranslationHelper, target, hide)
				t.Run(version+"/"+tool.Tool.Name, func(t *testing.T) {
					route := getIssueCommentRoute
					switch target.name {
					case "pull_request_review_comment":
						route = getReviewCommentRoute
					case "pull_request_review":
						route = getReviewRoute
					}
					matcher := unminimizeCommentMatcher("NODE_1")
					expected := MinimizeCommentResult{NodeID: "NODE_1"}
					legacyText := `{"node_id":"NODE_1","is_minimized":false}`
					if hide {
						matcher = minimizeCommentMatcher("NODE_1", "OFF_TOPIC", "off-topic")
						expected.IsMinimized, expected.MinimizedReason = true, "off-topic"
						legacyText = `{"node_id":"NODE_1","is_minimized":true,"minimized_reason":"off-topic"}`
					}
					deps := BaseDeps{
						Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
							route: mockResponse(t, http.StatusOK, map[string]any{"node_id": "NODE_1"}),
						})),
						GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher)),
					}
					inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{tool}).
						WithToolsets([]string{"all"}).
						WithFeatureChecker(func(context.Context, string) (bool, error) { return true, nil }).Build()
					require.NoError(t, err)
					server := mcp.NewServer(&mcp.Implementation{Name: "visibility-test", Version: "v1"}, nil)
					server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
					inv.RegisterTools(context.Background(), server, deps)
					session := connectCommentVisibilityClient(t, server, version)
					list, err := session.ListTools(context.Background(), nil)
					require.NoError(t, err)
					require.Len(t, list.Tools, 1)
					if version == "2025-11-25" {
						assert.Nil(t, list.Tools[0].OutputSchema)
					} else {
						assert.NotNil(t, list.Tools[0].OutputSchema)
					}
					args := map[string]any{"owner": "owner", "repo": "repo", "extra": "ignored"}
					if target.name == "pull_request_review" {
						args["pullNumber"], args["review_id"] = "42", "3"
					} else {
						args["comment_id"] = "1e0"
					}
					if hide {
						args["classifier"] = "oFf_ToPiC"
					}
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: args})
					require.NoError(t, err)
					require.False(t, result.IsError, result)
					require.Len(t, result.Content, 1)
					textResult := getTextResult(t, result).Text
					if version == "2025-11-25" {
						assert.Equal(t, legacyText, textResult)
						assert.Nil(t, result.StructuredContent)
					} else {
						var textOutput MinimizeCommentResult
						require.NoError(t, json.Unmarshal([]byte(textResult), &textOutput))
						assert.Equal(t, expected, textOutput)
						text, err := json.Marshal(textOutput)
						require.NoError(t, err)
						structured, err := json.Marshal(result.StructuredContent)
						require.NoError(t, err)
						assert.JSONEq(t, string(text), string(structured))
						schema := tool.Tool.OutputSchema.(*jsonschema.Schema)
						resolved, err := schema.Resolve(nil)
						require.NoError(t, err)
						var value any
						require.NoError(t, json.Unmarshal(structured, &value))
						require.NoError(t, resolved.Validate(value))
					}
					invalid := maps.Clone(args)
					delete(invalid, "owner")
					result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: invalid})
					require.NoError(t, err)
					require.True(t, result.IsError)
					assert.Nil(t, result.StructuredContent)
					assert.Contains(t, getErrorResult(t, result).Text, "missing required parameter: owner")
				})
			}
		}
	}
}

func TestCommentVisibilityProtocolGating(t *testing.T) {
	for _, version := range []string{"2025-11-25", "2026-07-28"} {
		for _, target := range []commentVisibilityTarget{issueCommentVisibilityTarget, pullRequestReviewCommentVisibilityTarget, pullRequestReviewVisibilityTarget} {
			for _, hide := range []bool{true, false} {
				tool := commentVisibilityTool(translations.NullTranslationHelper, target, hide)
				for _, gate := range []string{"feature disabled", "missing repo scope", "read only"} {
					t.Run(version+"/"+tool.Tool.Name+"/"+gate, func(t *testing.T) {
						builder := inventory.NewBuilder().SetTools([]inventory.ServerTool{tool}).
							WithToolsets([]string{"all"}).
							WithFeatureChecker(func(context.Context, string) (bool, error) { return gate != "feature disabled", nil })
						if gate == "missing repo scope" {
							builder.WithFilter(CreateToolScopeFilter([]string{"public_repo"}))
						}
						if gate == "read only" {
							builder.WithReadOnly(true)
						}
						inv, err := builder.Build()
						require.NoError(t, err)
						server := mcp.NewServer(&mcp.Implementation{Name: "visibility-gates", Version: "v1"}, nil)
						inv.RegisterTools(context.Background(), server, nil)
						session := connectCommentVisibilityClient(t, server, version)
						list, err := session.ListTools(context.Background(), nil)
						require.NoError(t, err)
						assert.Empty(t, list.Tools)
						result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name})
						if err == nil {
							require.NotNil(t, result)
							assert.True(t, result.IsError)
							assert.Nil(t, result.StructuredContent)
						}
					})
				}
			}
		}
	}
}

func TestNormalizeCommentVisibilityInput(t *testing.T) {
	for _, target := range []commentVisibilityTarget{issueCommentVisibilityTarget, pullRequestReviewCommentVisibilityTarget, pullRequestReviewVisibilityTarget} {
		for _, tc := range []struct {
			name  string
			value any
			want  int64
		}{
			{"number", float64(1), 1},
			{"string", "1", 1},
			{"decimal string", "1.0", 1},
			{"exponent string", "1e0", 1},
			{"signed string", "+1", 1},
			{"leading zero string", "001", 1},
			{"legacy float precision", "9007199254740993", 9007199254740992},
			{"largest round-trippable positive ID", "9223372036854774784", 9223372036854774784},
		} {
			t.Run(target.name+"/"+tc.name, func(t *testing.T) {
				args := map[string]any{"owner": "owner", "repo": "repo", "classifier": "sPaM"}
				for _, name := range target.required {
					args[name] = tc.value
				}
				if target.name == "pull_request_review" {
					args["pullNumber"] = "42"
				}
				input, err := normalizeCommentVisibilityInput(args, target, true)
				require.NoError(t, err)
				tool := commentVisibilityTool(translations.NullTranslationHelper, target, true)
				schema, err := tool.Tool.InputSchema.(*jsonschema.Schema).Resolve(nil)
				require.NoError(t, err)
				encoded, err := json.Marshal(input)
				require.NoError(t, err)
				var canonical any
				require.NoError(t, json.Unmarshal(encoded, &canonical))
				require.NoError(t, schema.Validate(canonical))
				assert.Equal(t, "SPAM", input.Classifier)
				if target.name == "pull_request_review" {
					assert.Equal(t, tc.want, input.ReviewID)
					assert.Equal(t, 42, input.PullNumber)
				} else {
					assert.Equal(t, tc.want, input.CommentID)
				}
			})
		}
	}
}

func Test_HideAndUnhideComments(t *testing.T) {
	issueComment := mockResponse(t, http.StatusOK, &github.IssueComment{ID: new(int64(1)), NodeID: new("IC_1")})
	reviewComment := mockResponse(t, http.StatusOK, &github.PullRequestComment{ID: new(int64(2)), NodeID: new("PRRC_2")})
	review := mockResponse(t, http.StatusOK, &github.PullRequestReview{ID: new(int64(3)), NodeID: new("PRR_3")})
	notFound := mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`)

	type visibilityTestCase struct {
		name           string
		tool           inventory.ServerTool
		restHandlers   map[string]http.HandlerFunc
		gqlMatchers    []githubv4mock.Matcher
		requestArgs    map[string]any
		expectedResult MinimizeCommentResult
		expectedErrMsg string
	}
	tests := []visibilityTestCase{
		{
			name:           "hide issue comment",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentMatcher("IC_1", "SPAM", "spam")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1), "classifier": "spam"},
			expectedResult: MinimizeCommentResult{NodeID: "IC_1", IsMinimized: true, MinimizedReason: "spam"},
		},
		{
			name:           "unhide issue comment",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentMatcher("IC_1")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedResult: MinimizeCommentResult{NodeID: "IC_1", IsMinimized: false},
		},
		{
			name:           "hide pull request review comment",
			tool:           GranularHidePullRequestReviewComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewCommentRoute: reviewComment},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentMatcher("PRRC_2", "OUTDATED", "outdated")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(2), "classifier": "OUTDATED"},
			expectedResult: MinimizeCommentResult{NodeID: "PRRC_2", IsMinimized: true, MinimizedReason: "outdated"},
		},
		{
			name:           "unhide pull request review comment",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewCommentRoute: reviewComment},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentMatcher("PRRC_2")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(2)},
			expectedResult: MinimizeCommentResult{NodeID: "PRRC_2", IsMinimized: false},
		},
		{
			name:           "hide pull request review",
			tool:           GranularHidePullRequestReview(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewRoute: review},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentMatcher("PRR_3", "RESOLVED", "resolved")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(3), "classifier": "RESOLVED"},
			expectedResult: MinimizeCommentResult{NodeID: "PRR_3", IsMinimized: true, MinimizedReason: "resolved"},
		},
		{
			name:           "unhide pull request review",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewRoute: review},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentMatcher("PRR_3")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(3)},
			expectedResult: MinimizeCommentResult{NodeID: "PRR_3", IsMinimized: false},
		},
		{
			name:           "issue comment not found",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: notFound},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "failed to get issue comment",
		},
		{
			name:           "pull request review comment not found",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewCommentRoute: notFound},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(2)},
			expectedErrMsg: "failed to get pull request review comment",
		},
		{
			name:           "pull request review not found",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getReviewRoute: notFound},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(3)},
			expectedErrMsg: "failed to get pull request review",
		},
		{
			name: "response without node ID",
			tool: GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers: map[string]http.HandlerFunc{
				getIssueCommentRoute: mockResponse(t, http.StatusOK, &github.IssueComment{ID: new(int64(1))}),
			},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "response has no node ID",
		},
		{
			name:           "hide mutation fails",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{minimizeCommentErrorMatcher("IC_1", "SPAM")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1), "classifier": "SPAM"},
			expectedErrMsg: "failed to minimize comment",
		},
		{
			name:           "unhide mutation fails",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentErrorMatcher("IC_1")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "failed to unminimize comment",
		},
		{
			name:           "invalid classifier is rejected before any API call",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1), "classifier": "BOGUS"},
			expectedErrMsg: `invalid classifier "BOGUS"`,
		},
		{
			name:           "negative issue comment_id",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(-1), "classifier": "SPAM"},
			expectedErrMsg: "comment_id must be greater than 0",
		},
		{
			name:           "negative pull request review comment_id",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(-2)},
			expectedErrMsg: "comment_id must be greater than 0",
		},
		{
			name:           "negative review_id",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42), "review_id": float64(-3)},
			expectedErrMsg: "review_id must be greater than 0",
		},
		{
			name:           "negative pullNumber",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(-42), "review_id": float64(3)},
			expectedErrMsg: "pullNumber must be greater than 0",
		},
		{
			name:           "missing owner",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "owner",
		},
		{
			name:           "missing repo",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "comment_id": float64(1)},
			expectedErrMsg: "repo",
		},
		{
			name:           "missing classifier",
			tool:           GranularHideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": float64(1)},
			expectedErrMsg: "classifier",
		},
		{
			name:           "missing issue comment_id",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo"},
			expectedErrMsg: "comment_id",
		},
		{
			name:           "missing pull request review comment_id",
			tool:           GranularUnhidePullRequestReviewComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo"},
			expectedErrMsg: "comment_id",
		},
		{
			name:           "missing pullNumber",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "review_id": float64(3)},
			expectedErrMsg: "pullNumber",
		},
		{
			name:           "missing review_id",
			tool:           GranularUnhidePullRequestReview(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "pullNumber": float64(42)},
			expectedErrMsg: "review_id",
		},
		{
			name:           "unhide ignores classifier and unrelated identifiers",
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			restHandlers:   map[string]http.HandlerFunc{getIssueCommentRoute: issueComment},
			gqlMatchers:    []githubv4mock.Matcher{unminimizeCommentMatcher("IC_1")},
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": "1.0", "classifier": false, "review_id": "invalid", "pullNumber": nil},
			expectedResult: MinimizeCommentResult{NodeID: "IC_1"},
		},
	}
	for _, tc := range []struct {
		name  string
		value any
		error string
	}{
		{"zero", float64(0), "missing required parameter: comment_id"},
		{"string zero", "0", "missing required parameter: comment_id"},
		{"fractional", 1.5, "non-integer numeric value"},
		{"fractional string", "1.5", "non-integer numeric value"},
		{"invalid string", "abc", "invalid numeric value"},
		{"null", nil, "expected number, got <nil>"},
		{"boolean", true, "expected number, got bool"},
		{"string NaN", "NaN", "non-finite numeric value"},
		{"string infinity", "+Inf", "non-finite numeric value"},
		{"overflow", "9223372036854777856", "too large to fit in int64"},
	} {
		tests = append(tests, visibilityTestCase{
			name:           "issue ID " + tc.name,
			tool:           GranularUnhideIssueComment(translations.NullTranslationHelper),
			requestArgs:    map[string]any{"owner": "owner", "repo": "repo", "comment_id": tc.value},
			expectedErrMsg: tc.error,
		})
	}

	for _, tc := range tests {
		for _, version := range []string{"2025-11-25", "2026-07-28"} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				deps := BaseDeps{
					Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(tc.restHandlers)),
					GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tc.gqlMatchers...)),
				}
				inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{tc.tool}).
					WithToolsets([]string{"all"}).
					WithFeatureChecker(func(context.Context, string) (bool, error) { return true, nil }).Build()
				require.NoError(t, err)
				server := mcp.NewServer(&mcp.Implementation{Name: "visibility-behavior", Version: "v1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				inv.RegisterTools(context.Background(), server, deps)
				session := connectCommentVisibilityClient(t, server, version)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.requestArgs})
				require.NoError(t, err)

				if tc.expectedErrMsg != "" {
					require.True(t, result.IsError)
					assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
					assert.Nil(t, result.StructuredContent)
					return
				}

				require.False(t, result.IsError, getTextResult(t, result).Text)
				var got MinimizeCommentResult
				require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &got))
				assert.Equal(t, tc.expectedResult, got)
			})
		}
	}
}
