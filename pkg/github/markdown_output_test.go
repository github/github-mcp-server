package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONTextToMarkdownRendersRecordArray(t *testing.T) {
	input := `[
		{
			"author": {"login": "monalisa"},
			"body": "Looks good!\nShip | it",
			"created_at": "2026-07-31T10:00:00Z",
			"empty": "",
			"enabled": false,
			"labels": ["bug", "help wanted"],
			"missing": null
		}
	]`

	markdown, err := JSONTextToMarkdown(input)
	require.NoError(t, err)
	assert.Equal(t, `## item 1

- body: Looks good!\nShip \| it
- created_at: 2026-07-31T10:00:00Z
- author: {"login":"monalisa"}
- empty: ""
- enabled: false
- labels: ["bug","help wanted"]
- missing: null`, markdown)
}

func TestJSONTextToMarkdownRendersWrapperMetadata(t *testing.T) {
	input := `{
		"total_count": 2,
		"incomplete_results": false,
		"items": [
			{"number": 42, "title": "First"},
			{"number": 43, "title": "Second"}
		]
	}`

	markdown, err := JSONTextToMarkdown(input)
	require.NoError(t, err)
	assert.Equal(t, `- total_count: 2
- incomplete_results: false

## items

| number | title |
| --- | --- |
| 42 | First |
| 43 | Second |`, markdown)
}

func TestJSONTextToMarkdownPreservesEmptyAndAmbiguousValues(t *testing.T) {
	input := `{
		"id": "123",
		"url": "https://github.com/owner/repo/pull/42",
		"labels": [],
		"empty_object": {},
		"literal_boolean": "true",
		"literal_array": "[]",
		"leading_zero": "007",
		"whitespace": " surrounded "
	}`

	markdown, err := JSONTextToMarkdown(input)
	require.NoError(t, err)
	assert.Equal(t, `- id: "123"
- url: https://github.com/owner/repo/pull/42
- empty_object: {}
- labels: []
- leading_zero: "007"
- literal_array: "[]"
- literal_boolean: "true"
- whitespace: " surrounded "`, markdown)
}

func TestJSONTextToMarkdownKeepsLabelsOnSingleObject(t *testing.T) {
	input := `{"number":42,"title":"A pull request","labels":["bug"]}`

	markdown, err := JSONTextToMarkdown(input)
	require.NoError(t, err)
	assert.Equal(t, `- number: 42
- title: A pull request
- labels: ["bug"]`, markdown)
}

func TestJSONTextToMarkdownEscapesDistinctColumnPaths(t *testing.T) {
	input := `[
		{"a.b":"literal","a":{"b":"nested"},"pipe|key":"pipe"},
		{"a.b":"literal 2","a":{"b":"nested 2"},"pipe|key":"pipe 2"}
	]`

	markdown, err := JSONTextToMarkdown(input)
	require.NoError(t, err)
	assert.Contains(t, markdown, `| a.b | a\.b | pipe\|key |`)
	assert.Contains(t, markdown, `| nested | literal | pipe |`)
}

func TestJSONTextToMarkdownPreservesMultiRowCellDistinctions(t *testing.T) {
	input := `[
		{"name":"first","body":"line 1\nline 2 | value","empty":"","nil":null},
		{"name":"second","other":"present"}
	]`

	markdown, err := JSONTextToMarkdown(input)
	require.NoError(t, err)
	assert.Equal(t, `| body | name | empty | nil | other |
| --- | --- | --- | --- | --- |
| line 1\nline 2 \| value | first | "" | null |  |
|  | second |  |  | present |`, markdown)
}

func TestJSONTextToMarkdownHandlesEmptyDocuments(t *testing.T) {
	tests := map[string]string{
		"array":  "[]",
		"object": "{}",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			markdown, err := JSONTextToMarkdown(input)
			require.NoError(t, err)
			assert.Equal(t, input, markdown)
		})
	}
}

func TestJSONTextToMarkdownRejectsMultipleValues(t *testing.T) {
	_, err := JSONTextToMarkdown(`{"one":1} {"two":2}`)
	require.ErrorContains(t, err, "multiple JSON values")
}

func TestMarkdownOutputWrapsOnlySelectedTools(t *testing.T) {
	const response = `[{"number":42,"title":"A pull request"}]`

	for name := range markdownOutputTools {
		t.Run(name, func(t *testing.T) {
			tools := withMarkdownOutput([]inventory.ServerTool{testCSVOutputTool(name, response)})
			require.Len(t, tools, 1)

			result, err := tools[0].Handler(newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput))(
				ContextWithDeps(context.Background(), newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput)),
				testCSVOutputRequest(),
			)
			require.NoError(t, err)
			assert.Equal(t, "## item 1\n\n- number: 42\n- title: A pull request", textResult(t, result))
		})
	}

	tools := withMarkdownOutput([]inventory.ServerTool{testCSVOutputTool("get_issue", response)})
	result, err := tools[0].Handler(newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput))(
		ContextWithDeps(context.Background(), newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput)),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.JSONEq(t, response, textResult(t, result))
}

func TestMarkdownOutputPreservesJSONWhenFlagOff(t *testing.T) {
	const response = `[{"number":42}]`
	tools := withMarkdownOutput([]inventory.ServerTool{testCSVOutputTool("list_pull_requests", response)})
	deps := newMarkdownOutputTestDeps()

	result, err := tools[0].Handler(deps)(ContextWithDeps(context.Background(), deps), testCSVOutputRequest())
	require.NoError(t, err)
	assert.JSONEq(t, response, textResult(t, result))
}

func TestMarkdownOutputLeavesExistingTextAndResourcesUnchanged(t *testing.T) {
	diff := utils.NewToolResultText("diff --git a/file b/file")
	assert.Same(t, diff, convertJSONTextResultToMarkdown(diff))

	resource := utils.NewToolResultResource("downloaded file", &mcp.ResourceContents{
		URI:      "repo://owner/repo/refs/heads/main/contents/file",
		Text:     "contents",
		MIMEType: "text/plain",
	})
	assert.Same(t, resource, convertJSONTextResultToMarkdown(resource))
}

func TestMarkdownOutputPreservesResultMetadata(t *testing.T) {
	result := utils.NewToolResultText(`{"id":"123","url":"https://example.com"}`)
	result.Meta = mcp.Meta{"ifc": map[string]any{"confidentiality": "public"}}
	result.StructuredContent = map[string]any{"id": "123"}

	converted := convertJSONTextResultToMarkdown(result)
	assert.Equal(t, result.Meta, converted.Meta)
	assert.Nil(t, converted.StructuredContent)
	assert.Equal(t, "- id: \"123\"\n- url: https://example.com", textResult(t, converted))
}

func TestMarkdownOutputTakesPrecedenceOverCSV(t *testing.T) {
	const response = `[{"number":42,"title":"A pull request"}]`
	tools := withMarkdownOutput(withCSVOutput([]inventory.ServerTool{
		testCSVOutputTool("list_pull_requests", response),
	}))
	deps := newMarkdownOutputTestDeps(FeatureFlagCSVOutput, FeatureFlagMarkdownOutput)

	result, err := tools[0].Handler(deps)(ContextWithDeps(context.Background(), deps), testCSVOutputRequest())
	require.NoError(t, err)
	assert.Equal(t, "## item 1\n\n- number: 42\n- title: A pull request", textResult(t, result))
}

type markdownOutputTestDeps struct {
	stubDeps
	enabled map[string]bool
}

func newMarkdownOutputTestDeps(flags ...string) markdownOutputTestDeps {
	enabled := make(map[string]bool, len(flags))
	for _, flag := range flags {
		enabled[flag] = true
	}
	return markdownOutputTestDeps{
		stubDeps: stubDeps{obsv: stubExporters()},
		enabled:  enabled,
	}
}

func (d markdownOutputTestDeps) IsFeatureEnabled(_ context.Context, flag string) bool {
	return d.enabled[flag]
}

func TestMarkdownOutputToolResponsesRemainJSONDecodableWhenDisabled(t *testing.T) {
	const response = `{"id":"123","url":"https://example.com"}`
	tool := withMarkdownOutput([]inventory.ServerTool{testCSVOutputTool("create_pull_request", response)})[0]
	deps := newMarkdownOutputTestDeps()

	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), testCSVOutputRequest())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(textResult(t, result)), &decoded))
	assert.Equal(t, "123", decoded["id"])
}
