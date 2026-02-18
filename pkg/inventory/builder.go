package inventory

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

var (
	// ErrUnknownTools is returned when tools specified via WithTools() are not recognized.
	ErrUnknownTools = errors.New("unknown tools specified in WithTools")
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
	readOnly             bool
	toolsetIDs           []string // raw input, processed at Build()
	toolsetIDsIsNil      bool     // tracks if nil was passed (nil = defaults)
	additionalTools      []string // raw input, processed at Build()
	featureChecker       FeatureFlagChecker
	filters              []ToolFilter // filters to apply to all tools
	generateInstructions bool
	insidersMode         bool
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
	maps.Copy(b.deprecatedAliases, aliases)
	return b
}

// WithReadOnly sets whether only read-only tools should be available.
// When true, write tools are filtered out. Returns self for chaining.
func (b *Builder) WithReadOnly(readOnly bool) *Builder {
	b.readOnly = readOnly
	return b
}

func (b *Builder) WithServerInstructions() *Builder {
	b.generateInstructions = true
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
// Input is cleaned (trimmed, deduplicated) during Build().
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

// WithDisallowedTools specifies tools that should be disabled regardless of other settings.
// These tools will be excluded even if their toolset is enabled or they are in the
// additional tools list. This takes precedence over all other tool enablement settings.
// Input is cleaned (trimmed, deduplicated) before applying.
// Returns self for chaining.
func (b *Builder) WithDisallowedTools(toolNames []string) *Builder {
	cleaned := cleanTools(toolNames)
	if len(cleaned) > 0 {
		b.filters = append(b.filters, CreateDisallowedToolsFilter(cleaned))
	}
	return b
}

// WithInsidersMode enables or disables insiders mode features.
// When insiders mode is disabled (default), UI metadata is removed from tools
// so clients won't attempt to load UI resources.
// Returns self for chaining.
func (b *Builder) WithInsidersMode(enabled bool) *Builder {
	b.insidersMode = enabled
	return b
}

// CreateDisallowedToolsFilter creates a ToolFilter that excludes tools by name.
// Any tool whose name appears in the disallowed list will be filtered out.
// The input slice should already be cleaned (trimmed, deduplicated).
func CreateDisallowedToolsFilter(disallowed []string) ToolFilter {
	set := make(map[string]struct{}, len(disallowed))
	for _, name := range disallowed {
		set[name] = struct{}{}
	}
	return func(_ context.Context, tool *ServerTool) (bool, error) {
		_, blocked := set[tool.Tool.Name]
		return !blocked, nil
	}
}

// cleanTools trims whitespace and removes duplicates from tool names.
// Empty strings after trimming are excluded.
func cleanTools(tools []string) []string {
	seen := make(map[string]bool)
	var cleaned []string
	for _, name := range tools {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

// Build creates the final Inventory with all configuration applied.
// This processes toolset filtering, tool name resolution, and sets up
// the inventory for use. The returned Inventory is ready for use with
// AvailableTools(), RegisterAll(), etc.
//
// Build returns an error if any tools specified via WithTools() are not recognized
// (i.e., they don't exist in the tool set and are not deprecated aliases).
// This ensures invalid tool configurations fail fast at build time.
func (b *Builder) Build() (*Inventory, error) {
	// When insiders mode is disabled, strip insiders-only features from tools
	tools := b.tools
	if !b.insidersMode {
		tools = stripInsidersFeatures(b.tools)
	}

	r := &Inventory{
		tools:             tools,
		resourceTemplates: b.resourceTemplates,
		prompts:           b.prompts,
		deprecatedAliases: b.deprecatedAliases,
		readOnly:          b.readOnly,
		featureChecker:    b.featureChecker,
		filters:           b.filters,
	}

	// Process toolsets and pre-compute metadata in a single pass
	r.enabledToolsets, r.unrecognizedToolsets, r.toolsetIDs, r.toolsetIDSet, r.defaultToolsetIDs, r.toolsetDescriptions = b.processToolsets()

	// Build set of valid tool names for validation
	validToolNames := make(map[string]bool, len(tools))
	for i := range tools {
		validToolNames[tools[i].Tool.Name] = true
	}

	// Process additional tools (clean, resolve aliases, and track unrecognized)
	if len(b.additionalTools) > 0 {
		cleanedTools := cleanTools(b.additionalTools)

		r.additionalTools = make(map[string]bool, len(cleanedTools))
		var unrecognizedTools []string
		for _, name := range cleanedTools {
			// Always include the original name - this handles the case where
			// the tool exists but is controlled by a feature flag that's OFF.
			r.additionalTools[name] = true
			// Also include the canonical name if this is a deprecated alias.
			// This handles the case where the feature flag is ON and only
			// the new consolidated tool is available.
			if canonical, isAlias := b.deprecatedAliases[name]; isAlias {
				r.additionalTools[canonical] = true
			} else if !validToolNames[name] {
				// Not a valid tool and not a deprecated alias - track as unrecognized
				unrecognizedTools = append(unrecognizedTools, name)
			}
		}

		// Error out if there are unrecognized tools
		if len(unrecognizedTools) > 0 {
			return nil, fmt.Errorf("%w: %s", ErrUnknownTools, strings.Join(unrecognizedTools, ", "))
		}
	}

	if b.generateInstructions {
		r.instructions = generateInstructions(r)
	}

	return r, nil
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
	slices.Sort(allToolsetIDs)

	defaultToolsetIDList := make([]ToolsetID, 0, len(defaultIDs))
	for id := range defaultIDs {
		defaultToolsetIDList = append(defaultToolsetIDList, id)
	}
	slices.Sort(defaultToolsetIDList)

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

// insidersOnlyMetaKeys lists the Meta keys that are only available in insiders mode.
// Add new experimental feature keys here to have them automatically stripped
// when insiders mode is disabled.
var insidersOnlyMetaKeys = []string{
	"ui", // MCP Apps UI metadata
}

// stripInsidersFeatures removes insiders-only features from tools.
// This includes removing tools marked with InsidersOnly and stripping
// Meta keys listed in insidersOnlyMetaKeys from remaining tools.
func stripInsidersFeatures(tools []ServerTool) []ServerTool {
	result := make([]ServerTool, 0, len(tools))
	for _, tool := range tools {
		// Skip tools marked as insiders-only
		if tool.InsidersOnly {
			continue
		}
		if stripped := stripInsidersMetaFromTool(tool); stripped != nil {
			result = append(result, *stripped)
		} else {
			result = append(result, tool)
		}
	}
	return result
}

// stripInsidersMetaFromTool removes insiders-only Meta keys from a single tool.
// Returns a modified copy if changes were made, nil otherwise.
func stripInsidersMetaFromTool(tool ServerTool) *ServerTool {
	if tool.Tool.Meta == nil {
		return nil
	}

	// Check if any insiders-only keys exist
	hasInsidersKeys := false
	for _, key := range insidersOnlyMetaKeys {
		if tool.Tool.Meta[key] != nil {
			hasInsidersKeys = true
			break
		}
	}
	if !hasInsidersKeys {
		return nil
	}

	// Make a shallow copy and remove insiders-only keys
	toolCopy := tool
	newMeta := make(map[string]any, len(tool.Tool.Meta))
	for k, v := range tool.Tool.Meta {
		if !slices.Contains(insidersOnlyMetaKeys, k) {
			newMeta[k] = v
		}
	}

	if len(newMeta) == 0 {
		toolCopy.Tool.Meta = nil
	} else {
		toolCopy.Tool.Meta = newMeta
	}
	return &toolCopy
}
