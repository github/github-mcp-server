package githubapp

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOwnerFromRequestRESTPaths(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "repo", url: "https://api.github.com/repos/octo-org/repo", want: "octo-org"},
		{name: "repo sub-resource", url: "https://api.github.com/repos/octo-org/repo/issues/1/comments", want: "octo-org"},
		{name: "org", url: "https://api.github.com/orgs/octo-org/repos", want: "octo-org"},
		{name: "user", url: "https://api.github.com/users/octocat/repos", want: "octocat"},
		{name: "GHES prefix", url: "https://ghes.example.com/api/v3/repos/octo-org/repo", want: "octo-org"},
		{name: "repo named like a keyword", url: "https://api.github.com/repos/octo-org/orgs/contents", want: "octo-org"},
		{name: "query string ignored", url: "https://api.github.com/orgs/octo-org/repos?per_page=100", want: "octo-org"},
		{name: "not owner scoped", url: "https://api.github.com/rate_limit", want: ""},
		{name: "authenticated user", url: "https://api.github.com/user", want: ""},
		{name: "repository by ID", url: "https://api.github.com/repositories/1300192", want: ""},
		{name: "truncated path", url: "https://api.github.com/repos", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, OwnerFromRequest(req))
		})
	}
}

func TestOwnerFromRequestGraphQL(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "owner variable", body: `{"variables":{"owner":"octo-org","name":"repo"}}`, want: "octo-org"},
		{name: "login variable", body: `{"variables":{"login":"octo-org"}}`, want: "octo-org"},
		{name: "owner preferred over login", body: `{"variables":{"login":"other-org","owner":"octo-org"}}`, want: "octo-org"},
		{name: "no owner variable", body: `{"variables":{"first":10}}`, want: ""},
		{name: "no variables", body: `{"query":"query{viewer{login}}"}`, want: ""},
		{name: "non-string owner", body: `{"variables":{"owner":42}}`, want: ""},
		{name: "malformed body", body: `not json`, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "https://api.github.com/graphql", strings.NewReader(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.want, OwnerFromRequest(req))
		})
	}
}

// TestOwnerFromRequestGraphQLLeavesBodyIntact guards the transport contract:
// inspecting the body must not consume it before the request is sent.
func TestOwnerFromRequestGraphQLLeavesBodyIntact(t *testing.T) {
	body := `{"variables":{"owner":"octo-org"}}`
	req, err := http.NewRequest(http.MethodPost, "https://api.github.com/graphql", strings.NewReader(body))
	require.NoError(t, err)

	require.Equal(t, "octo-org", OwnerFromRequest(req))

	buf := make([]byte, len(body))
	n, _ := req.Body.Read(buf)
	assert.Equal(t, body, string(buf[:n]))
}

func TestOwnerFromRequestNilSafe(t *testing.T) {
	assert.Empty(t, OwnerFromRequest(nil))
	assert.Empty(t, OwnerFromRequest(&http.Request{}))
}
