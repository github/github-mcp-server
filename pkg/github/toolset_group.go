package github

import (
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
)

// NewToolsetGroup creates a ToolsetGroup with all available tools, resources, and prompts.
// Tools are self-describing with their toolset metadata embedded.
// The "default" keyword in WithToolsets will expand to GetDefaultToolsetIDs().
func NewToolsetGroup(t translations.TranslationHelperFunc, getClient GetClientFn, getRawClient raw.GetRawClientFn) *toolsets.ToolsetGroup {
	tsg := toolsets.NewToolsetGroup(
		AllTools(t),
		AllResources(t, getClient, getRawClient),
		AllPrompts(t),
	)
	tsg.SetDefaultToolsetIDs(GetDefaultToolsetIDs())
	return tsg
}
