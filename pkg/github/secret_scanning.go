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

func GetSecretScanningAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecretProtection,
		mcp.Tool{
			Name:        "get_secret_scanning_alert",
			Description: t("TOOL_GET_SECRET_SCANNING_ALERT_DESCRIPTION", "Get details of a specific secret scanning alert in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_SECRET_SCANNING_ALERT_USER_TITLE", "Get secret scanning alert"),
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
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			alert, resp, err := client.SecretScanning.GetAlert(ctx, owner, repo, int64(alertNumber))
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
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get alert", resp, body), nil, nil
			}

			r, err := json.Marshal(alert)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal alert: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListSecretScanningAlerts(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecretProtection,
		mcp.Tool{
			Name:        "list_secret_scanning_alerts",
			Description: t("TOOL_LIST_SECRET_SCANNING_ALERTS_DESCRIPTION", "List secret scanning alerts in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_SECRET_SCANNING_ALERTS_USER_TITLE", "List secret scanning alerts"),
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
						Description: "Filter by state",
						Enum:        []any{"open", "resolved"},
					},
					"secret_type": {
						Type:        "string",
						Description: "A comma-separated list of secret types to return. All default secret patterns are returned. To return generic patterns, pass the token name(s) in the parameter.",
					},
					"resolution": {
						Type:        "string",
						Description: "Filter by resolution",
						Enum:        []any{"false_positive", "wont_fix", "revoked", "pattern_edited", "pattern_deleted", "used_in_tests"},
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
			secretType, err := OptionalParam[string](args, "secret_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			resolution, err := OptionalParam[string](args, "resolution")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			alerts, resp, err := client.SecretScanning.ListAlertsForRepo(ctx, owner, repo, &github.SecretScanningAlertListOptions{State: state, SecretType: secretType, Resolution: resolution})
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
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list alerts", resp, body), nil, nil
			}

			r, err := json.Marshal(alerts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal alerts: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
