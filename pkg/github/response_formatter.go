package github

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// FormatResponse is an universal response formatter
func FormatResponse(data any, flags FeatureFlags) (*mcp.CallToolResult, error) {
	// Use TOON format when TOONFormat is enabled
	// if flags.TOONFormat {
	// 	output, err := gotoon.Encode(data)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to encode as TOON: %w", err)
	// 	}
	// 	return mcp.NewToolResultText(string(output)), nil
	// }

	// Use CSV format when CSVFormat is enabled
	if flags.CSVFormat {
		// Extract actual data and metadata (search and pagination info)
		itemsData, metadata := extractDataAndMetadata(data)

		csvOutput, err := toCSV(itemsData)
		if err != nil {
			return nil, fmt.Errorf("failed to format as CSV: %w", err)
		}

		if csvOutput == "" {
			return mcp.NewToolResultText("No data has been found"), nil
		}

		// Prepend metadata as comments if provided
		if metadata != "" {
			csvOutput = metadata + csvOutput
		}

		return mcp.NewToolResultText(csvOutput), nil
	}

	// Default to original JSON format
	output, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode as JSON: %w", err)
	}
	return mcp.NewToolResultText(string(output)), nil
}

// extractDataAndMetadata separates main data and metadata from the response
func extractDataAndMetadata(data any) (any, string) {
	v := reflect.ValueOf(data)
	v, isNil := unwrap(v)
	if isNil || !v.IsValid() || v.Kind() != reflect.Struct {
		return data, ""
	}

	t := v.Type()

	// Find the first slice/array field (the main data) and collect metadata from other fields
	arrayFieldIndex := -1
	var metadataStr strings.Builder

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldKind := v.Field(i).Kind()
		isArray := fieldKind == reflect.Slice || fieldKind == reflect.Array

		// First array field becomes the data
		if isArray && arrayFieldIndex < 0 {
			arrayFieldIndex = i
			continue
		}

		// All other exported fields become metadata (whether we found array yet or not)
		if !isArray {
			if metadataStr.Len() == 0 {
				metadataStr.WriteString("# Metadata\n")
			}
			jsonBytes, _ := json.Marshal(v.Field(i).Interface())
			metadataStr.WriteString(fmt.Sprintf("# %s: %s\n", getFieldName(field), string(jsonBytes)))
		}
	}

	// No array field found - return struct as-is
	if arrayFieldIndex < 0 {
		return v.Interface(), ""
	}

	arrayData := v.Field(arrayFieldIndex).Interface()

	if metadataStr.Len() > 0 {
		metadataStr.WriteString("#\n")
		return arrayData, metadataStr.String()
	}

	return arrayData, ""
}

// toCSV converts data to CSV format with the following rules:
// - Nested objects are flattened one level with dot notation (e.g., "user.login")
// - Only primitive fields are extracted from nested structs (we skip complex objects)
// - URL fields are filtered out to reduce token cost
// - Empty columns are automatically removed
func toCSV(data any) (string, error) {
	v, isNil := unwrap(reflect.ValueOf(data))
	if isNil {
		return "", nil
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return "", fmt.Errorf("toCSV expects slice/array, got %v", v.Kind())
	}

	if v.Len() == 0 {
		return "", nil
	}

	return sliceToCSV(v)
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
// NOTE: Two passes are needed just because we opted to try the fill rate filtering approach.
// If we decide to not go with this (e.g., maybe just check if the column is empty),
// we can simplify to a single pass where we stream directly to CSV.
func sliceToCSV(slice reflect.Value) (string, error) {
	if slice.Len() == 0 {
		return "", nil
	}

	// Get all possible headers from first element
	firstElem, isNil := unwrap(slice.Index(0))
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
	values := make([]string, len(headers))
	v, isNil := unwrap(v)
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
