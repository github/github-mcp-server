package github

import (
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
)

// NewRegistry creates a Registry with all available tools, resources, and prompts.
// Tools, resources, and prompts are self-describing with their toolset metadata embedded.
// This function is stateless - no dependencies are captured.
// Handlers are generated on-demand during registration via RegisterAll(ctx, server, deps).
// The "default" keyword in WithToolsets will expand to toolsets marked with Default: true.
func NewRegistry(t translations.TranslationHelperFunc) *toolsets.Registry {
	return toolsets.NewRegistry().
		SetTools(AllTools(t)).
		SetResources(AllResources(t)).
		SetPrompts(AllPrompts(t))
}
