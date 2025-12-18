package inventory

import (
	"context"
	"fmt"
	"os"
	"sort"
)

// FeatureFlagChecker is a function that checks if a feature flag is enabled.
// The context can be used to extract actor/user information for flag evaluation.
// Returns (enabled, error). If error occurs, the caller should log and treat as false.
type FeatureFlagChecker func(ctx context.Context, flagName string) (bool, error)

// isToolsetEnabled checks if a toolset is enabled based on current filters.
func (r *Inventory) isToolsetEnabled(toolsetID ToolsetID) bool {
	// Check enabled toolsets filter
	if r.enabledToolsets != nil {
		return r.enabledToolsets[toolsetID]
	}
	return true
}

// checkFeatureFlag checks a feature flag using the feature checker.
// Returns false if checker is nil or returns an error (errors are logged).
func (r *Inventory) checkFeatureFlag(ctx context.Context, flagName string) bool {
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
func (r *Inventory) isFeatureFlagAllowed(ctx context.Context, enableFlag, disableFlag string) bool {
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

// buildRequestMask creates a RequestMask for the current request context.
// This computes all condition values once for O(1) evaluation of each tool.
func (r *Inventory) buildRequestMask(ctx context.Context) *RequestMask {
	if r.conditionCompiler == nil {
		return nil
	}

	var bits uint64
	bools := contextBoolsFromContext(ctx)
	checker := FeatureCheckerFromContext(ctx)

	r.conditionCompiler.mu.RLock()
	defer r.conditionCompiler.mu.RUnlock()

	for key, bit := range r.conditionCompiler.keyToBit {
		// Keys are formatted as "ctx:key_name" or "ff:flag_name"
		if len(key) < 4 { // Minimum: "ff:x" or "ctx:" prefix + 1 char
			continue
		}

		switch {
		case len(key) > 4 && key[:4] == "ctx:":
			// Context bool: "ctx:key_name"
			name := key[4:]
			if bools != nil && bools[name] {
				bits |= 1 << bit
			}
		case len(key) > 3 && key[:3] == "ff:":
			// Feature flag: "ff:flag_name"
			name := key[3:]
			if checker != nil {
				enabled, err := checker(ctx, name)
				if err == nil && enabled {
					bits |= 1 << bit
				}
			}
		}
	}

	return &RequestMask{
		bits: bits,
		ctx:  ctx,
	}
}

// isToolEnabled checks if a specific tool is enabled based on current filters.
// Filter evaluation order:
//  1. Tool.Enabled (legacy tool self-filtering - deprecated)
//  2. Tool.EnableCondition via compiled bitmask (O(1) evaluation)
//  3. FeatureFlagEnable/FeatureFlagDisable (legacy - deprecated)
//  4. Read-only filter
//  5. Builder filters (via WithFilter)
//  6. Toolset/additional tools
func (r *Inventory) isToolEnabled(ctx context.Context, tool *ServerTool, toolIndex int, rm *RequestMask) bool {
	// 1. Check tool's legacy Enabled function first (for backward compatibility)
	if tool.Enabled != nil {
		enabled, err := tool.Enabled(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Tool.Enabled check error for %q: %v\n", tool.Tool.Name, err)
			return false
		}
		if !enabled {
			return false
		}
	}
	// 2. Check tool's EnableCondition via compiled bitmask (O(1) evaluation)
	if toolIndex >= 0 && toolIndex < len(r.compiledConditions) && r.compiledConditions[toolIndex] != nil {
		if rm != nil {
			enabled, err := r.compiledConditions[toolIndex].Evaluate(rm)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Tool.EnableCondition check error for %q: %v\n", tool.Tool.Name, err)
				return false
			}
			if !enabled {
				return false
			}
		} else if tool.EnableCondition != nil {
			// Fallback to tree-based evaluation if no request mask
			enabled, err := tool.EnableCondition.Evaluate(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Tool.EnableCondition check error for %q: %v\n", tool.Tool.Name, err)
				return false
			}
			if !enabled {
				return false
			}
		}
	} else if tool.EnableCondition != nil {
		// Fallback to tree-based evaluation if no compiled condition
		enabled, err := tool.EnableCondition.Evaluate(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Tool.EnableCondition check error for %q: %v\n", tool.Tool.Name, err)
			return false
		}
		if !enabled {
			return false
		}
	}
	// 3. Check legacy feature flags (for backward compatibility)
	if !r.isFeatureFlagAllowed(ctx, tool.FeatureFlagEnable, tool.FeatureFlagDisable) {
		return false
	}
	// 4. Check read-only filter (applies to all tools)
	if r.readOnly && !tool.IsReadOnly() {
		return false
	}
	// 5. Apply builder filters
	for _, filter := range r.filters {
		allowed, err := filter(ctx, tool)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Builder filter error for tool %q: %v\n", tool.Tool.Name, err)
			return false
		}
		if !allowed {
			return false
		}
	}
	// 6. Check if tool is in additionalTools (bypasses toolset filter)
	if r.additionalTools != nil && r.additionalTools[tool.Tool.Name] {
		return true
	}
	// 6. Check toolset filter
	if !r.isToolsetEnabled(tool.Toolset.ID) {
		return false
	}
	return true
}

// AvailableTools returns the tools that pass all current filters,
// sorted deterministically by toolset ID, then tool name.
// The context is used for feature flag evaluation.
// Uses O(1) bitmask evaluation for EnableConditions when possible.
// Note: Tools are pre-sorted at build time, so filtering preserves order.
func (r *Inventory) AvailableTools(ctx context.Context) []ServerTool {
	// Build request mask once for O(1) condition evaluation
	rm := r.buildRequestMask(ctx)

	// Tools are pre-sorted at build time; filtering preserves order
	var result []ServerTool
	for i := range r.tools {
		tool := &r.tools[i]
		if r.isToolEnabled(ctx, tool, i, rm) {
			result = append(result, *tool)
		}
	}

	return result
}

// AvailableResourceTemplates returns resource templates that pass all current filters,
// sorted deterministically by toolset ID, then template name.
// The context is used for feature flag evaluation.
// Note: Resources are pre-sorted at build time, so filtering preserves order.
func (r *Inventory) AvailableResourceTemplates(ctx context.Context) []ServerResourceTemplate {
	// Resources are pre-sorted at build time; filtering preserves order
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

	return result
}

// AvailablePrompts returns prompts that pass all current filters,
// sorted deterministically by toolset ID, then prompt name.
// The context is used for feature flag evaluation.
// Note: Prompts are pre-sorted at build time, so filtering preserves order.
func (r *Inventory) AvailablePrompts(ctx context.Context) []ServerPrompt {
	// Prompts are pre-sorted at build time; filtering preserves order
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

	return result
}

// filterToolsByName returns tools matching the given name, checking deprecated aliases.
// Uses linear scan - optimized for single-lookup per-request scenarios (ForMCPRequest).
func (r *Inventory) filterToolsByName(name string) []ServerTool {
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
// Uses linear scan - optimized for single-lookup per-request scenarios (ForMCPRequest).
func (r *Inventory) filterResourcesByURI(uri string) []ServerResourceTemplate {
	for i := range r.resourceTemplates {
		if r.resourceTemplates[i].Template.URITemplate == uri {
			return []ServerResourceTemplate{r.resourceTemplates[i]}
		}
	}
	return []ServerResourceTemplate{}
}

// filterPromptsByName returns prompts matching the given name.
// Uses linear scan - optimized for single-lookup per-request scenarios (ForMCPRequest).
func (r *Inventory) filterPromptsByName(name string) []ServerPrompt {
	for i := range r.prompts {
		if r.prompts[i].Prompt.Name == name {
			return []ServerPrompt{r.prompts[i]}
		}
	}
	return []ServerPrompt{}
}

// ToolsForToolset returns all tools belonging to a specific toolset.
// This method bypasses the toolset enabled filter (for dynamic toolset registration),
// but still respects the read-only filter.
// Note: Tools are pre-sorted at build time, so filtering preserves order.
func (r *Inventory) ToolsForToolset(toolsetID ToolsetID) []ServerTool {
	// Tools are pre-sorted at build time; filtering preserves order
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

	return result
}

// IsToolsetEnabled checks if a toolset is currently enabled based on filters.
func (r *Inventory) IsToolsetEnabled(toolsetID ToolsetID) bool {
	return r.isToolsetEnabled(toolsetID)
}

// EnableToolset marks a toolset as enabled in this group.
// This is used by dynamic toolset management to track which toolsets have been enabled.
func (r *Inventory) EnableToolset(toolsetID ToolsetID) {
	if r.enabledToolsets == nil {
		// nil means all enabled, so nothing to do
		return
	}
	r.enabledToolsets[toolsetID] = true
}

// EnabledToolsetIDs returns the list of enabled toolset IDs based on current filters.
// Returns all toolset IDs if no filter is set.
func (r *Inventory) EnabledToolsetIDs() []ToolsetID {
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

// FilteredTools returns tools filtered by the Enabled function and builder filters.
// This provides an explicit API for accessing filtered tools, currently implemented
// as an alias for AvailableTools.
//
// The error return is currently always nil but is included for future extensibility.
// Library consumers (e.g., remote server implementations) may need to surface
// recoverable filter errors rather than silently logging them. Having the error
// return in the API now avoids breaking changes later.
//
// The context is used for Enabled function evaluation and builder filter checks.
func (r *Inventory) FilteredTools(ctx context.Context) ([]ServerTool, error) {
	return r.AvailableTools(ctx), nil
}
