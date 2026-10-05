package inventory

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ServerPrompt pairs a prompt with its toolset metadata.
type ServerPrompt struct {
	Prompt  mcp.Prompt
	Handler mcp.PromptHandler
	// Toolset identifies which toolset this prompt belongs to
	Toolset ToolsetMetadata
	// FeatureRule controls whether this prompt is available.
	FeatureRule FeatureRule
}

// NewServerPrompt creates a new ServerPrompt with toolset metadata.
func NewServerPrompt(toolset ToolsetMetadata, prompt mcp.Prompt, handler mcp.PromptHandler) ServerPrompt {
	return ServerPrompt{
		Prompt:  prompt,
		Handler: handler,
		Toolset: toolset,
	}
}
