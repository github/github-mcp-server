package response

import (
	"encoding/json"
	"fmt"
	"strings"
)

// defaultFillRateThreshold is the default proportion of items that must have a key for it to survive
const defaultFillRateThreshold = 0.1

// minFillRateRows is the minimum number of items required to apply fill-rate filtering
const minFillRateRows = 3

// maxFlattenDepth is the maximum nesting depth that flatten will recurse into.
// Deeper nested maps are silently dropped.
const maxFlattenDepth = 2

// preservedFields is a set of keys that are exempt from all destructive strategies except whitespace normalization.
// Keys are matched against post-flatten map keys, so for nested fields like "user.html_url", the dotted key must be
// added explicitly. Empty collections are still dropped. Wins over collectionFieldExtractors.
var preservedFields = map[string]bool{
	"html_url": true,
	"draft":    true,
}

// collectionFieldExtractors controls how array fields are handled instead of being summarized as "[N items]".
//   - 1 sub-field: comma-joined into a flat string ("bug, enhancement").
//   - Multiple sub-fields: keep the array, but trim each element to only those fields.
//
// These are explicitly exempt from fill-rate filtering; if we asked for the extraction, it's likely important
// to preserve the data even if only one item has it.
var collectionFieldExtractors = map[string][]string{
	"labels":              {"name"},
	"requested_reviewers": {"login"},
}

// MarshalItems is the single entry point for response optimization.
// Handles two shapes: plain JSON arrays and wrapped objects with metadata.
// An optional maxDepth controls how many nesting levels flatten will recurse
// into; it defaults to maxFlattenDepth when omitted.
func MarshalItems(data any, maxDepth ...int) ([]byte, error) {
	depth := maxFlattenDepth
	if len(maxDepth) > 0 {
		depth = maxDepth[0]
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	switch raw[0] {
	case '[':
		return optimizeArray(raw, depth)
	case '{':
		return optimizeObject(raw, depth)
	default:
		return raw, nil
	}
}

// OptimizeItems runs the full optimization pipeline on a slice of items:
// flatten, remove URLs, remove zero-values, normalize whitespace,
// summarize collections, and fill-rate filtering.
func OptimizeItems(items []map[string]any, depth int) []map[string]any {
	if len(items) == 0 {
		return items
	}

	for i, item := range items {
		flattenedItem := flattenTo(item, depth)
		items[i] = optimizeItem(flattenedItem)
	}

	if len(items) >= minFillRateRows {
		items = filterByFillRate(items, defaultFillRateThreshold)
	}

	return items
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
func filterByFillRate(items []map[string]any, threshold float64) []map[string]any {
	keyCounts := make(map[string]int)
	for _, item := range items {
		for key := range item {
			keyCounts[key]++
		}
	}

	minCount := int(threshold * float64(len(items)))
	keepKeys := make(map[string]bool, len(keyCounts))
	for key, count := range keyCounts {
		_, hasExtractor := collectionFieldExtractors[key]
		if count > minCount || preservedFields[key] || hasExtractor {
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

// optimizeArray is the entry point for optimizing a raw JSON array.
func optimizeArray(raw []byte, depth int) ([]byte, error) {
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return json.Marshal(OptimizeItems(items, depth))
}

// optimizeObject is the entry point for optimizing a raw JSON object.
func optimizeObject(raw []byte, depth int) ([]byte, error) {
	var wrapper map[string]any
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// find the actual data array within the wrapper; rest is metadata to be preserved as is
	var dataKey string
	for key, value := range wrapper {
		if _, ok := value.([]any); ok {
			dataKey = key
			break
		}
	}
	// if no data array found, just return the original response
	if dataKey == "" {
		return raw, nil
	}

	rawItems := wrapper[dataKey].([]any)
	items := make([]map[string]any, 0, len(rawItems))
	for _, rawItem := range rawItems {
		if m, ok := rawItem.(map[string]any); ok {
			items = append(items, m)
		}
	}
	wrapper[dataKey] = OptimizeItems(items, depth)

	return json.Marshal(wrapper)
}

// optimizeItem applies per-item strategies in a single pass: remove URLs,
// remove zero-values, normalize whitespace, summarize collections.
// Preserved fields skip everything except whitespace normalization.
func optimizeItem(item map[string]any) map[string]any {
	result := make(map[string]any, len(item))
	for key, value := range item {
		preserved := preservedFields[key]
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
			} else if fields, ok := collectionFieldExtractors[key]; ok {
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
	base := key
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		base = base[idx+1:]
	}
	return base == "url" || strings.HasSuffix(base, "_url")
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
