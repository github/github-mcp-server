package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_PullRequestStackRead_ToolDefinition(t *testing.T) {
	serverTool := PullRequestStackRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_stack_read", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.ElementsMatch(t, []string{"method", "owner", "repo"}, schema.Required)
	assert.ElementsMatch(t, []any{"get", "list"}, schema.Properties["method"].Enum)
	assert.Contains(t, schema.Properties, "stackNumber")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.True(t, serverTool.ScopeAccess.Visible(nil))
}

func Test_PullRequestStackWrite_ToolDefinition(t *testing.T) {
	serverTool := PullRequestStackWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_stack_write", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.True(t, *tool.Annotations.DestructiveHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.ElementsMatch(t, []string{"method", "owner", "repo"}, schema.Required)
	assert.ElementsMatch(t, []any{"create", "add", "unstack"}, schema.Properties["method"].Enum)
	assert.Contains(t, schema.Properties, "stackNumber")
	assert.Contains(t, schema.Properties, "pullNumbers")
	assert.Equal(t, 1, *schema.Properties["pullNumbers"].MinItems)
	assert.Equal(t, 100, *schema.Properties["pullNumbers"].MaxItems)
	assert.True(t, serverTool.ScopeAccess.Visible([]string{"public_repo"}))
	assert.False(t, serverTool.ScopeAccess.Visible(nil))
}

func Test_PullRequestStackRead_Get(t *testing.T) {
	client := newPullRequestStackTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/repos/owner/repo/stacks/42", r.URL.Path)
		assert.Equal(t, pullRequestStacksAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
		writePullRequestStack(t, w, http.StatusOK)
	})

	result := callPullRequestStackTool(t, PullRequestStackRead, client, map[string]any{
		"method":      "get",
		"owner":       "owner",
		"repo":        "repo",
		"stackNumber": float64(42),
	})

	assert.False(t, result.IsError)
	text := getTextResult(t, result).Text
	assert.Contains(t, text, `"number":42`)
	assert.Contains(t, text, `"base":{"ref":"main"}`)
	assert.Contains(t, text, `"head":{"ref":"feature","sha":"abc123"`)
	assert.NotContains(t, text, `"title"`)
	assert.NotContains(t, text, `"user"`)
}

func Test_PullRequestStackRead_List(t *testing.T) {
	client := newPullRequestStackTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/repos/owner/repo/stacks", r.URL.Path)
		assert.Equal(t, "17", r.URL.Query().Get("pull_request"))
		assert.Equal(t, "2", r.URL.Query().Get("page"))
		assert.Equal(t, "25", r.URL.Query().Get("per_page"))
		assert.Equal(t, pullRequestStacksAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
		w.Header().Set("Link", `<https://api.github.test/repositories/1/stacks?page=3>; rel="next"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode([]PullRequestStack{testPullRequestStack()}))
	})

	result := callPullRequestStackTool(t, PullRequestStackRead, client, map[string]any{
		"method":     "list",
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(17),
		"page":       float64(2),
		"perPage":    float64(25),
	})

	assert.False(t, result.IsError)
	text := getTextResult(t, result).Text
	assert.Contains(t, text, `"stacks":[{"id":9876543`)
	assert.Contains(t, text, `"pageInfo":{"hasNextPage":true,"nextPage":3}`)
}

func Test_PullRequestStackWrite_Create(t *testing.T) {
	client := newPullRequestStackTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/repos/owner/repo/stacks", r.URL.Path)
		assert.Equal(t, pullRequestStacksAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
		var input pullRequestStackInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
		assert.Equal(t, []int{101, 102}, input.PullRequests)
		writePullRequestStack(t, w, http.StatusCreated)
	})

	result := callPullRequestStackTool(t, PullRequestStackWrite, client, map[string]any{
		"method":      "create",
		"owner":       "owner",
		"repo":        "repo",
		"pullNumbers": []any{float64(101), "102"},
	})

	assert.False(t, result.IsError)
	assert.Contains(t, getTextResult(t, result).Text, `"number":42`)
}

func Test_PullRequestStackWrite_Add(t *testing.T) {
	client := newPullRequestStackTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/repos/owner/repo/stacks/42/add", r.URL.Path)
		var input pullRequestStackInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
		assert.Equal(t, []int{103}, input.PullRequests)
		writePullRequestStack(t, w, http.StatusOK)
	})

	result := callPullRequestStackTool(t, PullRequestStackWrite, client, map[string]any{
		"method":      "add",
		"owner":       "owner",
		"repo":        "repo",
		"stackNumber": float64(42),
		"pullNumbers": []int{103},
	})

	assert.False(t, result.IsError)
}

func Test_PullRequestStackWrite_Unstack(t *testing.T) {
	t.Run("remaining locked pull requests", func(t *testing.T) {
		client := newPullRequestStackTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/repos/owner/repo/stacks/42/unstack", r.URL.Path)
			writePullRequestStack(t, w, http.StatusOK)
		})

		result := callPullRequestStackTool(t, PullRequestStackWrite, client, map[string]any{
			"method":      "unstack",
			"owner":       "owner",
			"repo":        "repo",
			"stackNumber": float64(42),
		})

		assert.False(t, result.IsError)
		text := getTextResult(t, result).Text
		assert.Contains(t, text, `"dissolved":false`)
		assert.Contains(t, text, `"stack":{"id":9876543`)
	})

	t.Run("dissolved stack", func(t *testing.T) {
		client := newPullRequestStackTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/repos/owner/repo/stacks/42/unstack", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		})

		result := callPullRequestStackTool(t, PullRequestStackWrite, client, map[string]any{
			"method":      "unstack",
			"owner":       "owner",
			"repo":        "repo",
			"stackNumber": float64(42),
		})

		assert.False(t, result.IsError)
		assert.JSONEq(t, `{"dissolved":true,"stack_number":42}`, getTextResult(t, result).Text)
	})
}

func Test_PullRequestStackTool_Validation(t *testing.T) {
	client := newPullRequestStackTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("validation should fail before making a request")
	})

	tests := []struct {
		name string
		tool func(translations.TranslationHelperFunc) inventory.ServerTool
		args map[string]any
		want string
	}{
		{
			name: "get requires stack number",
			tool: PullRequestStackRead,
			args: map[string]any{"method": "get", "owner": "owner", "repo": "repo"},
			want: "missing required parameter: stackNumber",
		},
		{
			name: "get rejects negative stack number",
			tool: PullRequestStackRead,
			args: map[string]any{"method": "get", "owner": "owner", "repo": "repo", "stackNumber": float64(-1)},
			want: "parameter stackNumber must be greater than zero",
		},
		{
			name: "list rejects zero pull number",
			tool: PullRequestStackRead,
			args: map[string]any{"method": "list", "owner": "owner", "repo": "repo", "pullNumber": float64(0)},
			want: "parameter pullNumber must be greater than zero",
		},
		{
			name: "create requires two pull requests",
			tool: PullRequestStackWrite,
			args: map[string]any{"method": "create", "owner": "owner", "repo": "repo", "pullNumbers": []any{float64(1)}},
			want: "method create requires at least two pullNumbers",
		},
		{
			name: "add requires pull requests",
			tool: PullRequestStackWrite,
			args: map[string]any{"method": "add", "owner": "owner", "repo": "repo", "stackNumber": float64(1)},
			want: "method add requires at least one pullNumber",
		},
		{
			name: "duplicate pull request",
			tool: PullRequestStackWrite,
			args: map[string]any{"method": "create", "owner": "owner", "repo": "repo", "pullNumbers": []any{float64(1), float64(1)}},
			want: "duplicates pull request 1",
		},
		{
			name: "invalid pull request",
			tool: PullRequestStackWrite,
			args: map[string]any{"method": "create", "owner": "owner", "repo": "repo", "pullNumbers": []any{float64(1), float64(-2)}},
			want: "must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := callPullRequestStackTool(t, tt.tool, client, tt.args)
			assert.True(t, result.IsError)
			assert.Contains(t, getTextResult(t, result).Text, tt.want)
		})
	}
}

func Test_PullRequestStackTool_APIError(t *testing.T) {
	client := newPullRequestStackTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Pull requests must form a stack"}`))
	})

	result := callPullRequestStackTool(t, PullRequestStackWrite, client, map[string]any{
		"method":      "create",
		"owner":       "owner",
		"repo":        "repo",
		"pullNumbers": []any{float64(101), float64(102)},
	})

	assert.True(t, result.IsError)
	assert.Contains(t, getTextResult(t, result).Text, "failed to create pull request stack")
	assert.Contains(t, getTextResult(t, result).Text, "Pull requests must form a stack")
}

func callPullRequestStackTool(
	t *testing.T,
	tool func(translations.TranslationHelperFunc) inventory.ServerTool,
	client *github.Client,
	args map[string]any,
) *mcp.CallToolResult {
	t.Helper()
	deps := BaseDeps{Client: client}
	request := createMCPRequest(args)
	serverTool := tool(translations.NullTranslationHelper)
	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func newPullRequestStackTestClient(t *testing.T, handler http.HandlerFunc) *github.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseURL := server.URL + "/"
	client, err := github.NewClient(
		github.WithHTTPClient(server.Client()),
		github.WithURLs(&baseURL, nil),
	)
	require.NoError(t, err)
	return client
}

func writePullRequestStack(t *testing.T, w http.ResponseWriter, status int) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(testPullRequestStack()))
}

func testPullRequestStack() PullRequestStack {
	mergedAt := "2026-08-01T12:00:00Z"
	return PullRequestStack{
		ID:        9876543,
		Number:    42,
		NodeID:    "S_kwDOABCDEF4AAAAA",
		URL:       "https://api.github.test/repos/owner/repo/stacks/42",
		Base:      PullRequestStackRef{Ref: "main"},
		Open:      true,
		CreatedAt: "2026-08-01T10:00:00Z",
		PullRequests: []PullRequestStackPullRequest{
			{
				ID:       100001,
				NodeID:   "PR_kwDOABCDEF4AAAAA",
				Number:   101,
				URL:      "https://api.github.test/repos/owner/repo/pulls/101",
				HTMLURL:  "https://github.test/owner/repo/pull/101",
				State:    "closed",
				MergedAt: &mergedAt,
				Draft:    false,
				Head: PullRequestStackRef{
					Ref: "feature",
					SHA: "abc123",
					Repo: &PullRequestStackRepository{
						ID:   1,
						Name: "repo",
						URL:  "https://api.github.test/repos/owner/repo",
					},
				},
				Base: &PullRequestStackRef{
					Ref: "main",
					SHA: "def456",
					Repo: &PullRequestStackRepository{
						ID:   1,
						Name: "repo",
						URL:  "https://api.github.test/repos/owner/repo",
					},
				},
			},
		},
	}
}
