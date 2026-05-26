package inventory

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ServerPrompt pairs a prompt with its toolset metadata.
type ServerPrompt struct {
	Prompt  mcp.Prompt
	Handler mcp.PromptHandler
	// Toolset identifies which toolset this prompt belongs to
	Toolset ToolsetMetadata
	// FeatureFlagEnable specifies feature flags that must all be enabled for this prompt
	// to be available. If any flag is not enabled, the prompt is omitted.
	FeatureFlagEnable []string
	// FeatureFlagDisable specifies feature flags that, when any is enabled, cause this prompt
	// to be omitted. Used to disable prompts when a feature flag is on.
	FeatureFlagDisable []string
}

// NewServerPrompt creates a new ServerPrompt with toolset metadata.
func NewServerPrompt(toolset ToolsetMetadata, prompt mcp.Prompt, handler mcp.PromptHandler) ServerPrompt {
	return ServerPrompt{
		Prompt:  prompt,
		Handler: handler,
		Toolset: toolset,
	}
}
