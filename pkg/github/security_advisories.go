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

func ListGlobalSecurityAdvisories(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := mcp.Tool{
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
	}
	return NewToolWithSchemaOptions[ListGlobalSecurityAdvisoriesInput, []*GlobalSecurityAdvisoryOutput](
		ToolsetMetadataSecurityAdvisories,
		tool,
		scopes.RequireAll(scopes.SecurityEvents),
		inventory.TypedSchemaOptions{
			ValidationInputSchema: inventory.CloneSchemaWithoutDefaults(tool.InputSchema.(*jsonschema.Schema)),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ListGlobalSecurityAdvisoriesInput) (*mcp.CallToolResult, []*GlobalSecurityAdvisoryOutput, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.ListGlobalSecurityAdvisoriesOptions{}

			if args.GHSAID != "" {
				opts.GHSAID = &args.GHSAID
			}
			if args.Type != "" {
				opts.Type = &args.Type
			}
			if args.CVEID != "" {
				opts.CVEID = &args.CVEID
			}
			if args.Ecosystem != "" {
				opts.Ecosystem = &args.Ecosystem
			}
			if args.Severity != "" {
				opts.Severity = &args.Severity
			}
			if len(args.CWEs) > 0 {
				opts.CWEs = args.CWEs
			}

			if args.IsWithdrawn {
				opts.IsWithdrawn = &args.IsWithdrawn
			}

			if args.Affects != "" {
				opts.Affects = &args.Affects
			}
			if args.Published != "" {
				opts.Published = &args.Published
			}
			if args.Updated != "" {
				opts.Updated = &args.Updated
			}
			if args.Modified != "" {
				opts.Modified = &args.Modified
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

			result := utils.NewToolResultText(string(r))
			// Global advisories come from the world-readable GitHub Advisory
			// Database (public) but contain externally authored prose
			// (untrusted).
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelGlobalSecurityAdvisory())
			return result, mapSecurityOutputs(advisories, globalSecurityAdvisoryOutput), nil
		},
		normalizeGlobalAdvisoryArguments,
	)
}

func ListRepositorySecurityAdvisories(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[ListRepositorySecurityAdvisoriesInput, []*SecurityAdvisoryOutput](
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
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ListRepositorySecurityAdvisoriesInput) (*mcp.CallToolResult, []*SecurityAdvisoryOutput, error) {
			if args.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if args.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.ListRepositorySecurityAdvisoriesOptions{}
			if args.Direction != "" {
				opts.Direction = args.Direction
			}
			if args.Sort != "" {
				opts.Sort = args.Sort
			}
			if args.State != "" {
				opts.State = args.State
			}

			advisories, resp, err := client.SecurityAdvisories.ListRepositorySecurityAdvisories(ctx, args.Owner, args.Repo, opts)
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

			result := utils.NewToolResultText(string(r))
			// Repository advisories carry externally authored prose (untrusted).
			// Confidentiality follows repo visibility, but draft/triage/closed
			// advisories are not world-readable even on a public repo, so the
			// result is only public when every returned advisory is published.
			allPublished := allAdvisoriesPublished(advisories)
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, args.Owner, args.Repo, result,
				func(isPrivate bool) ifc.SecurityLabel {
					return ifc.LabelRepositorySecurityAdvisory(isPrivate, allPublished)
				})
			return result, mapSecurityOutputs(advisories, securityAdvisoryOutput), nil
		},
	)
}

func GetGlobalSecurityAdvisory(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[GetGlobalSecurityAdvisoryInput, *GlobalSecurityAdvisoryOutput](
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
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GetGlobalSecurityAdvisoryInput) (*mcp.CallToolResult, *GlobalSecurityAdvisoryOutput, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			if args.GHSAID == "" {
				return utils.NewToolResultError("invalid ghsaId: missing required parameter: ghsaId"), nil, nil
			}

			advisory, resp, err := client.SecurityAdvisories.GetGlobalSecurityAdvisories(ctx, args.GHSAID)
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

			result := utils.NewToolResultText(string(r))
			// A global advisory is world-readable (public) but externally
			// authored (untrusted).
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelGlobalSecurityAdvisory())
			return result, globalSecurityAdvisoryOutput(advisory), nil
		},
	)
}

func ListOrgRepositorySecurityAdvisories(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[ListOrgRepositorySecurityAdvisoriesInput, []*SecurityAdvisoryOutput](
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
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ListOrgRepositorySecurityAdvisoriesInput) (*mcp.CallToolResult, []*SecurityAdvisoryOutput, error) {
			if args.Org == "" {
				return utils.NewToolResultError("missing required parameter: org"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.ListRepositorySecurityAdvisoriesOptions{}
			if args.Direction != "" {
				opts.Direction = args.Direction
			}
			if args.Sort != "" {
				opts.Sort = args.Sort
			}
			if args.State != "" {
				opts.State = args.State
			}

			advisories, resp, err := client.SecurityAdvisories.ListRepositorySecurityAdvisoriesForOrg(ctx, args.Org, opts)
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

			result := utils.NewToolResultText(string(r))
			// Org-wide advisory listings span the organization's repositories
			// (including private ones) and are restricted to org members, so
			// they are conservatively labeled private-untrusted (isPrivate=true,
			// which forces private regardless of publication state).
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelRepositorySecurityAdvisory(true, false))
			return result, mapSecurityOutputs(advisories, securityAdvisoryOutput), nil
		},
	)
}

// allAdvisoriesPublished reports whether every advisory in the slice is in the
// "published" state. Repository security advisories can also be in draft,
// triage, or closed states, none of which are world-readable even on a public
// repository. An empty slice is treated as published (true) since there is no
// non-public content to protect. Used to decide whether a repository advisory
// listing may carry a public confidentiality label.
func allAdvisoriesPublished(advisories []*github.SecurityAdvisory) bool {
	for _, advisory := range advisories {
		if advisory.GetState() != "published" {
			return false
		}
	}
	return true
}
