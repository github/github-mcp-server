package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/skills"
	"github.com/github/github-mcp-server/v2/pkg/skills/catalog"
)

func skillsPublisher(ctx context.Context, cfg *MCPServerConfig, deps ToolDependencies, inv *inventory.Inventory) (*skills.Publisher, error) {
	if !deps.IsFeatureEnabled(ctx, FeatureFlagSkillsExtension) {
		return nil, nil
	}

	source := cfg.SkillsSource
	if source == nil {
		var err error
		source, err = catalog.Source()
		if err != nil {
			return nil, fmt.Errorf("load bundled skills: %w", err)
		}
	}
	available := make(map[string]bool)
	for _, tool := range inv.ConfiguredTools(ctx) {
		available[tool.Tool.Name] = true
	}
	filtered, err := source.ForTools(func(name string) bool { return available[name] })
	if err != nil {
		return nil, fmt.Errorf("filter skills: %w", err)
	}
	return skills.NewPublisher(filtered), nil
}
