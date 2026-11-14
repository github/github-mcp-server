package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestReadAcceptsSnakeCasePullNumber(t *testing.T) {
	tool := PullRequestRead(translations.NullTranslationHelper)
	normalized, err := tool.GetInputNormalizer()(
		json.RawMessage(`{"method":"get","owner":"owner","repo":"repo","pull_number":"42"}`),
	)
	require.NoError(t, err)

	var args map[string]any
	require.NoError(t, json.Unmarshal(normalized, &args))
	assert.Equal(t, float64(42), args["pullNumber"])
	assert.NotContains(t, args, "pull_number")
}

func TestIssueReadAcceptsCamelCaseIssueNumber(t *testing.T) {
	tool := IssueRead(translations.NullTranslationHelper)
	normalized, err := tool.GetInputNormalizer()(
		json.RawMessage(`{"method":"get","owner":"owner","repo":"repo","issueNumber":"42"}`),
	)
	require.NoError(t, err)

	var args map[string]any
	require.NoError(t, json.Unmarshal(normalized, &args))
	assert.Equal(t, float64(42), args["issue_number"])
	assert.NotContains(t, args, "issueNumber")
}

func TestTypedCallsAcceptParameterAliases(t *testing.T) {
	tests := []struct {
		name     string
		tool     func() inventory.ServerTool
		args     map[string]any
		route    string
		response any
	}{
		{
			name: "pull request snake case",
			tool: func() inventory.ServerTool { return PullRequestRead(translations.NullTranslationHelper) },
			args: map[string]any{
				"method": "get", "owner": "owner", "repo": "repo", "pull_number": 42,
			},
			route:    GetReposPullsByOwnerByRepoByPullNumber,
			response: &github.PullRequest{Number: new(42), Title: new("Test PR")},
		},
		{
			name: "issue camel case",
			tool: func() inventory.ServerTool { return IssueRead(translations.NullTranslationHelper) },
			args: map[string]any{
				"method": "get", "owner": "owner", "repo": "repo", "issueNumber": 42,
			},
			route:    GetReposIssuesByOwnerByRepoByIssueNumber,
			response: &github.Issue{Number: new(42), Title: new("Test issue")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.tool()
			deps := BaseDeps{
				Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					tc.route: mockResponse(t, http.StatusOK, tc.response),
				})),
			}
			session := connectTypedSearchClient(t, tool, deps, inventory.ProtocolVersionMultiRoundTrip)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: tool.Tool.Name, Arguments: tc.args,
			})
			require.NoError(t, err)
			assert.False(t, result.IsError, mustMarshalJSON(t, result))
		})
	}
}

func TestInputAliasesRejectConflictingValues(t *testing.T) {
	pullRequestRead := PullRequestRead(translations.NullTranslationHelper)
	issueRead := IssueRead(translations.NullTranslationHelper)
	tests := []struct {
		name       string
		normalizer func(json.RawMessage) (json.RawMessage, error)
		input      string
	}{
		{
			name:       "pull request number",
			normalizer: pullRequestRead.GetInputNormalizer(),
			input:      `{"method":"get","owner":"owner","repo":"repo","pullNumber":42,"pull_number":42}`,
		},
		{
			name:       "issue number",
			normalizer: issueRead.GetInputNormalizer(),
			input:      `{"method":"get","owner":"owner","repo":"repo","issue_number":42,"issueNumber":42}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.normalizer(json.RawMessage(tc.input))
			require.ErrorContains(t, err, "cannot both be provided")
		})
	}
}
