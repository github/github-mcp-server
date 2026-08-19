package lockdown

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/muesli/cache2go"
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

// TestRepoAccessCacheIdentityScopedKeys covers the key derivation that keeps
// identities isolated inside a single shared cache table.
func TestRepoAccessCacheIdentityScopedKeys(t *testing.T) {
	restClient := newMockRESTServer(t, "write")
	gqlClient, _ := newMockGQLClient(testUser, false)

	newCache := func(opts ...RepoAccessOption) *RepoAccessCache {
		return NewRepoAccessCache(gqlClient, restClient, opts...)
	}

	unscoped := newCache().cacheKey(testOwner, testRepo)
	alice := newCache(WithIdentity("token-alice")).cacheKey(testOwner, testRepo)
	aliceAgain := newCache(WithIdentity("token-alice")).cacheKey(testOwner, testRepo)
	bob := newCache(WithIdentity("token-bob")).cacheKey(testOwner, testRepo)

	require.Equal(t, alice, aliceAgain, "the same identity must map to the same key so it keeps a warm cache")
	require.NotEqual(t, alice, bob, "different identities must map to different keys")
	require.NotEqual(t, alice, unscoped, "a scoped identity must not collide with unscoped entries")
	require.NotContains(t, alice, "token-alice", "the raw identity must never appear in a cache key")

	require.Equal(t, unscoped, newCache(WithIdentity("")).cacheKey(testOwner, testRepo),
		"an empty identity must leave entries unscoped")
	require.Equal(t, alice, newCache(WithIdentity("token-alice")).cacheKey(strings.ToUpper(testOwner), strings.ToUpper(testRepo)),
		"identity scoping must preserve owner/repo case-insensitivity")
}

// TestRepoAccessCacheIdentityScopingIsolatesWithinOneTable is a regression
// test for issue #3107. Isolating identities by allocating a cache2go table
// per token grows a process-wide registry that is never reclaimed, so
// isolation must instead come from the entry key inside a single table. This
// asserts both halves: different identities cannot see each other's trust
// decisions, and their entries share one table so ordinary TTL cleanup can
// reclaim them.
func TestRepoAccessCacheIdentityScopingIsolatesWithinOneTable(t *testing.T) {
	ctx := t.Context()

	restClient := newMockRESTServer(t, "write")
	table := cache2go.Cache(t.Name())
	t.Cleanup(table.Flush)

	newCache := func(gqlClient *githubv4.Client, identity string) *RepoAccessCache {
		return NewRepoAccessCache(gqlClient, restClient, WithCacheName(t.Name()), WithIdentity(identity))
	}

	aliceGQL, aliceTransport := newMockGQLClient("alice", true)
	_, err := newCache(aliceGQL, "token-alice").getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, aliceTransport.CallCount())

	bobGQL, bobTransport := newMockGQLClient("bob", true)
	_, err = newCache(bobGQL, "token-bob").getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, bobTransport.CallCount(),
		"a different identity must fetch its own trust decision, not reuse another identity's cached entry")

	require.EqualValues(t, 2, table.Count(),
		"per-identity entries must be stored in one shared table rather than a table per identity")

	// Repeating the same identity must hit the warm cache and must not
	// allocate additional storage.
	_, err = newCache(aliceGQL, "token-alice").getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, aliceTransport.CallCount(), "repeated requests from the same identity should reuse the warm cache")
	require.EqualValues(t, 2, table.Count(), "a repeated request from a known identity must not add another entry")
}

// TestRepoAccessCacheIdentityScopedEntriesAreReclaimed proves the storage held
// for distinct identities is bounded: because identity scoping lives in the
// entry key, per-identity state is removed by the cache table's ordinary TTL
// cleanup. A table-per-identity design could not shrink this way, since
// cache2go retains every named table for the life of the process.
func TestRepoAccessCacheIdentityScopedEntriesAreReclaimed(t *testing.T) {
	ctx := t.Context()

	restClient := newMockRESTServer(t, "write")
	table := cache2go.Cache(t.Name())
	t.Cleanup(table.Flush)

	identities := []string{"token-a", "token-b", "token-c"}
	for _, identity := range identities {
		gqlClient, _ := newMockGQLClient(testUser, false)
		cache := NewRepoAccessCache(gqlClient, restClient,
			WithCacheName(t.Name()),
			WithIdentity(identity),
			WithTTL(500*time.Millisecond),
		)
		_, err := cache.getRepoAccessInfo(ctx, testUser, testOwner, testRepo)
		require.NoError(t, err)
	}

	require.EqualValues(t, len(identities), table.Count(), "each identity should hold exactly one entry in the shared table")

	require.Eventually(t, func() bool { return table.Count() == 0 }, 30*time.Second, 10*time.Millisecond,
		"per-identity entries must be reclaimed by ordinary TTL cleanup so cache storage stays bounded")
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
