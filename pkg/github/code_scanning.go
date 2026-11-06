package github

import (
	"context"
	"encoding/json"
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

func GetCodeScanningAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[GetSecurityAlertInput, *CodeScanningAlertOutput](
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
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GetSecurityAlertInput) (*mcp.CallToolResult, *CodeScanningAlertOutput, error) {
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
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			alert, resp, err := client.CodeScanning.GetAlert(ctx, args.Owner, args.Repo, int64(args.AlertNumber))
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

			result := utils.NewToolResultText(string(r))
			// Code scanning alerts are access-restricted regardless of repo
			// visibility and embed attacker-influenceable code snippets, so the
			// label is always private-untrusted.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelSecurityAlert())
			return result, codeScanningAlertOutput(alert), nil
		},
		normalizeSecurityIntegerArguments("alertNumber"),
	)
}

func ListCodeScanningAlerts(t translations.TranslationHelperFunc) inventory.ServerTool {
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
	}
	WithPagination(schema)

	return NewToolWithSchemaOptions[ListCodeScanningAlertsInput, []*CodeScanningAlertOutput](
		ToolsetMetadataCodeSecurity,
		mcp.Tool{
			Name:        "list_code_scanning_alerts",
			Description: t("TOOL_LIST_CODE_SCANNING_ALERTS_DESCRIPTION", "List code scanning alerts in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_CODE_SCANNING_ALERTS_USER_TITLE", "List code scanning alerts"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.SecurityEvents),
		inventory.TypedSchemaOptions{
			ValidationInputSchema: inventory.CloneSchemaWithoutDefaults(schema),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ListCodeScanningAlertsInput) (*mcp.CallToolResult, []*CodeScanningAlertOutput, error) {
			if args.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if args.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}

			pagination := securityPagination(args.Page, args.PerPage)
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			alerts, resp, err := client.CodeScanning.ListAlertsForRepo(ctx, args.Owner, args.Repo, &github.AlertListOptions{
				Ref:      args.Ref,
				State:    args.State,
				Severity: args.Severity,
				ToolName: args.ToolName,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			})
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

			result := utils.NewToolResultText(string(r))
			// Code scanning alerts are access-restricted regardless of repo
			// visibility and embed attacker-influenceable code snippets, so the
			// label is always private-untrusted.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelSecurityAlert())
			return result, mapSecurityOutputs(alerts, codeScanningAlertOutput), nil
		},
		normalizeSecurityIntegerArguments("page", "perPage"),
		normalizeTypedReadArguments(nil, false),
	)
}
