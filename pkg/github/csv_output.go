package github

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Ordered by preference when a response wrapper contains multiple arrays.
var primaryCSVRowKeys = []string{
	"items",
	"issues",
	"discussions",
	"categories",
	"labels",
	"alerts",
	"advisories",
	"notifications",
	"gists",
	"repositories",
	"commits",
	"branches",
	"tags",
	"releases",
	"users",
	"teams",
	"members",
	"projects",
	"nodes",
}

type csvOutputDocument struct {
	metadata map[string]string
	rows     []map[string]string
}

func withCSVOutputVariants(tools []inventory.ServerTool) []inventory.ServerTool {
	result := make([]inventory.ServerTool, 0, len(tools))
	for _, tool := range tools {
		if !isCSVOutputTool(tool) {
			result = append(result, tool)
			continue
		}

		jsonOnly := tool
		jsonOnly.FeatureFlagDisable = FeatureFlagCSVOutput
		result = append(result, jsonOnly)

		csvCapable := tool
		csvCapable.FeatureFlagEnable = FeatureFlagCSVOutput
		csvCapable.HandlerFunc = wrapHandlerWithCSVOutput(tool.HandlerFunc)
		result = append(result, csvCapable)
	}
	return result
}

func isCSVOutputTool(tool inventory.ServerTool) bool {
	if !tool.Toolset.Default {
		return false
	}
	if !strings.HasPrefix(tool.Tool.Name, "list_") {
		return false
	}
	return tool.FeatureFlagEnable == "" && tool.FeatureFlagDisable == ""
}

func wrapHandlerWithCSVOutput(next inventory.HandlerFunc) inventory.HandlerFunc {
	return func(deps any) mcp.ToolHandler {
		handler := next(deps)
		return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := handler(ctx, req)
			if err != nil || result == nil || result.IsError {
				return result, err
			}

			return convertJSONTextResultToCSV(result), nil
		}
	}
}

func convertJSONTextResultToCSV(result *mcp.CallToolResult) *mcp.CallToolResult {
	if len(result.Content) != 1 {
		return utils.NewToolResultError("failed to convert response to CSV: expected a single text content response")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return utils.NewToolResultError("failed to convert response to CSV: expected a text content response")
	}

	csvText, err := jsonTextToCSV(text.Text)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to convert response to CSV", err)
	}

	result.Content = []mcp.Content{&mcp.TextContent{Text: csvText}}
	result.StructuredContent = nil
	return result
}

func jsonTextToCSV(text string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON text: %w", err)
	}

	doc := csvDocument(value)
	if len(doc.metadata) == 0 && len(doc.rows) == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	writeCSVMetadata(&buf, doc.metadata)
	if len(doc.rows) == 0 {
		return buf.String(), nil
	}

	headers := csvHeaders(doc.rows)
	if len(headers) == 0 {
		return buf.String(), nil
	}

	writer := csv.NewWriter(&buf)
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, row := range doc.rows {
		record := make([]string, len(headers))
		for i, header := range headers {
			record[i] = row[header]
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("failed to flush CSV: %w", err)
	}
	return buf.String(), nil
}

func csvDocument(value any) csvOutputDocument {
	switch v := value.(type) {
	case []any:
		return csvOutputDocument{rows: csvRowsFromArray(v)}
	case map[string]any:
		if rows, metadata, ok := primaryRowsFromMap(v); ok {
			return csvOutputDocument{
				metadata: newFlattenedCSVRow(metadata),
				rows:     csvRowsFromArray(rows),
			}
		}
		return csvOutputDocument{rows: []map[string]string{newFlattenedCSVRow(v)}}
	default:
		return csvOutputDocument{rows: []map[string]string{scalarCSVRow(v)}}
	}
}

func primaryRowsFromMap(value map[string]any) ([]any, map[string]any, bool) {
	if rows, path, ok := primaryRowsAtCurrentLevel(value); ok {
		return rows, metadataWithoutPath(value, path), true
	}
	if rows, path, ok := primaryRowsOneLevelDown(value); ok {
		return rows, metadataWithoutPath(value, path), true
	}
	return nil, nil, false
}

func primaryRowsAtCurrentLevel(value map[string]any) ([]any, []string, bool) {
	if key, ok := preferredPrimaryRowKey(value); ok {
		rows, _ := value[key].([]any)
		return rows, []string{key}, true
	}
	if key, ok := singleArrayKey(value); ok {
		rows, _ := value[key].([]any)
		return rows, []string{key}, true
	}
	return nil, nil, false
}

func primaryRowsOneLevelDown(value map[string]any) ([]any, []string, bool) {
	var matchedRows []any
	var matchedPath []string
	for key, raw := range value {
		child, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rows, path, ok := primaryRowsAtCurrentLevel(child)
		if !ok {
			continue
		}
		if matchedPath != nil {
			return nil, nil, false
		}
		matchedRows = rows
		matchedPath = append([]string{key}, path...)
	}
	if matchedPath == nil {
		return nil, nil, false
	}
	return matchedRows, matchedPath, true
}

func metadataWithoutPath(value map[string]any, path []string) map[string]any {
	metadata := make(map[string]any, len(value))
	for key, raw := range value {
		if key != path[0] {
			metadata[key] = raw
			continue
		}

		if len(path) == 1 {
			continue
		}
		child, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		childMetadata := metadataWithoutPath(child, path[1:])
		if len(childMetadata) > 0 {
			metadata[key] = childMetadata
		}
	}
	return metadata
}

func csvRowsFromArray(values []any) []map[string]string {
	if len(values) == 0 {
		return nil
	}

	rows := make([]map[string]string, 0, len(values))
	for _, value := range values {
		var row map[string]string
		switch v := value.(type) {
		case map[string]any:
			row = make(map[string]string)
			appendFlattenedCSVFields(row, v, "")
		default:
			row = scalarCSVRow(v)
		}
		rows = append(rows, row)
	}
	return rows
}

func writeCSVMetadata(buf *bytes.Buffer, metadata map[string]string) {
	if len(metadata) == 0 {
		return
	}

	headers := make([]string, 0, len(metadata))
	for header := range metadata {
		headers = append(headers, header)
	}
	sort.Strings(headers)

	for _, header := range headers {
		fmt.Fprintf(buf, "# %s: %s\n", header, normalizeCSVWhitespace(metadata[header]))
	}
	buf.WriteByte('\n')
}

func newFlattenedCSVRow(value map[string]any) map[string]string {
	row := make(map[string]string)
	appendFlattenedCSVFields(row, value, "")
	return row
}

func appendFlattenedCSVFields(row map[string]string, value map[string]any, prefix string) {
	if value == nil {
		return
	}

	for key, raw := range value {
		column := csvColumnName(prefix, key)
		switch v := raw.(type) {
		case map[string]any:
			appendFlattenedCSVFields(row, v, column)
		case []any:
			row[column] = csvArrayValue(v)
		default:
			row[column] = csvColumnValue(column, v)
		}
	}
}

func csvHeaders(rows []map[string]string) []string {
	headerSet := make(map[string]struct{})
	for _, row := range rows {
		for header := range row {
			headerSet[header] = struct{}{}
		}
	}

	headers := make([]string, 0, len(headerSet))
	for header := range headerSet {
		headers = append(headers, header)
	}
	sort.Strings(headers)
	return headers
}

func csvColumnName(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func preferredPrimaryRowKey(value map[string]any) (string, bool) {
	for _, key := range primaryCSVRowKeys {
		if _, ok := value[key].([]any); ok {
			return key, true
		}
	}
	return "", false
}

func singleArrayKey(value map[string]any) (string, bool) {
	var arrayKey string
	for key, raw := range value {
		if _, ok := raw.([]any); !ok {
			continue
		}
		if arrayKey != "" {
			return "", false
		}
		arrayKey = key
	}
	if arrayKey == "" {
		return "", false
	}
	return arrayKey, true
}

func csvColumnValue(column string, value any) string {
	str := scalarCSVValue(value)
	if isBodyColumn(column) {
		return normalizeCSVWhitespace(str)
	}
	return str
}

func csvArrayValue(values []any) string {
	if len(values) == 0 {
		return ""
	}

	// Scalar arrays use semicolons for compactness. This is lossy if an
	// element contains a semicolon; use JSON mode when exact reconstruction matters.
	parts := make([]string, 0, len(values))
	for _, value := range values {
		switch value.(type) {
		case map[string]any, []any:
			encoded, err := json.Marshal(value)
			if err != nil {
				parts = append(parts, scalarCSVValue(value))
			} else {
				parts = append(parts, string(encoded))
			}
		default:
			parts = append(parts, scalarCSVValue(value))
		}
	}
	return strings.Join(parts, ";")
}

func scalarCSVRow(value any) map[string]string {
	return map[string]string{"value": scalarCSVValue(value)}
}

func scalarCSVValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func isBodyColumn(column string) bool {
	return column == "body" || strings.HasSuffix(column, ".body")
}

func normalizeCSVWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
