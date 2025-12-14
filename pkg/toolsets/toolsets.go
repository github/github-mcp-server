package toolsets

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"

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

// ToolsetGroup holds a collection of tools, resources, and prompts.
// It supports immutable filtering operations that return new ToolsetGroups
// without modifying the original. This design allows for:
//   - Building a full set of tools/resources/prompts once
//   - Applying filters (read-only, feature flags, enabled toolsets) without mutation
//   - Deterministic ordering for documentation generation
//   - Lazy dependency injection only when registering with a server
type ToolsetGroup struct {
	// tools holds all tools in this group
	tools []ServerTool
	// resourceTemplates holds all resource templates in this group
	resourceTemplates []ServerResourceTemplate
	// prompts holds all prompts in this group
	prompts []ServerPrompt
	// deprecatedAliases maps old tool names to new canonical names
	deprecatedAliases map[string]string
	// defaultToolsetIDs are the toolset IDs that "default" expands to
	defaultToolsetIDs []ToolsetID

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
}

// FeatureFlagChecker is a function that checks if a feature flag is enabled.
// The context can be used to extract actor/user information for flag evaluation.
// Returns (enabled, error). If error occurs, the caller should log and treat as false.
type FeatureFlagChecker func(ctx context.Context, flagName string) (bool, error)

// NewToolsetGroup creates a new ToolsetGroup from the provided tools, resources, and prompts.
// The group is created with no filters applied.
func NewToolsetGroup(tools []ServerTool, resources []ServerResourceTemplate, prompts []ServerPrompt) *ToolsetGroup {
	return &ToolsetGroup{
		tools:             tools,
		resourceTemplates: resources,
		prompts:           prompts,
		deprecatedAliases: make(map[string]string),
		readOnly:          false,
		enabledToolsets:   nil,
		additionalTools:   nil,
		featureChecker:    nil,
	}
}

// copy creates a shallow copy of the ToolsetGroup for immutable operations.
func (tg *ToolsetGroup) copy() *ToolsetGroup {
	newTG := &ToolsetGroup{
		tools:             tg.tools, // slices are shared (immutable)
		resourceTemplates: tg.resourceTemplates,
		prompts:           tg.prompts,
		deprecatedAliases: tg.deprecatedAliases,
		defaultToolsetIDs: tg.defaultToolsetIDs,
		readOnly:          tg.readOnly,
		featureChecker:    tg.featureChecker,
	}

	// Copy maps if they exist
	if tg.enabledToolsets != nil {
		newTG.enabledToolsets = make(map[ToolsetID]bool, len(tg.enabledToolsets))
		for k, v := range tg.enabledToolsets {
			newTG.enabledToolsets[k] = v
		}
	}
	if tg.additionalTools != nil {
		newTG.additionalTools = make(map[string]bool, len(tg.additionalTools))
		for k, v := range tg.additionalTools {
			newTG.additionalTools[k] = v
		}
	}

	return newTG
}

// WithReadOnly returns a new ToolsetGroup with read-only mode set.
// When true, write tools are filtered out from Available* methods.
func (tg *ToolsetGroup) WithReadOnly(readOnly bool) *ToolsetGroup {
	newTG := tg.copy()
	newTG.readOnly = readOnly
	return newTG
}

// SetDefaultToolsetIDs configures which toolset IDs the "default" keyword expands to.
// This should be called before WithToolsets if you want "default" to be recognized.
func (tg *ToolsetGroup) SetDefaultToolsetIDs(ids []ToolsetID) *ToolsetGroup {
	tg.defaultToolsetIDs = ids
	return tg
}

// WithToolsets returns a new ToolsetGroup that only includes items from the specified toolsets.
// Special keywords:
//   - "all": enables all toolsets
//   - "default": expands to the default toolset IDs (set via SetDefaultToolsetIDs)
//
// Pass nil to use default toolsets. Pass an empty slice to disable all toolsets
// (useful for dynamic toolsets mode where tools are enabled on demand).
func (tg *ToolsetGroup) WithToolsets(toolsetIDs []string) *ToolsetGroup {
	newTG := tg.copy()

	// Check for "all" keyword - enables all toolsets
	for _, id := range toolsetIDs {
		if id == "all" {
			newTG.enabledToolsets = nil
			return newTG
		}
	}

	// nil means use defaults, empty slice means no toolsets
	if toolsetIDs == nil {
		toolsetIDs = []string{"default"}
	}

	// Expand "default" keyword and collect other IDs
	seen := make(map[ToolsetID]bool)
	expanded := make([]ToolsetID, 0, len(toolsetIDs))
	for _, id := range toolsetIDs {
		if id == "default" {
			for _, defaultID := range tg.defaultToolsetIDs {
				if !seen[defaultID] {
					seen[defaultID] = true
					expanded = append(expanded, defaultID)
				}
			}
		} else {
			tsID := ToolsetID(id)
			if !seen[tsID] {
				seen[tsID] = true
				expanded = append(expanded, tsID)
			}
		}
	}

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

// WithTools returns a new ToolsetGroup with additional tools that bypass toolset filtering.
// These tools are additive - they will be included even if their toolset is not enabled.
// Read-only filtering still applies to these tools.
// Deprecated tool aliases are automatically resolved to their canonical names.
// Pass nil or empty slice to clear additional tools.
func (tg *ToolsetGroup) WithTools(toolNames []string) *ToolsetGroup {
	newTG := tg.copy()
	if len(toolNames) == 0 {
		newTG.additionalTools = nil
		return newTG
	}
	newTG.additionalTools = make(map[string]bool, len(toolNames))
	for _, name := range toolNames {
		// Resolve deprecated aliases to canonical names
		if canonical, isAlias := tg.deprecatedAliases[name]; isAlias {
			newTG.additionalTools[canonical] = true
		} else {
			newTG.additionalTools[name] = true
		}
	}
	return newTG
}

// WithFeatureChecker returns a new ToolsetGroup with a feature checker function.
// The checker receives a context (for actor extraction) and feature flag name, returns (enabled, error).
// If error occurs, it will be logged and treated as false.
// If checker is nil, all feature flag checks return false (items with FeatureFlagEnable are excluded,
// items with FeatureFlagDisable are included).
func (tg *ToolsetGroup) WithFeatureChecker(checker FeatureFlagChecker) *ToolsetGroup {
	newTG := tg.copy()
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

// ForMCPRequest returns a ToolsetGroup optimized for a specific MCP request.
// This is designed for servers that create a new instance per request (like the remote server),
// allowing them to only register the items needed for that specific request rather than all ~90 tools.
//
// Parameters:
//   - method: The MCP method being called (use MCP* constants)
//   - itemName: Name of specific item for call/get methods (tool name, resource URI, or prompt name)
//
// Returns a new ToolsetGroup containing only the items relevant to the request:
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
func (tg *ToolsetGroup) ForMCPRequest(method string, itemName string) *ToolsetGroup {
	result := tg.copy()

	switch method {
	case MCPMethodInitialize:
		// Capabilities only - no items need to be registered
		// The server capabilities (tools, resources, prompts support) are set via ServerOptions
		result.tools = []ServerTool{}
		result.resourceTemplates = []ServerResourceTemplate{}
		result.prompts = []ServerPrompt{}

	case MCPMethodToolsList:
		// All available tools, but no resources or prompts
		result.resourceTemplates = []ServerResourceTemplate{}
		result.prompts = []ServerPrompt{}

	case MCPMethodToolsCall:
		// Only the specific tool (if found), no resources or prompts
		result.resourceTemplates = []ServerResourceTemplate{}
		result.prompts = []ServerPrompt{}
		if itemName != "" {
			result.tools = tg.filterToolsByName(itemName)
		}

	case MCPMethodResourcesList, MCPMethodResourcesTemplatesList:
		// All available resources, but no tools or prompts
		result.tools = []ServerTool{}
		result.prompts = []ServerPrompt{}

	case MCPMethodResourcesRead:
		// Only the specific resource template, no tools or prompts
		result.tools = []ServerTool{}
		result.prompts = []ServerPrompt{}
		if itemName != "" {
			result.resourceTemplates = tg.filterResourcesByURI(itemName)
		}

	case MCPMethodPromptsList:
		// All available prompts, but no tools or resources
		result.tools = []ServerTool{}
		result.resourceTemplates = []ServerResourceTemplate{}

	case MCPMethodPromptsGet:
		// Only the specific prompt, no tools or resources
		result.tools = []ServerTool{}
		result.resourceTemplates = []ServerResourceTemplate{}
		if itemName != "" {
			result.prompts = tg.filterPromptsByName(itemName)
		}

	default:
		// Unknown method - register nothing
		result.tools = []ServerTool{}
		result.resourceTemplates = []ServerResourceTemplate{}
		result.prompts = []ServerPrompt{}
	}

	return result
}

// filterToolsByName returns tools matching the given name, checking deprecated aliases.
// Returns from the current tools slice (respects existing filter chain).
func (tg *ToolsetGroup) filterToolsByName(name string) []ServerTool {
	// First check for exact match
	for i := range tg.tools {
		if tg.tools[i].Tool.Name == name {
			return []ServerTool{tg.tools[i]}
		}
	}
	// Check if name is a deprecated alias
	if canonical, isAlias := tg.deprecatedAliases[name]; isAlias {
		for i := range tg.tools {
			if tg.tools[i].Tool.Name == canonical {
				return []ServerTool{tg.tools[i]}
			}
		}
	}
	return []ServerTool{}
}

// filterResourcesByURI returns resource templates matching the given URI pattern.
func (tg *ToolsetGroup) filterResourcesByURI(uri string) []ServerResourceTemplate {
	for i := range tg.resourceTemplates {
		// Check if URI matches the template pattern (exact match on URITemplate string)
		if tg.resourceTemplates[i].Template.URITemplate == uri {
			return []ServerResourceTemplate{tg.resourceTemplates[i]}
		}
	}
	return []ServerResourceTemplate{}
}

// filterPromptsByName returns prompts matching the given name.
func (tg *ToolsetGroup) filterPromptsByName(name string) []ServerPrompt {
	for i := range tg.prompts {
		if tg.prompts[i].Prompt.Name == name {
			return []ServerPrompt{tg.prompts[i]}
		}
	}
	return []ServerPrompt{}
}

// WithDeprecatedToolAliases returns a new ToolsetGroup with the given deprecated aliases added.
// Aliases map old tool names to new canonical names.
func (tg *ToolsetGroup) WithDeprecatedToolAliases(aliases map[string]string) *ToolsetGroup {
	newTG := tg.copy()
	// Ensure we have a fresh map
	newTG.deprecatedAliases = make(map[string]string, len(tg.deprecatedAliases)+len(aliases))
	for k, v := range tg.deprecatedAliases {
		newTG.deprecatedAliases[k] = v
	}
	for oldName, newName := range aliases {
		newTG.deprecatedAliases[oldName] = newName
	}
	return newTG
}

// isToolsetEnabled checks if a toolset is enabled based on current filters.
func (tg *ToolsetGroup) isToolsetEnabled(toolsetID ToolsetID) bool {
	// Check enabled toolsets filter
	if tg.enabledToolsets != nil {
		return tg.enabledToolsets[toolsetID]
	}
	return true
}

// checkFeatureFlag checks a feature flag using the feature checker.
// Returns false if checker is nil or returns an error (errors are logged).
func (tg *ToolsetGroup) checkFeatureFlag(ctx context.Context, flagName string) bool {
	if tg.featureChecker == nil || flagName == "" {
		return false
	}
	enabled, err := tg.featureChecker(ctx, flagName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", flagName, err)
		return false
	}
	return enabled
}

// isFeatureFlagAllowed checks if an item passes feature flag filtering.
// - If FeatureFlagEnable is set, the item is only allowed if the flag is enabled
// - If FeatureFlagDisable is set, the item is excluded if the flag is enabled
func (tg *ToolsetGroup) isFeatureFlagAllowed(ctx context.Context, enableFlag, disableFlag string) bool {
	// Check enable flag - item requires this flag to be on
	if enableFlag != "" && !tg.checkFeatureFlag(ctx, enableFlag) {
		return false
	}
	// Check disable flag - item is excluded if this flag is on
	if disableFlag != "" && tg.checkFeatureFlag(ctx, disableFlag) {
		return false
	}
	return true
}

// isToolEnabled checks if a specific tool is enabled based on current filters.
func (tg *ToolsetGroup) isToolEnabled(ctx context.Context, tool *ServerTool) bool {
	// Check read-only filter first (applies to all tools)
	if tg.readOnly && !tool.IsReadOnly() {
		return false
	}
	// Check feature flags
	if !tg.isFeatureFlagAllowed(ctx, tool.FeatureFlagEnable, tool.FeatureFlagDisable) {
		return false
	}
	// Check if tool is in additionalTools (bypasses toolset filter)
	if tg.additionalTools != nil && tg.additionalTools[tool.Tool.Name] {
		return true
	}
	// Check toolset filter
	if !tg.isToolsetEnabled(tool.Toolset.ID) {
		return false
	}
	return true
}

// AvailableTools returns the tools that pass all current filters,
// sorted deterministically by toolset ID, then tool name.
// The context is used for feature flag evaluation.
func (tg *ToolsetGroup) AvailableTools(ctx context.Context) []ServerTool {
	var result []ServerTool
	for i := range tg.tools {
		tool := &tg.tools[i]
		if tg.isToolEnabled(ctx, tool) {
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
func (tg *ToolsetGroup) AvailableResourceTemplates(ctx context.Context) []ServerResourceTemplate {
	var result []ServerResourceTemplate
	for i := range tg.resourceTemplates {
		res := &tg.resourceTemplates[i]
		// Check feature flags
		if !tg.isFeatureFlagAllowed(ctx, res.FeatureFlagEnable, res.FeatureFlagDisable) {
			continue
		}
		if tg.isToolsetEnabled(res.Toolset.ID) {
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
func (tg *ToolsetGroup) AvailablePrompts(ctx context.Context) []ServerPrompt {
	var result []ServerPrompt
	for i := range tg.prompts {
		prompt := &tg.prompts[i]
		// Check feature flags
		if !tg.isFeatureFlagAllowed(ctx, prompt.FeatureFlagEnable, prompt.FeatureFlagDisable) {
			continue
		}
		if tg.isToolsetEnabled(prompt.Toolset.ID) {
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
func (tg *ToolsetGroup) ToolsetIDs() []ToolsetID {
	seen := make(map[ToolsetID]bool)
	for i := range tg.tools {
		seen[tg.tools[i].Toolset.ID] = true
	}
	for i := range tg.resourceTemplates {
		seen[tg.resourceTemplates[i].Toolset.ID] = true
	}
	for i := range tg.prompts {
		seen[tg.prompts[i].Toolset.ID] = true
	}

	ids := make([]ToolsetID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// ToolsetDescriptions returns a map of toolset ID to description for all toolsets.
func (tg *ToolsetGroup) ToolsetDescriptions() map[ToolsetID]string {
	descriptions := make(map[ToolsetID]string)
	for i := range tg.tools {
		t := &tg.tools[i]
		if t.Toolset.Description != "" {
			descriptions[t.Toolset.ID] = t.Toolset.Description
		}
	}
	for i := range tg.resourceTemplates {
		r := &tg.resourceTemplates[i]
		if r.Toolset.Description != "" {
			descriptions[r.Toolset.ID] = r.Toolset.Description
		}
	}
	for i := range tg.prompts {
		p := &tg.prompts[i]
		if p.Toolset.Description != "" {
			descriptions[p.Toolset.ID] = p.Toolset.Description
		}
	}
	return descriptions
}

// ToolsForToolset returns all tools belonging to a specific toolset.
// This method bypasses the toolset enabled filter (for dynamic toolset registration),
// but still respects the read-only filter.
func (tg *ToolsetGroup) ToolsForToolset(toolsetID ToolsetID) []ServerTool {
	var result []ServerTool
	for i := range tg.tools {
		tool := &tg.tools[i]
		// Only check read-only filter, not toolset enabled filter
		if tool.Toolset.ID == toolsetID {
			if tg.readOnly && !tool.IsReadOnly() {
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
func (tg *ToolsetGroup) RegisterTools(ctx context.Context, s *mcp.Server, deps any) {
	for _, tool := range tg.AvailableTools(ctx) {
		tool.RegisterFunc(s, deps)
	}
}

// RegisterResourceTemplates registers all available resource templates with the server.
// The context is used for feature flag evaluation.
func (tg *ToolsetGroup) RegisterResourceTemplates(ctx context.Context, s *mcp.Server, deps any) {
	for _, res := range tg.AvailableResourceTemplates(ctx) {
		s.AddResourceTemplate(&res.Template, res.Handler(deps))
	}
}

// RegisterPrompts registers all available prompts with the server.
// The context is used for feature flag evaluation.
func (tg *ToolsetGroup) RegisterPrompts(ctx context.Context, s *mcp.Server) {
	for _, prompt := range tg.AvailablePrompts(ctx) {
		s.AddPrompt(&prompt.Prompt, prompt.Handler)
	}
}

// RegisterAll registers all available tools, resources, and prompts with the server.
// The context is used for feature flag evaluation.
func (tg *ToolsetGroup) RegisterAll(ctx context.Context, s *mcp.Server, deps any) {
	tg.RegisterTools(ctx, s, deps)
	tg.RegisterResourceTemplates(ctx, s, deps)
	tg.RegisterPrompts(ctx, s)
}

// ResolveToolAliases resolves deprecated tool aliases to their canonical names.
// It logs a warning to stderr for each deprecated alias that is resolved.
// Returns:
//   - resolved: tool names with aliases replaced by canonical names
//   - aliasesUsed: map of oldName → newName for each alias that was resolved
func (tg *ToolsetGroup) ResolveToolAliases(toolNames []string) (resolved []string, aliasesUsed map[string]string) {
	resolved = make([]string, 0, len(toolNames))
	aliasesUsed = make(map[string]string)
	for _, toolName := range toolNames {
		if canonicalName, isAlias := tg.deprecatedAliases[toolName]; isAlias {
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
func (tg *ToolsetGroup) FindToolByName(toolName string) (*ServerTool, ToolsetID, error) {
	for i := range tg.tools {
		tool := &tg.tools[i]
		if tool.Tool.Name == toolName {
			return tool, tool.Toolset.ID, nil
		}
	}
	return nil, "", NewToolDoesNotExistError(toolName)
}

// HasToolset checks if any tool/resource/prompt belongs to the given toolset.
func (tg *ToolsetGroup) HasToolset(toolsetID ToolsetID) bool {
	for i := range tg.tools {
		if tg.tools[i].Toolset.ID == toolsetID {
			return true
		}
	}
	for i := range tg.resourceTemplates {
		if tg.resourceTemplates[i].Toolset.ID == toolsetID {
			return true
		}
	}
	for i := range tg.prompts {
		if tg.prompts[i].Toolset.ID == toolsetID {
			return true
		}
	}
	return false
}

// EnabledToolsetIDs returns the list of enabled toolset IDs based on current filters.
// Returns all toolset IDs if no filter is set.
func (tg *ToolsetGroup) EnabledToolsetIDs() []ToolsetID {
	if tg.enabledToolsets == nil {
		return tg.ToolsetIDs()
	}

	ids := make([]ToolsetID, 0, len(tg.enabledToolsets))
	for id := range tg.enabledToolsets {
		if tg.HasToolset(id) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// IsToolsetEnabled checks if a toolset is currently enabled based on filters.
func (tg *ToolsetGroup) IsToolsetEnabled(toolsetID ToolsetID) bool {
	return tg.isToolsetEnabled(toolsetID)
}

// EnableToolset marks a toolset as enabled in this group.
// This is used by dynamic toolset management to track which toolsets have been enabled.
func (tg *ToolsetGroup) EnableToolset(toolsetID ToolsetID) {
	if tg.enabledToolsets == nil {
		// nil means all enabled, so nothing to do
		return
	}
	tg.enabledToolsets[toolsetID] = true
}

// AllTools returns all tools without any filtering, sorted deterministically.
func (tg *ToolsetGroup) AllTools() []ServerTool {
	result := slices.Clone(tg.tools)

	// Sort deterministically: by toolset ID, then by tool name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Toolset.ID != result[j].Toolset.ID {
			return result[i].Toolset.ID < result[j].Toolset.ID
		}
		return result[i].Tool.Name < result[j].Tool.Name
	})

	return result
}
