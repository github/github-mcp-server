package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func GetCodeScanningAlert(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_code_scanning_alert",
			mcp.WithDescription(t("TOOL_GET_CODE_SCANNING_ALERT_DESCRIPTION", "Get details of a specific code scanning alert in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_CODE_SCANNING_ALERT_USER_TITLE", "Get code scanning alert"),
				ReadOnlyHint: ToBoolPtr(true),
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
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
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
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get alert",
					resp,
					err,
				), nil
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
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The owner of the repository."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("The name of the repository."),
			),
			mcp.WithString("state",
				mcp.Description("Filter code scanning alerts by state. Defaults to open"),
				mcp.DefaultString("open"),
				mcp.Enum("open", "closed", "dismissed", "fixed"),
			),
			mcp.WithString("ref",
				mcp.Description("The Git reference for the results you want to list."),
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
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
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
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list alerts",
					resp,
					err,
				), nil
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

func ListOrgCodeScanningAlerts(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_org_code_scanning_alerts",
			mcp.WithDescription(t("TOOL_LIST_ORG_CODE_SCANNING_ALERTS_DESCRIPTION", "List code scanning alerts for a GitHub organization.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ORG_CODE_SCANNING_ALERTS_USER_TITLE", "List org code scanning alerts"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("The organization of the repository."),
			),
			mcp.WithString("sort",
				mcp.Description("Sort by"),
				mcp.Enum("created", "updated"),
			),
			mcp.WithString("severity",
				mcp.Description("Filter code scanning alerts by severity"),
				mcp.Enum("critical", "high", "medium", "low", "warning", "note", "error"),
			),
			mcp.WithString("tool_name",
				mcp.Description("The name of the tool used for code scanning."),
			),
			mcp.WithString("state",
				mcp.Description("Filter code scanning alerts by state. Defaults to open"),
				mcp.DefaultString("open"),
				mcp.Enum("open", "closed", "dismissed", "fixed"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := requiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := OptionalParam[string](request, "sort")
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
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			alerts, resp, err := client.CodeScanning.ListAlertsForOrg(ctx, org, &github.AlertListOptions{Sort: sort, State: state, Severity: severity, ToolName: toolName})
			if err != nil {
				return nil, fmt.Errorf("failed to list organization alerts: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list organization alerts: %s", string(body))), nil
			}

			r, err := json.Marshal(alerts)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal alerts: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func UpdateCodeScanningAlert(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("update_code_scanning_alert",
			mcp.WithDescription(t("TOOL_UPDATE_CODE_SCANNING_ALERT_DESCRIPTION", "Update details of a specific code scanning alert in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UPDATE_CODE_SCANNING_ALERT_USER_TITLE", "Update code scanning alert"),
				ReadOnlyHint: toBoolPtr(false),
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
			mcp.WithString("state",
				mcp.Required(),
				mcp.Description("State of the alert"),
				mcp.Enum("open", "dismissed"),
			),
			mcp.WithString("dismissed_reason",
				mcp.Description("Reason for dismissing or closing the alert"),
				mcp.Enum("false positive", "won't fix", "used in tests"),
			),
			mcp.WithString("dismissed_comment",
				mcp.Description("Dismissal comment associated with the dismissal of the alert"),
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
			state, err := requiredParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dismissed_reason, err := OptionalParam[string](request, "dismissed_reason")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dismissed_comment, err := OptionalParam[string](request, "dismissed_comment")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if state == "dismissed" && dismissed_reason == "" {
				return nil, fmt.Errorf("dismissed_reason required for 'dismissed' state ")
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			alert, resp, err := client.CodeScanning.UpdateAlert(ctx, owner, repo, int64(alertNumber), &github.CodeScanningAlertState{State: state, DismissedReason: &dismissed_reason, DismissedComment: &dismissed_comment})
			if err != nil {
				return nil, fmt.Errorf("failed to update alert: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to update alert: %s", string(body))), nil
			}

			r, err := json.Marshal(alert)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal alert: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
