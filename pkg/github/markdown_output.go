package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	adaptiveMarkdownMinimumRows    = 10
	adaptiveMarkdownMaxSizePercent = 95
)

type adaptiveMarkdownPolicy struct {
	rowPath []string
}

var adaptiveMarkdownPolicies = map[string]adaptiveMarkdownPolicy{
	"get_file_contents":    {},
	"list_issues":          {rowPath: []string{"issues"}},
	"search_issues":        {rowPath: []string{"items"}},
	"list_pull_requests":   {},
	"search_pull_requests": {rowPath: []string{"items"}},
}

var markdownColumnPriority = map[string]int{
	"id":                 -1,
	"number":             0,
	"title":              1,
	"body":               2,
	"state":              3,
	"draft":              4,
	"merged":             5,
	"status":             6,
	"conclusion":         7,
	"name":               8,
	"path":               9,
	"filename":           10,
	"sha":                11,
	"html_url":           12,
	"url":                13,
	"created_at":         14,
	"updated_at":         15,
	"total_count":        16,
	"totalCount":         17,
	"incomplete_results": 18,
}

// withAdaptiveMarkdownOutput wraps only the high-value tools whose successful
// responses contain a known collection of repeated records.
func withAdaptiveMarkdownOutput(tools []inventory.ServerTool) []inventory.ServerTool {
	for i := range tools {
		policy, ok := adaptiveMarkdownPolicies[tools[i].Tool.Name]
		if !ok {
			continue
		}
		tools[i].HandlerFunc = wrapHandlerWithAdaptiveMarkdownOutput(tools[i].HandlerFunc, policy)
	}
	return tools
}

func isAdaptiveMarkdownOutputTool(name string) bool {
	_, ok := adaptiveMarkdownPolicies[name]
	return ok
}

func wrapHandlerWithAdaptiveMarkdownOutput(next inventory.HandlerFunc, policy adaptiveMarkdownPolicy) inventory.HandlerFunc {
	return func(deps any) mcp.ToolHandler {
		handler := next(deps)
		markdownDeps, _ := deps.(ToolDependencies)
		return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := handler(ctx, req)
			if err != nil || result == nil || result.IsError {
				return result, err
			}
			if markdownDeps == nil || !markdownDeps.IsFeatureEnabled(ctx, FeatureFlagMarkdownOutput) {
				return result, nil
			}

			converted, err := convertJSONTextResultToAdaptiveMarkdown(result, policy)
			if err != nil {
				return nil, fmt.Errorf("failed to convert response to Markdown: %w", err)
			}
			return converted, nil
		}
	}
}

func convertJSONTextResultToAdaptiveMarkdown(result *mcp.CallToolResult, policy adaptiveMarkdownPolicy) (*mcp.CallToolResult, error) {
	if result == nil || result.IsError || len(result.Content) != 1 {
		return result, nil
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !json.Valid([]byte(text.Text)) {
		return result, nil
	}

	markdown, rowCount, ok, err := renderAdaptiveMarkdown(text.Text, policy)
	if err != nil {
		return nil, err
	}
	if !ok || rowCount < adaptiveMarkdownMinimumRows {
		return result, nil
	}
	if len(markdown)*100 > len(text.Text)*adaptiveMarkdownMaxSizePercent {
		return result, nil
	}

	text.Text = markdown
	result.StructuredContent = nil
	return result, nil
}

func renderAdaptiveMarkdown(text string, policy adaptiveMarkdownPolicy) (string, int, bool, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", 0, false, fmt.Errorf("failed to unmarshal JSON text: %w", err)
	}

	rows, metadata, collectionName, ok := markdownRows(value, policy.rowPath)
	if !ok {
		return "", 0, false, nil
	}
	if len(rows) < adaptiveMarkdownMinimumRows {
		return "", len(rows), true, nil
	}

	flattenedRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		object, ok := row.(map[string]any)
		if !ok {
			return "", 0, false, nil
		}
		flattened := flattenedMarkdownFields(object)
		if len(flattened) == 0 {
			flattened = map[string]any{"value": map[string]any{}}
		}
		flattenedRows = append(flattenedRows, flattened)
	}

	var buf bytes.Buffer
	if len(metadata) > 0 {
		if err := writeMarkdownFields(&buf, metadata); err != nil {
			return "", 0, false, err
		}
		buf.WriteString("\n\n")
	}
	if collectionName != "" {
		fmt.Fprintf(&buf, "## %s\n\n", escapeMarkdownText(collectionName, false))
	}
	if err := writeMarkdownTable(&buf, flattenedRows); err != nil {
		return "", 0, false, err
	}

	return strings.TrimSuffix(buf.String(), "\n"), len(rows), true, nil
}

func markdownRows(value any, path []string) ([]any, map[string]any, string, bool) {
	if len(path) == 0 {
		rows, ok := value.([]any)
		return rows, nil, "", ok
	}

	root, ok := value.(map[string]any)
	if !ok {
		return nil, nil, "", false
	}

	var current any = root
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, nil, "", false
		}
		current, ok = object[key]
		if !ok {
			return nil, nil, "", false
		}
	}

	rows, ok := current.([]any)
	if !ok {
		return nil, nil, "", false
	}
	return rows, metadataWithoutPath(root, path), strings.Join(path, "."), true
}

func writeMarkdownFields(buf *bytes.Buffer, value map[string]any) error {
	headers := markdownHeaders([]map[string]any{value})
	for i, header := range headers {
		rendered, err := markdownValue(value[header])
		if err != nil {
			return err
		}
		if i > 0 {
			buf.WriteByte('\n')
		}
		fmt.Fprintf(buf, "- %s: %s", markdownColumnName("", header), rendered)
	}
	return nil
}

func writeMarkdownTable(buf *bytes.Buffer, rows []map[string]any) error {
	headers := markdownHeaders(rows)
	writeMarkdownTableRow(buf, headers)

	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	writeMarkdownTableRow(buf, separators)

	for _, row := range rows {
		cells := make([]string, len(headers))
		for i, header := range headers {
			value, ok := row[header]
			if !ok {
				continue
			}
			rendered, err := markdownValue(value)
			if err != nil {
				return err
			}
			cells[i] = rendered
		}
		writeMarkdownTableRow(buf, cells)
	}
	return nil
}

func writeMarkdownTableRow(buf *bytes.Buffer, cells []string) {
	buf.WriteString("| ")
	buf.WriteString(strings.Join(cells, " | "))
	buf.WriteString(" |\n")
}

func flattenedMarkdownFields(value map[string]any) map[string]any {
	fields := make(map[string]any)
	appendFlattenedMarkdownFields(fields, value, "")
	return fields
}

func appendFlattenedMarkdownFields(fields map[string]any, value map[string]any, prefix string) {
	for key, raw := range value {
		column := markdownColumnName(prefix, key)
		child, ok := raw.(map[string]any)
		if !ok {
			fields[column] = raw
			continue
		}
		if len(child) == 0 {
			fields[column] = child
			continue
		}
		appendFlattenedMarkdownFields(fields, child, column)
	}
}

func markdownColumnName(prefix, key string) string {
	key = strings.ReplaceAll(escapeMarkdownText(key, false), `.`, `\.`)
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func markdownHeaders(rows []map[string]any) []string {
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
	sort.Slice(headers, func(i, j int) bool {
		leftPriority, leftPreferred := markdownColumnPriority[headers[i]]
		rightPriority, rightPreferred := markdownColumnPriority[headers[j]]
		switch {
		case leftPreferred && rightPreferred:
			return leftPriority < rightPriority
		case leftPreferred:
			return true
		case rightPreferred:
			return false
		default:
			return headers[i] < headers[j]
		}
	})
	return headers
}

func markdownValue(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "null", nil
	case string:
		return markdownString(v), nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal Markdown value: %w", err)
		}
		return escapeMarkdownText(string(encoded), false), nil
	}
}

func markdownString(value string) string {
	quoted := markdownStringNeedsQuotes(value)
	return escapeMarkdownText(value, quoted)
}

func markdownStringNeedsQuotes(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return true
	}
	if json.Valid([]byte(value)) {
		return true
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func escapeMarkdownText(value string, quoted bool) string {
	var buf strings.Builder
	if quoted {
		buf.WriteByte('"')
	}
	for _, r := range value {
		switch r {
		case '\\':
			buf.WriteString(`\\`)
		case '|':
			buf.WriteString(`\|`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		case '"':
			if quoted {
				buf.WriteString(`\"`)
			} else {
				buf.WriteRune(r)
			}
		default:
			if unicode.IsControl(r) {
				fmt.Fprintf(&buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	if quoted {
		buf.WriteByte('"')
	}
	return buf.String()
}
