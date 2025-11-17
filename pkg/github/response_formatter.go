package github

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/alpkeskin/gotoon"
	"github.com/mark3labs/mcp-go/mcp"
)

// FormatResponse is an universal response formatter with optional pagination metadata.
func FormatResponse(data interface{}, flags FeatureFlags, dataKey string, metadata ...interface{}) (*mcp.CallToolResult, error) {
	// TOON format
	if flags.TOONFormat {
		output, err := gotoon.Encode(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode as TOON: %w", err)
		}
		return mcp.NewToolResultText(string(output)), nil
	}

	// JSON format - this is just the original behaviour when JSONFormat is enabled
	if flags.JSONFormat {
		var responseData interface{} = data
		if len(metadata) > 0 && dataKey != "" {
			if m, ok := metadata[0].(map[string]interface{}); ok {
				// Build response with data under dataKey + metadata fields
				response := map[string]interface{}{
					dataKey: data,
				}
				for key, value := range m {
					response[key] = value
				}

				responseData = response
			}
		}

		output, err := json.Marshal(responseData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode as JSON: %w", err)
		}
		return mcp.NewToolResultText(string(output)), nil
	}

	// Default CSV format (only applied to `list_commits`, `list_issues`, and `list_pull_requests` for now)
	csvOutput, err := toCSV(data)
	if err != nil {
		return nil, fmt.Errorf("failed to format as CSV: %w", err)
	}

	// Prepend metadata as comments if provided
	if len(metadata) > 0 {
		csvOutput = formatMetadata(metadata[0]) + csvOutput
	}

	return mcp.NewToolResultText(csvOutput), nil
}

// formatMetadata formats pagination metadata as CSV comments
// TODO: When expanding this response format to other `list_` tools, what is the best way to generalize metadata extraction?
// i..e, REST vs GraphQL based tools
func formatMetadata(metadata interface{}) string {
	m, ok := metadata.(map[string]interface{})
	if !ok {
		return ""
	}

	var buf strings.Builder
	buf.WriteString("# Pagination Metadata\n")

	// Extract metadata
	if pageInfo, ok := m["pageInfo"].(map[string]interface{}); ok {
		if hasNextPage, ok := pageInfo["hasNextPage"].(bool); ok && hasNextPage {
			buf.WriteString(fmt.Sprintf("# pageInfo.hasNextPage: %t\n", hasNextPage))
		}
		if hasPreviousPage, ok := pageInfo["hasPreviousPage"].(bool); ok && hasPreviousPage {
			buf.WriteString(fmt.Sprintf("# pageInfo.hasPreviousPage: %t\n", hasPreviousPage))
		}
		if startCursor, ok := pageInfo["startCursor"].(string); ok && startCursor != "" {
			buf.WriteString(fmt.Sprintf("# pageInfo.startCursor: %s\n", startCursor))
		}
		if endCursor, ok := pageInfo["endCursor"].(string); ok && endCursor != "" {
			buf.WriteString(fmt.Sprintf("# pageInfo.endCursor: %s\n", endCursor))
		}
	}

	if totalCount, ok := m["totalCount"].(int); ok && totalCount > 0 {
		buf.WriteString(fmt.Sprintf("# totalCount: %d\n", totalCount))
	}

	buf.WriteString("#\n")
	return buf.String()
}

// toCSV converts any data to CSV format with the following rules:
// - Nested objects are flattened one level with dot notation (e.g., "user.login")
// - Only primitive fields are extracted from nested structs (we skip complex objects)
// - URL fields are filtered out to reduce token cost
// - Empty columns are automatically removed
func toCSV(data interface{}) (string, error) {
	v, isNil := unwrap(reflect.ValueOf(data))
	if isNil {
		return "", nil
	}

	// Handle arrays/slices
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		if v.Len() == 0 {
			return "", nil
		}
		return sliceToCSV(v)
	}

	// NOTE: Should we handle single objects, for instance, for non list_ tools - i.e., single object retrieval?
	// How much values does it add? Using CSV for a single object feels a bit odd
	// 		-> honestly none imo - tested current implementation on get_commit and this does more harm than good
	//       as we lose insight on certain, deeply nested, fields
	if v.Kind() == reflect.Struct {
		slice := reflect.MakeSlice(reflect.SliceOf(v.Type()), 1, 1)
		slice.Index(0).Set(v)
		return sliceToCSV(slice)
	}

	return "", fmt.Errorf("unsupported data type: %v", v.Kind())
}

// unwrap dereferences pointers and interfaces until reaching a concrete value
func unwrap(v reflect.Value) (reflect.Value, bool) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v, true
		}
		v = v.Elem()
	}
	return v, false
}

// sliceToCSV converts a slice to CSV with column filtering:
// - First pass: collect all data and count non-empty values per column
// - Second pass: build CSV with only columns that have sufficient data
//
// Two passes are needed just because we opted to try the fill rate filtering approach.
// If we decide to not go with this (e.g., maybe just check if the column is empty),
// we can simplify to a single pass where we stream directly to CSV.
func sliceToCSV(slice reflect.Value) (string, error) {
	if slice.Len() == 0 {
		return "", nil
	}

	// Get all possible headers from first element
	firstElem := slice.Index(0)
	firstElem, isNil := unwrap(firstElem)
	if isNil {
		return "", nil
	}
	allHeaders := extractStructHeaders(firstElem, "")
	if len(allHeaders) == 0 {
		return "", fmt.Errorf("no fields found in data")
	}

	// First pass
	allRows := make([][]string, slice.Len())
	columnFillCount := make([]int, len(allHeaders))

	for i := 0; i < slice.Len(); i++ {
		elem := slice.Index(i)
		row := extractValues(elem, allHeaders)
		allRows[i] = row

		// Count non-empty values per column
		for j, val := range row {
			if val != "" {
				columnFillCount[j]++
			}
		}
	}

	// Build list of columns with sufficient fill rate
	const minFillRate = 0.1 // TODO: Experiment using different thresholds
	totalRows := slice.Len()
	var activeHeaders []string
	var activeColumnIndices []int

	for i := range allHeaders {
		fillRate := float64(columnFillCount[i]) / float64(totalRows)
		if fillRate > minFillRate {
			activeHeaders = append(activeHeaders, allHeaders[i])
			activeColumnIndices = append(activeColumnIndices, i)
		}
	}

	if len(activeHeaders) == 0 {
		return "", nil
	}

	// Build CSV with only active columns
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	if err := writer.Write(activeHeaders); err != nil {
		return "", err
	}

	for _, row := range allRows {
		activeRow := make([]string, len(activeColumnIndices))
		for i, colIdx := range activeColumnIndices {
			activeRow[i] = row[colIdx]
		}
		if err := writer.Write(activeRow); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getFieldName extracts field name from struct field, preferring json tag
func getFieldName(field reflect.StructField) string {
	if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		if idx := strings.Index(jsonTag, ","); idx > 0 {
			return jsonTag[:idx]
		}
		return jsonTag
	}
	return field.Name
}

// extractStructHeaders gets field names from a struct with selective flattening
func extractStructHeaders(v reflect.Value, prefix string) []string {
	var headers []string
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		fieldValue := v.Field(i)
		fieldValue, isNil := unwrap(fieldValue)

		// When flattening nested structs (prefix != ""), only extract primitive fields
		if prefix != "" {
			// Skip nil fields
			if isNil || !fieldValue.IsValid() {
				continue
			}

			kind := fieldValue.Kind()

			// Skip complex types (arrays, slices, interfaces), only allow primitives and timestamps
			if kind == reflect.Struct {
				typeName := fieldValue.Type().Name()
				if typeName != "Timestamp" && typeName != "Time" {
					continue
				}
			} else if kind == reflect.Slice || kind == reflect.Array || kind == reflect.Interface {
				continue
			}
		}

		// Build full name with prefix
		fieldName := getFieldName(field)
		// TODO: Research how much value do *_url fields add? - these actually add a significant amount of tokens
		if strings.HasSuffix(strings.ToLower(fieldName), "_url") || strings.ToLower(fieldName) == "url" {
			continue
		}

		fullName := fieldName
		if prefix != "" {
			fullName = prefix + "." + fieldName
		}

		// If field is nil, add the header
		if isNil {
			headers = append(headers, fullName)
			continue
		}

		// Only flatten one level deep
		if fieldValue.Kind() == reflect.Struct && prefix == "" {
			typeName := fieldValue.Type().Name()

			// Skip complex structs with no primitives
			if typeName == "Timestamp" || typeName == "Time" {
				headers = append(headers, fullName)
			} else if hasPrimitiveFields(fieldValue) {
				headers = append(headers, extractStructHeaders(fieldValue, fullName)...)
			}
		} else {
			headers = append(headers, fullName)
		}
	}

	return headers
}

// hasPrimitiveFields checks if a struct has at least one primitive field
func hasPrimitiveFields(v reflect.Value) bool {
	for i := 0; i < v.NumField(); i++ {
		field, isNil := unwrap(v.Field(i))
		if isNil || !field.IsValid() {
			continue
		}
		kind := field.Kind()

		// Check for any primitive types
		if kind == reflect.String || kind == reflect.Int || kind == reflect.Int64 || kind == reflect.Bool {
			return true
		}

		// Timestamps count as primitives
		if kind == reflect.Struct {
			typeName := field.Type().Name()
			if typeName == "Timestamp" || typeName == "Time" {
				return true
			}
		}
	}

	return false
}

// extractValues gets field values from a struct in the same order as headers
func extractValues(v reflect.Value, headers []string) []string {
	v, isNil := unwrap(v)
	values := make([]string, len(headers))
	if isNil {
		return values
	}

	for i, header := range headers {
		values[i] = extractFieldValue(v, header)
	}

	return values
}

// extractFieldValue gets a single field value by path
func extractFieldValue(v reflect.Value, path string) string {
	if !v.IsValid() {
		return ""
	}

	for part := range strings.SplitSeq(path, ".") {
		v, _ = unwrap(v)
		if !v.IsValid() {
			return ""
		}

		if v.Kind() == reflect.Struct {
			v = getStructField(v, part)
		} else {
			return ""
		}
	}

	return formatValue(v)
}

// getStructField gets a field from a struct by name
func getStructField(v reflect.Value, name string) reflect.Value {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if getFieldName(t.Field(i)) == name || t.Field(i).Name == name {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// formatTimestamp formats time.Time or Timestamp types to RFC3339 string
func formatTimestamp(v reflect.Value) string {
	if method := v.MethodByName("IsZero"); method.IsValid() {
		if results := method.Call(nil); len(results) > 0 && results[0].Bool() {
			return ""
		}
	}
	if method := v.MethodByName("Format"); method.IsValid() {
		if results := method.Call([]reflect.Value{reflect.ValueOf("2006-01-02T15:04:05Z07:00")}); len(results) > 0 {
			return results[0].String()
		}
	}
	return ""
}

// formatValue converts a reflect.Value to a CSV-safe string with token optimization
func formatValue(v reflect.Value) string {
	v, isNil := unwrap(v)
	if isNil || !v.IsValid() {
		return ""
	}

	switch v.Kind() {
	case reflect.String:
		// Normalize whitespace
		return strings.Join(strings.Fields(v.String()), " ")

	// Handle numeric and boolean types
	case reflect.Int, reflect.Int64:
		if n := v.Int(); n != 0 {
			return fmt.Sprintf("%d", n)
		}

	case reflect.Bool:
		if v.Bool() {
			return "true"
		}

	// Handle collections and nested types
	case reflect.Slice, reflect.Array:
		if n := v.Len(); n > 0 {
			return fmt.Sprintf("[%d items]", n)
		}

	case reflect.Map:
		if n := v.Len(); n > 0 {
			return fmt.Sprintf("[%d keys]", n)
		}

	case reflect.Struct:
		typeName := v.Type().Name()
		if typeName == "Timestamp" || typeName == "Time" {
			return formatTimestamp(v)
		}
		// Skip other nested structs

	default:
		return fmt.Sprintf("%v", v.Interface())
	}

	return ""
}
