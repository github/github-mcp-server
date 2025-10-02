package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v73/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func ListGlobalSecurityAdvisories(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_global_security_advisories",
			mcp.WithDescription(t("TOOL_LIST_GLOBAL_SECURITY_ADVISORIES_DESCRIPTION", "List global security advisories from the GitHub Advisory Database.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_GLOBAL_SECURITY_ADVISORIES_USER_TITLE", "List global security advisories"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("ghsa_id",
				mcp.Description("If specified, only advisories with this GHSA (GitHub Security Advisory) identifier will be returned."),
			),
			mcp.WithString("type",
				mcp.Description("If specified, only advisories of this type will be returned. By default, a request with no other parameters defined will only return reviewed advisories that are not malware."),
				mcp.Enum("reviewed", "malware", "unreviewed"),
			),
			mcp.WithString("cve_id",
				mcp.Description("If specified, only advisories with this CVE (Common Vulnerabilities and Exposures) identifier will be returned."),
			),
			mcp.WithString("ecosystem",
				mcp.Description("If specified, only advisories for this ecosystem will be returned."),
				mcp.Enum("rubygems", "npm", "pip", "maven", "nuget", "composer", "go", "rust", "erlang", "actions", "pub", "other", "swift"),
			),
			mcp.WithString("severity",
				mcp.Description("If specified, only advisories with this severity will be returned."),
				mcp.Enum("unknown", "low", "medium", "high", "critical"),
			),
			mcp.WithString("cwes",
				mcp.Description("If specified, only advisories with these CWEs will be returned. Multiple CWEs can be separated by commas. Example: cwes=79,284,22"),
			),
			mcp.WithString("is_withdrawn",
				mcp.Description("Whether to only return advisories that have been withdrawn."),
				mcp.Enum("true", "false"),
			),
			mcp.WithString("affects",
				mcp.Description("If specified, only return advisories that affect any of package or package@version. A maximum of 1000 packages can be specified. Example: affects=package1,package2@1.0.0,package3@^2.0.0"),
			),
			mcp.WithString("published",
				mcp.Description("If specified, only return advisories that were published on a date or date range. Format: YYYY-MM-DD or YYYY-MM-DD..YYYY-MM-DD for range."),
			),
			mcp.WithString("updated",
				mcp.Description("If specified, only return advisories that were updated on a date or date range. Format: YYYY-MM-DD or YYYY-MM-DD..YYYY-MM-DD for range."),
			),
			mcp.WithString("modified",
				mcp.Description("If specified, only show advisories that were updated or published on a date or date range. Format: YYYY-MM-DD or YYYY-MM-DD..YYYY-MM-DD for range."),
			),
			mcp.WithString("epss_percentage",
				mcp.Description("If specified, only return advisories that have an EPSS percentage score that matches the provided value. The EPSS percentage represents the likelihood of a CVE being exploited."),
			),
			mcp.WithString("epss_percentile",
				mcp.Description("If specified, only return advisories that have an EPSS percentile score that matches the provided value. The EPSS percentile represents the relative rank of the CVE's likelihood of being exploited compared to other CVEs."),
			),
			mcp.WithString("before",
				mcp.Description("A cursor, as given in the Link header. If specified, the query only searches for results before this cursor."),
			),
			mcp.WithString("after",
				mcp.Description("A cursor, as given in the Link header. If specified, the query only searches for results after this cursor."),
			),
			mcp.WithString("direction",
				mcp.Description("The direction to sort the results by."),
				mcp.Enum("asc", "desc"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("The number of results per page (max 100). Default: 30"),
			),
			mcp.WithString("sort",
				mcp.Description("The property to sort the results by."),
				mcp.Enum("updated", "published", "epss_percentage", "epss_percentile"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Parse optional parameters
			opts := &github.ListGlobalSecurityAdvisoriesOptions{}

			if ghsaID, err := OptionalParam[string](request, "ghsa_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if ghsaID != "" {
				opts.GHSAID = &ghsaID
			}

			if advisoryType, err := OptionalParam[string](request, "type"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if advisoryType != "" {
				opts.Type = &advisoryType
			}

			if cveID, err := OptionalParam[string](request, "cve_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if cveID != "" {
				opts.CVEID = &cveID
			}

			if ecosystem, err := OptionalParam[string](request, "ecosystem"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if ecosystem != "" {
				opts.Ecosystem = &ecosystem
			}

			if severity, err := OptionalParam[string](request, "severity"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if severity != "" {
				opts.Severity = &severity
			}

			if cwes, err := OptionalParam[string](request, "cwes"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if cwes != "" {
				// Split comma-separated CWEs
				opts.CWEs = strings.Split(cwes, ",")
			}

			if isWithdrawn, err := OptionalParam[string](request, "is_withdrawn"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if isWithdrawn != "" {
				withdrawn := isWithdrawn == "true"
				opts.IsWithdrawn = &withdrawn
			}

			if affects, err := OptionalParam[string](request, "affects"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if affects != "" {
				opts.Affects = &affects
			}

			if published, err := OptionalParam[string](request, "published"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if published != "" {
				opts.Published = &published
			}

			if updated, err := OptionalParam[string](request, "updated"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if updated != "" {
				opts.Updated = &updated
			}

			if modified, err := OptionalParam[string](request, "modified"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if modified != "" {
				opts.Modified = &modified
			}

			// Note: EPSS parameters may not be supported in current Go SDK version
			// Check if these fields exist before using them
			// For now, we accept the parameters but don't use them in the API call
			if _, err := OptionalParam[string](request, "epss_percentage"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if _, err := OptionalParam[string](request, "epss_percentile"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if before, err := OptionalParam[string](request, "before"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if before != "" {
				opts.Before = before
			}

			if after, err := OptionalParam[string](request, "after"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if after != "" {
				opts.After = after
			}

			// Note: Direction and Sort parameters may not be supported in current Go SDK version
			// For now, we accept the parameters but don't use them in the API call
			if _, err := OptionalParam[string](request, "direction"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if _, err := OptionalParam[string](request, "sort"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if perPage, err := OptionalIntParam(request, "per_page"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if perPage != 0 {
				opts.PerPage = perPage
			}

			advisories, resp, err := client.SecurityAdvisories.ListGlobalSecurityAdvisories(ctx, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list global security advisories",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to list global security advisories: %s", string(body))), nil
			}

			r, err := json.Marshal(advisories)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal advisories: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func GetGlobalSecurityAdvisory(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_global_security_advisory",
			mcp.WithDescription(t("TOOL_GET_GLOBAL_SECURITY_ADVISORY_DESCRIPTION", "Get a global security advisory using its GitHub Security Advisory (GHSA) identifier.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_GLOBAL_SECURITY_ADVISORY_USER_TITLE", "Get global security advisory"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("ghsa_id",
				mcp.Required(),
				mcp.Description("The GHSA (GitHub Security Advisory) identifier of the advisory."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ghsaID, err := RequiredParam[string](request, "ghsa_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			advisory, resp, err := client.SecurityAdvisories.GetGlobalSecurityAdvisories(ctx, ghsaID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get global security advisory '%s'", ghsaID),
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get global security advisory: %s", string(body))), nil
			}

			r, err := json.Marshal(advisory)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal advisory: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
