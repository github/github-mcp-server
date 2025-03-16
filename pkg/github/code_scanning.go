package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func getCodeScanningAlert(client *github.Client) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_code_scanning_alert",
			mcp.WithDescription("Get details of a specific code scanning alert in a GitHub repository."),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The owner of the repository."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("The name of the repository."),
			),
			mcp.WithNumber("alert_number",
				mcp.Required(),
				mcp.Description("The number of the alert."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, _ := request.Params.Arguments["owner"].(string)
			repo, _ := request.Params.Arguments["repo"].(string)
			alertNumber, _ := request.Params.Arguments["alert_number"].(float64)

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

func listCodeScanningAlerts(client *github.Client) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_code_scanning_alerts",
			mcp.WithDescription("List code scanning alerts in a GitHub repository."),
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
				mcp.Description("State of the code scanning alerts to list. Set to closed to list only closed code scanning alerts. Default: open"),
				mcp.DefaultString("open"),
			),
			mcp.WithString("severity",
				mcp.Description("Only code scanning alerts with this severity will be returned. Possible values are: critical, high, medium, low, warning, note, error."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, _ := request.Params.Arguments["owner"].(string)
			repo, _ := request.Params.Arguments["repo"].(string)
			ref, _ := request.Params.Arguments["ref"].(string)
			state, _ := request.Params.Arguments["state"].(string)
			severity, _ := request.Params.Arguments["severity"].(string)

			alerts, resp, err := client.CodeScanning.ListAlertsForRepo(ctx, owner, repo, &github.AlertListOptions{Ref: ref, State: state, Severity: severity})
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
