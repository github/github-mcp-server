package github

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"
)

// Change represents a single semantic difference between two structured values.
type Change struct {
	Path string // dot/bracket notation path, e.g. "users[1].name"
	Type string // "modified", "added", "removed", "type_changed"
	Old  string // formatted old value (empty for additions)
	New  string // formatted new value (empty for removals)
}

// SemanticDiff compares two byte slices and returns a human-readable diff.
// For supported formats (JSON, YAML), it produces a semantic diff showing only value changes.
// For unsupported formats, it falls back to unified diff.
// Returns: diff output, format name, whether fallback was used, and any error.
func SemanticDiff(base, head []byte, filePath string) (string, string, bool, error) {
	format := detectFormat(filePath)

	var changes []Change
	var err error

	switch format {
	case "json":
		changes, err = compareJSON(base, head)
	case "yaml":
		changes, err = compareYAML(base, head)
	default:
		diff, uerr := unifiedDiff(base, head, filePath)
		if uerr != nil {
			return "", "", false, uerr
		}
		ext := filepath.Ext(filePath)
		if ext != "" {
			ext = ext[1:] // strip leading dot
		} else {
			ext = "txt"
		}
		return diff, ext, true, nil
	}

	if err != nil {
		return "", format, false, err
	}

	return formatChanges(changes), format, false, nil
}

// detectFormat returns the semantic diff format based on file extension.
func detectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return ""
	}
}

// compareJSON parses two JSON byte slices and computes semantic differences.
func compareJSON(base, head []byte) ([]Change, error) {
	var baseVal, headVal any
	if err := json.Unmarshal(base, &baseVal); err != nil {
		return nil, fmt.Errorf("failed to parse base JSON: %w", err)
	}
	if err := json.Unmarshal(head, &headVal); err != nil {
		return nil, fmt.Errorf("failed to parse head JSON: %w", err)
	}
	return deepCompare("", baseVal, headVal), nil
}

// compareYAML parses two YAML byte slices and computes semantic differences.
func compareYAML(base, head []byte) ([]Change, error) {
	var baseVal, headVal any
	if err := yaml.Unmarshal(base, &baseVal); err != nil {
		return nil, fmt.Errorf("failed to parse base YAML: %w", err)
	}
	if err := yaml.Unmarshal(head, &headVal); err != nil {
		return nil, fmt.Errorf("failed to parse head YAML: %w", err)
	}
	// Normalize YAML maps (map[string]any vs map[any]any)
	baseVal = normalizeYAML(baseVal)
	headVal = normalizeYAML(headVal)
	return deepCompare("", baseVal, headVal), nil
}

// normalizeYAML converts map[any]any (from yaml.v3) to map[string]any for consistent comparison.
func normalizeYAML(v any) any {
	switch val := v.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(val))
		for k, v := range val {
			normalized[k] = normalizeYAML(v)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(val))
		for k, v := range val {
			normalized[fmt.Sprintf("%v", k)] = normalizeYAML(v)
		}
		return normalized
	case []any:
		normalized := make([]any, len(val))
		for i, v := range val {
			normalized[i] = normalizeYAML(v)
		}
		return normalized
	default:
		return v
	}
}

// deepCompare recursively compares two values and returns a list of changes.
func deepCompare(path string, base, head any) []Change {
	if base == nil && head == nil {
		return nil
	}

	if base == nil {
		return []Change{{Path: path, Type: "added", New: formatValue(head)}}
	}
	if head == nil {
		return []Change{{Path: path, Type: "removed", Old: formatValue(base)}}
	}

	// Check for type mismatch
	baseMap, baseIsMap := base.(map[string]any)
	headMap, headIsMap := head.(map[string]any)
	baseSlice, baseIsSlice := base.([]any)
	headSlice, headIsSlice := head.([]any)

	if baseIsMap && headIsMap {
		return compareMaps(path, baseMap, headMap)
	}
	if baseIsSlice && headIsSlice {
		return compareSlices(path, baseSlice, headSlice)
	}

	// If types differ between map/slice/scalar, it's a type change
	if baseIsMap != headIsMap || baseIsSlice != headIsSlice {
		return []Change{{Path: path, Type: "type_changed", Old: formatValue(base), New: formatValue(head)}}
	}

	// Scalar comparison — normalize numeric types for comparison
	if !scalarEqual(base, head) {
		return []Change{{Path: path, Type: "modified", Old: formatValue(base), New: formatValue(head)}}
	}

	return nil
}

// compareMaps compares two maps and returns changes with sorted keys for deterministic output.
func compareMaps(path string, base, head map[string]any) []Change {
	var changes []Change

	// Collect all keys
	allKeys := make(map[string]bool)
	for k := range base {
		allKeys[k] = true
	}
	for k := range head {
		allKeys[k] = true
	}

	// Sort for deterministic output
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		childPath := joinPath(path, k)
		baseVal, inBase := base[k]
		headVal, inHead := head[k]

		switch {
		case inBase && !inHead:
			changes = append(changes, Change{Path: childPath, Type: "removed", Old: formatValue(baseVal)})
		case !inBase && inHead:
			changes = append(changes, Change{Path: childPath, Type: "added", New: formatValue(headVal)})
		default:
			changes = append(changes, deepCompare(childPath, baseVal, headVal)...)
		}
	}

	return changes
}

// compareSlices compares two slices element by element.
func compareSlices(path string, base, head []any) []Change {
	var changes []Change
	maxLen := len(base)
	if len(head) > maxLen {
		maxLen = len(head)
	}

	for i := 0; i < maxLen; i++ {
		childPath := fmt.Sprintf("%s[%d]", path, i)
		switch {
		case i >= len(base):
			changes = append(changes, Change{Path: childPath, Type: "added", New: formatValue(head[i])})
		case i >= len(head):
			changes = append(changes, Change{Path: childPath, Type: "removed", Old: formatValue(base[i])})
		default:
			changes = append(changes, deepCompare(childPath, base[i], head[i])...)
		}
	}

	return changes
}

// scalarEqual compares two scalar values, normalizing numeric types.
func scalarEqual(a, b any) bool {
	// Normalize floats that are whole numbers to int for comparison
	// JSON unmarshals all numbers as float64, YAML may use int
	af, aIsFloat := a.(float64)
	bf, bIsFloat := b.(float64)
	ai, aIsInt := a.(int)
	bi, bIsInt := b.(int)

	switch {
	case aIsFloat && bIsInt:
		return af == float64(bi)
	case aIsInt && bIsFloat:
		return float64(ai) == bf
	case aIsFloat && bIsFloat:
		return af == bf
	case aIsInt && bIsInt:
		return ai == bi
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

// formatValue formats a value for display in the diff output.
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case nil:
		return "null"
	case bool:
		return fmt.Sprintf("%t", val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case int:
		return fmt.Sprintf("%d", val)
	case map[string]any, []any:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// joinPath builds a dot-notation path, handling the root case.
func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

// formatChanges renders a list of changes as human-readable text.
func formatChanges(changes []Change) string {
	if len(changes) == 0 {
		return "No changes detected"
	}

	var sb strings.Builder
	for _, c := range changes {
		switch c.Type {
		case "modified":
			fmt.Fprintf(&sb, "%s: %s → %s\n", c.Path, c.Old, c.New)
		case "type_changed":
			fmt.Fprintf(&sb, "%s: %s → %s (type changed)\n", c.Path, c.Old, c.New)
		case "added":
			fmt.Fprintf(&sb, "+ %s: %s\n", c.Path, c.New)
		case "removed":
			fmt.Fprintf(&sb, "- %s: %s\n", c.Path, c.Old)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// unifiedDiff produces a standard unified diff between two byte slices.
func unifiedDiff(base, head []byte, filePath string) (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(base)),
		B:        difflib.SplitLines(string(head)),
		FromFile: filePath + " (base)",
		ToFile:   filePath + " (head)",
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}
