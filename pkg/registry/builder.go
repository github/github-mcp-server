package registry

import (
	"strings"
)

// Builder builds a Registry with the specified configuration.
// Use NewBuilder to create a builder, chain configuration methods,
// then call Build() to create the final Registry.
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
}

// NewBuilder creates a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		deprecatedAliases: make(map[string]string),
		toolsetIDsIsNil:   true, // default to nil (use defaults)
	}
}

// SetTools sets the tools for the registry. Returns self for chaining.
func (b *Builder) SetTools(tools []ServerTool) *Builder {
	b.tools = tools
	return b
}

// SetResources sets the resource templates for the registry. Returns self for chaining.
func (b *Builder) SetResources(resources []ServerResourceTemplate) *Builder {
	b.resourceTemplates = resources
	return b
}

// SetPrompts sets the prompts for the registry. Returns self for chaining.
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

// Build creates the final Registry with all configuration applied.
// This processes toolset filtering, tool name resolution, and sets up
// the registry for use. The returned Registry is ready for use with
// AvailableTools(), RegisterAll(), etc.
func (b *Builder) Build() *Registry {
	r := &Registry{
		tools:             b.tools,
		resourceTemplates: b.resourceTemplates,
		prompts:           b.prompts,
		deprecatedAliases: b.deprecatedAliases,
		readOnly:          b.readOnly,
		featureChecker:    b.featureChecker,
	}

	// Process toolsets
	r.enabledToolsets, r.unrecognizedToolsets = b.processToolsets()

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
func (b *Builder) processToolsets() (map[ToolsetID]bool, []string) {
	// Build a set of valid toolset IDs for validation
	validIDs := make(map[ToolsetID]bool)
	for _, t := range b.tools {
		validIDs[t.Toolset.ID] = true
	}
	for _, r := range b.resourceTemplates {
		validIDs[r.Toolset.ID] = true
	}
	for _, p := range b.prompts {
		validIDs[p.Toolset.ID] = true
	}

	toolsetIDs := b.toolsetIDs

	// Check for "all" keyword - enables all toolsets
	for _, id := range toolsetIDs {
		if strings.TrimSpace(id) == "all" {
			return nil, nil // nil means all enabled
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
			for _, defaultID := range b.defaultToolsetIDs() {
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
		return make(map[ToolsetID]bool), unrecognized
	}

	enabledToolsets := make(map[ToolsetID]bool, len(expanded))
	for _, id := range expanded {
		enabledToolsets[id] = true
	}
	return enabledToolsets, unrecognized
}

// defaultToolsetIDs returns toolset IDs marked as Default in their metadata.
func (b *Builder) defaultToolsetIDs() []ToolsetID {
	seen := make(map[ToolsetID]bool)
	for i := range b.tools {
		if b.tools[i].Toolset.Default {
			seen[b.tools[i].Toolset.ID] = true
		}
	}
	for i := range b.resourceTemplates {
		if b.resourceTemplates[i].Toolset.Default {
			seen[b.resourceTemplates[i].Toolset.ID] = true
		}
	}
	for i := range b.prompts {
		if b.prompts[i].Toolset.Default {
			seen[b.prompts[i].Toolset.ID] = true
		}
	}

	ids := make([]ToolsetID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}
