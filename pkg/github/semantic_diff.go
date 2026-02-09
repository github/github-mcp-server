package github

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// MaxSemanticDiffFileSize is the maximum file size (in bytes) for semantic diff processing.
// Files larger than this fall back to unified diff to prevent excessive server-side processing.
const MaxSemanticDiffFileSize = 1024 * 1024 // 1MB

// DiffFormat represents the format used for diffing.
type DiffFormat string

const (
	DiffFormatJSON     DiffFormat = "json"
	DiffFormatYAML     DiffFormat = "yaml"
	DiffFormatCSV      DiffFormat = "csv"
	DiffFormatTOML     DiffFormat = "toml"
	DiffFormatUnified  DiffFormat = "unified"
	DiffFormatFallback DiffFormat = "fallback"
)

// SemanticDiffResult holds the output of a semantic diff operation.
type SemanticDiffResult struct {
	Format  DiffFormat `json:"format"`
	Diff    string     `json:"diff"`
	Message string     `json:"message,omitempty"`
}

// SemanticDiff compares two versions of a file and returns a semantic diff
// for supported formats, or a unified diff as a fallback.
// A nil base indicates a new file; a nil head indicates a deleted file.
func SemanticDiff(path string, base, head []byte) SemanticDiffResult {
	if base == nil && head == nil {
		return SemanticDiffResult{
			Format: DiffFormatUnified,
			Diff:   "no changes detected",
		}
	}

	if base == nil {
		return SemanticDiffResult{
			Format: DetectDiffFormat(path),
			Diff:   "file added",
		}
	}

	if head == nil {
		return SemanticDiffResult{
			Format: DetectDiffFormat(path),
			Diff:   "file deleted",
		}
	}

	if len(base) > MaxSemanticDiffFileSize || len(head) > MaxSemanticDiffFileSize {
		return SemanticDiffResult{
			Format:  DiffFormatFallback,
			Diff:    unifiedDiff(path, base, head),
			Message: "file exceeds maximum size for semantic diff, using unified diff",
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return semanticDiffJSON(path, base, head)
	case ".yaml", ".yml":
		return semanticDiffYAML(path, base, head)
	case ".csv":
		return semanticDiffCSV(path, base, head)
	case ".toml":
		return semanticDiffTOML(path, base, head)
	default:
		return SemanticDiffResult{
			Format: DiffFormatUnified,
			Diff:   unifiedDiff(path, base, head),
		}
	}
}

// semanticDiffJSON parses both versions as JSON and produces a path-based diff.
func semanticDiffJSON(path string, base, head []byte) SemanticDiffResult {
	var baseVal, headVal any
	if err := json.Unmarshal(base, &baseVal); err != nil {
		return fallbackResult(path, base, head, "failed to parse base as JSON")
	}
	if err := json.Unmarshal(head, &headVal); err != nil {
		return fallbackResult(path, base, head, "failed to parse head as JSON")
	}

	changes := compareValues("", baseVal, headVal)
	if len(changes) == 0 {
		return SemanticDiffResult{
			Format: DiffFormatJSON,
			Diff:   "no changes detected",
		}
	}

	return SemanticDiffResult{
		Format: DiffFormatJSON,
		Diff:   strings.Join(changes, "\n"),
	}
}

// semanticDiffYAML parses both versions as YAML and produces a path-based diff.
func semanticDiffYAML(path string, base, head []byte) SemanticDiffResult {
	var baseVal, headVal any
	if err := yaml.Unmarshal(base, &baseVal); err != nil {
		return fallbackResult(path, base, head, "failed to parse base as YAML")
	}
	if err := yaml.Unmarshal(head, &headVal); err != nil {
		return fallbackResult(path, base, head, "failed to parse head as YAML")
	}

	changes := compareValues("", baseVal, headVal)
	if len(changes) == 0 {
		return SemanticDiffResult{
			Format: DiffFormatYAML,
			Diff:   "no changes detected",
		}
	}

	return SemanticDiffResult{
		Format: DiffFormatYAML,
		Diff:   strings.Join(changes, "\n"),
	}
}

// semanticDiffCSV parses both versions as CSV and produces row/cell-level diffs.
func semanticDiffCSV(path string, base, head []byte) SemanticDiffResult {
	baseRows, err := csv.NewReader(bytes.NewReader(base)).ReadAll()
	if err != nil {
		return fallbackResult(path, base, head, "failed to parse base as CSV")
	}
	headRows, err := csv.NewReader(bytes.NewReader(head)).ReadAll()
	if err != nil {
		return fallbackResult(path, base, head, "failed to parse head as CSV")
	}

	changes := compareCSV(baseRows, headRows)
	if len(changes) == 0 {
		return SemanticDiffResult{
			Format: DiffFormatCSV,
			Diff:   "no changes detected",
		}
	}

	return SemanticDiffResult{
		Format: DiffFormatCSV,
		Diff:   strings.Join(changes, "\n"),
	}
}

// semanticDiffTOML parses both versions as TOML and produces a path-based diff.
func semanticDiffTOML(path string, base, head []byte) SemanticDiffResult {
	var baseVal, headVal map[string]any
	if err := toml.Unmarshal(base, &baseVal); err != nil {
		return fallbackResult(path, base, head, "failed to parse base as TOML")
	}
	if err := toml.Unmarshal(head, &headVal); err != nil {
		return fallbackResult(path, base, head, "failed to parse head as TOML")
	}

	changes := compareValues("", any(baseVal), any(headVal))
	if len(changes) == 0 {
		return SemanticDiffResult{
			Format: DiffFormatTOML,
			Diff:   "no changes detected",
		}
	}

	return SemanticDiffResult{
		Format: DiffFormatTOML,
		Diff:   strings.Join(changes, "\n"),
	}
}

// compareValues recursively compares two decoded values and returns change descriptions.
// Note: JSON integers larger than 2^53 may lose precision due to float64 representation.
func compareValues(path string, base, head any) []string {
	// Normalize numeric types from different decoders (JSON uses float64, YAML may use int)
	base = normalizeValue(base)
	head = normalizeValue(head)

	baseIsNil := base == nil
	headIsNil := head == nil

	if baseIsNil && headIsNil {
		return nil
	}
	if baseIsNil {
		return []string{formatChange(path, "changed", formatValue(base), formatValue(head))}
	}
	if headIsNil {
		return []string{formatChange(path, "changed", formatValue(base), formatValue(head))}
	}

	switch b := base.(type) {
	case map[string]any:
		h, ok := head.(map[string]any)
		if !ok {
			return []string{formatChange(path, "changed type", formatValue(base), formatValue(head))}
		}
		return compareMaps(path, b, h)

	case []any:
		h, ok := head.([]any)
		if !ok {
			return []string{formatChange(path, "changed type", formatValue(base), formatValue(head))}
		}
		return compareSlices(path, b, h)

	default:
		if fmt.Sprintf("%v", base) != fmt.Sprintf("%v", head) {
			return []string{formatChange(path, "changed", formatValue(base), formatValue(head))}
		}
		return nil
	}
}

// normalizeValue converts numeric types to float64 for consistent comparison.
func normalizeValue(v any) any {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case float32:
		return float64(n)
	case uint:
		return float64(n)
	case uint64:
		return float64(n)
	case map[any]any:
		// YAML can produce map[any]any, convert to map[string]any
		result := make(map[string]any, len(n))
		for k, val := range n {
			result[fmt.Sprintf("%v", k)] = val
		}
		return result
	default:
		return v
	}
}

// compareMaps compares two maps and returns change descriptions.
func compareMaps(path string, base, head map[string]any) []string {
	var changes []string

	// Collect all keys from both maps
	allKeys := make(map[string]bool)
	for k := range base {
		allKeys[k] = true
	}
	for k := range head {
		allKeys[k] = true
	}

	// Sort keys for deterministic output
	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		childPath := joinPath(path, key)
		baseVal, inBase := base[key]
		headVal, inHead := head[key]

		switch {
		case inBase && !inHead:
			changes = append(changes, formatChange(childPath, "removed", formatValue(baseVal), ""))
		case !inBase && inHead:
			changes = append(changes, formatChange(childPath, "added", "", formatValue(headVal)))
		default:
			changes = append(changes, compareValues(childPath, baseVal, headVal)...)
		}
	}

	return changes
}

// compareSlices compares two slices and returns change descriptions.
func compareSlices(path string, base, head []any) []string {
	var changes []string

	maxLen := len(base)
	if len(head) > maxLen {
		maxLen = len(head)
	}

	for i := range maxLen {
		childPath := fmt.Sprintf("%s[%d]", path, i)
		switch {
		case i >= len(base):
			changes = append(changes, formatChange(childPath, "added", "", formatValue(head[i])))
		case i >= len(head):
			changes = append(changes, formatChange(childPath, "removed", formatValue(base[i]), ""))
		default:
			changes = append(changes, compareValues(childPath, base[i], head[i])...)
		}
	}

	return changes
}

// compareCSV compares CSV data with header awareness.
func compareCSV(base, head [][]string) []string {
	var changes []string

	// Use headers from base if available
	var headers []string
	if len(base) > 0 {
		headers = base[0]
	} else if len(head) > 0 {
		headers = head[0]
	}

	// Check if headers changed
	if len(base) > 0 && len(head) > 0 {
		baseHeaders := base[0]
		headHeaders := head[0]
		if !slicesEqual(baseHeaders, headHeaders) {
			changes = append(changes, fmt.Sprintf("headers changed: %v → %v", baseHeaders, headHeaders))
			// If headers changed, fall back to row-level comparison
			headers = nil
		}
	}

	// Compare data rows (skip header row)
	baseStart, headStart := 1, 1
	if len(base) == 0 {
		baseStart = 0
	}
	if len(head) == 0 {
		headStart = 0
	}

	baseData := safeSlice(base, baseStart)
	headData := safeSlice(head, headStart)

	maxRows := len(baseData)
	if len(headData) > maxRows {
		maxRows = len(headData)
	}

	for i := range maxRows {
		rowLabel := fmt.Sprintf("row %d", i+1)
		switch {
		case i >= len(baseData):
			changes = append(changes, fmt.Sprintf("%s: added %v", rowLabel, headData[i]))
		case i >= len(headData):
			changes = append(changes, fmt.Sprintf("%s: removed %v", rowLabel, baseData[i]))
		default:
			rowChanges := compareCSVRow(rowLabel, headers, baseData[i], headData[i])
			changes = append(changes, rowChanges...)
		}
	}

	return changes
}

// compareCSVRow compares individual CSV rows cell by cell.
func compareCSVRow(rowLabel string, headers, base, head []string) []string {
	var changes []string

	maxCols := len(base)
	if len(head) > maxCols {
		maxCols = len(head)
	}

	for i := range maxCols {
		var colLabel string
		if headers != nil && i < len(headers) {
			colLabel = fmt.Sprintf("%s.%s", rowLabel, headers[i])
		} else {
			colLabel = fmt.Sprintf("%s[%d]", rowLabel, i)
		}

		var baseVal, headVal string
		if i < len(base) {
			baseVal = base[i]
		}
		if i < len(head) {
			headVal = head[i]
		}

		if baseVal != headVal {
			changes = append(changes, formatChange(colLabel, "changed", quote(baseVal), quote(headVal)))
		}
	}

	return changes
}

// formatChange formats a single change entry.
func formatChange(path, changeType, oldVal, newVal string) string {
	switch changeType {
	case "added":
		return fmt.Sprintf("%s: added %s", path, newVal)
	case "removed":
		return fmt.Sprintf("%s: removed (was %s)", path, oldVal)
	case "changed", "changed type":
		return fmt.Sprintf("%s: %s → %s", path, oldVal, newVal)
	default:
		return fmt.Sprintf("%s: %s", path, changeType)
	}
}

// formatValue formats a value for display in a diff.
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return quote(val)
	case nil:
		return "null"
	case map[string]any, []any:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// quote wraps a string in double quotes.
func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

// joinPath creates a dotted path, handling the root case.
func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// unifiedDiff produces a simple unified diff between two byte slices.
func unifiedDiff(path string, base, head []byte) string {
	baseLines := splitLines(string(base))
	headLines := splitLines(string(head))

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("--- a/%s\n", path))
	buf.WriteString(fmt.Sprintf("+++ b/%s\n", path))

	// Simple line-by-line comparison (not a full Myers diff, but sufficient for context)
	maxLines := len(baseLines)
	if len(headLines) > maxLines {
		maxLines = len(headLines)
	}

	for i := range maxLines {
		switch {
		case i >= len(baseLines):
			buf.WriteString(fmt.Sprintf("+%s\n", headLines[i]))
		case i >= len(headLines):
			buf.WriteString(fmt.Sprintf("-%s\n", baseLines[i]))
		case baseLines[i] != headLines[i]:
			buf.WriteString(fmt.Sprintf("-%s\n", baseLines[i]))
			buf.WriteString(fmt.Sprintf("+%s\n", headLines[i]))
		}
	}

	return buf.String()
}

// splitLines splits text into lines, handling various line endings.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// slicesEqual checks if two string slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// safeSlice returns a sub-slice starting at index, or empty if index is out of bounds.
func safeSlice(s [][]string, start int) [][]string {
	if start >= len(s) {
		return nil
	}
	return s[start:]
}

// fallbackResult returns a unified diff with a message explaining why semantic diff failed.
func fallbackResult(path string, base, head []byte, message string) SemanticDiffResult {
	return SemanticDiffResult{
		Format:  DiffFormatFallback,
		Diff:    unifiedDiff(path, base, head),
		Message: message + ", using unified diff",
	}
}

// DetectDiffFormat returns the DiffFormat for a file path based on extension.
func DetectDiffFormat(path string) DiffFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return DiffFormatJSON
	case ".yaml", ".yml":
		return DiffFormatYAML
	case ".csv":
		return DiffFormatCSV
	case ".toml":
		return DiffFormatTOML
	default:
		return DiffFormatUnified
	}
}
