package inventory

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ServerPrompt pairs a prompt with its toolset metadata.
type ServerPrompt struct {
	Prompt  mcp.Prompt
	Handler mcp.PromptHandler
	// Toolset identifies which toolset this prompt belongs to
	Toolset ToolsetMetadata
	// FeatureFlagEnable specifies a feature flag that must be enabled for this prompt
	// to be available. If set and the flag is not enabled, the prompt is omitted.
	FeatureFlagEnable string
	// FeatureFlagDisable specifies a feature flag that, when enabled, causes this prompt
	// to be omitted. Used to disable prompts when a feature flag is on.
	FeatureFlagDisable string
}

// NewServerPrompt creates a new ServerPrompt with toolset metadata.
func NewServerPrompt(toolset ToolsetMetadata, prompt mcp.Prompt, handler mcp.PromptHandler) ServerPrompt {
	return ServerPrompt{
		Prompt:  prompt,
		Handler: handler,
		Toolset: toolset,
	}
}
