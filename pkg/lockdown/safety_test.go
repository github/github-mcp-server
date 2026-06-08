package lockdown

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/muesli/cache2go"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"
)

// newSafetyTestCache builds an isolated RepoAccessCache that bypasses the
// GetInstance singleton (so tests don't share state) and whose GraphQL client
// returns a repository-access response derived from the supplied parameters.
//
// permission is the collaborator permission returned for username; an empty
// string means username is not returned as a collaborator at all.
func newSafetyTestCache(t *testing.T, username, viewerLogin string, isPrivate bool, permission string) (*RepoAccessCache, *countingTransport) {
	t.Helper()

	var query repoAccessQuery
	variables := map[string]any{
		"owner":    githubv4.String(testOwner),
		"name":     githubv4.String(testRepo),
		"username": githubv4.String(username),
	}

	edges := []any{}
	if permission != "" {
		edges = append(edges, map[string]any{
			"permission": permission,
			"node": map[string]any{
				"login": username,
			},
		})
	}

	response := githubv4mock.DataResponse(map[string]any{
		"viewer": map[string]any{
			"login": viewerLogin,
		},
		"repository": map[string]any{
			"isPrivate": isPrivate,
			"collaborators": map[string]any{
				"edges": edges,
			},
		},
	})

	httpClient := githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(query, variables, response))
	counting := &countingTransport{next: httpClient.Transport}
	httpClient.Transport = counting

	cache := &RepoAccessCache{
		client:           githubv4.NewClient(httpClient),
		cache:            cache2go.Cache(t.Name()),
		ttl:              defaultRepoAccessTTL,
		trustedBotLogins: map[string]struct{}{"copilot": {}},
	}

	return cache, counting
}

func TestIsSafeContent(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		viewerLogin string
		isPrivate   bool
		permission  string
		want        bool
	}{
		{
			name:     "trusted bot is always safe",
			username: "copilot", viewerLogin: "someoneelse", isPrivate: false, permission: "",
			want: true,
		},
		{
			name:     "private repository is safe",
			username: "stranger", viewerLogin: "someoneelse", isPrivate: true, permission: "",
			want: true,
		},
		{
			name:     "content authored by the viewer is safe",
			username: "octocat", viewerLogin: "octocat", isPrivate: false, permission: "",
			want: true,
		},
		{
			name:     "viewer match is case-insensitive",
			username: "OctoCat", viewerLogin: "octocat", isPrivate: false, permission: "",
			want: true,
		},
		{
			name:     "author with write access is safe",
			username: "writer", viewerLogin: "someoneelse", isPrivate: false, permission: "WRITE",
			want: true,
		},
		{
			name:     "author with admin access is safe",
			username: "admin", viewerLogin: "someoneelse", isPrivate: false, permission: "ADMIN",
			want: true,
		},
		{
			name:     "author with maintain access is safe",
			username: "maintainer", viewerLogin: "someoneelse", isPrivate: false, permission: "MAINTAIN",
			want: true,
		},
		{
			name:     "untrusted author with read-only access is not safe",
			username: "reader", viewerLogin: "someoneelse", isPrivate: false, permission: "READ",
			want: false,
		},
		{
			name:     "untrusted non-collaborator is not safe",
			username: "stranger", viewerLogin: "someoneelse", isPrivate: false, permission: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, _ := newSafetyTestCache(t, tt.username, tt.viewerLogin, tt.isPrivate, tt.permission)

			got, err := cache.IsSafeContent(t.Context(), tt.username, testOwner, testRepo)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsSafeContentCachesRepeatedLookups(t *testing.T) {
	cache, transport := newSafetyTestCache(t, "writer", "someoneelse", false, "WRITE")

	for i := 0; i < 3; i++ {
		safe, err := cache.IsSafeContent(t.Context(), "writer", testOwner, testRepo)
		require.NoError(t, err)
		require.True(t, safe)
	}

	require.EqualValues(t, 1, transport.CallCount(), "repeated lookups for the same user should hit the cache")
}

func TestIsSafeContentPropagatesQueryError(t *testing.T) {
	// A nil GraphQL client forces queryRepoAccessInfo to fail, which must
	// surface as an error (and an unsafe result) rather than silently
	// allowing access.
	cache := &RepoAccessCache{
		client:           nil,
		cache:            cache2go.Cache(t.Name()),
		ttl:              defaultRepoAccessTTL,
		trustedBotLogins: map[string]struct{}{"copilot": {}},
	}

	safe, err := cache.IsSafeContent(t.Context(), testUser, testOwner, testRepo)
	require.Error(t, err)
	require.False(t, safe)
}

func TestGetRepoAccessInfoNilReceiver(t *testing.T) {
	var cache *RepoAccessCache
	_, err := cache.getRepoAccessInfo(t.Context(), testUser, testOwner, testRepo)
	require.Error(t, err)
}

func TestIsTrustedBot(t *testing.T) {
	cache := &RepoAccessCache{
		trustedBotLogins: map[string]struct{}{"copilot": {}},
	}

	require.True(t, cache.isTrustedBot("copilot"))
	require.True(t, cache.isTrustedBot("Copilot"), "trusted bot match should be case-insensitive")
	require.False(t, cache.isTrustedBot("octocat"))
	require.False(t, cache.isTrustedBot(""))
}

func TestCacheKey(t *testing.T) {
	require.Equal(t, "octo-org/octo-repo", cacheKey("Octo-Org", "Octo-Repo"))
	require.Equal(t, cacheKey("owner", "repo"), cacheKey("OWNER", "REPO"))
}

func TestRepoAccessOptions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	c := &RepoAccessCache{}
	WithTTL(time.Minute)(c)
	WithLogger(logger)(c)
	WithCacheName("custom-cache-name-for-test")(c)

	require.Equal(t, time.Minute, c.ttl)
	require.Same(t, logger, c.logger)
	require.NotNil(t, c.cache)

	// An empty cache name is a no-op and must not clobber an existing table.
	existing := c.cache
	WithCacheName("")(c)
	require.Same(t, existing, c.cache)
}

func TestSetLogger(t *testing.T) {
	c := &RepoAccessCache{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	c.SetLogger(logger)
	require.Same(t, logger, c.logger)
}

// ensure the http import is used even if the helper above changes shape.
var _ http.RoundTripper = (*countingTransport)(nil)
