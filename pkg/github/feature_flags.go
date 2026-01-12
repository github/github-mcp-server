package github

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
}

// RemoteMCPExperimental is a long-lived feature flag for experimental remote MCP features.
// This flag enables experimental behaviors in tools that are being tested for remote server deployment.
const RemoteMCPExperimental = "remote_mcp_experimental"
