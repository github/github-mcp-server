package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

func marshalLegacyCodeQualityFinding(rawFinding json.RawMessage) ([]byte, error) {
	// Match main's map-based formatting, including key ordering and number encoding.
	var finding map[string]any
	if err := json.Unmarshal(rawFinding, &finding); err != nil {
		return nil, err
	}
	return json.Marshal(finding)
}
