package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func ListGlobalSecurityAdvisories(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_global_security_advisories",
			mcp.WithDescription(t("TOOL_LIST_GLOBAL_SECURITY_ADVISORIES_DESCRIPTION", "List global security advisories from GitHub.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_GLOBAL_SECURITY_ADVISORIES_USER_TITLE", "List global security advisories"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("ghsaId",
				mcp.Description("Filter by GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx)."),
			),
			mcp.WithString("type",
				mcp.Description("Advisory type."),
				mcp.Enum("reviewed", "malware", "unreviewed"),
				mcp.DefaultString("reviewed"),
			),
			mcp.WithString("cveId",
				mcp.Description("Filter by CVE ID."),
			),
			mcp.WithString("ecosystem",
				mcp.Description("Filter by package ecosystem."),
				mcp.Enum("actions", "composer", "erlang", "go", "maven", "npm", "nuget", "other", "pip", "pub", "rubygems", "rust"),
			),
			mcp.WithString("severity",
				mcp.Description("Filter by severity."),
				mcp.Enum("unknown", "low", "medium", "high", "critical"),
			),
			mcp.WithArray("cwes",
				mcp.Description("Filter by Common Weakness Enumeration IDs (e.g. [\"79\", \"284\", \"22\"])."),
				mcp.Items(map[string]any{
					"type": "string",
				}),
			),
			mcp.WithBoolean("isWithdrawn",
				mcp.Description("Whether to only return withdrawn advisories."),
			),
			mcp.WithString("affects",
				mcp.Description("Filter advisories by affected package or version (e.g. \"package1,package2@1.0.0\")."),
			),
			mcp.WithString("published",
				mcp.Description("Filter by publish date or date range (ISO 8601 date or range)."),
			),
			mcp.WithString("updated",
				mcp.Description("Filter by update date or date range (ISO 8601 date or range)."),
			),
			mcp.WithString("modified",
				mcp.Description("Filter by publish or update date or date range (ISO 8601 date or range)."),
			),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			ghsaID, err := OptionalParam[string](request, "ghsaId")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid ghsaId: %v", err)), nil
			}

			typ, err := OptionalParam[string](request, "type")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid type: %v", err)), nil
			}

			cveID, err := OptionalParam[string](request, "cveId")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid cveId: %v", err)), nil
			}

			eco, err := OptionalParam[string](request, "ecosystem")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid ecosystem: %v", err)), nil
			}

			sev, err := OptionalParam[string](request, "severity")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid severity: %v", err)), nil
			}

			cwes, err := OptionalParam[[]string](request, "cwes")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid cwes: %v", err)), nil
			}

			isWithdrawn, err := OptionalParam[bool](request, "isWithdrawn")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid isWithdrawn: %v", err)), nil
			}

			affects, err := OptionalParam[string](request, "affects")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid affects: %v", err)), nil
			}

			published, err := OptionalParam[string](request, "published")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid published: %v", err)), nil
			}

			updated, err := OptionalParam[string](request, "updated")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid updated: %v", err)), nil
			}

			modified, err := OptionalParam[string](request, "modified")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid modified: %v", err)), nil
			}

			opts := &github.ListGlobalSecurityAdvisoriesOptions{}

			if ghsaID != "" {
				opts.GHSAID = &ghsaID
			}
			if typ != "" {
				opts.Type = &typ
			}
			if cveID != "" {
				opts.CVEID = &cveID
			}
			if eco != "" {
				opts.Ecosystem = &eco
			}
			if sev != "" {
				opts.Severity = &sev
			}
			if len(cwes) > 0 {
				opts.CWEs = cwes
			}

			if isWithdrawn {
				opts.IsWithdrawn = &isWithdrawn
			}

			if affects != "" {
				opts.Affects = &affects
			}
			if published != "" {
				opts.Published = &published
			}
			if updated != "" {
				opts.Updated = &updated
			}
			if modified != "" {
				opts.Modified = &modified
			}

			advisories, resp, err := client.SecurityAdvisories.ListGlobalSecurityAdvisories(ctx, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list global security advisories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list advisories: %s", string(body))), nil
			}

			r, err := json.Marshal(advisories)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal advisories: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func ListRepositorySecurityAdvisories(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_repository_security_advisories",
			mcp.WithDescription(t("TOOL_LIST_REPOSITORY_SECURITY_ADVISORIES_DESCRIPTION", "List repository security advisories for a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_REPOSITORY_SECURITY_ADVISORIES_USER_TITLE", "List repository security advisories"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The owner of the repository."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("The name of the repository."),
			),
			mcp.WithString("direction",
				mcp.Description("Sort direction."),
				mcp.Enum("asc", "desc"),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field."),
				mcp.Enum("created", "updated", "published"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by advisory state."),
				mcp.Enum("triage", "draft", "published", "closed"),
			),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sortField, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.ListRepositorySecurityAdvisoriesOptions{}
			if direction != "" {
				opts.Direction = direction
			}
			if sortField != "" {
				opts.Sort = sortField
			}
			if state != "" {
				opts.State = state
			}

			advisories, resp, err := client.SecurityAdvisories.ListRepositorySecurityAdvisories(ctx, owner, repo, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list repository security advisories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list repository advisories: %s", string(body))), nil
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
			mcp.WithDescription(t("TOOL_GET_GLOBAL_SECURITY_ADVISORY_DESCRIPTION", "Get a global security advisory")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_GLOBAL_SECURITY_ADVISORY_USER_TITLE", "Get a global security advisory"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("ghsaId",
				mcp.Description("GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx)."),
				mcp.Required(),
			),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			ghsaID, err := RequiredParam[string](request, "ghsaId")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid ghsaId: %v", err)), nil
			}

			advisory, resp, err := client.SecurityAdvisories.GetGlobalSecurityAdvisories(ctx, ghsaID)
			if err != nil {
				return nil, fmt.Errorf("failed to get advisory: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get advisory: %s", string(body))), nil
			}

			r, err := json.Marshal(advisory)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal advisory: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func ListOrgRepositorySecurityAdvisories(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_org_repository_security_advisories",
			mcp.WithDescription(t("TOOL_LIST_ORG_REPOSITORY_SECURITY_ADVISORIES_DESCRIPTION", "List repository security advisories for a GitHub organization.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ORG_REPOSITORY_SECURITY_ADVISORIES_USER_TITLE", "List org repository security advisories"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("The organization login."),
			),
			mcp.WithString("direction",
				mcp.Description("Sort direction."),
				mcp.Enum("asc", "desc"),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field."),
				mcp.Enum("created", "updated", "published"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by advisory state."),
				mcp.Enum("triage", "draft", "published", "closed"),
			),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sortField, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.ListRepositorySecurityAdvisoriesOptions{}
			if direction != "" {
				opts.Direction = direction
			}
			if sortField != "" {
				opts.Sort = sortField
			}
			if state != "" {
				opts.State = state
			}

			advisories, resp, err := client.SecurityAdvisories.ListRepositorySecurityAdvisoriesForOrg(ctx, org, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list organization repository security advisories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list organization repository advisories: %s", string(body))), nil
			}

			r, err := json.Marshal(advisories)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal advisories: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
