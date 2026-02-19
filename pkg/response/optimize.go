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
	maxDepth             int
	preservedFields      map[string]bool
	collectionExtractors map[string][]string
}

type OptimizeListOption func(*OptimizeListConfig)

// WithMaxDepth sets the maximum nesting depth for flattening.
// Deeper nested maps are silently dropped.
func WithMaxDepth(d int) OptimizeListOption {
	return func(c *OptimizeListConfig) {
		c.maxDepth = d
	}
}

// WithPreservedFields sets keys that are exempt from all destructive strategies except whitespace normalization.
// Keys are matched against post-flatten map keys, so for nested fields like "user.html_url", the dotted key must be
// added explicitly. Empty collections are still dropped. Wins over collectionExtractors.
func WithPreservedFields(fields map[string]bool) OptimizeListOption {
	return func(c *OptimizeListConfig) {
		c.preservedFields = fields
	}
}

// WithCollectionExtractors controls how array fields are handled instead of being summarized as "[N items]".
//   - 1 sub-field: comma-joined into a flat string ("bug, enhancement").
//   - Multiple sub-fields: keep the array, but trim each element to only those fields.
//
// These are explicitly exempt from fill-rate filtering; if we asked for the extraction, it's likely important
// to preserve the data even if only one item has it.
func WithCollectionExtractors(extractors map[string][]string) OptimizeListOption {
	return func(c *OptimizeListConfig) {
		c.collectionExtractors = extractors
	}
}

// OptimizeList optimizes a list of items by applying flattening, URL removal, zero-value removal,
// whitespace normalization, collection summarization, and fill-rate filtering.
func OptimizeList[T any](items []T, opts ...OptimizeListOption) ([]byte, error) {
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
// Preserved fields and extractor keys always survive.
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
		_, hasExtractor := cfg.collectionExtractors[key]
		if count > minCount || cfg.preservedFields[key] || hasExtractor {
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
// remove zero-values, normalize whitespace, summarize collections.
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

		switch v := value.(type) {
		case string:
			result[key] = strings.Join(strings.Fields(v), " ")
		case []any:
			if len(v) == 0 {
				continue
			}

			if preserved {
				result[key] = value
			} else if fields, ok := cfg.collectionExtractors[key]; ok {
				if len(fields) == 1 {
					result[key] = extractSubField(v, fields[0])
				} else {
					result[key] = trimArrayFields(v, fields)
				}
			} else {
				result[key] = fmt.Sprintf("[%d items]", len(v))
			}
		default:
			result[key] = value
		}
	}

	return result
}

// extractSubField pulls a named sub-field from each slice element and joins
// them with ", ". Elements missing the field are silently skipped.
func extractSubField(items []any, field string) string {
	var vals []string
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		v, ok := m[field]
		if !ok || v == nil {
			continue
		}

		switch s := v.(type) {
		case string:
			if s != "" {
				vals = append(vals, s)
			}
		default:
			vals = append(vals, fmt.Sprintf("%v", v))
		}
	}

	return strings.Join(vals, ", ")
}

// trimArrayFields keeps only the specified fields from each object in a slice.
// The trimmed objects are returned as is, no further strategies are applied.
func trimArrayFields(items []any, fields []string) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		trimmed := make(map[string]any, len(fields))
		for _, f := range fields {
			if v, exists := m[f]; exists {
				trimmed[f] = v
			}
		}

		if len(trimmed) > 0 {
			result = append(result, trimmed)
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
