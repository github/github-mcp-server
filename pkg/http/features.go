package http

import (
	"context"
	"slices"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
)

// KnownFeatureFlags are the feature flags that can be enabled via X-MCP-Features header.
var KnownFeatureFlags = []string{
	github.FeatureFlagConsolidatedProjects,
	github.FeatureFlagConsolidatedActions,
}

// ComposeFeatureChecker combines header-based feature flags with a static checker.
func ComposeFeatureChecker(headerFeatures []string, staticChecker inventory.FeatureFlagChecker) inventory.FeatureFlagChecker {
	if len(headerFeatures) == 0 && staticChecker == nil {
		return nil
	}

	// Only accept header features that are in the known list
	headerSet := make(map[string]bool, len(headerFeatures))
	for _, f := range headerFeatures {
		if slices.Contains(KnownFeatureFlags, f) {
			headerSet[f] = true
		}
	}

	return func(ctx context.Context, flag string) (bool, error) {
		// Header-based: static string matching
		if headerSet[flag] {
			return true, nil
		}
		// Static checker
		if staticChecker != nil {
			return staticChecker(ctx, flag)
		}
		return false, nil
	}
}
