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
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func GetDependabotAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDependabot,
		mcp.Tool{
			Name:        "get_dependabot_alert",
			Description: t("TOOL_GET_DEPENDABOT_ALERT_DESCRIPTION", "Get details of a specific dependabot alert in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_DEPENDABOT_ALERT_USER_TITLE", "Get dependabot alert"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The owner of the repository.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository.",
					},
					"alertNumber": {
						Type:        "number",
						Description: "The number of the alert.",
					},
				},
				Required: []string{"owner", "repo", "alertNumber"},
			},
		},
		[]scopes.Scope{scopes.SecurityEvents},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			alertNumber, err := RequiredInt(args, "alertNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}

			alert, resp, err := client.Dependabot.GetRepoAlert(ctx, owner, repo, alertNumber)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get alert with number '%d'", alertNumber),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, err
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get alert", resp, body), nil, nil
			}

			r, err := json.Marshal(alert)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal alert", err), nil, err
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListOrgDependabotAlerts(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDependabot,
		mcp.Tool{
			Name:        "list_org_dependabot_alerts",
			Description: t("TOOL_LIST_ORG_DEPENDABOT_ALERTS_DESCRIPTION", "List Dependabot alerts for a GitHub organization."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ORG_DEPENDABOT_ALERTS_USER_TITLE", "List org Dependabot alerts"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: "The organization name.",
					},
					"state": {
						Type:        "string",
						Description: "Filter Dependabot alerts by state. Defaults to open",
						Enum:        []any{"open", "fixed", "dismissed", "auto_dismissed"},
						Default:     json.RawMessage(`"open"`),
					},
					"severity": {
						Type:        "string",
						Description: "Filter Dependabot alerts by severity",
						Enum:        []any{"low", "medium", "high", "critical"},
					},
					"ecosystem": {
						Type:        "string",
						Description: "Filter Dependabot alerts by package ecosystem (e.g. npm, pip, maven)",
					},
					"package": {
						Type:        "string",
						Description: "Filter Dependabot alerts by package name",
					},
				},
				Required: []string{"org"},
			}),
		},
		[]scopes.Scope{scopes.SecurityEvents},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			severity, err := OptionalParam[string](args, "severity")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ecosystem, err := OptionalParam[string](args, "ecosystem")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pkg, err := OptionalParam[string](args, "package")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}

			alerts, resp, err := client.Dependabot.ListOrgAlerts(ctx, org, &github.ListAlertsOptions{
				State:     ToStringPtr(state),
				Severity:  ToStringPtr(severity),
				Ecosystem: ToStringPtr(ecosystem),
				Package:   ToStringPtr(pkg),
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list alerts for organisation '%s'", org),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, err
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list alerts", resp, body), nil, nil
			}

			r, err := json.Marshal(alerts)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal alerts", err), nil, err
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func UpdateDependabotAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDependabot,
		mcp.Tool{
			Name:        "update_dependabot_alert",
			Description: t("TOOL_UPDATE_DEPENDABOT_ALERT_DESCRIPTION", "Update the state of a Dependabot alert in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_DEPENDABOT_ALERT_USER_TITLE", "Update Dependabot alert"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The owner of the repository.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository.",
					},
					"alertNumber": {
						Type:        "number",
						Description: "The number of the alert.",
					},
					"state": {
						Type:        "string",
						Description: "The state to set for the alert.",
						Enum:        []any{"open", "dismissed"},
					},
					"dismissedReason": {
						Type:        "string",
						Description: "Required when state is dismissed. The reason for dismissing the alert.",
						Enum:        []any{"fix_started", "inaccurate", "no_bandwidth", "not_used", "tolerable_risk"},
					},
					"dismissedComment": {
						Type:        "string",
						Description: "An optional comment associated with dismissing the alert.",
					},
				},
				Required: []string{"owner", "repo", "alertNumber", "state"},
			},
		},
		[]scopes.Scope{scopes.SecurityEvents},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			alertNumber, err := RequiredInt(args, "alertNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := RequiredParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			dismissedReason, err := OptionalParam[string](args, "dismissedReason")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			dismissedComment, err := OptionalParam[string](args, "dismissedComment")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}

			alert, resp, err := client.Dependabot.UpdateAlert(ctx, owner, repo, alertNumber, &github.DependabotAlertState{
				State:            state,
				DismissedReason:  ToStringPtr(dismissedReason),
				DismissedComment: ToStringPtr(dismissedComment),
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to update alert with number '%d'", alertNumber),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, err
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update alert", resp, body), nil, nil
			}

			r, err := json.Marshal(alert)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal alert", err), nil, err
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListDependabotAlerts(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDependabot,
		mcp.Tool{
			Name:        "list_dependabot_alerts",
			Description: t("TOOL_LIST_DEPENDABOT_ALERTS_DESCRIPTION", "List dependabot alerts in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_DEPENDABOT_ALERTS_USER_TITLE", "List dependabot alerts"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The owner of the repository.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository.",
					},
					"state": {
						Type:        "string",
						Description: "Filter dependabot alerts by state. Defaults to open",
						Enum:        []any{"open", "fixed", "dismissed", "auto_dismissed"},
						Default:     json.RawMessage(`"open"`),
					},
					"severity": {
						Type:        "string",
						Description: "Filter dependabot alerts by severity",
						Enum:        []any{"low", "medium", "high", "critical"},
					},
				},
				Required: []string{"owner", "repo"},
			}),
		},
		[]scopes.Scope{scopes.SecurityEvents},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			severity, err := OptionalParam[string](args, "severity")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}

			alerts, resp, err := client.Dependabot.ListRepoAlerts(ctx, owner, repo, &github.ListAlertsOptions{
				State:    ToStringPtr(state),
				Severity: ToStringPtr(severity),
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list alerts for repository '%s/%s'", owner, repo),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, err
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list alerts", resp, body), nil, nil
			}

			r, err := json.Marshal(alerts)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal alerts", err), nil, err
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
