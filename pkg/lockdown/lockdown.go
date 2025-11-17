package lockdown

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/shurcooL/githubv4"
)

// RepoAccessCache caches repository metadata related to lockdown checks so that
// multiple tools can reuse the same access information safely across goroutines.
type RepoAccessCache struct {
	client *githubv4.Client
	mu     sync.Mutex
	cache  map[string]*repoAccessCacheEntry
	ttl    time.Duration
	logger *slog.Logger
}

type repoAccessCacheEntry struct {
	isPrivate  bool
	knownUsers map[string]bool // normalized login -> has push access
	ready      bool
	timer      *time.Timer
}

const defaultRepoAccessTTL = 5 * time.Minute

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

// NewRepoAccessCache returns a cache bound to the provided GitHub GraphQL
// client. The cache is safe for concurrent use.
func NewRepoAccessCache(client *githubv4.Client, opts ...RepoAccessOption) *RepoAccessCache {
	c := &RepoAccessCache{
		client: client,
		cache:  make(map[string]*repoAccessCacheEntry),
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
	for key, entry := range c.cache {
		entry.scheduleExpiry(c, key)
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
	entry := c.ensureEntry(key)
	if entry.ready {
		if cachedHasPush, known := entry.knownUsers[userKey]; known {
			entry.scheduleExpiry(c, key)
			c.logDebug("repo access cache hit", "owner", owner, "repo", repo, "user", username)
			cachedPrivate := entry.isPrivate
			c.mu.Unlock()
			return cachedPrivate, cachedHasPush, nil
		}
	}
	c.mu.Unlock()
	c.logDebug("repo access cache miss", "owner", owner, "repo", repo, "user", username)

	isPrivate, hasPush, err := c.queryRepoAccessInfo(ctx, username, owner, repo)
	if err != nil {
		return false, false, err
	}

	c.mu.Lock()
	entry = c.ensureEntry(key)
	entry.ready = true
	entry.isPrivate = isPrivate
	entry.knownUsers[userKey] = hasPush
	entry.scheduleExpiry(c, key)
	c.mu.Unlock()

	return isPrivate, hasPush, nil
}

func (c *RepoAccessCache) ensureEntry(key string) *repoAccessCacheEntry {
	if c.cache == nil {
		c.cache = make(map[string]*repoAccessCacheEntry)
	}
	entry, ok := c.cache[key]
	if !ok {
		entry = &repoAccessCacheEntry{
			knownUsers: make(map[string]bool),
		}
		c.cache[key] = entry
	}
	return entry
}

func (entry *repoAccessCacheEntry) scheduleExpiry(c *RepoAccessCache, key string) {
	if entry.timer != nil {
		entry.timer.Stop()
		entry.timer = nil
	}

	dur := c.ttl
	if dur <= 0 {
		return
	}

	owner, repo := splitKey(key)
	entry.timer = time.AfterFunc(dur, func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		current, ok := c.cache[key]
		if !ok || current != entry {
			return
		}

		delete(c.cache, key)
		c.logDebug("repo access cache entry evicted", "owner", owner, "repo", repo)
	})
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

func splitKey(key string) (string, string) {
	owner, rest, found := strings.Cut(key, "/")
	if !found {
		return key, ""
	}
	return owner, rest
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
