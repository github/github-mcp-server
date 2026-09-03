package github

import (
	"context"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAffectedDispatchersReportSupportedMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tool       func() inventory.ServerTool
		requestArg map[string]any
	}{
		{
			name: "pull request read",
			tool: func() inventory.ServerTool {
				return PullRequestRead(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(1),
			},
		},
		{
			name: "issue read",
			tool: func() inventory.ServerTool {
				return IssueRead(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
			},
		},
		{
			name: "sub issue write",
			tool: func() inventory.ServerTool {
				return SubIssueWrite(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"sub_issue_id": float64(2),
			},
		},
		{
			name: "actions list",
			tool: func() inventory.ServerTool {
				return ActionsList(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"resource_id": "1",
			},
		},
		{
			name: "actions get",
			tool: func() inventory.ServerTool {
				return ActionsGet(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"resource_id": "1",
			},
		},
		{
			name: "actions run",
			tool: func() inventory.ServerTool {
				return ActionsRunTrigger(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(1),
			},
		},
		{
			name: "projects list",
			tool: func() inventory.ServerTool {
				return ProjectsList(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":      "owner",
				"owner_type": "org",
			},
		},
		{
			name: "projects get",
			tool: func() inventory.ServerTool {
				return ProjectsGet(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":          "owner",
				"owner_type":     "org",
				"project_number": float64(1),
			},
		},
		{
			name: "projects write",
			tool: func() inventory.ServerTool {
				return ProjectsWrite(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner":          "owner",
				"owner_type":     "org",
				"project_number": float64(1),
			},
		},
		{
			name: "ui get",
			tool: func() inventory.ServerTool {
				return UIGet(translations.NullTranslationHelper)
			},
			requestArg: map[string]any{
				"owner": "owner",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.tool()
			schema := tool.Tool.InputSchema.(*jsonschema.Schema)
			methodSchema := schema.Properties["method"]
			require.NotNil(t, methodSchema)
			require.NotEmpty(t, methodSchema.Enum)

			methods := make([]string, len(methodSchema.Enum))
			for i, method := range methodSchema.Enum {
				methods[i] = method.(string)
			}

			args := make(map[string]any, len(tc.requestArg)+1)
			maps.Copy(args, tc.requestArg)
			args["method"] = "unknown_method"

			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
			deps := BaseDeps{Client: client, GQLClient: defaultGQLClient}
			request := createMCPRequest(args)
			result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Equal(t,
				"unknown method: unknown_method. Supported methods are: "+strings.Join(methods, ", "),
				getErrorResult(t, result).Text,
			)
		})
	}
}

func TestPullRequestReviewWriteMissingMethodIsRequired(t *testing.T) {
	t.Parallel()

	tool := PullRequestReviewWrite(translations.NullTranslationHelper)
	deps := BaseDeps{GQLClient: defaultGQLClient}
	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(1),
	})

	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Equal(t, "missing required parameter: method", getErrorResult(t, result).Text)
}
