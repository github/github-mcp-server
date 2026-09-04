package inventory

import (
	"context"
	"fmt"
	"os"
	"sort"
)

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
func (r *Inventory) checkFeatureFlag(ctx context.Context, flagName FeatureFlag) bool {
	return ResolveFeature(ctx, r.featureChecker, flagName)
}

// isToolEnabled checks if a specific tool is enabled based on current filters.
// Filter evaluation order:
//  1. Tool.Enabled (tool self-filtering)
//  2. Read-only and builder filters
//  3. Toolset/additional and MCP availability filters
//  4. Functional feature rule
func (r *Inventory) isToolEnabled(ctx context.Context, tool *ServerTool, featureAsBool FeatureResolver) bool {
	// 1. Check tool's own Enabled function.
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
	// 2. Apply static inventory filters.
	if r.readOnly && !tool.IsReadOnly() {
		return false
	}
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
	// 3. Apply selection and request-static MCP availability.
	if (r.additionalTools == nil || !r.additionalTools[tool.Tool.Name]) && !r.isToolsetEnabled(tool.Toolset.ID) {
		return false
	}
	switch featureDecisionForToolAvailability(ctx, tool.availability()) {
	case excludeToolBeforeFeatureRule:
		return false
	case includeToolWithoutFeatureRule:
		return true
	}
	// 4. Check feature availability.
	if r.featureChecker != nil && !tool.FeatureRule.Enabled(featureAsBool) {
		return false
	}
	return true
}

// sortByToolsetThenName sorts items deterministically by their toolset ID,
// breaking ties by name. The two extractor closures keep this generic helper
// independent of the concrete inventory item shape (tools, resource templates,
// prompts).
func sortByToolsetThenName[T any](items []T, toolsetID func(T) ToolsetID, name func(T) string) {
	sort.Slice(items, func(i, j int) bool {
		idI, idJ := toolsetID(items[i]), toolsetID(items[j])
		if idI != idJ {
			return idI < idJ
		}
		return name(items[i]) < name(items[j])
	})
}

func sortTools(tools []ServerTool) {
	sortByToolsetThenName(tools,
		func(t ServerTool) ToolsetID { return t.Toolset.ID },
		func(t ServerTool) string { return t.Tool.Name },
	)
}

// AvailableTools returns the tools that pass all current filters,
// sorted deterministically by toolset ID, then tool name.
// The context is used for feature flag evaluation.
func (r *Inventory) AvailableTools(ctx context.Context) []ServerTool {
	ctx = WithFeatureState(ctx, r.featureChecker)
	return r.availableTools(ctx)
}

func (r *Inventory) availableTools(ctx context.Context) []ServerTool {
	featureAsBool := featureResolver(ctx, r.featureChecker)
	var result []ServerTool
	for i := range r.tools {
		tool := &r.tools[i]
		if r.isToolEnabled(ctx, tool, featureAsBool) {
			result = append(result, *tool)
		}
	}

	// Sort deterministically: by toolset ID, then by tool name
	sortTools(result)

	return result
}

func sortResourceTemplates(resourceTemplates []ServerResourceTemplate) {
	sortByToolsetThenName(resourceTemplates,
		func(r ServerResourceTemplate) ToolsetID { return r.Toolset.ID },
		func(r ServerResourceTemplate) string { return r.Template.Name },
	)
}

// AvailableResourceTemplates returns resource templates that pass all current filters,
// sorted deterministically by toolset ID, then template name.
// The context is used for feature flag evaluation.
func (r *Inventory) AvailableResourceTemplates(ctx context.Context) []ServerResourceTemplate {
	ctx = WithFeatureState(ctx, r.featureChecker)
	return r.availableResourceTemplates(ctx)
}

func (r *Inventory) availableResourceTemplates(ctx context.Context) []ServerResourceTemplate {
	featureAsBool := featureResolver(ctx, r.featureChecker)
	var result []ServerResourceTemplate
	for i := range r.resourceTemplates {
		res := &r.resourceTemplates[i]
		if !r.isToolsetEnabled(res.Toolset.ID) {
			continue
		}
		if r.featureChecker != nil && !res.FeatureRule.Enabled(featureAsBool) {
			continue
		}
		result = append(result, *res)
	}

	// Sort deterministically: by toolset ID, then by template name
	sortResourceTemplates(result)

	return result
}

func sortPrompts(prompts []ServerPrompt) {
	sortByToolsetThenName(prompts,
		func(p ServerPrompt) ToolsetID { return p.Toolset.ID },
		func(p ServerPrompt) string { return p.Prompt.Name },
	)
}

// AvailablePrompts returns prompts that pass all current filters,
// sorted deterministically by toolset ID, then prompt name.
// The context is used for feature flag evaluation.
func (r *Inventory) AvailablePrompts(ctx context.Context) []ServerPrompt {
	ctx = WithFeatureState(ctx, r.featureChecker)
	return r.availablePrompts(ctx)
}

func (r *Inventory) availablePrompts(ctx context.Context) []ServerPrompt {
	featureAsBool := featureResolver(ctx, r.featureChecker)
	var result []ServerPrompt
	for i := range r.prompts {
		prompt := &r.prompts[i]
		if !r.isToolsetEnabled(prompt.Toolset.ID) {
			continue
		}
		if r.featureChecker != nil && !prompt.FeatureRule.Enabled(featureAsBool) {
			continue
		}
		result = append(result, *prompt)
	}

	// Sort deterministically: by toolset ID, then by prompt name
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
