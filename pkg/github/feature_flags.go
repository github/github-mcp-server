package github

// MCPAppsFeatureFlag is the feature flag name that enables MCP Apps
// (interactive UI forms) for supported tools. When enabled, tools like
// get_me, issue_write, and create_pull_request can render rich UI via
// the MCP Apps extension instead of plain text responses.
const MCPAppsFeatureFlag = "remote_mcp_ui_apps"

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
	InsidersMode bool
}
