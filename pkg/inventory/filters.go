package inventory

import (
	"context"
	"fmt"
	"os"
	"slices"
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

// featureFlagAllowed reports whether an item with the given enable/disable
// flag pair is permitted under the supplied checker. The checker must be
// non-nil — callers that don't want feature filtering should not call this at
// all (this is also the contract for createFeatureFlagFilter, which is only
// installed when WithFeatureChecker received a non-nil checker).
//
//   - If FeatureFlagEnable is set, the item is only allowed if the flag is enabled.
//   - If FeatureFlagDisable is non-empty, the item is excluded if any listed flag is enabled.
func featureFlagAllowed(ctx context.Context, checker FeatureFlagChecker, enableFlag string, disableFlags []string) bool {
	// Error semantics match the previous checkFeatureFlag helper: a checker
	// error is logged and treated as "flag not enabled". So an enable-flag
	// check on error excludes the tool, but a disable-flag check on error
	// keeps it (the disable condition wasn't met).
	check := func(flag string) bool {
		enabled, err := checker(ctx, flag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", flag, err)
			return false
		}
		return enabled
	}
	if enableFlag != "" && !check(enableFlag) {
		return false
	}
	return !slices.ContainsFunc(disableFlags, check)
}

// createFeatureFlagFilter returns a ToolFilter that gates tools on their
// FeatureFlagEnable / FeatureFlagDisable annotations using the given checker.
// Builder.Build() installs this filter exactly once when WithFeatureChecker
// has been called with a non-nil checker, so "no feature filtering" is
// expressed structurally — by the absence of the filter — rather than by a
// runtime nil check inside the filter itself.
func createFeatureFlagFilter(checker FeatureFlagChecker) ToolFilter {
	return func(ctx context.Context, tool *ServerTool) (bool, error) {
		return featureFlagAllowed(ctx, checker, tool.FeatureFlagEnable, tool.FeatureFlagDisable), nil
	}
}

// isToolEnabled checks if a specific tool is enabled based on current filters.
// Filter evaluation order:
//  1. Tool.Enabled (tool self-filtering)
//  2. Read-only filter
//  3. Builder filters (via WithFilter; the feature-flag filter, when
//     installed via WithFeatureChecker, runs as part of this step)
//  4. Toolset/additional tools
func (r *Inventory) isToolEnabled(ctx context.Context, tool *ServerTool) bool {
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
	// 2. Check read-only filter (applies to all tools)
	if r.readOnly && !tool.IsReadOnly() {
		return false
	}
	// 3. Apply builder filters (includes the feature-flag filter when set)
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
	// 4. Check if tool is in additionalTools (bypasses toolset filter)
	if r.additionalTools != nil && r.additionalTools[tool.Tool.Name] {
		return true
	}
	// 4. Check toolset filter
	if !r.isToolsetEnabled(tool.Toolset.ID) {
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
	var result []ServerResourceTemplate
	for i := range r.resourceTemplates {
		res := &r.resourceTemplates[i]
		// Resources have no filter pipeline, so feature gating runs inline.
		// The featureChecker != nil guard mirrors the structural "no checker
		// = no filtering" contract used for tools (where the absence of a
		// pipeline step expresses the same thing).
		if r.featureChecker != nil && !featureFlagAllowed(ctx, r.featureChecker, res.FeatureFlagEnable, res.FeatureFlagDisable) {
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
	var result []ServerPrompt
	for i := range r.prompts {
		prompt := &r.prompts[i]
		// Prompts have no filter pipeline; see AvailableResourceTemplates for
		// the rationale behind the explicit nil guard.
		if r.featureChecker != nil && !featureFlagAllowed(ctx, r.featureChecker, prompt.FeatureFlagEnable, prompt.FeatureFlagDisable) {
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
