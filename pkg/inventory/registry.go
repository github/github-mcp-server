package inventory

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Inventory holds a collection of tools, resources, and prompts with filtering applied.
// Create a Inventory using Builder:
//
//	reg := NewBuilder().
//	    SetTools(tools).
//	    WithReadOnly(true).
//	    WithToolsets([]string{"repos"}).
//	    Build()
//
// The Inventory is configured at build time and provides:
//   - Filtered access to tools/resources/prompts via Available* methods
//   - Deterministic ordering for documentation generation
//   - Lazy dependency injection during registration via RegisterAll()
//   - Runtime toolset enabling for dynamic toolsets mode
type Inventory struct {
	// tools holds all tools in this group (ordered for iteration)
	tools []ServerTool
	// resourceTemplates holds all resource templates in this group (ordered for iteration)
	resourceTemplates []ServerResourceTemplate
	// prompts holds all prompts in this group (ordered for iteration)
	prompts []ServerPrompt
	// deprecatedAliases maps old tool names to new canonical names
	deprecatedAliases map[string]string

	// Pre-computed toolset metadata (set during Build)
	toolsetIDs          []ToolsetID          // sorted list of all toolset IDs
	toolsetIDSet        map[ToolsetID]bool   // set for O(1) HasToolset lookup
	defaultToolsetIDs   []ToolsetID          // sorted list of default toolset IDs
	toolsetDescriptions map[ToolsetID]string // toolset ID -> description

	// Filters - these control what's returned by Available* methods
	// readOnly when true filters out write tools
	readOnly bool
	// enabledToolsets when non-nil, only include tools/resources/prompts from these toolsets
	// when nil, all toolsets are enabled
	enabledToolsets map[ToolsetID]bool
	// additionalTools are specific tools that bypass toolset filtering (but still respect read-only)
	// These are additive - a tool is included if it matches toolset filters OR is in this set
	additionalTools map[string]bool
	// featureChecker when non-nil, checks if a feature flag is enabled.
	// Takes context and flag name, returns (enabled, error). If error, log and treat as false.
	// If checker is nil, all flag checks return false.
	featureChecker FeatureFlagChecker
	// filters are functions that will be applied to all tools during filtering.
	// If any filter returns false or an error, the tool is excluded.
	filters []ToolFilter
	// unrecognizedToolsets holds toolset IDs that were requested but don't match any registered toolsets
	unrecognizedToolsets []string
}

// UnrecognizedToolsets returns toolset IDs that were passed to WithToolsets but don't
// match any registered toolsets. This is useful for warning users about typos.
func (r *Inventory) UnrecognizedToolsets() []string {
	return r.unrecognizedToolsets
}

// MCP method constants for use with ForMCPRequest.
const (
	MCPMethodInitialize             = "initialize"
	MCPMethodToolsList              = "tools/list"
	MCPMethodToolsCall              = "tools/call"
	MCPMethodResourcesList          = "resources/list"
	MCPMethodResourcesRead          = "resources/read"
	MCPMethodResourcesTemplatesList = "resources/templates/list"
	MCPMethodPromptsList            = "prompts/list"
	MCPMethodPromptsGet             = "prompts/get"
)

// ForMCPRequest returns a Registry optimized for a specific MCP request.
// This is designed for servers that create a new instance per request (like the remote server),
// allowing them to only register the items needed for that specific request rather than all ~90 tools.
//
// Parameters:
//   - method: The MCP method being called (use MCP* constants)
//   - itemName: Name of specific item for call/get methods (tool name, resource URI, or prompt name)
//
// Returns a new Registry containing only the items relevant to the request:
//   - MCPMethodInitialize: Empty (capabilities are set via ServerOptions, not registration)
//   - MCPMethodToolsList: All available tools (no resources/prompts)
//   - MCPMethodToolsCall: Only the named tool
//   - MCPMethodResourcesList, MCPMethodResourcesTemplatesList: All available resources (no tools/prompts)
//   - MCPMethodResourcesRead: Only the named resource template
//   - MCPMethodPromptsList: All available prompts (no tools/resources)
//   - MCPMethodPromptsGet: Only the named prompt
//   - Unknown methods: Empty (no items registered)
//
// All existing filters (read-only, toolsets, etc.) still apply to the returned items.
func (r *Inventory) ForMCPRequest(method string, itemName string) *Inventory {
	// Create a shallow copy with shared filter settings
	// Note: lazy-init maps (toolsByName, etc.) are NOT copied - the new Registry
	// will initialize its own maps on first use if needed
	result := &Inventory{
		tools:                r.tools,
		resourceTemplates:    r.resourceTemplates,
		prompts:              r.prompts,
		deprecatedAliases:    r.deprecatedAliases,
		readOnly:             r.readOnly,
		enabledToolsets:      r.enabledToolsets, // shared, not modified
		additionalTools:      r.additionalTools, // shared, not modified
		featureChecker:       r.featureChecker,
		filters:              r.filters, // shared, not modified
		unrecognizedToolsets: r.unrecognizedToolsets,
	}

	// Helper to clear all item types
	clearAll := func() {
		result.tools = []ServerTool{}
		result.resourceTemplates = []ServerResourceTemplate{}
		result.prompts = []ServerPrompt{}
	}

	switch method {
	case MCPMethodInitialize:
		clearAll()
	case MCPMethodToolsList:
		result.resourceTemplates, result.prompts = nil, nil
	case MCPMethodToolsCall:
		result.resourceTemplates, result.prompts = nil, nil
		if itemName != "" {
			result.tools = r.filterToolsByName(itemName)
		}
	case MCPMethodResourcesList, MCPMethodResourcesTemplatesList:
		result.tools, result.prompts = nil, nil
	case MCPMethodResourcesRead:
		result.tools, result.prompts = nil, nil
		if itemName != "" {
			result.resourceTemplates = r.filterResourcesByURI(itemName)
		}
	case MCPMethodPromptsList:
		result.tools, result.resourceTemplates = nil, nil
	case MCPMethodPromptsGet:
		result.tools, result.resourceTemplates = nil, nil
		if itemName != "" {
			result.prompts = r.filterPromptsByName(itemName)
		}
	default:
		clearAll()
	}

	return result
}

// ToolsetIDs returns a sorted list of unique toolset IDs from all tools in this group.
func (r *Inventory) ToolsetIDs() []ToolsetID {
	return r.toolsetIDs
}

// DefaultToolsetIDs returns the IDs of toolsets marked as Default in their metadata.
// The IDs are returned in sorted order for deterministic output.
func (r *Inventory) DefaultToolsetIDs() []ToolsetID {
	return r.defaultToolsetIDs
}

// ToolsetDescriptions returns a map of toolset ID to description for all toolsets.
func (r *Inventory) ToolsetDescriptions() map[ToolsetID]string {
	return r.toolsetDescriptions
}

// RegisterTools registers all available tools with the server using the provided dependencies.
// The context is used for feature flag evaluation.
func (r *Inventory) RegisterTools(ctx context.Context, s *mcp.Server, deps any) {
	for _, tool := range r.AvailableTools(ctx) {
		tool.RegisterFunc(s, deps)
	}
}

// RegisterResourceTemplates registers all available resource templates with the server.
// The context is used for feature flag evaluation.
// Icons are automatically applied from the toolset metadata if not already set.
func (r *Inventory) RegisterResourceTemplates(ctx context.Context, s *mcp.Server, deps any) {
	for _, res := range r.AvailableResourceTemplates(ctx) {
		// Make a shallow copy to avoid mutating the original
		templateCopy := res.Template
		// Apply icons from toolset metadata if not already set
		if len(templateCopy.Icons) == 0 {
			templateCopy.Icons = res.Toolset.Icons()
		}
		s.AddResourceTemplate(&templateCopy, res.Handler(deps))
	}
}

// RegisterPrompts registers all available prompts with the server.
// The context is used for feature flag evaluation.
// Icons are automatically applied from the toolset metadata if not already set.
func (r *Inventory) RegisterPrompts(ctx context.Context, s *mcp.Server) {
	for _, prompt := range r.AvailablePrompts(ctx) {
		// Make a shallow copy to avoid mutating the original
		promptCopy := prompt.Prompt
		// Apply icons from toolset metadata if not already set
		if len(promptCopy.Icons) == 0 {
			promptCopy.Icons = prompt.Toolset.Icons()
		}
		s.AddPrompt(&promptCopy, prompt.Handler)
	}
}

// RegisterAll registers all available tools, resources, and prompts with the server.
// The context is used for feature flag evaluation.
func (r *Inventory) RegisterAll(ctx context.Context, s *mcp.Server, deps any) {
	r.RegisterTools(ctx, s, deps)
	r.RegisterResourceTemplates(ctx, s, deps)
	r.RegisterPrompts(ctx, s)
}

// ResolveToolAliases resolves deprecated tool aliases to their canonical names.
// It logs a warning to stderr for each deprecated alias that is resolved.
// Returns:
//   - resolved: tool names with aliases replaced by canonical names
//   - aliasesUsed: map of oldName → newName for each alias that was resolved
func (r *Inventory) ResolveToolAliases(toolNames []string) (resolved []string, aliasesUsed map[string]string) {
	resolved = make([]string, 0, len(toolNames))
	aliasesUsed = make(map[string]string)
	for _, toolName := range toolNames {
		if canonicalName, isAlias := r.deprecatedAliases[toolName]; isAlias {
			fmt.Fprintf(os.Stderr, "Warning: tool %q is deprecated, use %q instead\n", toolName, canonicalName)
			aliasesUsed[toolName] = canonicalName
			resolved = append(resolved, canonicalName)
		} else {
			resolved = append(resolved, toolName)
		}
	}
	return resolved, aliasesUsed
}

// FindToolByName searches all tools for one matching the given name.
// Returns the tool, its toolset ID, and an error if not found.
// This searches ALL tools regardless of filters.
func (r *Inventory) FindToolByName(toolName string) (*ServerTool, ToolsetID, error) {
	for i := range r.tools {
		if r.tools[i].Tool.Name == toolName {
			return &r.tools[i], r.tools[i].Toolset.ID, nil
		}
	}
	return nil, "", NewToolDoesNotExistError(toolName)
}

// HasToolset checks if any tool/resource/prompt belongs to the given toolset.
func (r *Inventory) HasToolset(toolsetID ToolsetID) bool {
	return r.toolsetIDSet[toolsetID]
}

// AllTools returns all tools without any filtering, sorted deterministically.
func (r *Inventory) AllTools() []ServerTool {
	result := slices.Clone(r.tools)

	// Sort deterministically: by toolset ID, then by tool name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Toolset.ID != result[j].Toolset.ID {
			return result[i].Toolset.ID < result[j].Toolset.ID
		}
		return result[i].Tool.Name < result[j].Tool.Name
	})

	return result
}

// AvailableToolsets returns the unique toolsets that have tools, in sorted order.
// This is the ordered intersection of toolsets with reality - only toolsets that
// actually contain tools are returned, sorted by toolset ID.
// Optional exclude parameter filters out specific toolset IDs from the result.
func (r *Inventory) AvailableToolsets(exclude ...ToolsetID) []ToolsetMetadata {
	tools := r.AllTools()
	if len(tools) == 0 {
		return nil
	}

	// Build exclude set for O(1) lookup
	excludeSet := make(map[ToolsetID]bool, len(exclude))
	for _, id := range exclude {
		excludeSet[id] = true
	}

	var result []ToolsetMetadata
	var lastID ToolsetID
	for _, tool := range tools {
		if tool.Toolset.ID != lastID {
			lastID = tool.Toolset.ID
			if !excludeSet[lastID] {
				result = append(result, tool.Toolset)
			}
		}
	}
	return result
}
