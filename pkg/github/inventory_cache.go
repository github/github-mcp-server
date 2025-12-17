package github

import (
	"sync"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// CachedInventory provides a cached inventory builder that builds tool definitions
// only once, regardless of how many times NewInventoryBuilder is called.
//
// This is particularly useful for stateless server patterns (like the remote server)
// where a new server instance is created per request. Without caching, every request
// would rebuild all ~130 tool definitions including JSON schema generation, causing
// significant performance overhead.
//
// Usage:
//
//	// Option 1: Initialize once at startup with your translator
//	github.InitInventoryCache(myTranslator)
//
//	// Then get pre-built inventory on each request
//	inv := github.CachedInventoryBuilder().
//	    WithReadOnly(cfg.ReadOnly).
//	    WithToolsets(cfg.Toolsets).
//	    Build()
//
//	// Option 2: Use NewInventory which doesn't use the cache (legacy behavior)
//	inv := github.NewInventory(myTranslator).Build()
//
// The cache stores the built []ServerTool, []ServerResourceTemplate, and []ServerPrompt.
// Per-request configuration (read-only, toolsets, feature flags, filters) is still
// applied when building the Inventory from the cached data.
type CachedInventory struct {
	once      sync.Once
	tools     []inventory.ServerTool
	resources []inventory.ServerResourceTemplate
	prompts   []inventory.ServerPrompt
}

// global singleton for caching
var globalInventoryCache = &CachedInventory{}

// InitInventoryCache initializes the global inventory cache with the given translator.
// This should be called once at startup before any requests are processed.
// It's safe to call multiple times - only the first call has any effect.
//
// For the local server, this is typically called with the configured translator.
// For the remote server, use translations.NullTranslationHelper since translations
// aren't needed per-request.
//
// Example:
//
//	func main() {
//	    t, _ := translations.TranslationHelper()
//	    github.InitInventoryCache(t)
//	    // ... start server
//	}
func InitInventoryCache(t translations.TranslationHelperFunc) {
	globalInventoryCache.init(t, nil, nil, nil)
}

// InitInventoryCacheWithExtras initializes the global inventory cache with the given
// translator plus additional tools, resources, and prompts.
//
// This is useful for the remote server which has additional tools (e.g., Copilot tools)
// that aren't part of the base github-mcp-server package.
//
// The extra items are appended to the base items from AllTools/AllResources/AllPrompts.
// It's safe to call multiple times - only the first call has any effect.
//
// Example:
//
//	func init() {
//	    github.InitInventoryCacheWithExtras(
//	        translations.NullTranslationHelper,
//	        remoteOnlyTools,      // []inventory.ServerTool
//	        remoteOnlyResources,  // []inventory.ServerResourceTemplate
//	        remoteOnlyPrompts,    // []inventory.ServerPrompt
//	    )
//	}
func InitInventoryCacheWithExtras(
	t translations.TranslationHelperFunc,
	extraTools []inventory.ServerTool,
	extraResources []inventory.ServerResourceTemplate,
	extraPrompts []inventory.ServerPrompt,
) {
	globalInventoryCache.init(t, extraTools, extraResources, extraPrompts)
}

// init initializes the cache with the given translator and optional extras (sync.Once protected).
func (c *CachedInventory) init(
	t translations.TranslationHelperFunc,
	extraTools []inventory.ServerTool,
	extraResources []inventory.ServerResourceTemplate,
	extraPrompts []inventory.ServerPrompt,
) {
	c.once.Do(func() {
		c.tools = AllTools(t)
		c.resources = AllResources(t)
		c.prompts = AllPrompts(t)

		// Append extra items if provided
		if len(extraTools) > 0 {
			c.tools = append(c.tools, extraTools...)
		}
		if len(extraResources) > 0 {
			c.resources = append(c.resources, extraResources...)
		}
		if len(extraPrompts) > 0 {
			c.prompts = append(c.prompts, extraPrompts...)
		}
	})
}

// CachedInventoryBuilder returns an inventory.Builder pre-populated with cached
// tool/resource/prompt definitions.
//
// The cache must be initialized via InitInventoryCache before calling this function.
// If the cache is not initialized, this will initialize it with NullTranslationHelper.
//
// Per-request configuration can still be applied via the builder methods:
//   - WithReadOnly(bool) - filter to read-only tools
//   - WithToolsets([]string) - enable specific toolsets
//   - WithTools([]string) - enable specific tools
//   - WithFeatureChecker(func) - per-request feature flag evaluation
//   - WithFilter(func) - custom filtering
//
// Example:
//
//	inv := github.CachedInventoryBuilder().
//	    WithReadOnly(cfg.ReadOnly).
//	    WithToolsets(cfg.EnabledToolsets).
//	    WithFeatureChecker(createFeatureChecker(cfg.EnabledFeatures)).
//	    Build()
func CachedInventoryBuilder() *inventory.Builder {
	// Ensure cache is initialized (with NullTranslationHelper as fallback)
	globalInventoryCache.init(translations.NullTranslationHelper, nil, nil, nil)

	return inventory.NewBuilder().
		SetTools(globalInventoryCache.tools).
		SetResources(globalInventoryCache.resources).
		SetPrompts(globalInventoryCache.prompts)
}

// IsCacheInitialized returns true if the inventory cache has been initialized.
// This is primarily useful for testing.
func IsCacheInitialized() bool {
	// We can't directly check sync.Once state, but we can check if tools are populated
	return len(globalInventoryCache.tools) > 0
}

// ResetInventoryCache resets the global inventory cache, allowing it to be
// reinitialized with a different translator. This should only be used in tests.
//
// WARNING: This is not thread-safe and should never be called in production code.
func ResetInventoryCache() {
	globalInventoryCache = &CachedInventory{}
}
