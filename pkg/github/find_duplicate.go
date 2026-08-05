package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// duplicateIssueResult mirrors a single "Ranked Similar Issue" element returned
// by the semantic-similarity endpoint. Issue is kept as raw JSON so the full
// issue representation is preserved verbatim, and Score is nullable because the
// API may omit a similarity score.
type duplicateIssueResult struct {
	Issue           json.RawMessage `json:"issue"`
	Score           *float64        `json:"score"`
	Confidence      string          `json:"confidence"`
	LikelyDuplicate bool            `json:"likely_duplicate"`
}

// FindDuplicate creates a read-only tool that returns ranked duplicate
// candidates for an existing issue. It is a separate, feature-flagged tool so
// duplicate detection is only advertised when explicitly opted in, keeping the
// default tool surface small. The semantic ranking itself is owned by the API;
// this tool only forwards the request and projects the ranked results.
func FindDuplicate(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "The owner of the repository",
			},
			"repo": {
				Type:        "string",
				Description: "The name of the repository",
			},
			"issue_number": {
				Type:        "number",
				Description: "The number of the existing issue to find duplicates for",
			},
			"confidence_threshold": {
				Type:        "number",
				Description: "Minimum similarity threshold for a candidate to be returned. When omitted, the API's high-precision default is used.",
				Minimum:     jsonschema.Ptr(0.0),
				Maximum:     jsonschema.Ptr(1.0),
			},
		},
		Required: []string{"owner", "repo", "issue_number"},
	}
	WithPagination(schema)

	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "find_duplicate",
			Description: t("TOOL_FIND_DUPLICATE_DESCRIPTION", "Find likely duplicate issues for an existing issue in a GitHub repository. This is a read-only search scoped to the source issue's repository: it returns ranked candidate issues with a similarity score and confidence, and does not close, link, comment on, or otherwise modify any issue."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_FIND_DUPLICATE_USER_TITLE", "Find duplicate issues"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Build the query preserving whether each optional value was supplied
			// so unset parameters fall back to the API's own defaults.
			query := url.Values{}
			if threshold, ok, err := OptionalParamOK[float64](args, "confidence_threshold"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				query.Set("threshold", strconv.FormatFloat(threshold, 'g', -1, 64))
			}
			if _, ok := args["perPage"]; ok {
				perPage, err := OptionalIntParam(args, "perPage")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				query.Set("per_page", strconv.Itoa(perPage))
			}
			if _, ok := args["page"]; ok {
				page, err := OptionalIntParam(args, "page")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				query.Set("page", strconv.Itoa(page))
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			apiURL := fmt.Sprintf("repos/%s/%s/issues/%d/semantically_similar", owner, repo, issueNumber)
			if encoded := query.Encode(); encoded != "" {
				apiURL += "?" + encoded
			}

			req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			var results []duplicateIssueResult
			resp, err := client.Do(req, &results)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to find duplicate issues", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			// When ranked duplicate detection is not enabled for the caller, the
			// endpoint returns bare issue resources instead of ranked results.
			// Surface that as an explicit error rather than incomplete candidates.
			for i := range results {
				if results[i].Confidence == "" || len(results[i].Issue) == 0 {
					return utils.NewToolResultError("ranked duplicate detection is unavailable: the semantic-similarity endpoint returned issues without ranking metadata (the server-side duplicate-ranking feature is not enabled for this caller or repository)"), nil, nil
				}
			}

			r, err := json.Marshal(results)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal duplicate candidates", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
	st.FeatureFlagEnable = FeatureFlagDuplicateDetection
	return st
}
