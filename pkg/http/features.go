package http

import (
	"context"
	"slices"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
)

// KnownFeatureFlags are the feature flags that can be enabled via X-MCP-Features header.
var KnownFeatureFlags = []string{
	github.FeatureFlagHoldbackConsolidatedProjects,
	github.FeatureFlagHoldbackConsolidatedActions,
}

// CreateHTTPFeatureChecker creates a feature checker that reads header features from context
func CreateHTTPFeatureChecker(staticChecker inventory.FeatureFlagChecker) inventory.FeatureFlagChecker {
	// Pre-compute whitelist as set for O(1) lookup
	knownSet := make(map[string]bool, len(KnownFeatureFlags))
	for _, f := range KnownFeatureFlags {
		knownSet[f] = true
	}

	return func(ctx context.Context, flag string) (bool, error) {
		// Check whitelist first, then header features
		if knownSet[flag] && slices.Contains(ghcontext.GetHeaderFeatures(ctx), flag) {
			return true, nil
		}
		// Fall back to static checker
		if staticChecker != nil {
			return staticChecker(ctx, flag)
		}
		return false, nil
	}
}
