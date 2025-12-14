package github

import (
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
)

// NewToolsetGroup creates a ToolsetGroup with all available tools, resources, and prompts.
// Tools, resources, and prompts are self-describing with their toolset metadata embedded.
// This function is stateless - no dependencies are captured.
// Handlers are generated on-demand during registration via RegisterAll(ctx, server, deps).
// The "default" keyword in WithToolsets will expand to GetDefaultToolsetIDs().
func NewToolsetGroup(t translations.TranslationHelperFunc) *toolsets.ToolsetGroup {
	tsg := toolsets.NewToolsetGroup(
		AllTools(t),
		AllResources(t),
		AllPrompts(t),
	)
	tsg.SetDefaultToolsetIDs(GetDefaultToolsetIDs())
	return tsg
}
