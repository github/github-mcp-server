package lockdown

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/muesli/cache2go"
	"github.com/shurcooL/githubv4"
)

// RepoAccessCache caches repository metadata related to lockdown checks so that
// multiple tools can reuse the same access information safely across goroutines.
// In HTTP mode each request must construct its own instance so viewer-scoped
// lookups run under the requesting user's credentials.
type RepoAccessCache struct {
	client           *githubv4.Client
	restClient       *github.Client
	cache            *cache2go.CacheTable
	ttl              time.Duration
	logger           *slog.Logger
	trustedBotLogins map[string]struct{}

	viewerMu    sync.Mutex
	viewerLogin string
}

type repoAccessCacheEntry struct {
	isPrivate  bool
	knownUsers map[string]bool // normalized login -> has push access
}

// RepoAccessInfo captures repository metadata needed for lockdown decisions.
type RepoAccessInfo struct {
	IsPrivate     bool
	HasPushAccess bool
}

const (
	defaultRepoAccessTTL      = 20 * time.Minute
	defaultRepoAccessCacheKey = "repo-access-cache"
)

// RepoAccessOption configures RepoAccessCache at construction time.
type RepoAccessOption func(*RepoAccessCache)

// WithTTL overrides the default TTL applied to cache entries. A non-positive
// duration disables expiration.
func WithTTL(ttl time.Duration) RepoAccessOption {
	return func(c *RepoAccessCache) {
		c.ttl = ttl
	}
}

// WithLogger sets the logger used for cache diagnostics.
func WithLogger(logger *slog.Logger) RepoAccessOption {
	return func(c *RepoAccessCache) {
		c.logger = logger
	}
}

// WithCacheName overrides the cache table name used for storing entries.
// Use this to isolate cache entries between tenants or in tests.
func WithCacheName(name string) RepoAccessOption {
	return func(c *RepoAccessCache) {
		if name != "" {
			c.cache = cache2go.Cache(name)
		}
	}
}

// NewRepoAccessCache creates a RepoAccessCache bound to the supplied clients.
func NewRepoAccessCache(client *githubv4.Client, restClient *github.Client, opts ...RepoAccessOption) *RepoAccessCache {
	c := &RepoAccessCache{
		client:     client,
		restClient: restClient,
		cache:      cache2go.Cache(defaultRepoAccessCacheKey),
		ttl:        defaultRepoAccessTTL,
		trustedBotLogins: map[string]struct{}{
			"copilot":             {},
			"github-actions[bot]": {},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// CacheStats summarizes cache activity counters.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
}

// IsSafeContent determines if the specified user can safely access the requested repository content.
// Safe access applies when any of the following is true:
// - the content was created by a trusted bot;
// - the author currently has push access to the repository;
// - the repository is private;
// - the content was created by the viewer.
func (c *RepoAccessCache) IsSafeContent(ctx context.Context, username, owner, repo string) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("nil repo access cache")
	}

	if c.isTrustedBot(username) {
		return true, nil
	}

	repoInfo, err := c.getRepoAccessInfo(ctx, username, owner, repo)
	if err != nil {
		return false, err
	}

	c.logDebug(ctx, fmt.Sprintf("evaluated repo access for user %s to %s/%s for content filtering, result: hasPushAccess=%t, isPrivate=%t",
		username, owner, repo, repoInfo.HasPushAccess, repoInfo.IsPrivate))

	if repoInfo.IsPrivate {
		return true, nil
	}
	if repoInfo.HasPushAccess {
		return true, nil
	}

	viewerLogin, err := c.viewerLoginFor(ctx)
	if err != nil {
		return false, err
	}
	return viewerLogin == strings.ToLower(username), nil
}

func (c *RepoAccessCache) viewerLoginFor(ctx context.Context) (string, error) {
	c.viewerMu.Lock()
	defer c.viewerMu.Unlock()
	if c.viewerLogin != "" {
		return c.viewerLogin, nil
	}
	if c.client == nil {
		return "", fmt.Errorf("nil GraphQL client")
	}
	var query struct {
		Viewer struct {
			Login githubv4.String
		}
	}
	if err := c.client.Query(ctx, &query, nil); err != nil {
		return "", fmt.Errorf("failed to query viewer login: %w", err)
	}
	login := strings.ToLower(string(query.Viewer.Login))
	if login == "" {
		return "", fmt.Errorf("viewer login returned empty")
	}
	c.viewerLogin = login
	return c.viewerLogin, nil
}

// setViewerLogin seeds the cached viewer login from a piggy-backed query response.
func (c *RepoAccessCache) setViewerLogin(login string) {
	if login == "" {
		return
	}
	c.viewerMu.Lock()
	defer c.viewerMu.Unlock()
	if c.viewerLogin == "" {
		c.viewerLogin = strings.ToLower(login)
	}
}

func (c *RepoAccessCache) getRepoAccessInfo(ctx context.Context, username, owner, repo string) (RepoAccessInfo, error) {
	if c == nil {
		return RepoAccessInfo{}, fmt.Errorf("nil repo access cache")
	}

	key := cacheKey(owner, repo)
	userKey := strings.ToLower(username)

	// Entries are immutable once added: the cache table is shared across instances,
	// so we publish a fresh entry with a cloned knownUsers map on every miss.
	if cacheItem, err := c.cache.Value(key); err == nil {
		entry := cacheItem.Data().(*repoAccessCacheEntry)
		if cachedHasPush, known := entry.knownUsers[userKey]; known {
			c.logDebug(ctx, fmt.Sprintf("repo access cache hit for user %s to %s/%s", username, owner, repo))
			return RepoAccessInfo{
				IsPrivate:     entry.isPrivate,
				HasPushAccess: cachedHasPush,
			}, nil
		}

		c.logDebug(ctx, "known users cache miss, fetching permission")

		hasPush, pushErr := c.checkPushAccess(ctx, username, owner, repo)
		if pushErr != nil {
			return RepoAccessInfo{}, pushErr
		}

		users := make(map[string]bool, len(entry.knownUsers)+1)
		maps.Copy(users, entry.knownUsers)
		users[userKey] = hasPush
		c.cache.Add(key, c.ttl, &repoAccessCacheEntry{
			isPrivate:  entry.isPrivate,
			knownUsers: users,
		})

		return RepoAccessInfo{
			IsPrivate:     entry.isPrivate,
			HasPushAccess: hasPush,
		}, nil
	}

	c.logDebug(ctx, fmt.Sprintf("repo access cache miss for user %s to %s/%s", username, owner, repo))

	isPrivate, viewerLogin, queryErr := c.queryRepoAccessInfo(ctx, owner, repo)
	if queryErr != nil {
		return RepoAccessInfo{}, queryErr
	}
	c.setViewerLogin(viewerLogin)

	hasPush, pushErr := c.checkPushAccess(ctx, username, owner, repo)
	if pushErr != nil {
		return RepoAccessInfo{}, pushErr
	}

	c.cache.Add(key, c.ttl, &repoAccessCacheEntry{
		knownUsers: map[string]bool{userKey: hasPush},
		isPrivate:  isPrivate,
	})

	return RepoAccessInfo{
		IsPrivate:     isPrivate,
		HasPushAccess: hasPush,
	}, nil
}

// queryRepoAccessInfo fetches repository visibility and the viewer login in a single GraphQL round-trip.
func (c *RepoAccessCache) queryRepoAccessInfo(ctx context.Context, owner, repo string) (bool, string, error) {
	if c.client == nil {
		return false, "", fmt.Errorf("nil GraphQL client")
	}

	var query struct {
		Viewer struct {
			Login githubv4.String
		}
		Repository struct {
			IsPrivate githubv4.Boolean
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner": githubv4.String(owner),
		"name":  githubv4.String(repo),
	}

	if err := c.client.Query(ctx, &query, variables); err != nil {
		return false, "", fmt.Errorf("failed to query repository metadata: %w", err)
	}

	c.logDebug(ctx, fmt.Sprintf("queried repo access info for %s/%s: isPrivate=%t", owner, repo, bool(query.Repository.IsPrivate)))

	return bool(query.Repository.IsPrivate), string(query.Viewer.Login), nil
}

// checkPushAccess checks if the user has push access to the repository via the REST permission endpoint.
func (c *RepoAccessCache) checkPushAccess(ctx context.Context, username, owner, repo string) (bool, error) {
	if c.restClient == nil {
		return false, fmt.Errorf("nil REST client")
	}

	permLevel, _, err := c.restClient.Repositories.GetPermissionLevel(ctx, owner, repo, username)
	if err != nil {
		return false, fmt.Errorf("failed to get user permission level: %w", err)
	}

	// REST API maps "maintain" to "write" (and "triage" to "read")
	// https://docs.github.com/en/rest/collaborators/collaborators#get-repository-permissions-for-a-user
	permission := permLevel.GetPermission()
	return permission == "admin" || permission == "write", nil
}

func (c *RepoAccessCache) log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if c == nil || c.logger == nil {
		return
	}
	if !c.logger.Enabled(ctx, level) {
		return
	}
	c.logger.LogAttrs(ctx, level, msg, attrs...)
}

func (c *RepoAccessCache) logDebug(ctx context.Context, msg string, attrs ...slog.Attr) {
	c.log(ctx, slog.LevelDebug, msg, attrs...)
}

func (c *RepoAccessCache) isTrustedBot(username string) bool {
	_, ok := c.trustedBotLogins[strings.ToLower(username)]
	return ok
}

func cacheKey(owner, repo string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(owner), strings.ToLower(repo))
}
