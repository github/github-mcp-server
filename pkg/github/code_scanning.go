package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func GetCodeScanningAlert(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_code_scanning_alert",
			mcp.WithDescription(t("TOOL_GET_CODE_SCANNING_ALERT_DESCRIPTION", "Get details of a specific code scanning alert in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_CODE_SCANNING_ALERT_USER_TITLE", "Get code scanning alert"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The owner of the repository."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("The name of the repository."),
			),
			mcp.WithNumber("alertNumber",
				mcp.Required(),
				mcp.Description("The number of the alert."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			alertNumber, err := RequiredInt(request, "alertNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			alert, resp, err := client.CodeScanning.GetAlert(ctx, owner, repo, int64(alertNumber))
			if err != nil {
				return nil, fmt.Errorf("failed to get alert: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get alert: %s", string(body))), nil
			}

			r, err := json.Marshal(alert)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal alert: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func ListCodeScanningAlerts(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_code_scanning_alerts",
			mcp.WithDescription(t("TOOL_LIST_CODE_SCANNING_ALERTS_DESCRIPTION", "List code scanning alerts in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_CODE_SCANNING_ALERTS_USER_TITLE", "List code scanning alerts"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The owner of the repository."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("The name of the repository."),
			),
			mcp.WithString("ref",
				mcp.Description("The Git reference for the results you want to list."),
			),
			mcp.WithString("state",
				mcp.Description("Filter code scanning alerts by state. Defaults to open"),
				mcp.DefaultString("open"),
				mcp.Enum("open", "closed", "dismissed", "fixed"),
			),
			mcp.WithString("severity",
				mcp.Description("Filter code scanning alerts by severity"),
				mcp.Enum("critical", "high", "medium", "low", "warning", "note", "error"),
			),
			mcp.WithString("tool_name",
				mcp.Description("The name of the tool used for code scanning."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ref, err := OptionalParam[string](request, "ref")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			severity, err := OptionalParam[string](request, "severity")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			toolName, err := OptionalParam[string](request, "tool_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			alerts, resp, err := client.CodeScanning.ListAlertsForRepo(ctx, owner, repo, &github.AlertListOptions{Ref: ref, State: state, Severity: severity, ToolName: toolName})
			if err != nil {
				return nil, fmt.Errorf("failed to list alerts: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list alerts: %s", string(body))), nil
			}

			r, err := json.Marshal(alerts)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal alerts: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
