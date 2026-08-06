package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptiveMarkdownOutputConvertsEligibleTools(t *testing.T) {
	for name, policy := range adaptiveMarkdownPolicies {
		t.Run(name, func(t *testing.T) {
			response := markdownTestResponse(t, policy, markdownTestRows(adaptiveMarkdownMinimumRows))
			tools := withAdaptiveMarkdownOutput([]inventory.ServerTool{testCSVOutputTool(name, response)})
			deps := newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput)

			result, err := tools[0].Handler(deps)(
				ContextWithDeps(context.Background(), deps),
				testCSVOutputRequest(),
			)
			require.NoError(t, err)

			text := textResult(t, result)
			assert.Contains(t, text, "| number | title | body | user.login |")
			assert.False(t, strings.HasPrefix(text, "{"))
			assert.False(t, strings.HasPrefix(text, "["))
			if len(policy.rowPath) > 0 {
				assert.Contains(t, text, "## "+strings.Join(policy.rowPath, "."))
			}
		})
	}
}

func TestAdaptiveMarkdownOutputPreservesJSONBelowThreshold(t *testing.T) {
	policy := adaptiveMarkdownPolicies["list_pull_requests"]
	response := markdownTestResponse(t, policy, markdownTestRows(adaptiveMarkdownMinimumRows-1))
	tools := withAdaptiveMarkdownOutput([]inventory.ServerTool{
		testCSVOutputTool("list_pull_requests", response),
	})
	deps := newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput)

	result, err := tools[0].Handler(deps)(
		ContextWithDeps(context.Background(), deps),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.JSONEq(t, response, textResult(t, result))
}

func TestAdaptiveMarkdownOutputPreservesJSONWhenMarkdownIsNotSmaller(t *testing.T) {
	rows := make([]map[string]any, adaptiveMarkdownMinimumRows)
	for i := range rows {
		rows[i] = map[string]any{fmt.Sprintf("field_%d", i): "x"}
	}
	policy := adaptiveMarkdownPolicies["list_pull_requests"]
	response := markdownTestResponse(t, policy, rows)
	tools := withAdaptiveMarkdownOutput([]inventory.ServerTool{
		testCSVOutputTool("list_pull_requests", response),
	})
	deps := newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput)

	result, err := tools[0].Handler(deps)(
		ContextWithDeps(context.Background(), deps),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.JSONEq(t, response, textResult(t, result))
}

func TestAdaptiveMarkdownOutputPreservesJSONWhenFlagDisabled(t *testing.T) {
	policy := adaptiveMarkdownPolicies["list_pull_requests"]
	response := markdownTestResponse(t, policy, markdownTestRows(adaptiveMarkdownMinimumRows))
	tools := withAdaptiveMarkdownOutput([]inventory.ServerTool{
		testCSVOutputTool("list_pull_requests", response),
	})
	deps := newMarkdownOutputTestDeps()

	result, err := tools[0].Handler(deps)(
		ContextWithDeps(context.Background(), deps),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.JSONEq(t, response, textResult(t, result))
}

func TestAdaptiveMarkdownOutputWrapsOnlySelectedTools(t *testing.T) {
	response := markdownTestResponse(t, adaptiveMarkdownPolicy{rowPath: []string{"items"}}, markdownTestRows(adaptiveMarkdownMinimumRows))
	tools := withAdaptiveMarkdownOutput([]inventory.ServerTool{
		testCSVOutputTool("search_repositories", response),
	})
	deps := newMarkdownOutputTestDeps(FeatureFlagMarkdownOutput)

	result, err := tools[0].Handler(deps)(
		ContextWithDeps(context.Background(), deps),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.JSONEq(t, response, textResult(t, result))
}

func TestAdaptiveMarkdownOutputPreservesDistinctValuesAndMetadata(t *testing.T) {
	rows := markdownTestRows(adaptiveMarkdownMinimumRows)
	rows[0]["body"] = "line 1\nline 2 | value"
	rows[0]["empty"] = ""
	rows[0]["labels"] = []any{"bug", "help wanted"}
	rows[0]["literal_boolean"] = "true"
	rows[0]["missing"] = nil

	encoded, err := json.Marshal(map[string]any{
		"issues": rows,
		"pageInfo": map[string]any{
			"endCursor":   "cursor-10",
			"hasNextPage": false,
		},
		"totalCount": len(rows),
	})
	require.NoError(t, err)

	response := string(encoded)
	result := utils.NewToolResultText(response)
	result.Meta = mcp.Meta{"ifc": map[string]any{"confidentiality": "public"}}
	result.StructuredContent = map[string]any{"issues": rows}

	converted, err := convertJSONTextResultToAdaptiveMarkdown(result, adaptiveMarkdownPolicies["list_issues"])
	require.NoError(t, err)

	text := textResult(t, converted)
	assert.Contains(t, text, "- totalCount: 10")
	assert.Contains(t, text, `- pageInfo: {"endCursor":"cursor-10","hasNextPage":false}`)
	assert.Contains(t, text, "## issues")
	assert.Contains(t, text, "user.login")
	assert.Contains(t, text, `line 1\nline 2 \| value`)
	assert.Contains(t, text, `["bug","help wanted"]`)
	assert.Contains(t, text, `""`)
	assert.Contains(t, text, `"true"`)
	assert.Contains(t, text, "null")
	assert.Equal(t, mcp.Meta{"ifc": map[string]any{"confidentiality": "public"}}, converted.Meta)
	assert.Nil(t, converted.StructuredContent)
}

func TestAdaptiveMarkdownOutputKeepsLiteralAndNestedColumnPathsDistinct(t *testing.T) {
	rows := make([]map[string]any, adaptiveMarkdownMinimumRows)
	for i := range rows {
		rows[i] = map[string]any{
			"a.b":      fmt.Sprintf("literal %d", i),
			"a":        map[string]any{"b": fmt.Sprintf("nested %d", i)},
			"pipe|key": fmt.Sprintf("pipe %d", i),
		}
	}

	response := markdownTestResponse(t, adaptiveMarkdownPolicies["list_pull_requests"], rows)
	result := utils.NewToolResultText(response)
	converted, err := convertJSONTextResultToAdaptiveMarkdown(result, adaptiveMarkdownPolicies["list_pull_requests"])
	require.NoError(t, err)

	text := textResult(t, converted)
	assert.Contains(t, text, `| a.b | a\.b | pipe\|key |`)
	assert.Contains(t, text, "| nested 0 | literal 0 | pipe 0 |")
}

func TestAdaptiveMarkdownOutputLeavesOtherContentUnchanged(t *testing.T) {
	policy := adaptiveMarkdownPolicies["list_pull_requests"]

	raw := utils.NewToolResultText("diff --git a/file b/file")
	converted, err := convertJSONTextResultToAdaptiveMarkdown(raw, policy)
	require.NoError(t, err)
	assert.Same(t, raw, converted)

	resource := utils.NewToolResultResource("downloaded file", &mcp.ResourceContents{
		URI:      "repo://owner/repo/refs/heads/main/contents/file",
		Text:     "contents",
		MIMEType: "text/plain",
	})
	converted, err = convertJSONTextResultToAdaptiveMarkdown(resource, policy)
	require.NoError(t, err)
	assert.Same(t, resource, converted)

	errorResult := utils.NewToolResultError(markdownTestResponse(t, policy, markdownTestRows(adaptiveMarkdownMinimumRows)))
	converted, err = convertJSONTextResultToAdaptiveMarkdown(errorResult, policy)
	require.NoError(t, err)
	assert.Same(t, errorResult, converted)
	assert.True(t, converted.IsError)
}

func TestAdaptiveMarkdownOutputTakesPrecedenceOverCSVForSelectedTools(t *testing.T) {
	candidateResponse := markdownTestResponse(
		t,
		adaptiveMarkdownPolicies["list_pull_requests"],
		markdownTestRows(adaptiveMarkdownMinimumRows),
	)
	otherResponse := `[{"name":"main"},{"name":"release"}]`
	tools := withAdaptiveMarkdownOutput(withCSVOutput([]inventory.ServerTool{
		testCSVOutputTool("list_pull_requests", candidateResponse),
		testCSVOutputTool("list_branches", otherResponse),
	}))
	deps := newMarkdownOutputTestDeps(FeatureFlagCSVOutput, FeatureFlagMarkdownOutput)

	candidateResult, err := tools[0].Handler(deps)(
		ContextWithDeps(context.Background(), deps),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.Contains(t, textResult(t, candidateResult), "| number | title | body | user.login |")

	otherResult, err := tools[1].Handler(deps)(
		ContextWithDeps(context.Background(), deps),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.Equal(t, "name\nmain\nrelease\n", textResult(t, otherResult))
}

func TestAdaptiveMarkdownOutputKeepsSmallSelectedResponsesAsJSONWhenCSVIsAlsoEnabled(t *testing.T) {
	policy := adaptiveMarkdownPolicies["list_pull_requests"]
	response := markdownTestResponse(t, policy, markdownTestRows(1))
	tools := withAdaptiveMarkdownOutput(withCSVOutput([]inventory.ServerTool{
		testCSVOutputTool("list_pull_requests", response),
	}))
	deps := newMarkdownOutputTestDeps(FeatureFlagCSVOutput, FeatureFlagMarkdownOutput)

	result, err := tools[0].Handler(deps)(
		ContextWithDeps(context.Background(), deps),
		testCSVOutputRequest(),
	)
	require.NoError(t, err)
	assert.JSONEq(t, response, textResult(t, result))
}

func markdownTestRows(count int) []map[string]any {
	rows := make([]map[string]any, count)
	for i := range rows {
		rows[i] = map[string]any{
			"number": i + 1,
			"title":  fmt.Sprintf("Item %d", i+1),
			"body":   "Looks good!",
			"user": map[string]any{
				"login": fmt.Sprintf("user-%d", i+1),
			},
		}
	}
	return rows
}

func markdownTestResponse(t *testing.T, policy adaptiveMarkdownPolicy, rows []map[string]any) string {
	t.Helper()

	var value any = rows
	if len(policy.rowPath) > 0 {
		value = map[string]any{
			policy.rowPath[0]: rows,
			"pageInfo": map[string]any{
				"endCursor":   fmt.Sprintf("cursor-%d", len(rows)),
				"hasNextPage": false,
			},
			"total_count": len(rows),
		}
	}

	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
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
