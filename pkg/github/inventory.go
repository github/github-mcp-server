package github

import (
	"github.com/github/github-mcp-server/pkg/binding"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// NewInventory creates an Inventory with all available tools, resources, and prompts.
// Tools, resources, and prompts are self-describing with their toolset metadata embedded.
// This function is stateless - no dependencies are captured.
// Handlers are generated on-demand during registration via RegisterAll(ctx, server, deps).
// The "default" keyword in WithToolsets will expand to toolsets marked with Default: true.
func NewInventory(t translations.TranslationHelperFunc) *inventory.Builder {
	return inventory.NewBuilder().
		SetTools(AllTools(t)).
		SetResources(AllResources(t)).
		SetPrompts(AllPrompts(t))
}

// NewScopedInventory creates an Inventory builder for a context-scoped server
// mode (repo, pull_request, or project). The full tool universe is transformed
// by the binding layer into the bespoke surface for the bound context before
// it reaches the builder, so the manifest is the only thing that decides which
// tools appear and how. All other builder configuration (read-only, feature
// flags, toolsets) still applies on top of the scoped set, exactly as for
// NewInventory.
func NewScopedInventory(t translations.TranslationHelperFunc, ctx binding.Context) (*inventory.Builder, error) {
	tools, err := binding.ApplyTools(AllTools(t), ctx)
	if err != nil {
		return nil, err
	}
	return inventory.NewBuilder().
		SetTools(tools).
		SetResources(binding.ApplyResources(AllResources(t), ctx)).
		SetPrompts(binding.ApplyPrompts(AllPrompts(t), ctx)), nil
}
