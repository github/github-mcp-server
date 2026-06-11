package lockdown

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/muesli/cache2go"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"
)

// These tests complement safety_test.go. They exercise the branches the
// existing suite leaves uncovered: merging a not-yet-known user into a repo
// entry that is already cached, propagating a GraphQL server-side error, and
// the debug-logging paths in (*RepoAccessCache).log.

// TestGetRepoAccessInfoMergesNewUserIntoCachedRepo covers the path where the
// repository entry is already cached but the queried user is unknown: exactly
// one GraphQL call is made and the result is merged into the existing entry,
// while a separately-cached user is served without any call.
func TestGetRepoAccessInfoMergesNewUserIntoCachedRepo(t *testing.T) {
	ctx := t.Context()
	// The mock answers only for "bob"; "alice" is pre-seeded into the cache.
	cache, transport := newSafetyTestCache(t, "bob", "viewer", false, "READ")

	cache.cache.Add(cacheKey(testOwner, testRepo), cache.ttl, &repoAccessCacheEntry{
		isPrivate:   false,
		viewerLogin: "viewer",
		knownUsers:  map[string]bool{"alice": true},
	})

	// Known user is served entirely from cache; no GraphQL call.
	info, err := cache.getRepoAccessInfo(ctx, "alice", testOwner, testRepo)
	require.NoError(t, err)
	require.True(t, info.HasPushAccess)
	require.EqualValues(t, 0, transport.CallCount())

	// Repo entry exists but the user is unknown -> exactly one query, merged in.
	info, err = cache.getRepoAccessInfo(ctx, "bob", testOwner, testRepo)
	require.NoError(t, err)
	require.False(t, info.HasPushAccess) // READ does not grant push
	require.EqualValues(t, 1, transport.CallCount())

	// The newly learned user is now cached.
	_, err = cache.getRepoAccessInfo(ctx, "bob", testOwner, testRepo)
	require.NoError(t, err)
	require.EqualValues(t, 1, transport.CallCount())
}

// TestQueryRepoAccessInfoPropagatesServerError covers the error-wrapping path
// when the GraphQL endpoint returns an error response (distinct from the
// nil-client guard already covered in safety_test.go).
func TestQueryRepoAccessInfoPropagatesServerError(t *testing.T) {
	var query repoAccessQuery
	variables := map[string]any{
		"owner":    githubv4.String(testOwner),
		"name":     githubv4.String(testRepo),
		"username": githubv4.String(testUser),
	}
	httpClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(query, variables, githubv4mock.ErrorResponse("boom")),
	)
	cache := &RepoAccessCache{
		client: githubv4.NewClient(httpClient),
		cache:  cache2go.Cache(t.Name()),
		ttl:    defaultRepoAccessTTL,
	}

	_, err := cache.queryRepoAccessInfo(t.Context(), testUser, testOwner, testRepo)
	require.Error(t, err)
}

// TestLogEmitsAndSuppressesByLevel covers the two branches of
// (*RepoAccessCache).log beyond the nil-logger early return: a message that is
// actually written, and one suppressed because the level is below threshold.
func TestLogEmitsAndSuppressesByLevel(t *testing.T) {
	t.Run("debug logger receives output", func(t *testing.T) {
		var buf bytes.Buffer
		cache, _ := newSafetyTestCache(t, "writer", "someoneelse", false, "WRITE")
		cache.SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

		_, err := cache.IsSafeContent(t.Context(), "writer", testOwner, testRepo)
		require.NoError(t, err)
		require.NotEmpty(t, buf.String(), "debug logging should produce output")
	})

	t.Run("level below threshold is suppressed", func(t *testing.T) {
		var buf bytes.Buffer
		cache := &RepoAccessCache{
			logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
		}
		cache.logDebug(t.Context(), "should be suppressed")
		require.Empty(t, buf.String())
	})
}
