package github

// MCPAppsFeatureFlag is the feature flag name for MCP Apps (interactive UI forms).
const MCPAppsFeatureFlag = "remote_mcp_ui_apps"

// FeatureFlagCSVOutput is the feature flag name for CSV output on list tools.
const FeatureFlagCSVOutput = "csv_output"

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
}

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
	InsidersMode bool
}

// ResolveFeatureFlags computes the effective set of enabled feature flags by:
//  1. Taking explicitly enabled features (from CLI flags or HTTP headers)
//  2. Adding insiders-expanded features when insiders mode is active
//  3. Validating all features against the AllowedFeatureFlags allowlist
//
// Returns a set (map) for O(1) lookup by the feature checker.
func ResolveFeatureFlags(enabledFeatures []string, insidersMode bool) map[string]bool {
	allowed := make(map[string]bool, len(AllowedFeatureFlags))
	for _, f := range AllowedFeatureFlags {
		allowed[f] = true
	}

	effective := make(map[string]bool)
	for _, f := range enabledFeatures {
		if allowed[f] {
			effective[f] = true
		}
	}
	if insidersMode {
		for _, f := range InsidersFeatureFlags {
			if allowed[f] {
				effective[f] = true
			}
		}
	}
	return effective
}
