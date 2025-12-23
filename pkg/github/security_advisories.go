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

func ListGlobalSecurityAdvisories(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecurityAdvisories,
		mcp.Tool{
			Name:        "list_global_security_advisories",
			Description: t("TOOL_LIST_GLOBAL_SECURITY_ADVISORIES_DESCRIPTION", "List global security advisories from GitHub."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_GLOBAL_SECURITY_ADVISORIES_USER_TITLE", "List global security advisories"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"ghsaId": {
						Type:        "string",
						Description: "Filter by GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx).",
					},
					"type": {
						Type:        "string",
						Description: "Advisory type.",
						Enum:        []any{"reviewed", "malware", "unreviewed"},
						Default:     json.RawMessage(`"reviewed"`),
					},
					"cveId": {
						Type:        "string",
						Description: "Filter by CVE ID.",
					},
					"ecosystem": {
						Type:        "string",
						Description: "Filter by package ecosystem.",
						Enum:        []any{"actions", "composer", "erlang", "go", "maven", "npm", "nuget", "other", "pip", "pub", "rubygems", "rust"},
					},
					"severity": {
						Type:        "string",
						Description: "Filter by severity.",
						Enum:        []any{"unknown", "low", "medium", "high", "critical"},
					},
					"cwes": {
						Type:        "array",
						Description: "Filter by Common Weakness Enumeration IDs (e.g. [\"79\", \"284\", \"22\"]).",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"isWithdrawn": {
						Type:        "boolean",
						Description: "Whether to only return withdrawn advisories.",
					},
					"affects": {
						Type:        "string",
						Description: "Filter advisories by affected package or version (e.g. \"package1,package2@1.0.0\").",
					},
					"published": {
						Type:        "string",
						Description: "Filter by publish date or date range (ISO 8601 date or range).",
					},
					"updated": {
						Type:        "string",
						Description: "Filter by update date or date range (ISO 8601 date or range).",
					},
					"modified": {
						Type:        "string",
						Description: "Filter by publish or update date or date range (ISO 8601 date or range).",
					},
				},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			ghsaID, err := OptionalParam[string](args, "ghsaId")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid ghsaId: %v", err)), nil, nil
			}

			typ, err := OptionalParam[string](args, "type")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid type: %v", err)), nil, nil
			}

			cveID, err := OptionalParam[string](args, "cveId")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid cveId: %v", err)), nil, nil
			}

			eco, err := OptionalParam[string](args, "ecosystem")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid ecosystem: %v", err)), nil, nil
			}

			sev, err := OptionalParam[string](args, "severity")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid severity: %v", err)), nil, nil
			}

			cwes, err := OptionalStringArrayParam(args, "cwes")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid cwes: %v", err)), nil, nil
			}

			isWithdrawn, err := OptionalParam[bool](args, "isWithdrawn")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid isWithdrawn: %v", err)), nil, nil
			}

			affects, err := OptionalParam[string](args, "affects")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid affects: %v", err)), nil, nil
			}

			published, err := OptionalParam[string](args, "published")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid published: %v", err)), nil, nil
			}

			updated, err := OptionalParam[string](args, "updated")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid updated: %v", err)), nil, nil
			}

			modified, err := OptionalParam[string](args, "modified")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid modified: %v", err)), nil, nil
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
				return nil, nil, fmt.Errorf("failed to list global security advisories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list advisories", resp, body), nil, nil
			}

			r, err := json.Marshal(advisories)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal advisories: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListRepositorySecurityAdvisories(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecurityAdvisories,
		mcp.Tool{
			Name:        "list_repository_security_advisories",
			Description: t("TOOL_LIST_REPOSITORY_SECURITY_ADVISORIES_DESCRIPTION", "List repository security advisories for a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_REPOSITORY_SECURITY_ADVISORIES_USER_TITLE", "List repository security advisories"),
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
					"direction": {
						Type:        "string",
						Description: "Sort direction.",
						Enum:        []any{"asc", "desc"},
					},
					"sort": {
						Type:        "string",
						Description: "Sort field.",
						Enum:        []any{"created", "updated", "published"},
					},
					"state": {
						Type:        "string",
						Description: "Filter by advisory state.",
						Enum:        []any{"triage", "draft", "published", "closed"},
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

			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sortField, err := OptionalParam[string](args, "sort")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return nil, nil, fmt.Errorf("failed to list repository security advisories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list repository advisories", resp, body), nil, nil
			}

			r, err := json.Marshal(advisories)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal advisories: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func GetGlobalSecurityAdvisory(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecurityAdvisories,
		mcp.Tool{
			Name:        "get_global_security_advisory",
			Description: t("TOOL_GET_GLOBAL_SECURITY_ADVISORY_DESCRIPTION", "Get a global security advisory"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_GLOBAL_SECURITY_ADVISORY_USER_TITLE", "Get a global security advisory"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"ghsaId": {
						Type:        "string",
						Description: "GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx).",
					},
				},
				Required: []string{"ghsaId"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			ghsaID, err := RequiredParam[string](args, "ghsaId")
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid ghsaId: %v", err)), nil, nil
			}

			advisory, resp, err := client.SecurityAdvisories.GetGlobalSecurityAdvisories(ctx, ghsaID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get advisory: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get advisory", resp, body), nil, nil
			}

			r, err := json.Marshal(advisory)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal advisory: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListOrgRepositorySecurityAdvisories(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecurityAdvisories,
		mcp.Tool{
			Name:        "list_org_repository_security_advisories",
			Description: t("TOOL_LIST_ORG_REPOSITORY_SECURITY_ADVISORIES_DESCRIPTION", "List repository security advisories for a GitHub organization."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ORG_REPOSITORY_SECURITY_ADVISORIES_USER_TITLE", "List org repository security advisories"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: "The organization login.",
					},
					"direction": {
						Type:        "string",
						Description: "Sort direction.",
						Enum:        []any{"asc", "desc"},
					},
					"sort": {
						Type:        "string",
						Description: "Sort field.",
						Enum:        []any{"created", "updated", "published"},
					},
					"state": {
						Type:        "string",
						Description: "Filter by advisory state.",
						Enum:        []any{"triage", "draft", "published", "closed"},
					},
				},
				Required: []string{"org"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sortField, err := OptionalParam[string](args, "sort")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return nil, nil, fmt.Errorf("failed to list organization repository security advisories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list organization repository advisories", resp, body), nil, nil
			}

			r, err := json.Marshal(advisories)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal advisories: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
