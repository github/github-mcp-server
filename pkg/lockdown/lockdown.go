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
	isPrivate   bool
	knownUsers  map[string]bool // normalized login -> has push access
	viewerLogin string
}

// RepoAccessInfo captures repository metadata needed for lockdown decisions.
type RepoAccessInfo struct {
	IsPrivate     bool
	HasPushAccess bool
	ViewerLogin   string
}

const (
	defaultRepoAccessTTL      = 20 * time.Minute
	defaultRepoAccessCacheKey = "repo-access-cache"
)

var (
	instance   *RepoAccessCache
	instanceMu sync.Mutex
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

// WithCacheName overrides the cache table name used for storing entries. This option is intended for tests
// that need isolated cache instances.
func WithCacheName(name string) RepoAccessOption {
	return func(c *RepoAccessCache) {
		if name != "" {
			c.cache = cache2go.Cache(name)
		}
	}
}

// GetInstance returns the singleton instance of RepoAccessCache.
// It initializes the instance on first call with the provided client and options.
// Subsequent calls ignore the client and options parameters and return the existing instance.
// This is the preferred way to access the cache in production code.
func GetInstance(client *githubv4.Client, opts ...RepoAccessOption) *RepoAccessCache {
	instanceMu.Lock()
	defer instanceMu.Unlock()
	if instance == nil {
		instance = &RepoAccessCache{
			client: client,
			cache:  cache2go.Cache(defaultRepoAccessCacheKey),
			ttl:    defaultRepoAccessTTL,
		}
		for _, opt := range opts {
			if opt != nil {
				opt(instance)
			}
		}
	}
	return instance
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

func (c *RepoAccessCache) IsSafeContent(ctx context.Context, username, owner, repo string) (bool, error) {
	repoInfo, err := c.getRepoAccessInfo(ctx, username, owner, repo)
	if err != nil {
		c.logDebug("error checking repo access info for content filtering", "owner", owner, "repo", repo, "user", username, "error", err)
		return false, err
	}
	if repoInfo.IsPrivate || repoInfo.ViewerLogin == username {
		return true, nil
	}
	return repoInfo.HasPushAccess, nil
}

func (c *RepoAccessCache) getRepoAccessInfo(ctx context.Context, username, owner, repo string) (RepoAccessInfo, error) {
	if c == nil {
		return RepoAccessInfo{}, fmt.Errorf("nil repo access cache")
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
			return RepoAccessInfo{
				IsPrivate:     entry.isPrivate,
				HasPushAccess: cachedHasPush,
				ViewerLogin:   entry.viewerLogin,
			}, nil
		}
		c.logDebug("known users cache miss", "owner", owner, "repo", repo, "user", username)
		info, queryErr := c.queryRepoAccessInfo(ctx, username, owner, repo)
		if queryErr != nil {
			return RepoAccessInfo{}, queryErr
		}
		entry.knownUsers[userKey] = info.HasPushAccess
		entry.viewerLogin = info.ViewerLogin
		entry.isPrivate = info.IsPrivate
		c.cache.Add(key, c.ttl, entry)
		return RepoAccessInfo{
			IsPrivate:     entry.isPrivate,
			HasPushAccess: entry.knownUsers[userKey],
			ViewerLogin:   entry.viewerLogin,
		}, nil
	}

	c.logDebug("repo access cache miss", "owner", owner, "repo", repo, "user", username)

	info, queryErr := c.queryRepoAccessInfo(ctx, username, owner, repo)
	if queryErr != nil {
		return RepoAccessInfo{}, queryErr
	}

	// Create new entry
	entry := &repoAccessCacheEntry{
		knownUsers:  map[string]bool{userKey: info.HasPushAccess},
		isPrivate:   info.IsPrivate,
		viewerLogin: info.ViewerLogin,
	}
	c.cache.Add(key, c.ttl, entry)

	return RepoAccessInfo{
		IsPrivate:     entry.isPrivate,
		HasPushAccess: entry.knownUsers[userKey],
		ViewerLogin:   entry.viewerLogin,
	}, nil
}

func (c *RepoAccessCache) queryRepoAccessInfo(ctx context.Context, username, owner, repo string) (RepoAccessInfo, error) {
	if c.client == nil {
		return RepoAccessInfo{}, fmt.Errorf("nil GraphQL client")
	}

	var query struct {
		Viewer struct {
			Login githubv4.String
		}
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
		return RepoAccessInfo{}, fmt.Errorf("failed to query repository access info: %w", err)
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

	return RepoAccessInfo{
		IsPrivate:     bool(query.Repository.IsPrivate),
		HasPushAccess: hasPush,
		ViewerLogin:   string(query.Viewer.Login),
	}, nil
}

func cacheKey(owner, repo string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(owner), strings.ToLower(repo))
}

func (c *RepoAccessCache) logDebug(msg string, args ...any) {
	if c != nil && c.logger != nil {
		c.logger.Debug(msg, args...)
	}
}
