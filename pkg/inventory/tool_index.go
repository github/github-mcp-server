package inventory

import (
	"context"
	"sort"
)

// ToolIndex provides O(1) bitmap-based filtering for tools.
//
// Instead of iterating through all tools and checking each filter condition,
// this pre-computes bitmaps for each filter dimension. Queries become fast
// bitmap AND/OR operations, and we only need to run dynamic Enabled() checks
// on the tools that survive static filtering.
//
// Memory: ~1-2KB for 130 tools with 15 toolsets and 10 feature flags.
// Query time: O(toolsets + features) bitmap ops + O(surviving tools) for dynamic checks.
type ToolIndex struct {
	// tools stores the actual tool data, indexed by bit position
	tools []ServerTool

	// toolPosition maps tool name to bitmap position for O(1) lookup
	toolPosition map[string]int

	// allTools has all bits set for tools in the index
	allTools ToolBitmap

	// Toolset indexes - each toolset maps to bitmap of tools in that toolset
	byToolset map[ToolsetID]ToolBitmap

	// Read-only filtering
	readOnlyTools ToolBitmap // tools with ReadOnlyHint=true
	writeTools    ToolBitmap // tools with ReadOnlyHint=false (write tools)

	// Feature flag indexes
	// requiresFeature[flag] = tools that require this flag to be ON
	requiresFeature map[string]ToolBitmap
	// disabledByFeature[flag] = tools that are disabled when this flag is ON
	disabledByFeature map[string]ToolBitmap

	// Dynamic check tracking
	// hasDynamicCheck contains tools that have a non-nil Enabled function
	hasDynamicCheck ToolBitmap
}

// BuildToolIndex creates a ToolIndex from a slice of ServerTools.
// This should be called once at startup; the index is then reused for all queries.
func BuildToolIndex(tools []ServerTool) *ToolIndex {
	idx := &ToolIndex{
		tools:             make([]ServerTool, len(tools)),
		toolPosition:      make(map[string]int, len(tools)),
		byToolset:         make(map[ToolsetID]ToolBitmap),
		requiresFeature:   make(map[string]ToolBitmap),
		disabledByFeature: make(map[string]ToolBitmap),
	}

	// Sort tools for deterministic ordering (by toolset, then name)
	sortedTools := make([]ServerTool, len(tools))
	copy(sortedTools, tools)
	sort.Slice(sortedTools, func(i, j int) bool {
		if sortedTools[i].Toolset.ID != sortedTools[j].Toolset.ID {
			return sortedTools[i].Toolset.ID < sortedTools[j].Toolset.ID
		}
		return sortedTools[i].Tool.Name < sortedTools[j].Tool.Name
	})

	for i, tool := range sortedTools {
		idx.tools[i] = tool
		idx.toolPosition[tool.Tool.Name] = i
		idx.allTools = idx.allTools.SetBit(i)

		// Index by toolset
		idx.byToolset[tool.Toolset.ID] = idx.byToolset[tool.Toolset.ID].SetBit(i)

		// Index by read-only status
		if tool.IsReadOnly() {
			idx.readOnlyTools = idx.readOnlyTools.SetBit(i)
		} else {
			idx.writeTools = idx.writeTools.SetBit(i)
		}

		// Index by feature flags
		if tool.FeatureFlagEnable != "" {
			idx.requiresFeature[tool.FeatureFlagEnable] =
				idx.requiresFeature[tool.FeatureFlagEnable].SetBit(i)
		}
		if tool.FeatureFlagDisable != "" {
			idx.disabledByFeature[tool.FeatureFlagDisable] =
				idx.disabledByFeature[tool.FeatureFlagDisable].SetBit(i)
		}

		// Track tools with dynamic checks
		if tool.Enabled != nil {
			idx.hasDynamicCheck = idx.hasDynamicCheck.SetBit(i)
		}
	}

	return idx
}

// QueryConfig specifies the filter criteria for a tool query.
type QueryConfig struct {
	// Toolset filtering
	AllToolsets     bool        // if true, include all toolsets
	EnabledToolsets []ToolsetID // specific toolsets to include

	// Additional tools to include (bypass toolset filter)
	AdditionalTools []string

	// Read-only mode - if true, exclude write tools
	ReadOnly bool

	// Feature flag states - map of flag name to enabled state
	// Only flags that are explicitly set are considered
	EnabledFeatures  []string // features that are ON
	DisabledFeatures []string // features that are OFF (explicit)
}

// QueryResult contains the result of a tool query.
type QueryResult struct {
	// Bitmap of tools that passed all static filters
	StaticFiltered ToolBitmap

	// Bitmap of tools in StaticFiltered that need dynamic Enabled() checks
	NeedsDynamicCheck ToolBitmap

	// Tools that passed static filters and have no dynamic check (immediately available)
	Guaranteed ToolBitmap
}

// Query executes a filter query and returns which tools match.
// This performs only bitmap operations - no iteration over tools.
//
// The result indicates:
// - StaticFiltered: all tools that passed static criteria
// - NeedsDynamicCheck: subset that requires runtime Enabled() evaluation
// - Guaranteed: subset that is definitely included (no dynamic check needed)
func (idx *ToolIndex) Query(cfg QueryConfig) QueryResult {
	var result ToolBitmap

	// Step 1: Toolset filtering - O(|toolsets|) bitmap ORs
	if cfg.AllToolsets {
		result = idx.allTools
	} else {
		for _, ts := range cfg.EnabledToolsets {
			if bm, ok := idx.byToolset[ts]; ok {
				result = result.Or(bm)
			}
		}
	}

	// Step 2: Add additional tools - O(|additional|) bit sets
	for _, name := range cfg.AdditionalTools {
		if pos, ok := idx.toolPosition[name]; ok {
			result = result.SetBit(pos)
		}
	}

	// Step 3: Read-only filter - O(1) bitmap AND
	if cfg.ReadOnly {
		result = result.And(idx.readOnlyTools)
	}

	// Step 4: Feature flag filtering - O(|features|) bitmap operations
	// Remove tools that require a feature that's OFF
	for _, flag := range cfg.DisabledFeatures {
		if bm, ok := idx.requiresFeature[flag]; ok {
			result = result.AndNot(bm)
		}
	}
	// Remove tools that are disabled by a feature that's ON
	for _, flag := range cfg.EnabledFeatures {
		if bm, ok := idx.disabledByFeature[flag]; ok {
			result = result.AndNot(bm)
		}
	}

	// For tools with FeatureFlagEnable that isn't in EnabledFeatures, filter them out
	// (They require the flag, but it's not enabled)
	enabledSet := make(map[string]bool, len(cfg.EnabledFeatures))
	for _, f := range cfg.EnabledFeatures {
		enabledSet[f] = true
	}
	for flag, bm := range idx.requiresFeature {
		if !enabledSet[flag] {
			// This flag is required but not enabled, remove these tools
			result = result.AndNot(bm)
		}
	}

	// Compute which tools need dynamic checks
	needsDynamic := result.And(idx.hasDynamicCheck)
	guaranteed := result.AndNot(idx.hasDynamicCheck)

	return QueryResult{
		StaticFiltered:    result,
		NeedsDynamicCheck: needsDynamic,
		Guaranteed:        guaranteed,
	}
}

// GetTool returns the tool at the given bitmap position.
func (idx *ToolIndex) GetTool(position int) *ServerTool {
	if position < 0 || position >= len(idx.tools) {
		return nil
	}
	return &idx.tools[position]
}

// GetToolByName returns the tool with the given name and its position.
func (idx *ToolIndex) GetToolByName(name string) (*ServerTool, int, bool) {
	if pos, ok := idx.toolPosition[name]; ok {
		return &idx.tools[pos], pos, true
	}
	return nil, -1, false
}

// Materialize converts a QueryResult into actual tools, running dynamic checks as needed.
// Only tools in NeedsDynamicCheck have their Enabled() function called.
func (idx *ToolIndex) Materialize(ctx context.Context, qr QueryResult) []ServerTool {
	// Pre-allocate with capacity = guaranteed + potential dynamic
	capacity := qr.Guaranteed.PopCount() + qr.NeedsDynamicCheck.PopCount()
	result := make([]ServerTool, 0, capacity)

	// Add all guaranteed tools (no dynamic check needed)
	qr.Guaranteed.Iterate(func(pos int) bool {
		result = append(result, idx.tools[pos])
		return true
	})

	// Check and add tools that need dynamic evaluation
	qr.NeedsDynamicCheck.Iterate(func(pos int) bool {
		tool := &idx.tools[pos]
		if tool.Enabled != nil {
			enabled, err := tool.Enabled(ctx)
			if err != nil || !enabled {
				return true // skip this tool, continue iteration
			}
		}
		result = append(result, *tool)
		return true
	})

	// Sort result for deterministic output
	sort.Slice(result, func(i, j int) bool {
		if result[i].Toolset.ID != result[j].Toolset.ID {
			return result[i].Toolset.ID < result[j].Toolset.ID
		}
		return result[i].Tool.Name < result[j].Tool.Name
	})

	return result
}

// ToolsetBitmap returns the bitmap for a specific toolset.
func (idx *ToolIndex) ToolsetBitmap(id ToolsetID) ToolBitmap {
	return idx.byToolset[id]
}

// AllToolsBitmap returns a bitmap with all tools.
func (idx *ToolIndex) AllToolsBitmap() ToolBitmap {
	return idx.allTools
}

// ToolCount returns the number of tools in the index.
func (idx *ToolIndex) ToolCount() int {
	return len(idx.tools)
}

// DynamicCheckCount returns how many tools have dynamic Enabled() checks.
func (idx *ToolIndex) DynamicCheckCount() int {
	return idx.hasDynamicCheck.PopCount()
}

// ToolsetIDs returns all toolset IDs in the index.
func (idx *ToolIndex) ToolsetIDs() []ToolsetID {
	ids := make([]ToolsetID, 0, len(idx.byToolset))
	for id := range idx.byToolset {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
