package toolsets

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolsetDoesNotExistError struct {
	Name string
}

func (e *ToolsetDoesNotExistError) Error() string {
	return fmt.Sprintf("toolset %s does not exist", e.Name)
}

func (e *ToolsetDoesNotExistError) Is(target error) bool {
	if target == nil {
		return false
	}
	if _, ok := target.(*ToolsetDoesNotExistError); ok {
		return true
	}
	return false
}

func NewToolsetDoesNotExistError(name string) *ToolsetDoesNotExistError {
	return &ToolsetDoesNotExistError{Name: name}
}

// ToolDoesNotExistError is returned when a tool is not found.
type ToolDoesNotExistError struct {
	Name string
}

func (e *ToolDoesNotExistError) Error() string {
	return fmt.Sprintf("tool %s does not exist", e.Name)
}

// NewToolDoesNotExistError creates a new ToolDoesNotExistError.
func NewToolDoesNotExistError(name string) *ToolDoesNotExistError {
	return &ToolDoesNotExistError{Name: name}
}

// ServerTool is defined in server_tool.go

// ResourceHandlerFunc is a function that takes dependencies and returns an MCP resource handler.
// This allows resources to be defined statically while their handlers are generated
// on-demand with the appropriate dependencies.
type ResourceHandlerFunc func(deps any) mcp.ResourceHandler

// ServerResourceTemplate pairs a resource template with its toolset metadata.
type ServerResourceTemplate struct {
	Template mcp.ResourceTemplate
	// HandlerFunc generates the handler when given dependencies.
	// This allows resources to be passed around without handlers being set up,
	// and handlers are only created when needed.
	HandlerFunc ResourceHandlerFunc
	// Toolset identifies which toolset this resource belongs to
	Toolset ToolsetMetadata
	// FeatureFlagEnable specifies a feature flag that must be enabled for this resource
	// to be available. If set and the flag is not enabled, the resource is omitted.
	FeatureFlagEnable string
	// FeatureFlagDisable specifies a feature flag that, when enabled, causes this resource
	// to be omitted. Used to disable resources when a feature flag is on.
	FeatureFlagDisable string
}

// HasHandler returns true if this resource has a handler function.
func (sr *ServerResourceTemplate) HasHandler() bool {
	return sr.HandlerFunc != nil
}

// Handler returns a resource handler by calling HandlerFunc with the given dependencies.
// Panics if HandlerFunc is nil - all resources should have handlers.
func (sr *ServerResourceTemplate) Handler(deps any) mcp.ResourceHandler {
	if sr.HandlerFunc == nil {
		panic("HandlerFunc is nil for resource: " + sr.Template.Name)
	}
	return sr.HandlerFunc(deps)
}

// NewServerResourceTemplate creates a new ServerResourceTemplate with toolset metadata.
func NewServerResourceTemplate(toolset ToolsetMetadata, resourceTemplate mcp.ResourceTemplate, handlerFn ResourceHandlerFunc) ServerResourceTemplate {
	return ServerResourceTemplate{
		Template:    resourceTemplate,
		HandlerFunc: handlerFn,
		Toolset:     toolset,
	}
}

// ServerPrompt pairs a prompt with its toolset metadata.
type ServerPrompt struct {
	Prompt  mcp.Prompt
	Handler mcp.PromptHandler
	// Toolset identifies which toolset this prompt belongs to
	Toolset ToolsetMetadata
	// FeatureFlagEnable specifies a feature flag that must be enabled for this prompt
	// to be available. If set and the flag is not enabled, the prompt is omitted.
	FeatureFlagEnable string
	// FeatureFlagDisable specifies a feature flag that, when enabled, causes this prompt
	// to be omitted. Used to disable prompts when a feature flag is on.
	FeatureFlagDisable string
}

// NewServerPrompt creates a new ServerPrompt with toolset metadata.
func NewServerPrompt(toolset ToolsetMetadata, prompt mcp.Prompt, handler mcp.PromptHandler) ServerPrompt {
	return ServerPrompt{
		Prompt:  prompt,
		Handler: handler,
		Toolset: toolset,
	}
}

// Registry holds a collection of tools, resources, and prompts.
// It supports immutable filtering operations that return new Registrys
// without modifying the original. This design allows for:
//   - Building a full set of tools/resources/prompts once
//   - Applying filters (read-only, feature flags, enabled toolsets) without mutation
//   - Deterministic ordering for documentation generation
//   - Lazy dependency injection only when registering with a server
type Registry struct {
	// tools holds all tools in this group
	tools []ServerTool
	// resourceTemplates holds all resource templates in this group
	resourceTemplates []ServerResourceTemplate
	// prompts holds all prompts in this group
	prompts []ServerPrompt
	// deprecatedAliases maps old tool names to new canonical names
	deprecatedAliases map[string]string

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
	// unrecognizedToolsets holds toolset IDs that were requested but don't match any registered toolsets
	unrecognizedToolsets []string
}

// FeatureFlagChecker is a function that checks if a feature flag is enabled.
// The context can be used to extract actor/user information for flag evaluation.
// Returns (enabled, error). If error occurs, the caller should log and treat as false.
type FeatureFlagChecker func(ctx context.Context, flagName string) (bool, error)

// NewRegistry creates a new empty Registry.
// Use SetTools, SetResources, SetPrompts to populate it.
func NewRegistry() *Registry {
	return &Registry{
		deprecatedAliases: make(map[string]string),
	}
}

// SetTools sets the tools for this group. Returns self for chaining.
func (r *Registry) SetTools(tools []ServerTool) *Registry {
	r.tools = tools
	return r
}

// SetResources sets the resource templates for this group. Returns self for chaining.
func (r *Registry) SetResources(resources []ServerResourceTemplate) *Registry {
	r.resourceTemplates = resources
	return r
}

// SetPrompts sets the prompts for this group. Returns self for chaining.
func (r *Registry) SetPrompts(prompts []ServerPrompt) *Registry {
	r.prompts = prompts
	return r
}

// copy creates a shallow copy of the Registry for immutable operations.
func (r *Registry) copy() *Registry {
	newTG := &Registry{
		tools:             r.tools, // slices are shared (immutable)
		resourceTemplates: r.resourceTemplates,
		prompts:           r.prompts,
		deprecatedAliases: r.deprecatedAliases,
		readOnly:          r.readOnly,
		featureChecker:    r.featureChecker,
	}

	// Copy maps if they exist
	if r.enabledToolsets != nil {
		newTG.enabledToolsets = make(map[ToolsetID]bool, len(r.enabledToolsets))
		for k, v := range r.enabledToolsets {
			newTG.enabledToolsets[k] = v
		}
	}
	if r.additionalTools != nil {
		newTG.additionalTools = make(map[string]bool, len(r.additionalTools))
		for k, v := range r.additionalTools {
			newTG.additionalTools[k] = v
		}
	}
	newTG.unrecognizedToolsets = r.unrecognizedToolsets

	return newTG
}

// WithReadOnly returns a new Registry with read-only mode set.
// When true, write tools are filtered out from Available* methods.
func (r *Registry) WithReadOnly(readOnly bool) *Registry {
	newTG := r.copy()
	newTG.readOnly = readOnly
	return newTG
}

// WithToolsets returns a new Registry that only includes items from the specified toolsets.
// Special keywords:
//   - "all": enables all toolsets
//   - "default": expands to toolsets marked with Default: true in their metadata
//
// Input strings are trimmed of whitespace and duplicates are removed.
// Toolset IDs that don't match any registered toolsets are tracked and can be
// retrieved via UnrecognizedToolsets() for warning purposes.
//
// Pass nil to use default toolsets. Pass an empty slice to disable all toolsets
// (useful for dynamic toolsets mode where tools are enabled on demand).
func (r *Registry) WithToolsets(toolsetIDs []string) *Registry {
	newTG := r.copy()
	newTG.unrecognizedToolsets = nil // reset for fresh calculation

	// Build a set of valid toolset IDs for validation
	validIDs := make(map[ToolsetID]bool)
	for _, t := range r.tools {
		validIDs[t.Toolset.ID] = true
	}
	for _, r := range r.resourceTemplates {
		validIDs[r.Toolset.ID] = true
	}
	for _, p := range r.prompts {
		validIDs[p.Toolset.ID] = true
	}

	// Check for "all" keyword - enables all toolsets
	for _, id := range toolsetIDs {
		if strings.TrimSpace(id) == "all" {
			newTG.enabledToolsets = nil
			return newTG
		}
	}

	// nil means use defaults, empty slice means no toolsets
	if toolsetIDs == nil {
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
			for _, defaultID := range r.DefaultToolsetIDs() {
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

	newTG.unrecognizedToolsets = unrecognized

	if len(expanded) == 0 {
		newTG.enabledToolsets = make(map[ToolsetID]bool)
		return newTG
	}

	newTG.enabledToolsets = make(map[ToolsetID]bool, len(expanded))
	for _, id := range expanded {
		newTG.enabledToolsets[id] = true
	}
	return newTG
}

// UnrecognizedToolsets returns toolset IDs that were passed to WithToolsets but don't
// match any registered toolsets. This is useful for warning users about typos.
func (r *Registry) UnrecognizedToolsets() []string {
	return r.unrecognizedToolsets
}

// WithTools returns a new Registry with additional tools that bypass toolset filtering.
// These tools are additive - they will be included even if their toolset is not enabled.
// Read-only filtering still applies to these tools.
// Deprecated tool aliases are automatically resolved to their canonical names.
// Pass nil or empty slice to clear additional tools.
func (r *Registry) WithTools(toolNames []string) *Registry {
	newTG := r.copy()
	if len(toolNames) == 0 {
		newTG.additionalTools = nil
		return newTG
	}
	newTG.additionalTools = make(map[string]bool, len(toolNames))
	for _, name := range toolNames {
		// Resolve deprecated aliases to canonical names
		if canonical, isAlias := r.deprecatedAliases[name]; isAlias {
			newTG.additionalTools[canonical] = true
		} else {
			newTG.additionalTools[name] = true
		}
	}
	return newTG
}

// WithFeatureChecker returns a new Registry with a feature checker function.
// The checker receives a context (for actor extraction) and feature flag name, returns (enabled, error).
// If error occurs, it will be logged and treated as false.
// If checker is nil, all feature flag checks return false (items with FeatureFlagEnable are excluded,
// items with FeatureFlagDisable are included).
func (r *Registry) WithFeatureChecker(checker FeatureFlagChecker) *Registry {
	newTG := r.copy()
	newTG.featureChecker = checker
	return newTG
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
func (r *Registry) ForMCPRequest(method string, itemName string) *Registry {
	result := r.copy()

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

// filterToolsByName returns tools matching the given name, checking deprecated aliases.
// Returns from the current tools slice (respects existing filter chain).
func (r *Registry) filterToolsByName(name string) []ServerTool {
	// First check for exact match
	for i := range r.tools {
		if r.tools[i].Tool.Name == name {
			return []ServerTool{r.tools[i]}
		}
	}
	// Check if name is a deprecated alias
	if canonical, isAlias := r.deprecatedAliases[name]; isAlias {
		for i := range r.tools {
			if r.tools[i].Tool.Name == canonical {
				return []ServerTool{r.tools[i]}
			}
		}
	}
	return []ServerTool{}
}

// filterResourcesByURI returns resource templates matching the given URI pattern.
func (r *Registry) filterResourcesByURI(uri string) []ServerResourceTemplate {
	for i := range r.resourceTemplates {
		// Check if URI matches the template pattern (exact match on URITemplate string)
		if r.resourceTemplates[i].Template.URITemplate == uri {
			return []ServerResourceTemplate{r.resourceTemplates[i]}
		}
	}
	return []ServerResourceTemplate{}
}

// filterPromptsByName returns prompts matching the given name.
func (r *Registry) filterPromptsByName(name string) []ServerPrompt {
	for i := range r.prompts {
		if r.prompts[i].Prompt.Name == name {
			return []ServerPrompt{r.prompts[i]}
		}
	}
	return []ServerPrompt{}
}

// WithDeprecatedToolAliases returns a new Registry with the given deprecated aliases added.
// Aliases map old tool names to new canonical names.
func (r *Registry) WithDeprecatedToolAliases(aliases map[string]string) *Registry {
	newTG := r.copy()
	// Ensure we have a fresh map
	newTG.deprecatedAliases = make(map[string]string, len(r.deprecatedAliases)+len(aliases))
	for k, v := range r.deprecatedAliases {
		newTG.deprecatedAliases[k] = v
	}
	for oldName, newName := range aliases {
		newTG.deprecatedAliases[oldName] = newName
	}
	return newTG
}

// isToolsetEnabled checks if a toolset is enabled based on current filters.
func (r *Registry) isToolsetEnabled(toolsetID ToolsetID) bool {
	// Check enabled toolsets filter
	if r.enabledToolsets != nil {
		return r.enabledToolsets[toolsetID]
	}
	return true
}

// checkFeatureFlag checks a feature flag using the feature checker.
// Returns false if checker is nil or returns an error (errors are logged).
func (r *Registry) checkFeatureFlag(ctx context.Context, flagName string) bool {
	if r.featureChecker == nil || flagName == "" {
		return false
	}
	enabled, err := r.featureChecker(ctx, flagName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", flagName, err)
		return false
	}
	return enabled
}

// isFeatureFlagAllowed checks if an item passes feature flag filtering.
// - If FeatureFlagEnable is set, the item is only allowed if the flag is enabled
// - If FeatureFlagDisable is set, the item is excluded if the flag is enabled
func (r *Registry) isFeatureFlagAllowed(ctx context.Context, enableFlag, disableFlag string) bool {
	// Check enable flag - item requires this flag to be on
	if enableFlag != "" && !r.checkFeatureFlag(ctx, enableFlag) {
		return false
	}
	// Check disable flag - item is excluded if this flag is on
	if disableFlag != "" && r.checkFeatureFlag(ctx, disableFlag) {
		return false
	}
	return true
}

// isToolEnabled checks if a specific tool is enabled based on current filters.
func (r *Registry) isToolEnabled(ctx context.Context, tool *ServerTool) bool {
	// Check read-only filter first (applies to all tools)
	if r.readOnly && !tool.IsReadOnly() {
		return false
	}
	// Check feature flags
	if !r.isFeatureFlagAllowed(ctx, tool.FeatureFlagEnable, tool.FeatureFlagDisable) {
		return false
	}
	// Check if tool is in additionalTools (bypasses toolset filter)
	if r.additionalTools != nil && r.additionalTools[tool.Tool.Name] {
		return true
	}
	// Check toolset filter
	if !r.isToolsetEnabled(tool.Toolset.ID) {
		return false
	}
	return true
}

// AvailableTools returns the tools that pass all current filters,
// sorted deterministically by toolset ID, then tool name.
// The context is used for feature flag evaluation.
func (r *Registry) AvailableTools(ctx context.Context) []ServerTool {
	var result []ServerTool
	for i := range r.tools {
		tool := &r.tools[i]
		if r.isToolEnabled(ctx, tool) {
			result = append(result, *tool)
		}
	}

	// Sort deterministically: by toolset ID, then by tool name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Toolset.ID != result[j].Toolset.ID {
			return result[i].Toolset.ID < result[j].Toolset.ID
		}
		return result[i].Tool.Name < result[j].Tool.Name
	})

	return result
}

// AvailableResourceTemplates returns resource templates that pass all current filters,
// sorted deterministically by toolset ID, then template name.
// The context is used for feature flag evaluation.
func (r *Registry) AvailableResourceTemplates(ctx context.Context) []ServerResourceTemplate {
	var result []ServerResourceTemplate
	for i := range r.resourceTemplates {
		res := &r.resourceTemplates[i]
		// Check feature flags
		if !r.isFeatureFlagAllowed(ctx, res.FeatureFlagEnable, res.FeatureFlagDisable) {
			continue
		}
		if r.isToolsetEnabled(res.Toolset.ID) {
			result = append(result, *res)
		}
	}

	// Sort deterministically: by toolset ID, then by template name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Toolset.ID != result[j].Toolset.ID {
			return result[i].Toolset.ID < result[j].Toolset.ID
		}
		return result[i].Template.Name < result[j].Template.Name
	})

	return result
}

// AvailablePrompts returns prompts that pass all current filters,
// sorted deterministically by toolset ID, then prompt name.
// The context is used for feature flag evaluation.
func (r *Registry) AvailablePrompts(ctx context.Context) []ServerPrompt {
	var result []ServerPrompt
	for i := range r.prompts {
		prompt := &r.prompts[i]
		// Check feature flags
		if !r.isFeatureFlagAllowed(ctx, prompt.FeatureFlagEnable, prompt.FeatureFlagDisable) {
			continue
		}
		if r.isToolsetEnabled(prompt.Toolset.ID) {
			result = append(result, *prompt)
		}
	}

	// Sort deterministically: by toolset ID, then by prompt name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Toolset.ID != result[j].Toolset.ID {
			return result[i].Toolset.ID < result[j].Toolset.ID
		}
		return result[i].Prompt.Name < result[j].Prompt.Name
	})

	return result
}

// ToolsetIDs returns a sorted list of unique toolset IDs from all tools in this group.
func (r *Registry) ToolsetIDs() []ToolsetID {
	seen := make(map[ToolsetID]bool)
	for i := range r.tools {
		seen[r.tools[i].Toolset.ID] = true
	}
	for i := range r.resourceTemplates {
		seen[r.resourceTemplates[i].Toolset.ID] = true
	}
	for i := range r.prompts {
		seen[r.prompts[i].Toolset.ID] = true
	}

	ids := make([]ToolsetID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// DefaultToolsetIDs returns the IDs of toolsets marked as Default in their metadata.
// The IDs are returned in sorted order for deterministic output.
func (r *Registry) DefaultToolsetIDs() []ToolsetID {
	seen := make(map[ToolsetID]bool)
	for i := range r.tools {
		if r.tools[i].Toolset.Default {
			seen[r.tools[i].Toolset.ID] = true
		}
	}
	for i := range r.resourceTemplates {
		if r.resourceTemplates[i].Toolset.Default {
			seen[r.resourceTemplates[i].Toolset.ID] = true
		}
	}
	for i := range r.prompts {
		if r.prompts[i].Toolset.Default {
			seen[r.prompts[i].Toolset.ID] = true
		}
	}

	ids := make([]ToolsetID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// ToolsetDescriptions returns a map of toolset ID to description for all toolsets.
func (r *Registry) ToolsetDescriptions() map[ToolsetID]string {
	descriptions := make(map[ToolsetID]string)
	for i := range r.tools {
		t := &r.tools[i]
		if t.Toolset.Description != "" {
			descriptions[t.Toolset.ID] = t.Toolset.Description
		}
	}
	for i := range r.resourceTemplates {
		r := &r.resourceTemplates[i]
		if r.Toolset.Description != "" {
			descriptions[r.Toolset.ID] = r.Toolset.Description
		}
	}
	for i := range r.prompts {
		p := &r.prompts[i]
		if p.Toolset.Description != "" {
			descriptions[p.Toolset.ID] = p.Toolset.Description
		}
	}
	return descriptions
}

// ToolsForToolset returns all tools belonging to a specific toolset.
// This method bypasses the toolset enabled filter (for dynamic toolset registration),
// but still respects the read-only filter.
func (r *Registry) ToolsForToolset(toolsetID ToolsetID) []ServerTool {
	var result []ServerTool
	for i := range r.tools {
		tool := &r.tools[i]
		// Only check read-only filter, not toolset enabled filter
		if tool.Toolset.ID == toolsetID {
			if r.readOnly && !tool.IsReadOnly() {
				continue
			}
			result = append(result, *tool)
		}
	}

	// Sort by tool name for deterministic order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Tool.Name < result[j].Tool.Name
	})

	return result
}

// RegisterTools registers all available tools with the server using the provided dependencies.
// The context is used for feature flag evaluation.
func (r *Registry) RegisterTools(ctx context.Context, s *mcp.Server, deps any) {
	for _, tool := range r.AvailableTools(ctx) {
		tool.RegisterFunc(s, deps)
	}
}

// RegisterResourceTemplates registers all available resource templates with the server.
// The context is used for feature flag evaluation.
func (r *Registry) RegisterResourceTemplates(ctx context.Context, s *mcp.Server, deps any) {
	for _, res := range r.AvailableResourceTemplates(ctx) {
		s.AddResourceTemplate(&res.Template, res.Handler(deps))
	}
}

// RegisterPrompts registers all available prompts with the server.
// The context is used for feature flag evaluation.
func (r *Registry) RegisterPrompts(ctx context.Context, s *mcp.Server) {
	for _, prompt := range r.AvailablePrompts(ctx) {
		s.AddPrompt(&prompt.Prompt, prompt.Handler)
	}
}

// RegisterAll registers all available tools, resources, and prompts with the server.
// The context is used for feature flag evaluation.
func (r *Registry) RegisterAll(ctx context.Context, s *mcp.Server, deps any) {
	r.RegisterTools(ctx, s, deps)
	r.RegisterResourceTemplates(ctx, s, deps)
	r.RegisterPrompts(ctx, s)
}

// ResolveToolAliases resolves deprecated tool aliases to their canonical names.
// It logs a warning to stderr for each deprecated alias that is resolved.
// Returns:
//   - resolved: tool names with aliases replaced by canonical names
//   - aliasesUsed: map of oldName → newName for each alias that was resolved
func (r *Registry) ResolveToolAliases(toolNames []string) (resolved []string, aliasesUsed map[string]string) {
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
func (r *Registry) FindToolByName(toolName string) (*ServerTool, ToolsetID, error) {
	for i := range r.tools {
		tool := &r.tools[i]
		if tool.Tool.Name == toolName {
			return tool, tool.Toolset.ID, nil
		}
	}
	return nil, "", NewToolDoesNotExistError(toolName)
}

// HasToolset checks if any tool/resource/prompt belongs to the given toolset.
func (r *Registry) HasToolset(toolsetID ToolsetID) bool {
	for i := range r.tools {
		if r.tools[i].Toolset.ID == toolsetID {
			return true
		}
	}
	for i := range r.resourceTemplates {
		if r.resourceTemplates[i].Toolset.ID == toolsetID {
			return true
		}
	}
	for i := range r.prompts {
		if r.prompts[i].Toolset.ID == toolsetID {
			return true
		}
	}
	return false
}

// EnabledToolsetIDs returns the list of enabled toolset IDs based on current filters.
// Returns all toolset IDs if no filter is set.
func (r *Registry) EnabledToolsetIDs() []ToolsetID {
	if r.enabledToolsets == nil {
		return r.ToolsetIDs()
	}

	ids := make([]ToolsetID, 0, len(r.enabledToolsets))
	for id := range r.enabledToolsets {
		if r.HasToolset(id) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// IsToolsetEnabled checks if a toolset is currently enabled based on filters.
func (r *Registry) IsToolsetEnabled(toolsetID ToolsetID) bool {
	return r.isToolsetEnabled(toolsetID)
}

// EnableToolset marks a toolset as enabled in this group.
// This is used by dynamic toolset management to track which toolsets have been enabled.
func (r *Registry) EnableToolset(toolsetID ToolsetID) {
	if r.enabledToolsets == nil {
		// nil means all enabled, so nothing to do
		return
	}
	r.enabledToolsets[toolsetID] = true
}

// AllTools returns all tools without any filtering, sorted deterministically.
func (r *Registry) AllTools() []ServerTool {
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
func (r *Registry) AvailableToolsets(exclude ...ToolsetID) []ToolsetMetadata {
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
