package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
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
			},
		},
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}

			alerts, resp, err := client.Dependabot.ListRepoAlerts(ctx, owner, repo, &github.ListAlertsOptions{
				State:    ToStringPtr(state),
				Severity: ToStringPtr(severity),
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
