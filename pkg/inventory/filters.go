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

// isToolEnabled checks if a specific tool is enabled based on current filters.
// Filter evaluation order:
//  1. Tool.Enabled (tool self-filtering)
//  2. FeatureFlagEnable/FeatureFlagDisable
//  3. Read-only filter
//  4. Builder filters (via WithFilter)
//  5. Toolset/additional tools
func (r *Inventory) isToolEnabled(ctx context.Context, tool *ServerTool) bool {
	return r.isToolEnabledWithFeatureFlags(ctx, tool, true)
}

func (r *Inventory) isToolEnabledWithFeatureFlags(ctx context.Context, tool *ServerTool, checkFeatureFlags bool) bool {
	// 1. Check tool's own Enabled function first
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
	// 2. Check feature flags
	if checkFeatureFlags && !r.isFeatureFlagAllowed(ctx, tool.FeatureFlagEnable, tool.FeatureFlagDisable) {
		return false
	}
	// 3. Check read-only filter (applies to all tools)
	if r.readOnly && !tool.IsReadOnly() {
		return false
	}
	// 4. Apply builder filters
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
	// 5. Check if tool is in additionalTools (bypasses toolset filter)
	if r.additionalTools != nil && r.additionalTools[tool.Tool.Name] {
		return true
	}
	// 5. Check toolset filter
	if !r.isToolsetEnabled(tool.Toolset.ID) {
		return false
	}
	return true
}

func sortTools(tools []ServerTool) {
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Toolset.ID != tools[j].Toolset.ID {
			return tools[i].Toolset.ID < tools[j].Toolset.ID
		}
		return tools[i].Tool.Name < tools[j].Tool.Name
	})
}

// AvailableTools returns the tools that pass all current filters,
// sorted deterministically by toolset ID, then tool name.
// The context is used for feature flag evaluation.
func (r *Inventory) AvailableTools(ctx context.Context) []ServerTool {
	var result []ServerTool
	for i := range r.tools {
		tool := &r.tools[i]
		if r.isToolEnabled(ctx, tool) {
			result = append(result, *tool)
		}
	}

	// Sort deterministically: by toolset ID, then by tool name
	sortTools(result)

	return result
}

// AvailableToolsWithoutFeatureFiltering returns tools that pass every filter
// except FeatureFlagEnable/FeatureFlagDisable.
func (r *Inventory) AvailableToolsWithoutFeatureFiltering(ctx context.Context) []ServerTool {
	var result []ServerTool
	for i := range r.tools {
		tool := &r.tools[i]
		if r.isToolEnabledWithFeatureFlags(ctx, tool, false) {
			result = append(result, *tool)
		}
	}

	sortTools(result)

	return result
}

func sortResourceTemplates(resourceTemplates []ServerResourceTemplate) {
	sort.Slice(resourceTemplates, func(i, j int) bool {
		if resourceTemplates[i].Toolset.ID != resourceTemplates[j].Toolset.ID {
			return resourceTemplates[i].Toolset.ID < resourceTemplates[j].Toolset.ID
		}
		return resourceTemplates[i].Template.Name < resourceTemplates[j].Template.Name
	})
}

// AvailableResourceTemplates returns resource templates that pass all current filters,
// sorted deterministically by toolset ID, then template name.
// The context is used for feature flag evaluation.
func (r *Inventory) AvailableResourceTemplates(ctx context.Context) []ServerResourceTemplate {
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
	sortResourceTemplates(result)

	return result
}

// AvailableResourceTemplatesWithoutFeatureFiltering returns resource templates
// that pass every filter except FeatureFlagEnable/FeatureFlagDisable.
func (r *Inventory) AvailableResourceTemplatesWithoutFeatureFiltering(_ context.Context) []ServerResourceTemplate {
	var result []ServerResourceTemplate
	for i := range r.resourceTemplates {
		res := &r.resourceTemplates[i]
		if r.isToolsetEnabled(res.Toolset.ID) {
			result = append(result, *res)
		}
	}

	sortResourceTemplates(result)

	return result
}

func sortPrompts(prompts []ServerPrompt) {
	sort.Slice(prompts, func(i, j int) bool {
		if prompts[i].Toolset.ID != prompts[j].Toolset.ID {
			return prompts[i].Toolset.ID < prompts[j].Toolset.ID
		}
		return prompts[i].Prompt.Name < prompts[j].Prompt.Name
	})
}

// AvailablePrompts returns prompts that pass all current filters,
// sorted deterministically by toolset ID, then prompt name.
// The context is used for feature flag evaluation.
func (r *Inventory) AvailablePrompts(ctx context.Context) []ServerPrompt {
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
	sortPrompts(result)

	return result
}

// AvailablePromptsWithoutFeatureFiltering returns prompts that pass every filter
// except FeatureFlagEnable/FeatureFlagDisable.
func (r *Inventory) AvailablePromptsWithoutFeatureFiltering(_ context.Context) []ServerPrompt {
	var result []ServerPrompt
	for i := range r.prompts {
		prompt := &r.prompts[i]
		if r.isToolsetEnabled(prompt.Toolset.ID) {
			result = append(result, *prompt)
		}
	}

	sortPrompts(result)

	return result
}

// filterToolsByName returns tools matching the given name, checking deprecated aliases.
// Uses linear scan - optimized for single-lookup per-request scenarios (ForMCPRequest).
// Returns ALL tools matching the name to support feature-flagged tool variants
// (e.g., GetJobLogs and ActionsGetJobLogs both use name "get_job_logs" but are
// controlled by different feature flags).
func (r *Inventory) filterToolsByName(name string) []ServerTool {
	var result []ServerTool
	// Check for exact matches - multiple tools may share the same name with different feature flags
	for i := range r.tools {
		if r.tools[i].Tool.Name == name {
			result = append(result, r.tools[i])
		}
	}
	if len(result) > 0 {
		return result
	}
	// Check if name is a deprecated alias
	if canonical, isAlias := r.deprecatedAliases[name]; isAlias {
		for i := range r.tools {
			if r.tools[i].Tool.Name == canonical {
				result = append(result, r.tools[i])
			}
		}
	}
	return result
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
