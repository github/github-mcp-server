package lockdown

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"
)

const (
	testOwner = "octo-org"
	testRepo  = "octo-repo"
	testUser  = "octocat"
)

type repoAccessQuery struct {
	Viewer struct {
		Login githubv4.String
	}
	Repository struct {
		IsPrivate     githubv4.Boolean
		Collaborators struct {
			Edges []struct {
				Permission githubv4.String
				Node       struct {
					Login githubv4.String
				}
			}
		} `graphql:"collaborators(query: $username, first: 1)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

type countingTransport struct {
	mu    sync.Mutex
	next  http.RoundTripper
	calls int
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return c.next.RoundTrip(req)
}

func (c *countingTransport) CallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func newMockRepoAccessCache(t *testing.T, ttl time.Duration) (*RepoAccessCache, *countingTransport) {
	t.Helper()

	var query repoAccessQuery

	variables := map[string]any{
		"owner":    githubv4.String(testOwner),
		"name":     githubv4.String(testRepo),
		"username": githubv4.String(testUser),
	}

	response := githubv4mock.DataResponse(map[string]any{
		"viewer": map[string]any{
			"login": testUser,
		},
		"repository": map[string]any{
			"isPrivate": false,
			"collaborators": map[string]any{
				"edges": []any{
					map[string]any{
						"permission": "WRITE",
						"node": map[string]any{
							"login": testUser,
						},
					},
				},
			},
		},
	})

	httpClient := githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(query, variables, response))
	counting := &countingTransport{next: httpClient.Transport}
	httpClient.Transport = counting

	gqlClient := githubv4.NewClient(httpClient)

	return GetInstance(gqlClient, WithTTL(ttl)), counting
}

func TestRepoAccessCacheEvictsAfterTTL(t *testing.T) {
	ctx := t.Context()

	cache, transport := newMockRepoAccessCache(t, 5*time.Millisecond)
	info, err := cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.Equal(t, testUser, info.ViewerLogin)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 1, transport.CallCount())

	time.Sleep(20 * time.Millisecond)

	info, err = cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.Equal(t, testUser, info.ViewerLogin)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 2, transport.CallCount())
}
