package lockdown

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/muesli/cache2go"
	"github.com/shurcooL/githubv4"
)

// RepoAccessCache caches repository metadata related to lockdown checks so that
// multiple tools can reuse the same access information safely across goroutines.
type RepoAccessCache struct {
	client *githubv4.Client
	mu     sync.Mutex
	cache  *cache2go.CacheTable
	ttl    time.Duration
	logger *slog.Logger
}

type repoAccessCacheEntry struct {
	isPrivate  bool
	knownUsers map[string]bool // normalized login -> has push access
}

const defaultRepoAccessTTL = 5 * time.Minute

var (
	instance     *RepoAccessCache
	instanceOnce sync.Once
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

// GetInstance returns the singleton instance of RepoAccessCache.
// It initializes the instance on first call with the provided client and options.
// Subsequent calls ignore the client and options parameters and return the existing instance.
// This is the preferred way to access the cache in production code.
func GetInstance(client *githubv4.Client, opts ...RepoAccessOption) *RepoAccessCache {
	instanceOnce.Do(func() {
		instance = newRepoAccessCache(client, opts...)
	})
	return instance
}

// newRepoAccessCache creates a new cache instance. This is a private helper function
// used by GetInstance.
func newRepoAccessCache(client *githubv4.Client, opts ...RepoAccessOption) *RepoAccessCache {
	cacheName := "repo-access-cache"
	c := &RepoAccessCache{
		client: client,
		cache:  cache2go.Cache(cacheName),
		ttl:    defaultRepoAccessTTL,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	c.logInfo("repo access cache initialized", "ttl", c.ttl)
	return c
}

// SetTTL overrides the default time-to-live used for cache entries. A non-positive
// duration disables expiration.
func (c *RepoAccessCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
	c.logInfo("repo access cache TTL updated", "ttl", ttl)

	// Collect all current entries
	entries := make(map[interface{}]*repoAccessCacheEntry)
	c.cache.Foreach(func(key interface{}, item *cache2go.CacheItem) {
		entries[key] = item.Data().(*repoAccessCacheEntry)
	})

	// Flush the cache
	c.cache.Flush()

	// Re-add all entries with the new TTL
	for key, entry := range entries {
		c.cache.Add(key, ttl, entry)
	}
}

// SetLogger updates the logger used for cache diagnostics.
func (c *RepoAccessCache) SetLogger(logger *slog.Logger) {
	c.mu.Lock()
	c.logger = logger
	c.mu.Unlock()
}

// CacheStats summarizes cache activity counters.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
}

// GetRepoAccessInfo returns the repository's privacy status and whether the
// specified user has push permissions. Results are cached per repository to
// avoid repeated GraphQL round-trips.
func (c *RepoAccessCache) GetRepoAccessInfo(ctx context.Context, username, owner, repo string) (bool, bool, error) {
	if c == nil {
		return false, false, fmt.Errorf("nil repo access cache")
	}

	key := cacheKey(owner, repo)
	userKey := strings.ToLower(username)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Try to get entry from cache - this will keep the item alive if it exists
	cacheItem, err := c.cache.Value(key)
	if err == nil {
		entry := cacheItem.Data().(*repoAccessCacheEntry)
		if cachedHasPush, known := entry.knownUsers[userKey]; known {
			c.logDebug("repo access cache hit", "owner", owner, "repo", repo, "user", username)
			return entry.isPrivate, cachedHasPush, nil
		}
		// Entry exists but user not in knownUsers, need to query
	}
	c.logDebug("repo access cache miss", "owner", owner, "repo", repo, "user", username)

	isPrivate, hasPush, queryErr := c.queryRepoAccessInfo(ctx, username, owner, repo)
	if queryErr != nil {
		return false, false, queryErr
	}

	// Repo access info retrieved, update or create cache entry
	var entry *repoAccessCacheEntry
	if err == nil && cacheItem != nil {
		entry = cacheItem.Data().(*repoAccessCacheEntry)
		entry.knownUsers[userKey] = hasPush
		return entry.isPrivate, entry.knownUsers[userKey], nil
	}

	// Create new entry
	entry = &repoAccessCacheEntry{
		knownUsers: map[string]bool{userKey: hasPush},
		isPrivate:  isPrivate,
	}
	c.cache.Add(key, c.ttl, entry)

	return entry.isPrivate, entry.knownUsers[userKey], nil
}

func (c *RepoAccessCache) queryRepoAccessInfo(ctx context.Context, username, owner, repo string) (bool, bool, error) {
	if c.client == nil {
		return false, false, fmt.Errorf("nil GraphQL client")
	}

	var query struct {
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

	variables := map[string]interface{}{
		"owner":    githubv4.String(owner),
		"name":     githubv4.String(repo),
		"username": githubv4.String(username),
	}

	if err := c.client.Query(ctx, &query, variables); err != nil {
		return false, false, fmt.Errorf("failed to query repository access info: %w", err)
	}

	hasPush := false
	for _, edge := range query.Repository.Collaborators.Edges {
		login := string(edge.Node.Login)
		if strings.EqualFold(login, username) {
			permission := string(edge.Permission)
			hasPush = permission == "WRITE" || permission == "ADMIN" || permission == "MAINTAIN"
			break
		}
	}

	return bool(query.Repository.IsPrivate), hasPush, nil
}

func cacheKey(owner, repo string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(owner), strings.ToLower(repo))
}

func (c *RepoAccessCache) logDebug(msg string, args ...any) {
	if c != nil && c.logger != nil {
		c.logger.Debug(msg, args...)
	}
}

func (c *RepoAccessCache) logInfo(msg string, args ...any) {
	if c != nil && c.logger != nil {
		c.logger.Info(msg, args...)
	}
}
