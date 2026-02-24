package response

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	defaultFillRateThreshold = 0.1 // default proportion of items that must have a key for it to survive
	minFillRateRows          = 3   // minimum number of items required to apply fill-rate filtering
	defaultMaxDepth          = 2   // default nesting depth that flatten will recurse into
)

// OptimizeListConfig controls the optimization pipeline behavior.
type OptimizeListConfig struct {
	maxDepth        int
	preservedFields map[string]bool
}

type OptimizeListOption func(*OptimizeListConfig)

// WithMaxDepth sets the maximum nesting depth for flattening.
// Deeper nested maps are silently dropped.
func WithMaxDepth(d int) OptimizeListOption {
	return func(c *OptimizeListConfig) {
		c.maxDepth = d
	}
}

// WithPreservedFields adds keys that are exempt from all destructive strategies except whitespace normalization.
// Keys are matched against post-flatten map keys, so for nested fields like "user.html_url", the dotted key must
// be added explicitly.
func WithPreservedFields(fields ...string) OptimizeListOption {
	return func(c *OptimizeListConfig) {
		if c.preservedFields == nil {
			c.preservedFields = make(map[string]bool, len(fields))
		}
		for _, f := range fields {
			c.preservedFields[f] = true
		}
	}
}

// OptimizeList optimizes a list of items by applying flattening, URL removal, zero-value removal,
// whitespace normalization, and fill-rate filtering.
func OptimizeList[T any](items []T, opts ...OptimizeListOption) ([]byte, error) {
	if items == nil {
		return []byte("[]"), nil
	}

	cfg := OptimizeListConfig{maxDepth: defaultMaxDepth}
	for _, opt := range opts {
		opt(&cfg)
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var maps []map[string]any
	if err := json.Unmarshal(raw, &maps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	for i, item := range maps {
		flattenedItem := flattenTo(item, cfg.maxDepth)
		maps[i] = optimizeItem(flattenedItem, cfg)
	}

	if len(maps) >= minFillRateRows {
		maps = filterByFillRate(maps, defaultFillRateThreshold, cfg)
	}

	return json.Marshal(maps)
}

// flattenTo recursively promotes values from nested maps into the parent
// using dot-notation keys ("user.login", "commit.author.date"). Arrays
// within nested maps are preserved at their dotted key position.
// Recursion stops at the given maxDepth; deeper nested maps are dropped.
func flattenTo(item map[string]any, maxDepth int) map[string]any {
	result := make(map[string]any, len(item))
	flattenInto(item, "", result, 1, maxDepth)
	return result
}

// flattenInto is the recursive worker for flattenTo.
// Nested maps at maxDepth are intentionally dropped, as deeply nested data is not useful in list responses.
func flattenInto(item map[string]any, prefix string, result map[string]any, depth int, maxDepth int) {
	for key, value := range item {
		fullKey := prefix + key
		if nested, ok := value.(map[string]any); ok && depth < maxDepth {
			flattenInto(nested, fullKey+".", result, depth+1, maxDepth)
		} else if !ok {
			result[fullKey] = value
		}
	}
}

// filterByFillRate drops keys that appear on less than the threshold proportion of items.
// Preserved fields always survive.
func filterByFillRate(items []map[string]any, threshold float64, cfg OptimizeListConfig) []map[string]any {
	keyCounts := make(map[string]int)
	for _, item := range items {
		for key := range item {
			keyCounts[key]++
		}
	}

	minCount := int(threshold * float64(len(items)))
	keepKeys := make(map[string]bool, len(keyCounts))
	for key, count := range keyCounts {
		if count > minCount || cfg.preservedFields[key] {
			keepKeys[key] = true
		}
	}

	for i, item := range items {
		filtered := make(map[string]any, len(keepKeys))
		for key, value := range item {
			if keepKeys[key] {
				filtered[key] = value
			}
		}
		items[i] = filtered
	}

	return items
}

// optimizeItem applies per-item strategies in a single pass: remove URLs,
// remove zero-values, normalize whitespace.
// Preserved fields skip everything except whitespace normalization.
func optimizeItem(item map[string]any, cfg OptimizeListConfig) map[string]any {
	result := make(map[string]any, len(item))
	for key, value := range item {
		preserved := cfg.preservedFields[key]
		if !preserved && isURLKey(key) {
			continue
		}
		if !preserved && isZeroValue(value) {
			continue
		}

		if s, ok := value.(string); ok {
			result[key] = strings.Join(strings.Fields(s), " ")
		} else {
			result[key] = value
		}
	}

	return result
}

// isURLKey matches "url", "*_url", and their dot-prefixed variants.
func isURLKey(key string) bool {
	if idx := strings.LastIndex(key, "."); idx >= 0 {
		key = key[idx+1:]
	}
	return key == "url" || strings.HasSuffix(key, "_url")
}

func isZeroValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return val == ""
	case bool:
		return !val
	case float64:
		return val == 0
	default:
		return false
	}
}
