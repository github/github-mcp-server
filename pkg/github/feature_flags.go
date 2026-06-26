package github

import "slices"

// MCPAppsFeatureFlag is the feature flag name for MCP Apps (interactive UI forms).
const MCPAppsFeatureFlag = "remote_mcp_ui_apps"

// FeatureFlagCSVOutput is the feature flag name for CSV output on list tools.
const FeatureFlagCSVOutput = "csv_output"

// FeatureFlagIFCLabels is the feature flag name for IFC security labels in tool results.
const FeatureFlagIFCLabels = "ifc_labels"

// FeatureFlagFileBlame is the feature flag name for the get_file_blame tool,
// which exposes git blame information for a file. It is gated so the extra tool
// is not advertised by default, keeping the tool surface small unless opted in.
const FeatureFlagFileBlame = "file_blame"

// FeatureFlagIssueDependencies is the feature flag name for the issue dependency
// tools (issue_dependency_read / issue_dependency_write), which read and edit an
// issue's blocked-by / blocking relationships. It is gated so these tools are not
// advertised in the default surface, keeping the fixed tool-schema cost small
// unless explicitly opted in.
const FeatureFlagIssueDependencies = "issue_dependencies"

// AllowedFeatureFlags is the allowlist of feature flags that can be enabled
// by users via --features CLI flag or X-MCP-Features HTTP header.
// Only flags in this list are accepted; unknown flags are silently ignored.
// This is the single source of truth for which flags are user-controllable.
var AllowedFeatureFlags = []string{
	MCPAppsFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagIFCLabels,
	FeatureFlagIssuesGranular,
	FeatureFlagPullRequestsGranular,
	FeatureFlagFileBlame,
	FeatureFlagIssueDependencies,
}

// InsidersFeatureFlags is the list of feature flags that insiders mode enables.
// When insiders mode is active, all flags in this list are treated as enabled.
// This is the single source of truth for what "insiders" means in terms of
// feature flag expansion.
var InsidersFeatureFlags = []string{
	MCPAppsFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagFileBlame,
	FeatureFlagIssueDependencies,
}

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
}

// ResolveFeatureFlags computes the effective set of enabled feature flags by:
//  1. Taking the user-supplied flags (from --features or X-MCP-Features) and
//     keeping only those present in AllowedFeatureFlags. Unknown or unsafe
//     flags from request input are silently dropped here.
//  2. If insiders mode is on, unioning in every flag from InsidersFeatureFlags.
//     Insiders is a server-controlled meta switch, so its expansion is NOT
//     re-validated against AllowedFeatureFlags.
//
// AllowedFeatureFlags and InsidersFeatureFlags are independent sets:
//   - A flag in AllowedFeatureFlags but not InsidersFeatureFlags is a regular
//     opt-in flag that insiders mode does not turn on automatically.
//   - A flag in InsidersFeatureFlags but not AllowedFeatureFlags is reachable
//     only through insiders mode and cannot be enabled by user input.
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
