package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// rankedSimilarIssue is a single "Ranked Similar Issue" element returned by the
// semantic-similarity endpoint. Only the issue fields the tool surfaces are
// decoded, and Score is nullable because the API may omit a similarity score.
type rankedSimilarIssue struct {
	Issue *struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
	} `json:"issue"`
	Score           *float64 `json:"score"`
	Confidence      string   `json:"confidence"`
	LikelyDuplicate bool     `json:"likely_duplicate"`
}

// DuplicateCandidate is the trimmed output for a ranked duplicate candidate,
// carrying only what an agent needs to explain and act on it.
type DuplicateCandidate struct {
	Issue           MinimalIssueRef `json:"issue"`
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
				Description: "Minimum similarity threshold a candidate must meet to be returned; higher values are stricter. When omitted, the API's high-precision default is used. The scale is defined by the API, so no client-side bounds are enforced.",
			},
		},
		Required: []string{"owner", "repo", "issue_number"},
	}
	WithPagination(schema)
	// This endpoint owns its defaults. Unlike issue_dependency_read, explicit
	// zero pagination values must be forwarded, not replaced by SDK defaults.
	for _, field := range []string{"page", "perPage"} {
		schema.Properties[field].Default = nil
		schema.Properties[field].Minimum = new(0.0)
	}

	st := NewTool[FindDuplicateInput, []DuplicateCandidate](
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
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input FindDuplicateInput) (*mcp.CallToolResult, []DuplicateCandidate, error) {
			owner, repo, issueNumber := input.Owner, input.Repo, input.IssueNumber
			if err := validateIssueCoordinate(owner, repo, issueNumber); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Build the query preserving whether each optional value was supplied
			// so unset parameters fall back to the API's own defaults.
			query := url.Values{}
			if input.ConfidenceThreshold != nil {
				query.Set("threshold", strconv.FormatFloat(*input.ConfidenceThreshold, 'g', -1, 64))
			}
			if input.PerPage != nil {
				query.Set("per_page", strconv.Itoa(*input.PerPage))
			}
			if input.Page != nil {
				query.Set("page", strconv.Itoa(*input.Page))
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

			var results []rankedSimilarIssue
			resp, err := client.Do(req, &results)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to find duplicate issues", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			candidates := make([]DuplicateCandidate, 0, len(results))
			for _, res := range results {
				// A bare issue (no ranking metadata) means ranked duplicate
				// detection is not enabled for this caller; fail clearly rather
				// than returning incomplete candidates.
				if res.Confidence == "" || res.Issue == nil {
					return utils.NewToolResultError("ranked duplicate detection is unavailable: the semantic-similarity endpoint returned issues without ranking metadata (the server-side duplicate-ranking feature is not enabled for this caller or repository)"), nil, nil
				}
				candidates = append(candidates, DuplicateCandidate{
					// Candidates are always scoped to the requested repository, so the
					// ref's repository field is left empty as it was before.
					Issue: newMinimalIssueRef(
						res.Issue.Number,
						res.Issue.Title,
						res.Issue.State,
						res.Issue.HTMLURL,
						"",
					),
					Score:           res.Score,
					Confidence:      res.Confidence,
					LikelyDuplicate: res.LikelyDuplicate,
				})
			}

			r, err := json.Marshal(candidates)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal duplicate candidates", err), nil, nil
			}

			// Candidate issue titles are user-authored content scoped to the source
			// repository, so classify the result like issue_read.
			result := utils.NewToolResultText(string(r))
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelRepoUserContent)
			return result, candidates, nil
		}, normalizeIssueStrings([]string{"owner", "repo"}, nil),
		normalizeIssueIntegers([]string{"issue_number"}, []string{"page", "perPage"}), normalizeDuplicateThreshold)
	st.FeatureRule = featureEnabledRule(FeatureFlagDuplicateDetection)
	return st
}
