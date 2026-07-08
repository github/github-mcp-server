package lockdown

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"
)

const (
	testOwner = "octo-org"
	testRepo  = "octo-repo"
	testUser  = "octocat"
)

type viewerLoginQuery struct {
	Viewer struct {
		Login githubv4.String
	}
}

type repoAccessQuery struct {
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

func newMockGQLClient(viewerLogin string, isPrivate bool) (*githubv4.Client, *countingTransport) {
	variables := map[string]any{
		"owner": githubv4.String(testOwner),
		"name":  githubv4.String(testRepo),
	}

	httpClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			viewerLoginQuery{},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"viewer": map[string]any{"login": viewerLogin},
			}),
		),
		githubv4mock.NewQueryMatcher(
			repoAccessQuery{},
			variables,
			githubv4mock.DataResponse(map[string]any{
				"viewer":     map[string]any{"login": viewerLogin},
				"repository": map[string]any{"isPrivate": isPrivate},
			}),
		),
	)
	counting := &countingTransport{next: httpClient.Transport}
	httpClient.Transport = counting
	gqlClient := githubv4.NewClient(httpClient)
	return gqlClient, counting
}

func newMockRESTServer(t *testing.T, permission string) *gogithub.Client {
	t.Helper()
	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := gogithub.RepositoryPermissionLevel{Permission: gogithub.Ptr(permission)}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(restServer.Close)
	restClient, err := gogithub.NewClient(gogithub.WithEnterpriseURLs(restServer.URL+"/", restServer.URL+"/"))
	require.NoError(t, err)
	return restClient
}

func newMockRepoAccessCache(t *testing.T, ttl time.Duration) (*RepoAccessCache, *countingTransport) {
	t.Helper()
	gqlClient, counting := newMockGQLClient(testUser, false)
	restClient := newMockRESTServer(t, "write")
	cache := NewRepoAccessCache(
		gqlClient,
		restClient,
		WithTTL(ttl),
		WithCacheName(t.Name()),
	)
	return cache, counting
}

func TestRepoAccessCacheEvictsAfterTTL(t *testing.T) {
	ctx := t.Context()

	cache, transport := newMockRepoAccessCache(t, 5*time.Millisecond)
	info, err := cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.False(t, info.IsPrivate)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 1, transport.CallCount())

	time.Sleep(20 * time.Millisecond)

	info, err = cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.False(t, info.IsPrivate)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 2, transport.CallCount())
}

func TestRepoAccessCacheIsolatesViewerPerInstance(t *testing.T) {
	ctx := t.Context()

	cacheName := t.Name()
	restClient := newMockRESTServer(t, "read")

	attackerGQL, _ := newMockGQLClient("attacker", false)
	attackerCache := NewRepoAccessCache(attackerGQL, restClient, WithCacheName(cacheName))
	safe, err := attackerCache.IsSafeContent(ctx, "attacker", testOwner, testRepo)
	require.NoError(t, err)
	require.True(t, safe)

	victimGQL, _ := newMockGQLClient("victim", false)
	victimCache := NewRepoAccessCache(victimGQL, restClient, WithCacheName(cacheName))
	safe, err = victimCache.IsSafeContent(ctx, "attacker", testOwner, testRepo)
	require.NoError(t, err)
	require.False(t, safe, "attacker-authored content must not be safe for the victim")

	safe, err = victimCache.IsSafeContent(ctx, "victim", testOwner, testRepo)
	require.NoError(t, err)
	require.True(t, safe)
}

type flakyTransport struct {
	mu    sync.Mutex
	failN int
	calls int
	next  http.RoundTripper
}

func (f *flakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.calls++
	shouldFail := f.calls <= f.failN
	f.mu.Unlock()
	if shouldFail {
		return nil, errors.New("simulated transient failure")
	}
	return f.next.RoundTrip(req)
}

func TestRepoAccessCacheRetriesViewerLoginAfterTransientError(t *testing.T) {
	ctx := t.Context()

	httpClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			viewerLoginQuery{},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"viewer": map[string]any{"login": testUser},
			}),
		),
	)
	flaky := &flakyTransport{next: httpClient.Transport, failN: 1}
	httpClient.Transport = flaky
	gqlClient := githubv4.NewClient(httpClient)

	cache := NewRepoAccessCache(gqlClient, nil, WithCacheName(t.Name()))

	_, err := cache.viewerLoginFor(ctx)
	require.Error(t, err, "first call should surface the transient failure")

	login, err := cache.viewerLoginFor(ctx)
	require.NoError(t, err, "second call must retry, not return the cached error")
	require.Equal(t, testUser, login)
}

func TestRepoAccessCacheRejectsEmptyViewerLogin(t *testing.T) {
	ctx := t.Context()

	httpClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			viewerLoginQuery{},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"viewer": map[string]any{"login": ""},
			}),
		),
	)
	gqlClient := githubv4.NewClient(httpClient)

	cache := NewRepoAccessCache(gqlClient, nil, WithCacheName(t.Name()))

	_, err := cache.viewerLoginFor(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}
