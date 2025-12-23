package github

import (
	"context"
	"encoding/json"
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

func GetCodeScanningAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataCodeSecurity,
		mcp.Tool{
			Name:        "get_code_scanning_alert",
			Description: t("TOOL_GET_CODE_SCANNING_ALERT_DESCRIPTION", "Get details of a specific code scanning alert in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_CODE_SCANNING_ALERT_USER_TITLE", "Get code scanning alert"),
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
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			alert, resp, err := client.CodeScanning.GetAlert(ctx, owner, repo, int64(alertNumber))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get alert",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get alert", resp, body), nil, nil
			}

			r, err := json.Marshal(alert)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal alert", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListCodeScanningAlerts(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataCodeSecurity,
		mcp.Tool{
			Name:        "list_code_scanning_alerts",
			Description: t("TOOL_LIST_CODE_SCANNING_ALERTS_DESCRIPTION", "List code scanning alerts in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_CODE_SCANNING_ALERTS_USER_TITLE", "List code scanning alerts"),
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
						Description: "Filter code scanning alerts by state. Defaults to open",
						Enum:        []any{"open", "closed", "dismissed", "fixed"},
						Default:     json.RawMessage(`"open"`),
					},
					"ref": {
						Type:        "string",
						Description: "The Git reference for the results you want to list.",
					},
					"severity": {
						Type:        "string",
						Description: "Filter code scanning alerts by severity",
						Enum:        []any{"critical", "high", "medium", "low", "warning", "note", "error"},
					},
					"tool_name": {
						Type:        "string",
						Description: "The name of the tool used for code scanning.",
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
			ref, err := OptionalParam[string](args, "ref")
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
			toolName, err := OptionalParam[string](args, "tool_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			alerts, resp, err := client.CodeScanning.ListAlertsForRepo(ctx, owner, repo, &github.AlertListOptions{Ref: ref, State: state, Severity: severity, ToolName: toolName})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list alerts",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list alerts", resp, body), nil, nil
			}

			r, err := json.Marshal(alerts)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal alerts", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
