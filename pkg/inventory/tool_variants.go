package inventory

import "context"

// ToolOverride allows replacing a tool's definition based on runtime conditions.
// Use this for the small number of tools that have different schemas/handlers
// depending on features, capabilities, or environment.
type ToolOverride struct {
	// ToolName is the canonical tool name to override
	ToolName string

	// Condition returns true if this override should apply
	Condition func(ctx context.Context) (bool, error)

	// Override is the replacement tool definition
	Override ServerTool
}

// ToolOverrides is a simple map for the few tools that need variant handling.
// Key is the tool name, value is the override to check.
type ToolOverrides map[string]ToolOverride

// Apply checks if an override should be used for the given tool.
// Returns the override if condition matches, nil otherwise.
func (o ToolOverrides) Apply(ctx context.Context, toolName string) *ServerTool {
	override, ok := o[toolName]
	if !ok {
		return nil
	}

	if override.Condition == nil {
		return &override.Override
	}

	matches, err := override.Condition(ctx)
	if err != nil || !matches {
		return nil
	}

	return &override.Override
}

// ApplyToTools applies overrides to a list of tools, returning a new list
// with overridden tools replaced. Tools without overrides are unchanged.
// If no overrides match, returns the original slice (no allocation).
func (o ToolOverrides) ApplyToTools(ctx context.Context, tools []*ServerTool) []*ServerTool {
	if len(o) == 0 {
		return tools
	}

	// First pass: check if any overrides apply (avoid allocation if not)
	var result []*ServerTool
	for i, tool := range tools {
		override, hasOverride := o[tool.Tool.Name]
		if !hasOverride {
			if result != nil {
				result[i] = tool
			}
			continue
		}

		// Check condition
		var applies bool
		if override.Condition == nil {
			applies = true
		} else if matches, err := override.Condition(ctx); err == nil && matches {
			applies = true
		}

		if applies {
			// Lazy allocation only when we find a match
			if result == nil {
				result = make([]*ServerTool, len(tools))
				copy(result[:i], tools[:i])
			}
			result[i] = &override.Override
		} else if result != nil {
			result[i] = tool
		}
	}

	if result == nil {
		return tools // No overrides matched, return original
	}
	return result
}
