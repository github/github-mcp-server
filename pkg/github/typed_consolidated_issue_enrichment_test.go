package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedIssueReadIFCLabels(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		t.Run("protocol="+protocol, func(t *testing.T) {
			deps := consolidatedIssueDeps(t)
			deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
			session, schemas := consolidatedIssueSession(t, deps, protocol, false)
			for _, method := range []string{"get", "get_comments", "get_sub_issues", "get_parent", "get_labels"} {
				args := map[string]any{"method": method, "owner": "owner", "repo": "repo", "issue_number": 1}
				if method == "get_sub_issues" {
					args["page"], args["perPage"] = 2, 5
				}
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "issue_read", Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError)
				label := unmarshalIFC(t, result.Meta["ifc"])
				assert.Equal(t, "untrusted", label["integrity"])
				assert.Equal(t, "public", label["confidentiality"])
				if schema := schemas["issue_read"]; schema != nil {
					require.NotNil(t, result.StructuredContent)
					require.NoError(t, schema.Validate(result.StructuredContent))
				} else {
					assert.Nil(t, result.StructuredContent)
				}
			}
		})
	}
}

func TestTypedIssueReadEnrichmentAndLockdown(t *testing.T) {
	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", ""} {
		for _, lockdown := range []bool{false, true} {
			t.Run(fmt.Sprintf("protocol=%s/lockdown=%t", protocol, lockdown), func(t *testing.T) {
				issue := &github.Issue{
					Number: new(1), NodeID: new("I_1"), Title: new("Child\u202e"),
					State: new("open"), User: &github.User{Login: new("author")},
					IssueFieldValues: []*github.IssueFieldValue{{IssueFieldID: 1, Value: "must not leak REST fields"}},
				}
				parent := map[string]any{
					"number": 7, "title": "Parent\u202e", "state": "OPEN", "url": "parent",
					"author":     map[string]any{"login": "unsafe"},
					"repository": map[string]any{"nameWithOwner": "owner/repo"},
				}
				closing := map[string]any{
					"number": 8, "title": "Closing\u202e", "state": "OPEN", "url": "closing",
					"author":     map[string]any{"login": "unsafe"},
					"repository": map[string]any{"nameWithOwner": "owner/repo"},
				}
				matcher := newIssueReadEnrichmentMatcher("I_1", githubv4mock.DataResponse(map[string]any{
					"nodes": []map[string]any{{
						"id": "I_1", "issueFieldValues": map[string]any{"nodes": []any{}},
						"parent": parent, "subIssuesSummary": map[string]any{"total": 2, "completed": 1, "percentCompleted": 50},
						"closedByPullRequestsReferences": map[string]any{"totalCount": 1, "nodes": []any{closing}},
					}},
				}))
				rest := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, issue),
				})
				deps := BaseDeps{
					Client: mustNewGHClient(t, rest), GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher)),
					RepoAccessCache: stubRepoAccessCache(mockRESTPermissionServer(t, "read", map[string]string{"author": "write"}), time.Minute),
					Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": lockdown}),
				}
				session, schemas := consolidatedIssueSession(t, deps, protocol, false)
				text := `{"number":1,"title":"Child","state":"open","user":{"login":"author"},"assignees":[],"has_parent":true,"has_children":true,`
				if !lockdown {
					text += `"parent":{"number":7,"title":"Parent","state":"OPEN","url":"parent","repository":"owner/repo"},`
				}
				text += `"sub_issues_summary":{"total":2,"completed":1,"percent_completed":50},"closed_by_pull_requests":{"total_count":1,"references":[`
				if !lockdown {
					text += `{"number":8,"title":"Closing","state":"OPEN","url":"closing","repository":"owner/repo"}`
				}
				text += `]}}`
				result := assertConsolidatedResult(t, session, schemas, "issue_read", map[string]any{
					"method": "get", "owner": "owner", "repo": "repo", "issue_number": 1,
				}, text)
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					assert.JSONEq(t, `{"method":"get","issue":`+text+`}`, mustMarshalJSON(t, result.StructuredContent))
				}
			})
		}
	}
}
