package lockdown

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"
)

const (
	testOwner = "octo-org"
	testRepo  = "octo-repo"
	testUser  = "octocat"
)

type repoMetadataQuery struct {
	Viewer struct {
		Login githubv4.String
	}
	Repository struct {
		IsPrivate githubv4.Boolean
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

	var query repoMetadataQuery

	variables := map[string]any{
		"owner": githubv4.String(testOwner),
		"name":  githubv4.String(testRepo),
	}

	response := githubv4mock.DataResponse(map[string]any{
		"viewer": map[string]any{
			"login": testUser,
		},
		"repository": map[string]any{
			"isPrivate": false,
		},
	})

	httpClient := githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(query, variables, response))
	counting := &countingTransport{next: httpClient.Transport}
	httpClient.Transport = counting
	gqlClient := githubv4.NewClient(httpClient)

	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := gogithub.RepositoryPermissionLevel{
			Permission: gogithub.Ptr("write"),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(restServer.Close)
	restClient := gogithub.NewClient(nil)
	restClient.BaseURL, _ = url.Parse(restServer.URL + "/")

	return NewRepoAccessCache(gqlClient, restClient, WithTTL(ttl)), counting
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
