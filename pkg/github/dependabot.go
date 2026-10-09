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

func GetDependabotAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[GetSecurityAlertInput, *DependabotAlertOutput](
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
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GetSecurityAlertInput) (*mcp.CallToolResult, *DependabotAlertOutput, error) {
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
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}

			alert, resp, err := client.Dependabot.GetRepoAlert(ctx, args.Owner, args.Repo, args.AlertNumber)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					dependabotErrMsg(fmt.Sprintf("failed to get alert with number '%d'", args.AlertNumber), args.Owner, args.Repo, resp),
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

			result := utils.NewToolResultText(string(r))
			// Dependabot alerts are access-restricted regardless of repo
			// visibility and embed attacker-influenceable advisory text, so the
			// label is always private-untrusted.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelSecurityAlert())
			return result, dependabotAlertOutput(alert), nil
		},
		normalizeSecurityIntegerArguments("alertNumber"),
	)
}

func ListDependabotAlerts(t translations.TranslationHelperFunc) inventory.ServerTool {
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
	}
	WithCursorPagination(schema)

	return NewToolWithSchemaOptions[ListDependabotAlertsInput, DependabotAlertsOutput](
		ToolsetMetadataDependabot,
		mcp.Tool{
			Name:        "list_dependabot_alerts",
			Description: t("TOOL_LIST_DEPENDABOT_ALERTS_DESCRIPTION", "List dependabot alerts in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_DEPENDABOT_ALERTS_USER_TITLE", "List dependabot alerts"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.SecurityEvents),
		inventory.TypedSchemaOptions{
			ValidationInputSchema: inventory.CloneSchemaWithoutDefaults(schema),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ListDependabotAlertsInput) (*mcp.CallToolResult, DependabotAlertsOutput, error) {
			if args.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), DependabotAlertsOutput{}, nil
			}
			if args.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), DependabotAlertsOutput{}, nil
			}

			pagination := securityCursorPagination(args.PerPage, args.After)
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), DependabotAlertsOutput{}, err
			}

			alerts, resp, err := client.Dependabot.ListRepoAlerts(ctx, args.Owner, args.Repo, &github.ListAlertsOptions{
				State:    ToStringPtr(args.State),
				Severity: ToStringPtr(args.Severity),
				ListCursorOptions: github.ListCursorOptions{
					PerPage: pagination.PerPage,
					After:   pagination.After,
				},
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					dependabotErrMsg(fmt.Sprintf("failed to list alerts for repository '%s/%s'", args.Owner, args.Repo), args.Owner, args.Repo, resp),
					resp,
					err,
				), DependabotAlertsOutput{}, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), DependabotAlertsOutput{}, err
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list alerts", resp, body), DependabotAlertsOutput{}, nil
			}

			apiPageInfo := buildPageInfo(resp)
			output := DependabotAlertsOutput{
				Alerts:   mapSecurityOutputs(alerts, dependabotAlertOutput),
				PageInfo: SecurityPageInfo(apiPageInfo),
			}

			response := struct {
				Alerts   []*github.DependabotAlert `json:"alerts"`
				PageInfo pageInfo                  `json:"pageInfo"`
			}{
				Alerts:   alerts,
				PageInfo: apiPageInfo,
			}
			r, err := json.Marshal(response)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal alerts", err), DependabotAlertsOutput{}, err
			}

			result := utils.NewToolResultText(string(r))
			// Dependabot alerts are access-restricted regardless of repo
			// visibility and embed attacker-influenceable advisory text, so the
			// label is always private-untrusted.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelSecurityAlert())
			return result, output, nil
		},
		normalizeTypedReadArguments(nil, false),
	)
}

// dependabotErrMsg enhances error messages for dependabot API failures by
// appending a hint about token permissions when the response indicates
// the token may lack access to the repository (403 or 404).
func dependabotErrMsg(base, owner, repo string, resp *github.Response) string {
	if resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound) {
		return fmt.Sprintf("%s. Your token may not have access to Dependabot alerts on %s/%s. "+
			"To access Dependabot alerts, the token needs the 'security_events' scope or, for fine-grained tokens, "+
			"Dependabot alerts read permission for this specific repository.",
			base, owner, repo)
	}
	return base
}
