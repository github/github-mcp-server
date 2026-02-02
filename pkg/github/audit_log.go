package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func SearchOrgAuditLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name:        "search_org_audit_logs",
			Description: t("TOOL_SEARCH_ORG_AUDIT_LOGS_DESCRIPTION", "Search audit logs for a GitHub organization"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_ORG_AUDIT_LOGS_USER_TITLE", "Search Organization Audit Logs"),
				ReadOnlyHint: true,
			},
			InputSchema: WithCursorPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: "The organization name. The name is not case sensitive.",
					},
					"phrase": {
						Type:        "string",
						Description: "A search phrase. Examples: 'created:>2025-01-01', 'action:repo.create', 'country:US', 'repo:my-repo', 'operation:access', 'actor:octocat'. Search phrase cannot be text only, it must be used with filters.",
					},
					"include": {
						Type:        "string",
						Description: "Events to include. Default value is 'all'.",
						Enum:        []any{"web", "git", "all"},
					},
					"order": {
						Type:        "string",
						Description: "The order of audit log events. Default value is 'desc'.",
						Enum:        []any{"asc", "desc"},
					},
				},
				Required: []string{"org"},
			}),
		},
		[]scopes.Scope{scopes.AdminOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			phrase, err := OptionalParam[string](args, "phrase")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			include, err := OptionalParam[string](args, "include")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			order, err := OptionalParam[string](args, "order")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pagination, err := OptionalCursorPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.GetAuditLogOptions{
				Phrase:  &phrase,
				Include: &include,
				Order:   &order,
				ListCursorOptions: github.ListCursorOptions{
					PerPage: pagination.PerPage,
					After:   pagination.After,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			auditEntries, resp, err := client.Organizations.GetAuditLog(ctx, org, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(
					ctx,
					"failed to get organization audit log",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get organization audit log", resp, body), nil, nil
			}

			response := map[string]any{
				"audit_entries": auditEntries,
				"page_info":     buildPageInfo(resp),
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
