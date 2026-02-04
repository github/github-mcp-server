package github

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
	InsidersMode bool
}

// FeatureFlagDisableEmbeddedResources is a feature flag that when enabled,
// returns file contents as regular MCP content (TextContent/ImageContent)
// instead of EmbeddedResource. This provides better compatibility with
// clients that don't support embedded resources.
const FeatureFlagDisableEmbeddedResources = "MCP_DISABLE_EMBEDDED_RESOURCES"
