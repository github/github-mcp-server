package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
)

func GetCodeQualityFinding(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[GetCodeQualityFindingInput, *CodeQualityFindingOutput](
		ToolsetMetadataCodeQuality,
		mcp.Tool{
			Name:        "get_code_quality_finding",
			Description: t("TOOL_GET_CODE_QUALITY_FINDING_DESCRIPTION", "Get details of a specific code quality finding in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_CODE_QUALITY_FINDING_USER_TITLE", "Get code quality finding"),
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
					"findingNumber": {
						Type:        "number",
						Description: "The number of the finding.",
					},
				},
				Required: []string{"owner", "repo", "findingNumber"},
			},
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GetCodeQualityFindingInput) (*mcp.CallToolResult, *CodeQualityFindingOutput, error) {
			if args.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if args.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}
			if args.FindingNumber == 0 {
				return utils.NewToolResultError("missing required parameter: findingNumber"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			apiURL := fmt.Sprintf("repos/%s/%s/code-quality/findings/%d", args.Owner, args.Repo, args.FindingNumber)
			req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			var rawFinding json.RawMessage
			resp, err := client.Do(req, &rawFinding)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get finding", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get finding", resp, body), nil, nil
			}

			var finding CodeQualityFindingOutput
			if err := json.Unmarshal(rawFinding, &finding); err != nil {
				return utils.NewToolResultErrorFromErr("failed to decode finding", err), nil, nil
			}
			r, err := marshalLegacyCodeQualityFinding(rawFinding)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal finding", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), &finding, nil
		},
		normalizeSecurityIntegerArguments("findingNumber"),
	)
}

func ListCodeQualityFindings(t translations.TranslationHelperFunc) inventory.ServerTool {
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
				Description: "Filter code quality findings by state. Defaults to open",
				Enum:        []any{"open", "dismissed"},
				Default:     json.RawMessage(`"open"`),
			},
			"direction": {
				Type:        "string",
				Description: "Sort by creation date: `desc` (newest first, default) or `asc` (oldest first).",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"owner", "repo"},
	}
	WithCursorPagination(schema)

	return NewToolWithSchemaOptions[ListCodeQualityFindingsInput, CodeQualityFindingsOutput](
		ToolsetMetadataCodeQuality,
		mcp.Tool{
			Name:        "list_code_quality_findings",
			Description: t("TOOL_LIST_CODE_QUALITY_FINDINGS_DESCRIPTION", "List code quality findings in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_CODE_QUALITY_FINDINGS_USER_TITLE", "List code quality findings"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.PublicRead(scopes.Repo),
		inventory.TypedSchemaOptions{
			ValidationInputSchema: inventory.CloneSchemaWithoutDefaults(schema),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ListCodeQualityFindingsInput) (*mcp.CallToolResult, CodeQualityFindingsOutput, error) {
			if args.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), CodeQualityFindingsOutput{}, nil
			}
			if args.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), CodeQualityFindingsOutput{}, nil
			}

			pagination := securityCursorPagination(args.PerPage, args.After)
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), CodeQualityFindingsOutput{}, nil
			}

			query := url.Values{}
			if args.State != "" {
				query.Set("state", args.State)
			}
			if args.Direction != "" {
				query.Set("direction", args.Direction)
			}
			query.Set("per_page", strconv.Itoa(pagination.PerPage))
			if pagination.After != "" {
				query.Set("after", pagination.After)
			}
			apiURL := fmt.Sprintf("repos/%s/%s/code-quality/findings?%s", url.PathEscape(args.Owner), url.PathEscape(args.Repo), query.Encode())
			req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), CodeQualityFindingsOutput{}, nil
			}

			var rawFindings json.RawMessage
			resp, err := client.Do(req, &rawFindings)
			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list findings for repository '%s/%s'", args.Owner, args.Repo),
					resp,
					err,
				), CodeQualityFindingsOutput{}, nil
			}

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), CodeQualityFindingsOutput{}, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list findings", resp, body), CodeQualityFindingsOutput{}, nil
			}

			if len(rawFindings) == 0 {
				rawFindings = json.RawMessage("null")
			}
			var findings []*CodeQualityFindingOutput
			if err := json.Unmarshal(rawFindings, &findings); err != nil {
				return utils.NewToolResultErrorFromErr("failed to decode findings", err), CodeQualityFindingsOutput{}, nil
			}

			apiPageInfo := buildPageInfo(resp)
			output := CodeQualityFindingsOutput{
				Findings: findings,
				PageInfo: SecurityPageInfo(apiPageInfo),
			}

			response := struct {
				Findings json.RawMessage `json:"findings"`
				PageInfo pageInfo        `json:"pageInfo"`
			}{
				Findings: rawFindings,
				PageInfo: apiPageInfo,
			}
			r, err := json.Marshal(response)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal findings", err), CodeQualityFindingsOutput{}, nil
			}

			return utils.NewToolResultText(string(r)), output, nil
		},
		normalizeTypedReadArguments(nil, false),
	)
}

func marshalLegacyCodeQualityFinding(rawFinding json.RawMessage) ([]byte, error) {
	// Match main's map-based formatting, including key ordering and number encoding.
	var finding map[string]any
	if err := json.Unmarshal(rawFinding, &finding); err != nil {
		return nil, err
	}
	return json.Marshal(finding)
}
