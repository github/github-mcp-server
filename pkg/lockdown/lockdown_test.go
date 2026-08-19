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

	cache, transport := newMockRepoAccessCache(t, time.Minute)
	start := time.Now()
	cache.now = func() time.Time { return start }

	info, err := cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.False(t, info.IsPrivate)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 1, transport.CallCount())

	cache.now = func() time.Time { return start.Add(2 * time.Minute) }

	info, err = cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.False(t, info.IsPrivate)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 2, transport.CallCount())
}

// TestRepoAccessCacheBoundedExpiryIgnoresRepeatedAccess is a regression test for
// issue #3107: a sliding-expiry cache extends an entry's life on every read, so
// a frequently-accessed entry never refreshes even once revoked access should
// have invalidated it. With bounded expiry, an entry's maximum age is measured
// from its original creation, not its last access, so repeated reads within
// the TTL are served from cache but the entry is still forced to refresh once
// its absolute age exceeds the TTL.
func TestRepoAccessCacheBoundedExpiryIgnoresRepeatedAccess(t *testing.T) {
	ctx := t.Context()

	const ttl = 100 * time.Second
	cache, transport := newMockRepoAccessCache(t, ttl)
	current := time.Now()
	cache.now = func() time.Time { return current }

	info, err := cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 1, transport.CallCount())

	// Repeatedly access the entry well within the window. A sliding-expiry
	// cache would extend the entry's life on every one of these reads and
	// never refresh it; bounded expiry must keep serving it from cache
	// without making new upstream calls, since the absolute age is still
	// under the TTL.
	for range 4 {
		current = current.Add(20 * time.Second)
		_, err = cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, transport.CallCount(), "repeated access within the bounded window must still be served from cache")

	// Cross the bound: total elapsed time since creation now exceeds the TTL,
	// even though every individual access happened well inside it.
	current = current.Add(30 * time.Second)
	info, err = cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 2, transport.CallCount(), "entry must refresh once its absolute age exceeds the TTL, regardless of access frequency")
}

// TestRepoAccessCacheNewUserDoesNotResetEntryAge ensures that learning about a
// newly-seen author on an existing repo entry does not reset the entry's
// bounded creation time, which would otherwise re-introduce sliding behavior
// through a different code path.
func TestRepoAccessCacheNewUserDoesNotResetEntryAge(t *testing.T) {
	ctx := t.Context()

	const ttl = 100 * time.Second
	gqlClient, transport := newMockGQLClient(testUser, false)
	restClient := newMockRESTServer(t, "write")
	cache := NewRepoAccessCache(gqlClient, restClient, WithTTL(ttl), WithCacheName(t.Name()))

	start := time.Now()
	cache.now = func() time.Time { return start }

	_, err := cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, transport.CallCount())

	// A different, previously-unseen user triggers a "known users" miss but
	// not a full entry miss, exercising the path that preserves createdAt.
	cache.now = func() time.Time { return start.Add(50 * time.Second) }
	_, err = cache.getRepoAccessInfo(ctx, "someone-else", testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, transport.CallCount(), "checking a new user against a cached repo entry must not re-query repo metadata")

	// Total elapsed time since the entry's original creation now exceeds the
	// TTL. If the new-user update above had reset createdAt, this would still
	// be considered fresh (50s < 100s from the reset point); it must not be.
	cache.now = func() time.Time { return start.Add(120 * time.Second) }
	_, err = cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 2, transport.CallCount(), "entry age must be bounded from its original creation, not reset by learning about a new user")
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

func TestCacheNameForIdentity(t *testing.T) {
	t.Run("deterministic for the same identity", func(t *testing.T) {
		require.Equal(t, CacheNameForIdentity("token-a"), CacheNameForIdentity("token-a"))
	})

	t.Run("distinct for different identities", func(t *testing.T) {
		require.NotEqual(t, CacheNameForIdentity("token-a"), CacheNameForIdentity("token-b"))
	})

	t.Run("empty identity yields empty name", func(t *testing.T) {
		require.Empty(t, CacheNameForIdentity(""))
	})

	t.Run("never contains the raw identity", func(t *testing.T) {
		name := CacheNameForIdentity("super-secret-token")
		require.NotContains(t, name, "super-secret-token")
	})
}

// TestRepoAccessCacheIdentityScopedNamesPreventCrossIdentityLeakage is a
// regression test for issue #3107. It mirrors how the HTTP server must
// construct a RepoAccessCache per request: reusing the same
// lockdown.RepoAccessOption slice across requests but scoping the cache table
// name to CacheNameForIdentity(token). Two different identities querying the
// same owner/repo/author must each hit their own upstream clients rather than
// one being served from the other's cached decision.
func TestRepoAccessCacheIdentityScopedNamesPreventCrossIdentityLeakage(t *testing.T) {
	ctx := t.Context()

	restClient := newMockRESTServer(t, "write")

	aliceGQL, aliceTransport := newMockGQLClient("alice", true)
	aliceCache := NewRepoAccessCache(aliceGQL, restClient, WithCacheName(CacheNameForIdentity("token-alice")))
	_, err := aliceCache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, aliceTransport.CallCount())

	bobGQL, bobTransport := newMockGQLClient("bob", true)
	bobCache := NewRepoAccessCache(bobGQL, restClient, WithCacheName(CacheNameForIdentity("token-bob")))
	_, err = bobCache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, bobTransport.CallCount(), "a different identity must fetch its own trust decision, not reuse another identity's cached entry")

	// The same identity repeating a request must still hit the warm cache.
	aliceCacheAgain := NewRepoAccessCache(aliceGQL, restClient, WithCacheName(CacheNameForIdentity("token-alice")))
	_, err = aliceCacheAgain.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, aliceTransport.CallCount(), "repeated requests from the same identity should reuse the warm cache")
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
