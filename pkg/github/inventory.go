package github

import (
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

// InventoryConfig holds configuration for building an inventory with standard filters.
// This struct enables consistent inventory building across the codebase.
type InventoryConfig struct {
	Translator      translations.TranslationHelperFunc
	ReadOnly        bool
	Toolsets        []string // nil = use defaults, empty = none
	Tools           []string // additional specific tools
	EnabledFeatures []string // feature flags
}

// NewStandardBuilder creates an inventory builder with all standard filters applied.
// This is the canonical way to create an inventory builder, ensuring consistency
// between OAuth scope computation, server initialization, and CLI tools.
//
// The returned builder can be further customized (e.g., WithFilter for scope filtering)
// before calling Build().
func NewStandardBuilder(cfg InventoryConfig) *inventory.Builder {
	return NewInventory(cfg.Translator).
		WithDeprecatedAliases(DeprecatedToolAliases).
		WithReadOnly(cfg.ReadOnly).
		WithToolsets(cfg.Toolsets).
		WithTools(cfg.Tools).
		WithFeatureChecker(inventory.NewSliceFeatureChecker(cfg.EnabledFeatures))
}
