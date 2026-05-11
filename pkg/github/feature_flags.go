package github

import "slices"

// MCPAppsFeatureFlag is the feature flag name for MCP Apps (interactive UI forms).
const MCPAppsFeatureFlag = "remote_mcp_ui_apps"

// FeatureFlagCSVOutput is the feature flag name for CSV output on list tools.
const FeatureFlagCSVOutput = "csv_output"

// FeatureFlagIFCLabels is the feature flag name for IFC security labels in tool results.
const FeatureFlagIFCLabels = "ifc_labels"

// AllowedFeatureFlags is the allowlist of feature flags that can be enabled
// by users via --features CLI flag or X-MCP-Features HTTP header.
// Only flags in this list are accepted; unknown flags are silently ignored.
// This is the single source of truth for which flags are user-controllable.
var AllowedFeatureFlags = []string{
	MCPAppsFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagIssuesGranular,
	FeatureFlagPullRequestsGranular,
}

// InsidersFeatureFlags is the list of feature flags that insiders mode enables.
// When insiders mode is active, all flags in this list are treated as enabled.
// This is the single source of truth for what "insiders" means in terms of
// feature flag expansion.
var InsidersFeatureFlags = []string{
	MCPAppsFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagIFCLabels,
}

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
}

// ResolveFeatureFlags computes the effective set of enabled feature flags by:
//  1. Taking explicitly enabled features validated against AllowedFeatureFlags
//  2. Adding features enabled by insiders mode from InsidersFeatureFlags
//
// Returns a set (map) for O(1) lookup by the feature checker.
func ResolveFeatureFlags(enabledFeatures []string, insidersMode bool) map[string]bool {
	effective := make(map[string]bool)
	for _, f := range enabledFeatures {
		if slices.Contains(AllowedFeatureFlags, f) {
			effective[f] = true
		}
	}
	if insidersMode {
		for _, f := range InsidersFeatureFlags {
			effective[f] = true
		}
	}
	return effective
}
