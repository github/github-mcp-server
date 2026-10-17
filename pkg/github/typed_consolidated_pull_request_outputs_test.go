package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var typedPullRequestProtocols = []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""}

func consolidatedPullRequestSession(t *testing.T, deps BaseDeps, protocol string, ui bool, flags ...inventory.FeatureFlag) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	tr := translations.NullTranslationHelper
	tools := []inventory.ServerTool{
		PullRequestRead(tr), CreatePullRequest(tr), UpdatePullRequest(tr), MergePullRequest(tr),
		UpdatePullRequestBranch(tr), PullRequestReviewWrite(tr), PullRequestReviewWriteWithResolutionReason(tr),
		AddCommentToPendingReview(tr), AddReplyToPullRequestComment(tr),
	}
	canonicalSchemas := make(map[string]string)
	for _, tool := range tools {
		if tool.Tool.Name == "pull_request_review_write" {
			continue
		}
		canonicalSchemas[tool.Tool.Name] = mustMarshalJSON(t, tool.Tool.OutputSchema)
	}
	review := PullRequestReviewWrite(tr)
	if slices.Contains(flags, FeatureFlagThreadResolutionReason) {
		review = PullRequestReviewWriteWithResolutionReason(tr)
		require.NoError(t, toolsnaps.Test("pull_request_review_write_resolution_reason", review.Tool))
	}
	canonicalSchemas[review.Tool.Name] = mustMarshalJSON(t, review.Tool.OutputSchema)
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).
		WithFeatureChecker(featureCheckerFor(flags...)).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "consolidated-pull-requests", Version: "v1"}, nil)
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
			result, err := next(ghcontext.WithUISupport(ctx, ui), method, request)
			if protocol == inventory.ProtocolVersionMultiRoundTrip && method == "tools/call" && err == nil {
				if call, ok := result.(*mcp.CallToolResult); ok && !call.IsError && len(call.Content) == 1 {
					if text, ok := call.Content[0].(*mcp.TextContent); ok && text.Text == "null" {
						// The client decodes explicit null and an omitted field as nil.
						// Check the serialized response to distinguish them.
						var wire map[string]json.RawMessage
						require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, call)), &wire))
						structured, present := wire["structuredContent"]
						assert.True(t, present, "successful nullable output must include structuredContent")
						assert.Equal(t, "null", string(structured))
					}
				}
			}
			return result, err
		}
	})
	version := protocol
	if version == "" {
		version = inventory.ProtocolVersionMultiRoundTrip
	}
	session := connectCommentVisibilityClient(t, server, version)
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	// Exactly one pull_request_review_write variant is exposed for a feature set.
	require.Len(t, list.Tools, len(tools)-1)
	schemas := make(map[string]*jsonschema.Resolved)
	for _, tool := range list.Tools {
		if protocol != inventory.ProtocolVersionMultiRoundTrip {
			assert.Nil(t, tool.OutputSchema, tool.Name)
			continue
		}
		require.NotNil(t, tool.OutputSchema, tool.Name)
		assert.JSONEq(t, canonicalSchemas[tool.Name], mustMarshalJSON(t, tool.OutputSchema), tool.Name)
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	return session, schemas
}

const (
	typedPRJSON          = `{"number":1,"title":"Subject","body":"body","state":"open","draft":false,"merged":false,"mergeable_state":"clean","html_url":"https://github.com/owner/repo/pull/1","user":{"login":"octocat"},"head":{"ref":"feature","sha":"abc"},"base":{"ref":"main","sha":"def"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
	typedPRDiff          = "diff --git a/file b/file\n+added\n"
	typedPRReviewThreads = `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[{"id":"T_1","isResolved":false,"isOutdated":false,"isCollapsed":false,"comments":{"nodes":[{"id":"C_1","body":"nit","path":"file","line":3,"originalLine":3,"startLine":null,"originalStartLine":null,"author":{"login":"octocat"},"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","url":"https://github.com/owner/repo/pull/1#discussion_r1"}],"totalCount":1}}],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"startCursor":"s","endCursor":"e"},"totalCount":1}}}}}`
)

func consolidatedPullRequestDeps(t *testing.T) BaseDeps {
	t.Helper()
	rest := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3")
		write := func(status int, body string) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}
		switch {
		case path == "/repos/owner/repo/pulls/1" && strings.Contains(r.Header.Get("Accept"), "diff"):
			write(http.StatusOK, typedPRDiff)
		case path == "/repos/owner/repo/pulls/1" || path == "/repos/owner/repo/pulls" && r.Method == http.MethodPost:
			status := http.StatusOK
			if r.Method == http.MethodPost {
				status = http.StatusCreated
			}
			write(status, typedPRJSON)
		case path == "/repos/owner/repo/pulls/2":
			// A sparse PR without a head SHA still yields a valid status payload.
			write(http.StatusOK, `{"number":2,"head":{"sha":"abc"}}`)
		case path == "/repos/owner/repo/pulls/3":
			// A successful null API payload is normalized to the legacy zero-value PR.
			write(http.StatusOK, `null`)
		case path == "/repos/owner/repo/commits/abc/status":
			write(http.StatusOK, `{"state":"success","sha":"abc","total_count":1,"statuses":[{"state":"success","context":"ci","target_url":"https://ci"}]}`)
		case path == "/repos/owner/repo/commits/abc/check-runs":
			if r.URL.Query().Get("page") == "2" {
				write(http.StatusOK, `{"total_count":0,"check_runs":[]}`)
			} else {
				write(http.StatusOK, `{"total_count":1,"check_runs":[{"id":5,"name":"build","status":"completed","conclusion":"success"}]}`)
			}
		case path == "/repos/owner/repo/pulls/1/files":
			write(http.StatusOK, `[{"filename":"file","status":"modified","additions":1,"changes":1,"patch":"+added"}]`)
		case path == "/repos/owner/repo/pulls/2/files", path == "/repos/owner/repo/pulls/2/commits",
			path == "/repos/owner/repo/pulls/2/reviews", path == "/repos/owner/repo/issues/2/comments":
			write(http.StatusOK, `[]`)
		case path == "/repos/owner/repo/pulls/1/commits":
			write(http.StatusOK, `[{"sha":"abc","html_url":"https://github.com/owner/repo/commit/abc","commit":{"message":"msg","author":{"name":"Octo","email":"octo@example.com","date":"2026-01-01T00:00:00Z"}}}]`)
		case path == "/repos/owner/repo/pulls/1/reviews":
			write(http.StatusOK, `[{"id":9,"state":"APPROVED","body":"lgtm","html_url":"https://github.com/owner/repo/pull/1#pullrequestreview-9","user":{"login":"octocat"},"commit_id":"abc","author_association":"MEMBER"}]`)
		case path == "/repos/owner/repo/pulls/3/reviews":
			// A successful null list payload is normalized to an empty list.
			write(http.StatusOK, `null`)
		case path == "/repos/owner/repo/issues/1/comments":
			write(http.StatusOK, `[{"id":42,"body":"hello","html_url":"https://github.com/owner/repo/pull/1#issuecomment-42"}]`)
		case path == "/repos/owner/repo/pulls/1/merge":
			write(http.StatusOK, `{"sha":"abc","merged":true,"message":"Pull Request successfully merged"}`)
		case path == "/repos/owner/repo/pulls/2/merge":
			write(http.StatusOK, `null`)
		case path == "/repos/owner/repo/pulls/1/update-branch":
			write(http.StatusAccepted, `{"message":"Updating pull request branch.","url":"https://github.com/owner/repo/pull/1"}`)
		case path == "/repos/owner/repo/pulls/2/update-branch":
			write(http.StatusUnprocessableEntity, `{"message":"merge conflict between base and head"}`)
		case path == "/repos/owner/repo/pulls/comments/42/reactions":
			write(http.StatusCreated, `{"id":77,"content":"heart"}`)
		case path == "/repos/owner/repo/pulls/1/comments":
			write(http.StatusCreated, `{"id":43,"body":"reply","html_url":"https://github.com/owner/repo/pull/1#discussion_r43"}`)
		default:
			t.Errorf("unexpected REST request: %s %s", r.Method, r.URL.Path)
			write(http.StatusInternalServerError, `{"message":"unexpected"}`)
		}
	})}}
	gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		var body string
		switch q := req.Query; {
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
		case strings.Contains(q, "reviewThreads("):
			body = typedPRReviewThreads
		case strings.Contains(q, "reviews(first: 100"):
			body = `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[{"id":"R_1","author":{"userId":"U_1"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}}`
		case strings.Contains(q, "viewer{"):
			body = `{"data":{"viewer":{"id":"U_1"}}}`
		case strings.Contains(q, "pullRequest(number: $prNum){id}"):
			body = `{"data":{"repository":{"pullRequest":{"id":"PR_1"}}}}`
		default:
			t.Errorf("unexpected GraphQL query: %s", req.Query)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(body))
	})}}
	return BaseDeps{
		Client: mustNewGHClient(t, rest), GQLClient: githubv4.NewClient(gql),
		RepoAccessCache: stubRepoAccessCache(nil, time.Minute),
	}
}

type typedPullRequestCase struct {
	tool string
	args map[string]any
	text string
}

func runTypedPullRequestCases(t *testing.T, session *mcp.ClientSession, schemas map[string]*jsonschema.Resolved, cases []typedPullRequestCase) {
	t.Helper()
	for _, tc := range cases {
		args := map[string]any{"owner": "owner", "repo": "repo"}
		maps.Copy(args, tc.args)
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
		require.NoError(t, err)
		require.False(t, result.IsError, "%s %v: %s", tc.tool, tc.args, mustMarshalJSON(t, result))
		require.Len(t, result.Content, 1)
		text := getTextResult(t, result).Text
		schema := schemas[tc.tool]
		if schema == nil {
			assert.Equal(t, tc.text, text, "%s %v", tc.tool, tc.args)
			assert.Nil(t, result.StructuredContent, "%s %v", tc.tool, tc.args)
			continue
		}
		if tc.text == "null" {
			assert.Equal(t, "null", text)
			// Explicit wire presence is checked by the session middleware.
			assert.Nil(t, result.StructuredContent, "%s %v", tc.tool, tc.args)
			require.NoError(t, schema.Validate(nil))
			continue
		}
		require.NotNil(t, result.StructuredContent, "%s %v", tc.tool, tc.args)
		var value any
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &value))
		require.NoError(t, schema.Validate(value), "%s %v", tc.tool, tc.args)
		// The foundation wrapper serializes the compact DTO once for both
		// modern JSON text and structuredContent, including diff and message DTOs.
		assert.JSONEq(t, mustMarshalJSON(t, value), text, "%s %v", tc.tool, tc.args)
		switch {
		case tc.args["method"] == "get_diff":
			assert.JSONEq(t, mustMarshalJSON(t, PullRequestDiffOutput{Diff: tc.text}), mustMarshalJSON(t, value))
		case tc.tool == "pull_request_review_write" || tc.tool == "add_comment_to_pending_review":
			assert.JSONEq(t, mustMarshalJSON(t, RepositoryMessageOutput{Message: tc.text}), mustMarshalJSON(t, value))
		case tc.tool == "update_pull_request_branch":
			assert.JSONEq(t, `{"message":"Pull request branch update is in progress"}`, mustMarshalJSON(t, value))
		case tc.tool == "create_pull_request" || tc.tool == "update_pull_request":
			assert.JSONEq(t, `{"id":"0","html_url":"https://github.com/owner/repo/pull/1"}`, mustMarshalJSON(t, value))
		case tc.tool == "add_reply_to_pull_request_comment":
			switch {
			case tc.args["body"] != nil && tc.args["reaction"] != nil:
				assert.JSONEq(t, `{"comment":{"id":"43","html_url":"https://github.com/owner/repo/pull/1#discussion_r43"},"reaction":{"id":"77"}}`, mustMarshalJSON(t, value))
			case tc.args["body"] != nil:
				assert.JSONEq(t, `{"id":"43","html_url":"https://github.com/owner/repo/pull/1#discussion_r43"}`, mustMarshalJSON(t, value))
			default:
				assert.JSONEq(t, `{"id":"77"}`, mustMarshalJSON(t, value))
			}
		default:
			assert.JSONEq(t, tc.text, mustMarshalJSON(t, value))
		}
	}
}

func TestTypedConsolidatedPullRequestOutputs(t *testing.T) {
	const minimalPR = `{"number":1,"title":"Subject","body":"body","state":"open","draft":false,"merged":false,"mergeable_state":"clean","html_url":"https://github.com/owner/repo/pull/1","user":{"login":"octocat"},"head":{"ref":"feature","sha":"abc"},"base":{"ref":"main","sha":"def"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
	const prRef = `{"id":"0","url":"https://github.com/owner/repo/pull/1"}`
	cases := []typedPullRequestCase{
		{"pull_request_read", map[string]any{"method": "get", "pullNumber": "1"}, minimalPR},
		{"pull_request_read", map[string]any{"method": "get", "pullNumber": 3}, `null`},
		{"pull_request_read", map[string]any{"method": "get_diff", "pullNumber": 1.0}, typedPRDiff},
		{"pull_request_read", map[string]any{"method": "get_status", "pullNumber": 1}, `{"state":"success","sha":"abc","total_count":1,"statuses":[{"state":"success","context":"ci","target_url":"https://ci"}]}`},
		{"pull_request_read", map[string]any{"method": "get_files", "pullNumber": 1, "page": "1", "perPage": 5}, `[{"filename":"file","status":"modified","additions":1,"changes":1,"patch":"+added"}]`},
		{"pull_request_read", map[string]any{"method": "get_files", "pullNumber": 2}, `[]`},
		{"pull_request_read", map[string]any{"method": "get_commits", "pullNumber": 1}, `[{"sha":"abc","html_url":"https://github.com/owner/repo/commit/abc","message":"msg","author":{"name":"Octo","email":"octo@example.com","date":"2026-01-01T00:00:00Z"}}]`},
		{"pull_request_read", map[string]any{"method": "get_commits", "pullNumber": 2}, `[]`},
		{"pull_request_read", map[string]any{"method": "get_review_comments", "pullNumber": 1, "perPage": 10, "after": "cursor"}, `{"review_threads":[{"id":"T_1","is_resolved":false,"is_outdated":false,"is_collapsed":false,"comments":[{"body":"nit","path":"file","line":3,"original_line":3,"author":"octocat","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/pull/1#discussion_r1"}],"total_count":1}],"totalCount":1,"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"startCursor":"s","endCursor":"e"}}`},
		{"pull_request_read", map[string]any{"method": "get_reviews", "pullNumber": 1}, `[{"id":9,"state":"APPROVED","body":"lgtm","html_url":"https://github.com/owner/repo/pull/1#pullrequestreview-9","user":{"login":"octocat"},"commit_id":"abc","author_association":"MEMBER"}]`},
		{"pull_request_read", map[string]any{"method": "get_reviews", "pullNumber": 2}, `[]`},
		{"pull_request_read", map[string]any{"method": "get_reviews", "pullNumber": 3}, `[]`},
		{"pull_request_read", map[string]any{"method": "get_comments", "pullNumber": 1}, `[{"id":42,"body":"hello","html_url":"https://github.com/owner/repo/pull/1#issuecomment-42"}]`},
		{"pull_request_read", map[string]any{"method": "get_comments", "pullNumber": 2}, `[]`},
		{"pull_request_read", map[string]any{"method": "get_check_runs", "pullNumber": 1}, `{"total_count":1,"check_runs":[{"id":5,"name":"build","status":"completed","conclusion":"success"}]}`},
		{"pull_request_read", map[string]any{"method": "get_check_runs", "pullNumber": 1, "page": 2}, `{"total_count":0,"check_runs":[]}`},
		{"create_pull_request", map[string]any{"title": "Subject", "head": "feature", "base": "main", "draft": false}, prRef},
		{"update_pull_request", map[string]any{"pullNumber": "1", "title": "Subject", "maintainer_can_modify": false}, prRef},
		{"merge_pull_request", map[string]any{"pullNumber": 1, "merge_method": "squash", "expectedHeadSha": "abc"}, `{"sha":"abc","merged":true,"message":"Pull Request successfully merged"}`},
		{"merge_pull_request", map[string]any{"pullNumber": 2}, `null`},
		{"merge_pull_request", map[string]any{"pullNumber": 1.0, "merge_method": "squash", "expectedHeadSha": "abc"}, `{"sha":"abc","merged":true,"message":"Pull Request successfully merged"}`},
		{"update_pull_request_branch", map[string]any{"pullNumber": "1", "expectedHeadSha": "abc"}, "Pull request branch update is in progress"},
		{"pull_request_review_write", map[string]any{"method": "create", "pullNumber": "1"}, "pending pull request created"},
		{"pull_request_review_write", map[string]any{"method": "create", "pullNumber": 1, "event": "APPROVE", "body": "lgtm", "commitID": "abc"}, "pull request review submitted successfully"},
		{"pull_request_review_write", map[string]any{"method": "submit_pending", "pullNumber": 1, "event": "COMMENT", "body": "done"}, "pending pull request review successfully submitted"},
		{"pull_request_review_write", map[string]any{"method": "delete_pending", "pullNumber": 1}, "pending pull request review successfully deleted"},
		{"pull_request_review_write", map[string]any{"method": "resolve_thread", "threadId": "T_1"}, "review thread resolved successfully"},
		{"pull_request_review_write", map[string]any{"method": "unresolve_thread", "threadId": "T_1"}, "review thread unresolved successfully"},
		{"add_comment_to_pending_review", map[string]any{"pullNumber": "1", "path": "file", "body": "nit", "subjectType": "LINE", "line": "3", "side": "RIGHT", "startLine": 2, "startSide": 7}, "pull request review comment successfully added to pending review"},
		{"add_reply_to_pull_request_comment", map[string]any{"pullNumber": "1", "commentId": "42", "body": "reply"}, `{"id":"43","url":"https://github.com/owner/repo/pull/1#discussion_r43"}`},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": 42, "pullNumber": "x", "reaction": "heart"}, `{"id":"77","url":"https://api.github.com/repos/owner/repo/pulls/comments/42/reactions/77"}`},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": 42, "reaction": "heart"}, `{"id":"77","url":"https://api.github.com/repos/owner/repo/pulls/comments/42/reactions/77"}`},
		{"add_reply_to_pull_request_comment", map[string]any{"pullNumber": 1, "commentId": 42.0, "body": "reply", "reaction": "heart"}, `{"comment":{"id":"43","url":"https://github.com/owner/repo/pull/1#discussion_r43"},"reaction":{"id":"77","url":"https://api.github.com/repos/owner/repo/pulls/comments/42/reactions/77"}}`},
	}
	for _, protocol := range typedPullRequestProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, schemas := consolidatedPullRequestSession(t, consolidatedPullRequestDeps(t), protocol, false)
			runTypedPullRequestCases(t, session, schemas, cases)
		})
	}
}

func TestTypedPullRequestFilesLinkPagination(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var requestedPages []string
			rest := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/repos/owner/repo/pulls/1/files", strings.TrimPrefix(r.URL.Path, "/api/v3"))
				require.Equal(t, "1", r.URL.Query().Get("per_page"))
				page := r.URL.Query().Get("page")
				requestedPages = append(requestedPages, page)
				switch page {
				case "1":
					w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/pulls/1/files?page=2&per_page=1>; rel="next"`)
					_, _ = w.Write([]byte(`[{"filename":"first","status":"modified","additions":1,"changes":1}]`))
				case "2":
					_, _ = w.Write([]byte(`[{"filename":"second","status":"added","additions":1,"changes":1}]`))
				default:
					t.Errorf("unexpected page query: %q", page)
					w.WriteHeader(http.StatusInternalServerError)
				}
			})}}
			deps := BaseDeps{
				Client: mustNewGHClient(t, rest), GQLClient: githubv4.NewClient(rest),
				RepoAccessCache: stubRepoAccessCache(nil, time.Minute),
			}
			session, schemas := consolidatedPullRequestSession(t, deps, protocol, false)
			runTypedPullRequestCases(t, session, schemas, []typedPullRequestCase{
				{"pull_request_read", map[string]any{"method": "get_files", "pullNumber": 1, "page": 1, "perPage": 1}, `[{"filename":"first","status":"modified","additions":1,"changes":1}]`},
				{"pull_request_read", map[string]any{"method": "get_files", "pullNumber": 1, "page": 2, "perPage": 1}, `[{"filename":"second","status":"added","additions":1,"changes":1}]`},
			})
			assert.Equal(t, []string{"1", "2"}, requestedPages, "explicit page calls must follow the API Link header's next-page target")
		})
	}
}

func TestTypedPullRequestReviewWriteResolutionReasonVariant(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, schemas := consolidatedPullRequestSession(t, consolidatedPullRequestDeps(t), protocol, false, FeatureFlagThreadResolutionReason)
			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			for _, tool := range list.Tools {
				if tool.Name == "pull_request_review_write" {
					assert.Contains(t, mustMarshalJSON(t, tool.InputSchema), "resolutionReason")
				}
			}
			runTypedPullRequestCases(t, session, schemas, []typedPullRequestCase{
				{"pull_request_review_write", map[string]any{"Method": "resolve_thread", "ThreadID": "T_1", "resolutionReason": "OUTDATED"}, "review thread resolved successfully"},
				{"pull_request_review_write", map[string]any{"method": "create", "pullNumber": 1.5}, "pending pull request created"},
			})
		})
	}
}

func TestTypedPullRequestWriteAwaitingForm(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, schemas := consolidatedPullRequestSession(t, consolidatedPullRequestDeps(t), protocol, true)
			for tool, args := range map[string]map[string]any{
				"create_pull_request": {"owner": "owner", "repo": "repo", "title": "Subject", "head": "feature", "base": "main"},
				"update_pull_request": {"owner": "owner", "repo": "repo", "pullNumber": 1, "title": "Subject"},
			} {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
				require.NoError(t, err)
				require.True(t, result.IsError, tool)
				assert.Contains(t, getErrorResult(t, result).Text, "interactive form")
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					require.NotNil(t, result.StructuredContent)
					assert.JSONEq(t, `{"status":"awaiting_user_submission","reason":"An interactive form is being shown to the user. The operation has not been performed."}`, mustMarshalJSON(t, result.StructuredContent))
					var value any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &value))
					require.NoError(t, schemas[tool].Validate(value))
				} else {
					assert.Nil(t, result.StructuredContent)
				}
				args["_ui_submitted"] = true
				runTypedPullRequestCases(t, session, schemas, []typedPullRequestCase{{tool, args, `{"id":"0","url":"https://github.com/owner/repo/pull/1"}`}})
			}
		})
	}
}

func TestTypedConsolidatedPullRequestErrors(t *testing.T) {
	cases := []typedPullRequestCase{
		{"pull_request_read", map[string]any{"pullNumber": 1}, "missing required parameter: method"},
		{"pull_request_read", map[string]any{"method": "get", "pullNumber": 0}, "missing required parameter: pullNumber"},
		{"pull_request_read", map[string]any{"method": "get", "pullNumber": "1.5"}, "parameter pullNumber is not a valid number: non-integer numeric value: 1.5"},
		{"pull_request_read", map[string]any{"method": "get_files", "pullNumber": 1, "page": "x"}, "parameter page is not a valid number"},
		{"pull_request_read", map[string]any{"method": "unknown", "pullNumber": 1}, "unknown method: unknown"},
		{"pull_request_read", map[string]any{"method": "get", "pullNumber": 1.5}, "parameter pullNumber is not a valid number: non-integer numeric value: 1.5"},
		{"create_pull_request", map[string]any{"title": "Subject", "head": "feature", "base": "main", "draft": "yes"}, "parameter draft is not of type bool, is string"},
		{"update_pull_request", map[string]any{"pullNumber": 0, "title": "Subject"}, "missing required parameter: pullNumber"},
		{"update_pull_request", map[string]any{"pullNumber": 1, "draft": "yes"}, "parameter draft is not of type bool, is string"},
		{"pull_request_review_write", map[string]any{"method": "nope", "pullNumber": 1}, "unknown method: nope"},
		{"add_comment_to_pending_review", map[string]any{"pullNumber": 1, "body": "nit", "subjectType": "LINE"}, "missing required parameter: path"},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": 0, "body": "reply"}, "missing required parameter: commentId"},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": -1, "body": "reply"}, "commentId must be greater than 0"},
		{"create_pull_request", map[string]any{"head": "feature", "base": "main"}, "missing required parameter: title"},
		{"update_pull_request", map[string]any{"pullNumber": 1}, "No update parameters provided"},
		{"merge_pull_request", map[string]any{"pullNumber": "abc"}, "parameter pullNumber is not a valid number: invalid numeric value: abc"},
		{"update_pull_request_branch", map[string]any{"pullNumber": 2}, "merge conflict between base and head"},
		{"pull_request_review_write", map[string]any{"method": "resolve_thread"}, "threadId is required for resolve_thread and unresolve_thread methods"},
		{"pull_request_review_write", map[string]any{"method": "create", "pullNumber": "x"}, "'PullNumber' cannot parse value as 'int32'"},
		{"add_comment_to_pending_review", map[string]any{"pullNumber": 1, "path": "file", "body": "nit", "subjectType": "LINE", "line": 1.5}, "parameter line is not a valid number"},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": 42}, "at least one of body or reaction is required"},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": 42, "body": "reply"}, "missing required parameter: pullNumber"},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": "1e30", "body": "reply"}, "parameter commentId is not a valid number: numeric value 1e+30 is too large to fit in int64"},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": 42, "pullNumber": "x", "body": "reply"}, "parameter pullNumber is not a valid number: invalid numeric value: x"},
		{"add_reply_to_pull_request_comment", map[string]any{"commentId": 42, "pullNumber": "x", "body": "reply", "reaction": 1}, "parameter reaction is not of type string, is float64"},
	}
	for _, protocol := range typedPullRequestProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, _ := consolidatedPullRequestSession(t, consolidatedPullRequestDeps(t), protocol, false)
			for _, tc := range cases {
				args := map[string]any{"owner": "owner", "repo": "repo"}
				maps.Copy(args, tc.args)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				require.NoError(t, err, "%s %v", tc.tool, tc.args)
				require.True(t, result.IsError, "%s %v: %s", tc.tool, tc.args, mustMarshalJSON(t, result))
				assert.Contains(t, getErrorResult(t, result).Text, tc.text, "%s %v", tc.tool, tc.args)
				assert.Nil(t, result.StructuredContent, "%s %v", tc.tool, tc.args)
			}
		})
	}
}

func TestTypedConsolidatedPullRequestAPIErrors(t *testing.T) {
	forbidden := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
	})}}
	calls := []typedPullRequestCase{
		{"create_pull_request", map[string]any{"title": "Subject", "head": "feature", "base": "main"}, ""},
		{"update_pull_request", map[string]any{"pullNumber": 1, "title": "Subject"}, ""},
		{"merge_pull_request", map[string]any{"pullNumber": 1}, ""},
		{"update_pull_request_branch", map[string]any{"pullNumber": 1}, ""},
		{"pull_request_review_write", map[string]any{"method": "create", "pullNumber": 1}, ""},
		{"pull_request_review_write", map[string]any{"method": "submit_pending", "pullNumber": 1}, ""},
		{"pull_request_review_write", map[string]any{"method": "delete_pending", "pullNumber": 1}, ""},
		{"pull_request_review_write", map[string]any{"method": "resolve_thread", "threadId": "T_1"}, ""},
		{"pull_request_review_write", map[string]any{"method": "unresolve_thread", "threadId": "T_1"}, ""},
		{"add_comment_to_pending_review", map[string]any{"pullNumber": 1, "path": "file", "body": "nit", "subjectType": "FILE"}, ""},
		{"add_reply_to_pull_request_comment", map[string]any{"pullNumber": 1, "commentId": 42, "body": "reply"}, ""},
	}
	for _, method := range []string{"get", "get_diff", "get_status", "get_files", "get_commits", "get_review_comments", "get_reviews", "get_comments", "get_check_runs"} {
		calls = append(calls, typedPullRequestCase{"pull_request_read", map[string]any{"method": method, "pullNumber": 1}, ""})
	}
	for _, protocol := range typedPullRequestProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			deps := BaseDeps{
				Client: mustNewGHClient(t, forbidden), GQLClient: githubv4.NewClient(forbidden),
				RepoAccessCache: stubRepoAccessCache(nil, time.Minute),
			}
			session, _ := consolidatedPullRequestSession(t, deps, protocol, false)
			for _, tc := range calls {
				args := map[string]any{"owner": "owner", "repo": "repo"}
				maps.Copy(args, tc.args)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				require.NoError(t, err, "%s %v", tc.tool, tc.args)
				require.True(t, result.IsError, "%s %v: %s", tc.tool, tc.args, mustMarshalJSON(t, result))
				assert.Nil(t, result.StructuredContent, "%s %v", tc.tool, tc.args)
			}
		})
	}
}

func TestTypedPullRequestContentParity(t *testing.T) {
	for _, protocol := range typedPullRequestProtocols {
		for _, permission := range []string{"read", "write"} {
			t.Run(protocol+"/"+permission, func(t *testing.T) {
				deps := consolidatedPullRequestDeps(t)
				deps.Client = mustNewGHClient(t, &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch strings.TrimPrefix(r.URL.Path, "/api/v3") {
					case "/repos/owner/repo":
						_, _ = w.Write([]byte(`{"private":false}`))
					case "/repos/owner/repo/pulls/1":
						_, _ = w.Write([]byte(`{"number":1,"title":"Subject","body":"bo\u200bdy","state":"open","html_url":"https://github.com/owner/repo/pull/1","user":{"login":"octocat"}}`))
					default:
						t.Errorf("unexpected request: %s", r.URL.Path)
						w.WriteHeader(http.StatusInternalServerError)
					}
				})}})
				deps.Flags = stubFeatureFlags(map[string]bool{"lockdown-mode": true})
				deps.RepoAccessCache = stubRepoAccessCache(mockRESTPermissionServer(t, permission, nil), time.Minute)
				deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
				session, schemas := consolidatedPullRequestSession(t, deps, protocol, false)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name:      "pull_request_read",
					Arguments: map[string]any{"method": "get", "owner": "owner", "repo": "repo", "pullNumber": 1},
				})
				require.NoError(t, err)
				if permission == "read" {
					require.True(t, result.IsError)
					assert.Equal(t, "access to pull request is restricted by lockdown mode", getErrorResult(t, result).Text)
					assert.Nil(t, result.StructuredContent)
					return
				}
				require.False(t, result.IsError)
				text := getTextResult(t, result).Text
				require.NotNil(t, result.Meta["ifc"])
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					assert.JSONEq(t, mustMarshalJSON(t, result.StructuredContent), text)
					require.NoError(t, schemas["pull_request_read"].Validate(result.StructuredContent))
				} else {
					assert.Nil(t, result.StructuredContent)
					assert.Equal(t, `{"number":1,"title":"Subject","body":"body","state":"open","draft":false,"merged":false,"html_url":"https://github.com/owner/repo/pull/1","user":{"login":"octocat"}}`, text)
				}
			})
		}
	}
}

func TestConsolidatedPullRequestOutputSchemas(t *testing.T) {
	for _, tc := range []struct {
		schema  *jsonschema.Schema
		valid   []string
		invalid []string
	}{
		{
			pullRequestReadOutputSchema(),
			[]string{`null`, `[]`, `{"diff":""}`, `{"number":1,"title":"","state":"","draft":false,"merged":false,"html_url":""}`, `{"state":"","sha":"","total_count":0,"statuses":[]}`, `{"total_count":0,"check_runs":[]}`, `{"review_threads":[],"totalCount":0,"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`, `[{"filename":"f"}]`, `[{"id":1,"html_url":""}]`},
			[]string{`{}`, `"diff"`, `{"diff":1}`, `[{"unknown":1}]`, `{"number":1}`, `{"number":1,"title":"","state":"unknown","draft":false,"merged":false,"html_url":""}`},
		},
		{
			pullRequestWriteOutputSchema(),
			[]string{`{"id":"","html_url":""}`, `{"status":"awaiting_user_submission","reason":"wait"}`},
			[]string{`{}`, `null`, `{"status":"created","reason":"wait"}`},
		},
		{
			pullRequestCommentReplyOutputSchema(),
			[]string{`{"id":"1","html_url":"u"}`, `{"comment":{"id":"1","html_url":"u"},"reaction":{"id":"2"}}`},
			[]string{`{}`, `{"comment":{"id":"1","html_url":"u"}}`, `null`, `{"id":"1","url":"https://api.github.com/derivable"}`},
		},
		{
			pullRequestMergeOutputSchema(),
			[]string{`null`, `{}`, `{"sha":"abc","merged":true,"message":"ok"}`},
			[]string{`[]`, `{"merged":"yes"}`},
		},
		{
			pullRequestBranchUpdateOutputSchema(),
			[]string{`null`, `{"message":"m"}`},
			[]string{`[]`, `{"message":1}`, `{"message":"m","url":"https://api.github.com/derivable"}`},
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
	// Zero outputs are validated by the SDK even for errors, so they must conform.
	for _, tc := range []struct {
		schema *jsonschema.Schema
		value  any
	}{
		{pullRequestReadOutputSchema(), PullRequestReadOutput{}},
		{pullRequestWriteOutputSchema(), PullRequestWriteOutput{}},
		{pullRequestCommentReplyOutputSchema(), PullRequestCommentReplyOutput{}},
		{pullRequestMergeOutputSchema(), PullRequestMergeOutput{}},
		{pullRequestBranchUpdateOutputSchema(), PullRequestBranchUpdateOutput{}},
	} {
		resolved, err := tc.schema.Resolve(nil)
		require.NoError(t, err)
		var value any
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tc.value)), &value))
		require.NoError(t, resolved.Validate(value), mustMarshalJSON(t, tc.value))
	}
}

func TestPullRequestOutputDescriptionsAreShapeAware(t *testing.T) {
	commitSchema := pullRequestOutputSchema[MinimalPullRequestCommit]()
	require.Contains(t, commitSchema.Properties, "message")
	assert.Empty(t, commitSchema.Properties["message"].Description)

	branchUpdateSchema := pullRequestBranchUpdateOutputSchema()
	require.Contains(t, branchUpdateSchema.Properties, "message")
	assert.Equal(t, "Operation result message.", branchUpdateSchema.Properties["message"].Description)
}

func TestConsolidatedPullRequestOutputTypesAreCurated(t *testing.T) {
	visited := make(map[reflect.Type]bool)
	var check func(reflect.Type)
	check = func(typ reflect.Type) {
		if visited[typ] {
			return
		}
		visited[typ] = true
		require.NotContains(t, typ.PkgPath(), "github.com/google/go-github", typ.String())
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			check(typ.Elem())
		case reflect.Struct:
			for field := range typ.Fields() {
				check(field.Type)
			}
		case reflect.Map, reflect.Interface:
			t.Errorf("non-concrete output field: %s", typ)
		}
	}
	for _, typ := range []reflect.Type{
		reflect.TypeFor[PullRequestReadOutput](), reflect.TypeFor[PullRequestWriteOutput](),
		reflect.TypeFor[PullRequestMergeOutput](), reflect.TypeFor[PullRequestBranchUpdateOutput](),
		reflect.TypeFor[PullRequestCommentReplyOutput](),
	} {
		check(typ)
	}
}
