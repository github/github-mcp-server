package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover the success (API-calling) branches of the repository
// resource completion resolvers. The existing tests only exercise the
// missing-dependency error paths, leaving the actual completion logic
// (filtering, search fallbacks, tree walking) uncovered.

func newCompletionClient(t *testing.T, handlers map[string]http.HandlerFunc) *github.Client {
	t.Helper()
	return github.NewClient(MockHTTPClientWithHandlers(handlers))
}

func TestCompleteOwner(t *testing.T) {
	t.Run("returns viewer and orgs without search when no filter", func(t *testing.T) {
		client := newCompletionClient(t, map[string]http.HandlerFunc{
			"GET /user": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"login":"octocat"}`))
			},
			"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`[{"login":"github"}]`))
			},
		})

		values, err := completeOwner(context.Background(), client, nil, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"octocat", "github"}, values)
	})

	t.Run("filters by argValue and falls back to user search", func(t *testing.T) {
		client := newCompletionClient(t, map[string]http.HandlerFunc{
			"GET /user": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"login":"octocat"}`))
			},
			"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`[{"login":"github"}]`))
			},
			"GET /search/users": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"total_count":1,"items":[{"login":"octo-search"}]}`))
			},
		})

		values, err := completeOwner(context.Background(), client, nil, "oct")
		require.NoError(t, err)
		// "octocat" matches the filter; "github" does not; search adds "octo-search".
		assert.Equal(t, []string{"octocat", "octo-search"}, values)
	})

	t.Run("returns error when org listing fails", func(t *testing.T) {
		client := newCompletionClient(t, map[string]http.HandlerFunc{
			"GET /user": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"login":"octocat"}`))
			},
			"GET /user/orgs": notFoundHandler("orgs unavailable"),
		})

		_, err := completeOwner(context.Background(), client, nil, "")
		require.Error(t, err)
	})
}

func TestCompleteRepo_Success(t *testing.T) {
	client := newCompletionClient(t, map[string]http.HandlerFunc{
		"GET /search/repositories": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"total_count":2,"items":[{"name":"repo-alpha"},{"name":"beta"}]}`))
		},
	})

	t.Run("no filter returns all repos", func(t *testing.T) {
		values, err := completeRepo(context.Background(), client, map[string]string{"owner": "acme"}, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"repo-alpha", "beta"}, values)
	})

	t.Run("prefix filter narrows results", func(t *testing.T) {
		values, err := completeRepo(context.Background(), client, map[string]string{"owner": "acme"}, "repo")
		require.NoError(t, err)
		assert.Equal(t, []string{"repo-alpha"}, values)
	})
}

func TestCompleteBranch_Success(t *testing.T) {
	client := newCompletionClient(t, map[string]http.HandlerFunc{
		"GET /repos/acme/widget/branches": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[{"name":"main"},{"name":"feature-x"}]`))
		},
	})
	resolved := map[string]string{"owner": "acme", "repo": "widget"}

	all, err := completeBranch(context.Background(), client, resolved, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"main", "feature-x"}, all)

	filtered, err := completeBranch(context.Background(), client, resolved, "feat")
	require.NoError(t, err)
	assert.Equal(t, []string{"feature-x"}, filtered)
}

func TestCompleteSHA_Success(t *testing.T) {
	client := newCompletionClient(t, map[string]http.HandlerFunc{
		"GET /repos/acme/widget/commits": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[{"sha":"abc123"},{"sha":"def456"}]`))
		},
	})
	resolved := map[string]string{"owner": "acme", "repo": "widget"}

	all, err := completeSHA(context.Background(), client, resolved, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"abc123", "def456"}, all)

	filtered, err := completeSHA(context.Background(), client, resolved, "abc")
	require.NoError(t, err)
	assert.Equal(t, []string{"abc123"}, filtered)
}

func TestCompleteTag_Success(t *testing.T) {
	client := newCompletionClient(t, map[string]http.HandlerFunc{
		"GET /repos/acme/widget/tags": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[{"name":"v1.0.0"},{"name":"v2.0.0"}]`))
		},
	})
	resolved := map[string]string{"owner": "acme", "repo": "widget"}

	all, err := completeTag(context.Background(), client, resolved, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.0.0", "v2.0.0"}, all)

	// completeTag uses Contains, so "2.0" matches the v2 tag anywhere in the name.
	filtered, err := completeTag(context.Background(), client, resolved, "2.0")
	require.NoError(t, err)
	assert.Equal(t, []string{"v2.0.0"}, filtered)
}

func TestCompletePRNumber_Success(t *testing.T) {
	client := newCompletionClient(t, map[string]http.HandlerFunc{
		"GET /search/issues": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"total_count":2,"items":[{"number":42},{"number":7}]}`))
		},
	})
	resolved := map[string]string{"owner": "acme", "repo": "widget"}

	all, err := completePRNumber(context.Background(), client, resolved, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"42", "7"}, all)

	filtered, err := completePRNumber(context.Background(), client, resolved, "4")
	require.NoError(t, err)
	assert.Equal(t, []string{"42"}, filtered)
}

func TestCompletePath_Success(t *testing.T) {
	treeJSON := `{
		"sha": "HEAD",
		"tree": [
			{"path": "README.md", "type": "blob"},
			{"path": "src", "type": "tree"},
			{"path": "src/main.go", "type": "blob"},
			{"path": "src/util.go", "type": "blob"}
		]
	}`
	client := newCompletionClient(t, map[string]http.HandlerFunc{
		"GET /repos/acme/widget/git/trees/HEAD": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(treeJSON))
		},
	})
	resolved := map[string]string{"owner": "acme", "repo": "widget"}

	t.Run("lists immediate children at root", func(t *testing.T) {
		values, err := completePath(context.Background(), client, resolved, "")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"README.md", "src/"}, values)
	})

	t.Run("lists children inside a directory prefix", func(t *testing.T) {
		values, err := completePath(context.Background(), client, resolved, "src/")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"src/main.go", "src/util.go"}, values)
	})

	t.Run("filters by partial last segment", func(t *testing.T) {
		values, err := completePath(context.Background(), client, resolved, "RE")
		require.NoError(t, err)
		assert.Equal(t, []string{"README.md"}, values)
	})

	t.Run("returns error when tree fetch fails", func(t *testing.T) {
		failClient := newCompletionClient(t, map[string]http.HandlerFunc{
			"GET /repos/acme/widget/git/trees/HEAD": notFoundHandler("tree not found"),
		})
		_, err := completePath(context.Background(), failClient, resolved, "")
		require.Error(t, err)
	})
}
