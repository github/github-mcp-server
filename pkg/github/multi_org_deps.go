package github

import (
	"context"
	"fmt"
	"sync"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/shurcooL/githubv4"
)

// MultiOrgDeps implements ToolDependencies by routing API calls to the correct
// org's GitHub App installation based on the owner in context.
//
// The owner is extracted from context by OwnerFromContext, which is populated
// by OwnerExtractMiddleware before tool handlers are invoked. If no owner is
// present in context, the factory falls back to the default installation (if
// configured), or returns an error (fail-closed).
type MultiOrgDeps struct {
	factory           *MultiOrgClientFactory
	t                 translations.TranslationHelperFunc
	flags             FeatureFlags
	contentWindowSize int
	featureChecker    inventory.FeatureFlagChecker
	lockdownMode      bool
	repoAccessOpts    []lockdown.RepoAccessOption

	// Per-installation lockdown caches. Each installation ID gets its own
	// RepoAccessCache backed by that installation's GQL client. Keyed by
	// installation ID (not owner string) to prevent unbounded growth.
	// Protected by repoAccessMu.
	repoAccessCaches map[int64]*lockdown.RepoAccessCache
	repoAccessMu     sync.RWMutex
}

// Compile-time assertion: MultiOrgDeps must implement ToolDependencies.
var _ ToolDependencies = (*MultiOrgDeps)(nil)

// NewMultiOrgDeps creates a MultiOrgDeps with the provided factory and configuration.
func NewMultiOrgDeps(
	factory *MultiOrgClientFactory,
	t translations.TranslationHelperFunc,
	flags FeatureFlags,
	contentWindowSize int,
	featureChecker inventory.FeatureFlagChecker,
	lockdownMode bool,
	repoAccessOpts []lockdown.RepoAccessOption,
) *MultiOrgDeps {
	return &MultiOrgDeps{
		factory:           factory,
		t:                 t,
		flags:             flags,
		contentWindowSize: contentWindowSize,
		featureChecker:    featureChecker,
		lockdownMode:      lockdownMode,
		repoAccessOpts:    repoAccessOpts,
		repoAccessCaches:  make(map[int64]*lockdown.RepoAccessCache),
	}
}

// GetClient implements ToolDependencies. Routes to the GitHub App installation
// for the owner stored in context. Falls back to the default installation if
// no owner is present.
func (d *MultiOrgDeps) GetClient(ctx context.Context) (*gogithub.Client, error) {
	owner := OwnerFromContext(ctx)
	return d.factory.GetRESTClient(ctx, owner)
}

// GetGQLClient implements ToolDependencies. Routes to the GitHub App
// installation for the owner stored in context.
func (d *MultiOrgDeps) GetGQLClient(ctx context.Context) (*githubv4.Client, error) {
	owner := OwnerFromContext(ctx)
	return d.factory.GetGQLClient(ctx, owner)
}

// GetRawClient implements ToolDependencies. Routes to the GitHub App
// installation for the owner stored in context.
func (d *MultiOrgDeps) GetRawClient(ctx context.Context) (*raw.Client, error) {
	owner := OwnerFromContext(ctx)
	return d.factory.GetRawClient(ctx, owner)
}

// GetRepoAccessCache implements ToolDependencies. Returns nil when lockdown
// mode is disabled. When enabled, returns a per-installation RepoAccessCache
// backed by that installation's GQL client. Caches are keyed by installation ID
// (not owner string) to prevent unbounded growth: unknown owners that fall back
// to the default installation share a single cache entry.
//
// Caches are created lazily and reused across requests for the same installation.
// Uses lockdown.NewRepoAccessCache (not the singleton GetInstance) so each
// installation gets an independent cache with its own GQL client.
func (d *MultiOrgDeps) GetRepoAccessCache(ctx context.Context) (*lockdown.RepoAccessCache, error) {
	if !d.lockdownMode {
		return nil, nil
	}

	// Resolve owner → installation ID to use as cache key. This mirrors the
	// transport cache fix: unknown owners that fall back to the default
	// installation share a single cache entry instead of creating unbounded
	// entries keyed by arbitrary owner strings.
	owner := OwnerFromContext(ctx)
	installID := d.factory.getInstallationID(owner)
	if installID == 0 {
		return nil, fmt.Errorf(
			"no GitHub App installation configured for org %q and no default installation set",
			owner,
		)
	}

	// Fast path: check if cache already exists (read lock).
	d.repoAccessMu.RLock()
	if cache, ok := d.repoAccessCaches[installID]; ok {
		d.repoAccessMu.RUnlock()
		return cache, nil
	}
	d.repoAccessMu.RUnlock()

	// Slow path: create cache under write lock (double-checked locking).
	d.repoAccessMu.Lock()
	defer d.repoAccessMu.Unlock()

	if cache, ok := d.repoAccessCaches[installID]; ok {
		return cache, nil
	}

	gqlClient, err := d.GetGQLClient(ctx)
	if err != nil {
		return nil, err
	}

	// Use installation ID in the cache2go table name (not pointer address)
	// to avoid collisions if Go GC reuses a pointer address for a new client.
	opts := append(d.repoAccessOpts,
		lockdown.WithCacheName(fmt.Sprintf("repo-access-install-%d", installID)),
	)
	cache := lockdown.NewRepoAccessCache(gqlClient, opts...)
	d.repoAccessCaches[installID] = cache
	return cache, nil
}

// GetT implements ToolDependencies.
func (d *MultiOrgDeps) GetT() translations.TranslationHelperFunc { return d.t }

// GetFlags implements ToolDependencies.
func (d *MultiOrgDeps) GetFlags(_ context.Context) FeatureFlags { return d.flags }

// GetContentWindowSize implements ToolDependencies.
func (d *MultiOrgDeps) GetContentWindowSize() int { return d.contentWindowSize }

// IsFeatureEnabled implements ToolDependencies. Returns false if the feature
// checker is nil, the flag name is empty, or an error occurs during the check.
func (d *MultiOrgDeps) IsFeatureEnabled(ctx context.Context, flagName string) bool {
	if d.featureChecker == nil || flagName == "" {
		return false
	}
	enabled, err := d.featureChecker(ctx, flagName)
	if err != nil {
		return false
	}
	return enabled
}
