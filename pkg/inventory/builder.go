package inventory

import (
	"context"
	"sort"
	"strings"
)

// ToolFilter is a function that determines if a tool should be included.
// Returns true if the tool should be included, false to exclude it.
type ToolFilter func(ctx context.Context, tool *ServerTool) (bool, error)

// Builder builds a Registry with the specified configuration.
// Use NewBuilder to create a builder, chain configuration methods,
// then call Build() to create the final inventory.
//
// Example:
//
//	reg := NewBuilder().
//	    SetTools(tools).
//	    SetResources(resources).
//	    SetPrompts(prompts).
//	    WithDeprecatedAliases(aliases).
//	    WithReadOnly(true).
//	    WithToolsets([]string{"repos", "issues"}).
//	    WithFeatureChecker(checker).
//	    WithFilter(myFilter).
//	    Build()
type Builder struct {
	tools             []ServerTool
	resourceTemplates []ServerResourceTemplate
	prompts           []ServerPrompt
	deprecatedAliases map[string]string

	// Configuration options (processed at Build time)
	readOnly        bool
	toolsetIDs      []string // raw input, processed at Build()
	toolsetIDsIsNil bool     // tracks if nil was passed (nil = defaults)
	additionalTools []string // raw input, processed at Build()
	featureChecker  FeatureFlagChecker
	filters         []ToolFilter // filters to apply to all tools
}

// NewBuilder creates a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		deprecatedAliases: make(map[string]string),
		toolsetIDsIsNil:   true, // default to nil (use defaults)
	}
}

// SetTools sets the tools for the inventory. Returns self for chaining.
func (b *Builder) SetTools(tools []ServerTool) *Builder {
	b.tools = tools
	return b
}

// SetResources sets the resource templates for the inventory. Returns self for chaining.
func (b *Builder) SetResources(resources []ServerResourceTemplate) *Builder {
	b.resourceTemplates = resources
	return b
}

// SetPrompts sets the prompts for the inventory. Returns self for chaining.
func (b *Builder) SetPrompts(prompts []ServerPrompt) *Builder {
	b.prompts = prompts
	return b
}

// WithDeprecatedAliases adds deprecated tool name aliases that map to canonical names.
// Returns self for chaining.
func (b *Builder) WithDeprecatedAliases(aliases map[string]string) *Builder {
	for oldName, newName := range aliases {
		b.deprecatedAliases[oldName] = newName
	}
	return b
}

// WithReadOnly sets whether only read-only tools should be available.
// When true, write tools are filtered out. Returns self for chaining.
func (b *Builder) WithReadOnly(readOnly bool) *Builder {
	b.readOnly = readOnly
	return b
}

// WithToolsets specifies which toolsets should be enabled.
// Special keywords:
//   - "all": enables all toolsets
//   - "default": expands to toolsets marked with Default: true in their metadata
//
// Input strings are trimmed of whitespace and duplicates are removed.
// Pass nil to use default toolsets. Pass an empty slice to disable all toolsets
// (useful for dynamic toolsets mode where tools are enabled on demand).
// Returns self for chaining.
func (b *Builder) WithToolsets(toolsetIDs []string) *Builder {
	b.toolsetIDs = toolsetIDs
	b.toolsetIDsIsNil = toolsetIDs == nil
	return b
}

// WithTools specifies additional tools that bypass toolset filtering.
// These tools are additive - they will be included even if their toolset is not enabled.
// Read-only filtering still applies to these tools.
// Deprecated tool aliases are automatically resolved to their canonical names during Build().
// Returns self for chaining.
func (b *Builder) WithTools(toolNames []string) *Builder {
	b.additionalTools = toolNames
	return b
}

// WithFeatureChecker sets the feature flag checker function.
// The checker receives a context (for actor extraction) and feature flag name,
// returns (enabled, error). If error occurs, it will be logged and treated as false.
// If checker is nil, all feature flag checks return false.
// Returns self for chaining.
func (b *Builder) WithFeatureChecker(checker FeatureFlagChecker) *Builder {
	b.featureChecker = checker
	return b
}

// WithFilter adds a filter function that will be applied to all tools.
// Multiple filters can be added and are evaluated in order.
// If any filter returns false or an error, the tool is excluded.
// Returns self for chaining.
func (b *Builder) WithFilter(filter ToolFilter) *Builder {
	b.filters = append(b.filters, filter)
	return b
}

// Build creates the final Inventory with all configuration applied.
// This processes toolset filtering, tool name resolution, and sets up
// the inventory for use. The returned Inventory is ready for use with
// AvailableTools(), RegisterAll(), etc.
func (b *Builder) Build() *Inventory {
	r := &Inventory{
		tools:             b.tools,
		resourceTemplates: b.resourceTemplates,
		prompts:           b.prompts,
		deprecatedAliases: b.deprecatedAliases,
		readOnly:          b.readOnly,
		featureChecker:    b.featureChecker,
		filters:           b.filters,
	}

	// Process toolsets and pre-compute metadata in a single pass
	r.enabledToolsets, r.unrecognizedToolsets, r.toolsetIDs, r.toolsetIDSet, r.defaultToolsetIDs, r.toolsetDescriptions = b.processToolsets()

	// Process additional tools (resolve aliases)
	if len(b.additionalTools) > 0 {
		r.additionalTools = make(map[string]bool, len(b.additionalTools))
		for _, name := range b.additionalTools {
			// Resolve deprecated aliases to canonical names
			if canonical, isAlias := b.deprecatedAliases[name]; isAlias {
				r.additionalTools[canonical] = true
			} else {
				r.additionalTools[name] = true
			}
		}
	}

	return r
}

// processToolsets processes the toolsetIDs configuration and returns:
// - enabledToolsets map (nil means all enabled)
// - unrecognizedToolsets list for warnings
// - allToolsetIDs sorted list of all toolset IDs
// - toolsetIDSet map for O(1) HasToolset lookup
// - defaultToolsetIDs sorted list of default toolset IDs
// - toolsetDescriptions map of toolset ID to description
func (b *Builder) processToolsets() (map[ToolsetID]bool, []string, []ToolsetID, map[ToolsetID]bool, []ToolsetID, map[ToolsetID]string) {
	// Single pass: collect all toolset metadata together
	validIDs := make(map[ToolsetID]bool)
	defaultIDs := make(map[ToolsetID]bool)
	descriptions := make(map[ToolsetID]string)

	for i := range b.tools {
		t := &b.tools[i]
		validIDs[t.Toolset.ID] = true
		if t.Toolset.Default {
			defaultIDs[t.Toolset.ID] = true
		}
		if t.Toolset.Description != "" {
			descriptions[t.Toolset.ID] = t.Toolset.Description
		}
	}
	for i := range b.resourceTemplates {
		r := &b.resourceTemplates[i]
		validIDs[r.Toolset.ID] = true
		if r.Toolset.Default {
			defaultIDs[r.Toolset.ID] = true
		}
		if r.Toolset.Description != "" {
			descriptions[r.Toolset.ID] = r.Toolset.Description
		}
	}
	for i := range b.prompts {
		p := &b.prompts[i]
		validIDs[p.Toolset.ID] = true
		if p.Toolset.Default {
			defaultIDs[p.Toolset.ID] = true
		}
		if p.Toolset.Description != "" {
			descriptions[p.Toolset.ID] = p.Toolset.Description
		}
	}

	// Build sorted slices from the collected maps
	allToolsetIDs := make([]ToolsetID, 0, len(validIDs))
	for id := range validIDs {
		allToolsetIDs = append(allToolsetIDs, id)
	}
	sort.Slice(allToolsetIDs, func(i, j int) bool { return allToolsetIDs[i] < allToolsetIDs[j] })

	defaultToolsetIDList := make([]ToolsetID, 0, len(defaultIDs))
	for id := range defaultIDs {
		defaultToolsetIDList = append(defaultToolsetIDList, id)
	}
	sort.Slice(defaultToolsetIDList, func(i, j int) bool { return defaultToolsetIDList[i] < defaultToolsetIDList[j] })

	toolsetIDs := b.toolsetIDs

	// Check for "all" keyword - enables all toolsets
	for _, id := range toolsetIDs {
		if strings.TrimSpace(id) == "all" {
			return nil, nil, allToolsetIDs, validIDs, defaultToolsetIDList, descriptions // nil means all enabled
		}
	}

	// nil means use defaults, empty slice means no toolsets
	if b.toolsetIDsIsNil {
		toolsetIDs = []string{"default"}
	}

	// Expand "default" keyword, trim whitespace, collect other IDs, and track unrecognized
	seen := make(map[ToolsetID]bool)
	expanded := make([]ToolsetID, 0, len(toolsetIDs))
	var unrecognized []string

	for _, id := range toolsetIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if trimmed == "default" {
			for _, defaultID := range defaultToolsetIDList {
				if !seen[defaultID] {
					seen[defaultID] = true
					expanded = append(expanded, defaultID)
				}
			}
		} else {
			tsID := ToolsetID(trimmed)
			if !seen[tsID] {
				seen[tsID] = true
				expanded = append(expanded, tsID)
				// Track if this toolset doesn't exist
				if !validIDs[tsID] {
					unrecognized = append(unrecognized, trimmed)
				}
			}
		}
	}

	if len(expanded) == 0 {
		return make(map[ToolsetID]bool), unrecognized, allToolsetIDs, validIDs, defaultToolsetIDList, descriptions
	}

	enabledToolsets := make(map[ToolsetID]bool, len(expanded))
	for _, id := range expanded {
		enabledToolsets[id] = true
	}
	return enabledToolsets, unrecognized, allToolsetIDs, validIDs, defaultToolsetIDList, descriptions
}
