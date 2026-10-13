package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func GetSecretScanningAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[GetSecurityAlertInput, *SecretScanningAlertOutput](
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
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GetSecurityAlertInput) (*mcp.CallToolResult, *SecretScanningAlertOutput, error) {
			if args.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if args.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}
			if args.AlertNumber == 0 {
				return utils.NewToolResultError("missing required parameter: alertNumber"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			alert, resp, err := client.SecretScanning.GetAlert(ctx, args.Owner, args.Repo, int64(args.AlertNumber))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get alert with number '%d'", args.AlertNumber),
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

			r, err := json.Marshal(alert) //nolint:gosec // G117: This security_events-scoped tool intentionally returns the alert's secret; the result is labeled private-untrusted.
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal alert: %w", err)
			}

			result := utils.NewToolResultText(string(r))
			// Secret scanning alerts are access-restricted regardless of repo
			// visibility and surface the matched secret material itself, so the
			// label is always private-untrusted.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelSecurityAlert())
			return result, secretScanningAlertOutput(alert), nil
		},
		normalizeSecurityIntegerArguments("alertNumber"),
	)
}

func ListSecretScanningAlerts(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
	}
	WithPagination(schema)

	return NewTool[ListSecretScanningAlertsInput, []*SecretScanningAlertOutput](
		ToolsetMetadataSecretProtection,
		mcp.Tool{
			Name:        "list_secret_scanning_alerts",
			Description: t("TOOL_LIST_SECRET_SCANNING_ALERTS_DESCRIPTION", "List secret scanning alerts in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_SECRET_SCANNING_ALERTS_USER_TITLE", "List secret scanning alerts"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ListSecretScanningAlertsInput) (*mcp.CallToolResult, []*SecretScanningAlertOutput, error) {
			if args.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if args.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}

			pagination := securityPagination(args.Page, args.PerPage)
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			alerts, resp, err := client.SecretScanning.ListAlertsForRepo(ctx, args.Owner, args.Repo, &github.SecretScanningAlertListOptions{
				State:      args.State,
				SecretType: args.SecretType,
				Resolution: args.Resolution,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list alerts for repository '%s/%s'", args.Owner, args.Repo),
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

			r, err := json.Marshal(alerts) //nolint:gosec // G117: This security_events-scoped tool intentionally returns alert secrets; the result is labeled private-untrusted.
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal alerts: %w", err)
			}

			result := utils.NewToolResultText(string(r))
			// Secret scanning alerts are access-restricted regardless of repo
			// visibility and surface the matched secret material itself, so the
			// label is always private-untrusted.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelSecurityAlert())
			return result, mapSecurityOutputs(alerts, secretScanningAlertOutput), nil
		},
		normalizeSecurityIntegerArguments("page", "perPage"),
		normalizeTypedReadArguments(nil, false),
	)
}
