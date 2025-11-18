package lockdown

import (
	"context"
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

	return NewRepoAccessCache(gqlClient, WithTTL(ttl)), counting
}

func requireAccess(ctx context.Context, t *testing.T, cache *RepoAccessCache) {
	t.Helper()

	isPrivate, hasPush, err := cache.GetRepoAccessInfo(ctx, testUser, testOwner, testRepo)
	require.NoError(t, err)
	require.False(t, isPrivate)
	require.True(t, hasPush)
}

func TestRepoAccessCacheEvictsAfterTTL(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, transport := newMockRepoAccessCache(t, 5*time.Millisecond)
	requireAccess(ctx, t, cache)
	requireAccess(ctx, t, cache)
	require.EqualValues(t, 1, transport.CallCount())

	time.Sleep(20 * time.Millisecond)

	requireAccess(ctx, t, cache)
	require.EqualValues(t, 2, transport.CallCount())
}

func TestRepoAccessCacheTTLDisabled(t *testing.T) {
	ctx := t.Context()
	t.Parallel()

	// make sure cache TTL is sufficiently large to avoid evictions during the test
	cache, transport := newMockRepoAccessCache(t, 1000*time.Millisecond)

	requireAccess(ctx, t, cache)
	requireAccess(ctx, t, cache)
	require.EqualValues(t, 1, transport.CallCount())

	requireAccess(ctx, t, cache)
	require.EqualValues(t, 1, transport.CallCount())
}

func TestRepoAccessCacheSetTTLReschedulesExistingEntry(t *testing.T) {
	ctx := t.Context()
	t.Parallel()

	cache, transport := newMockRepoAccessCache(t, 10*time.Millisecond)

	requireAccess(ctx, t, cache)
	require.EqualValues(t, 1, transport.CallCount())

	cache.SetTTL(5 * time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	requireAccess(ctx, t, cache)
	require.EqualValues(t, 2, transport.CallCount())

	requireAccess(ctx, t, cache)
	require.EqualValues(t, 2, transport.CallCount())
}

func TestGetInstanceReturnsSingleton(t *testing.T) {
	// Reset any existing singleton
	ResetInstance()
	defer ResetInstance() // Clean up after test

	gqlClient := githubv4.NewClient(nil)

	// Get instance twice, should return the same instance
	instance1 := GetInstance(gqlClient)
	instance2 := GetInstance(gqlClient)

	// Verify they're the same instance (same pointer)
	require.Same(t, instance1, instance2, "GetInstance should return the same singleton instance")

	// Verify subsequent calls with different options are ignored
	instance3 := GetInstance(gqlClient, WithTTL(1*time.Second))
	require.Same(t, instance1, instance3, "GetInstance should ignore options on subsequent calls")
	require.Equal(t, defaultRepoAccessTTL, instance3.ttl, "TTL should remain unchanged after first initialization")
}

func TestResetInstanceClearsSingleton(t *testing.T) {
	// Reset any existing singleton
	ResetInstance()
	defer ResetInstance() // Clean up after test

	gqlClient := githubv4.NewClient(nil)

	// Get first instance with default TTL
	instance1 := GetInstance(gqlClient)
	require.Equal(t, defaultRepoAccessTTL, instance1.ttl)

	// Reset the singleton
	ResetInstance()

	// Get new instance with custom TTL
	customTTL := 10 * time.Second
	instance2 := GetInstance(gqlClient, WithTTL(customTTL))
	require.NotSame(t, instance1, instance2, "After reset, GetInstance should return a new instance")
	require.Equal(t, customTTL, instance2.ttl, "New instance should have the custom TTL")
}

func TestNewRepoAccessCacheCreatesIndependentInstances(t *testing.T) {
	t.Parallel()

	gqlClient := githubv4.NewClient(nil)

	// NewRepoAccessCache should create independent instances
	cache1 := NewRepoAccessCache(gqlClient, WithTTL(1*time.Second))
	cache2 := NewRepoAccessCache(gqlClient, WithTTL(2*time.Second))

	require.NotSame(t, cache1, cache2, "NewRepoAccessCache should create different instances")
	require.Equal(t, 1*time.Second, cache1.ttl)
	require.Equal(t, 2*time.Second, cache2.ttl)
}
