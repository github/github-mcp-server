package github

// NOTE: Tests in this file intentionally do NOT use t.Parallel().
// They mutate the global inventory cache state via ResetInventoryCache,
// InitInventoryCache, and CachedInventoryBuilder. Running them in parallel
// would cause test flakiness and data races. Keep these tests serial even
// though most other tests in this package use t.Parallel().

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedInventory(t *testing.T) {
	// Reset cache before test
	ResetInventoryCache()

	t.Run("InitInventoryCache initializes tools", func(t *testing.T) {
		ResetInventoryCache()

		// Cache should be empty before init
		assert.False(t, IsCacheInitialized())

		// Initialize with null translator
		InitInventoryCache(translations.NullTranslationHelper)

		// Cache should now be initialized
		assert.True(t, IsCacheInitialized())
	})

	t.Run("CachedInventoryBuilder returns builder with cached tools", func(t *testing.T) {
		ResetInventoryCache()
		InitInventoryCache(translations.NullTranslationHelper)

		builder := CachedInventoryBuilder()
		require.NotNil(t, builder)

		inv := builder.Build()
		require.NotNil(t, inv)

		// Should have tools available
		tools := inv.AllTools()
		assert.Greater(t, len(tools), 0)
	})

	t.Run("CachedInventoryBuilder auto-initializes if not initialized", func(t *testing.T) {
		ResetInventoryCache()

		// Don't call InitInventoryCache

		// CachedInventoryBuilder should still work (auto-init with NullTranslationHelper)
		builder := CachedInventoryBuilder()
		require.NotNil(t, builder)

		inv := builder.Build()
		require.NotNil(t, inv)

		tools := inv.AllTools()
		assert.Greater(t, len(tools), 0)
	})

	t.Run("InitInventoryCache is idempotent", func(t *testing.T) {
		ResetInventoryCache()

		// First init with a custom translator that tracks calls
		callCount := 0
		customTranslator := func(_, defaultValue string) string {
			callCount++
			return defaultValue
		}

		InitInventoryCache(customTranslator)
		firstCallCount := callCount

		// Second init should be no-op
		callCount = 0
		InitInventoryCache(translations.NullTranslationHelper)

		assert.Equal(t, 0, callCount, "second InitInventoryCache should not call translator")

		// Verify first translator was used (tools are still populated from first call)
		assert.True(t, IsCacheInitialized())
		assert.Greater(t, firstCallCount, 0, "first init should have called translator")
	})

	t.Run("cached inventory preserves per-request configuration", func(t *testing.T) {
		ResetInventoryCache()
		InitInventoryCache(translations.NullTranslationHelper)

		// Build with read-only filter
		invReadOnly := CachedInventoryBuilder().
			WithReadOnly(true).
			WithToolsets([]string{"all"}).
			Build()

		// Build without read-only filter
		invAll := CachedInventoryBuilder().
			WithReadOnly(false).
			WithToolsets([]string{"all"}).
			Build()

		// AvailableTools applies filters - all tools should have more than read-only
		ctx := context.Background()
		allTools := invAll.AvailableTools(ctx)
		readOnlyTools := invReadOnly.AvailableTools(ctx)

		assert.Greater(t, len(allTools), len(readOnlyTools),
			"read-only inventory should have fewer tools")
	})

	t.Run("cached inventory supports toolset filtering", func(t *testing.T) {
		ResetInventoryCache()
		InitInventoryCache(translations.NullTranslationHelper)

		// Build with only one toolset
		invOneToolset := CachedInventoryBuilder().
			WithToolsets([]string{"context"}).
			Build()

		// Build with all toolsets
		invAll := CachedInventoryBuilder().
			WithToolsets([]string{"all"}).
			Build()

		ctx := context.Background()
		oneToolsetTools := invOneToolset.AvailableTools(ctx)
		allTools := invAll.AvailableTools(ctx)

		assert.Greater(t, len(allTools), len(oneToolsetTools),
			"all toolsets should have more tools than single toolset")
	})
}

func TestNewInventoryVsCachedInventoryBuilder(t *testing.T) {
	ResetInventoryCache()

	// NewInventory creates fresh tools each time
	inv1 := NewInventory(translations.NullTranslationHelper).Build()
	inv2 := NewInventory(translations.NullTranslationHelper).Build()

	// Both should have the same number of tools
	assert.Equal(t, len(inv1.AllTools()), len(inv2.AllTools()))

	// CachedInventoryBuilder uses cached tools
	InitInventoryCache(translations.NullTranslationHelper)
	cachedInv := CachedInventoryBuilder().Build()

	// Should also have the same number of tools
	assert.Equal(t, len(inv1.AllTools()), len(cachedInv.AllTools()))
}

func TestInitInventoryCacheWithExtras(t *testing.T) {
	ResetInventoryCache()

	// Create some extra tools for testing
	extraTools := []inventory.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "extra_tool_1",
				Description: "An extra tool for testing",
			},
			Toolset: ToolsetMetadataContext,
		},
		{
			Tool: mcp.Tool{
				Name:        "extra_tool_2",
				Description: "Another extra tool for testing",
			},
			Toolset: ToolsetMetadataRepos,
		},
	}

	extraResources := []inventory.ServerResourceTemplate{
		{
			Template: mcp.ResourceTemplate{
				Name:        "extra_resource",
				URITemplate: "extra://resource/{id}",
			},
			Toolset: ToolsetMetadataContext,
		},
	}

	extraPrompts := []inventory.ServerPrompt{
		{
			Prompt: mcp.Prompt{
				Name:        "extra_prompt",
				Description: "An extra prompt for testing",
			},
			Toolset: ToolsetMetadataContext,
		},
	}

	// Get baseline count without extras
	baselineInv := NewInventory(translations.NullTranslationHelper).Build()
	baseToolCount := len(baselineInv.AllTools())

	// Initialize cache with extras
	InitInventoryCacheWithExtras(
		translations.NullTranslationHelper,
		extraTools,
		extraResources,
		extraPrompts,
	)

	// Build inventory from cache
	inv := CachedInventoryBuilder().
		WithToolsets([]string{"all"}).
		Build()

	ctx := context.Background()
	cachedTools := inv.AvailableTools(ctx)

	// Should have base tools + extra tools
	assert.Equal(t, baseToolCount+len(extraTools), len(cachedTools),
		"cached inventory should include extra tools")

	// Verify extra tools are present
	toolNames := make(map[string]bool)
	for _, tool := range cachedTools {
		toolNames[tool.Tool.Name] = true
	}
	assert.True(t, toolNames["extra_tool_1"], "extra_tool_1 should be in cached tools")
	assert.True(t, toolNames["extra_tool_2"], "extra_tool_2 should be in cached tools")
}

func TestInitInventoryCacheWithExtrasIsIdempotent(t *testing.T) {
	ResetInventoryCache()

	extraTools1 := []inventory.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "first_extra",
				Description: "First extra tool",
			},
			Toolset: ToolsetMetadataContext,
		},
	}

	extraTools2 := []inventory.ServerTool{
		{
			Tool: mcp.Tool{
				Name:        "second_extra",
				Description: "Second extra tool",
			},
			Toolset: ToolsetMetadataContext,
		},
	}

	// First init with first set of extras
	InitInventoryCacheWithExtras(translations.NullTranslationHelper, extraTools1, nil, nil)

	// Second init should be ignored (sync.Once)
	InitInventoryCacheWithExtras(translations.NullTranslationHelper, extraTools2, nil, nil)

	inv := CachedInventoryBuilder().WithToolsets([]string{"all"}).Build()
	ctx := context.Background()
	tools := inv.AvailableTools(ctx)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Tool.Name] = true
	}

	// First extra should be present
	assert.True(t, toolNames["first_extra"], "first_extra should be in cached tools")
	// Second extra should NOT be present (second init was ignored)
	assert.False(t, toolNames["second_extra"], "second_extra should NOT be in cached tools")
}
