package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// rulesetLevelDescription documents the "level" parameter shared by the
// ruleset read and write tools.
const rulesetLevelDescription = "The level at which the ruleset is configured:\n" +
	"- 'repository': A ruleset on a single repository (requires 'owner' and 'repo').\n" +
	"- 'organization': A ruleset covering repositories in an organization (requires 'org').\n" +
	"- 'enterprise': A ruleset covering repositories across an enterprise (requires 'enterprise')."

// rulesetReadScopeAccess declares the exhaustive scope challenge policy for
// repository_ruleset_read. The exact scope challenged depends on the "level"
// argument: repository reads need "repo", organization reads need "read:org",
// and enterprise reads need "read:enterprise". A missing or unrecognized
// level returns no challenge so normal handler validation produces the error.
func rulesetReadScopeAccess() inventory.ScopeAccess {
	return scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.ReadOrg, scopes.ReadEnterprise},
		func([]string) bool {
			// Repository-level reads may target public repositories, so the
			// tool stays visible even for tokens without any of these scopes.
			return true
		},
		func(arguments map[string]any, activeScopes []string) []string {
			level, ok := arguments["level"].(string)
			if !ok {
				return nil
			}
			switch level {
			case "repository":
				return scopes.ChallengeAll(activeScopes, scopes.Repo)
			case "organization":
				return scopes.ChallengeAll(activeScopes, scopes.ReadOrg)
			case "enterprise":
				return scopes.ChallengeAll(activeScopes, scopes.ReadEnterprise)
			default:
				return nil
			}
		},
	)
}

// rulesetWriteScopeAccess declares the exhaustive scope challenge policy for
// create_repository_ruleset. The exact scope challenged depends on the
// "level" argument: repository writes need "repo", organization writes need
// "admin:org", and enterprise writes need "admin:enterprise". A missing or
// unrecognized level returns no challenge so normal handler validation
// produces the error.
func rulesetWriteScopeAccess() inventory.ScopeAccess {
	return scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.AdminOrg, scopes.AdminEnterprise},
		func(activeScopes []string) bool {
			return scopes.HasAll(activeScopes, scopes.Repo) ||
				scopes.HasAll(activeScopes, scopes.AdminOrg) ||
				scopes.HasAll(activeScopes, scopes.AdminEnterprise)
		},
		func(arguments map[string]any, activeScopes []string) []string {
			level, ok := arguments["level"].(string)
			if !ok {
				return nil
			}
			switch level {
			case "repository":
				return scopes.ChallengeAll(activeScopes, scopes.Repo)
			case "organization":
				return scopes.ChallengeAll(activeScopes, scopes.AdminOrg)
			case "enterprise":
				return scopes.ChallengeAll(activeScopes, scopes.AdminEnterprise)
			default:
				return nil
			}
		},
	)
}

// RepositoryRulesetRead creates a tool for ruleset and rule-suite reads.
func RepositoryRulesetRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "repository_ruleset_read",
			Description: t("TOOL_REPOSITORY_RULESET_READ_DESCRIPTION", "Read rulesets at the repository, organization, or enterprise level, and rule suites at the repository or organization level. Select the level with the 'level' parameter and the operation with the 'method' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_REPOSITORY_RULESET_READ_USER_TITLE", "Read repository rulesets"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"level": {
						Type:        "string",
						Enum:        []any{"repository", "organization", "enterprise"},
						Description: rulesetLevelDescription,
					},
					"method": {
						Type: "string",
						Enum: []any{"get", "list", "get_rules_for_branch", "list_rule_suites", "get_rule_suite"},
						Description: "Operation to perform:\n" +
							"- 'get': Get a specific ruleset by ID (requires 'ruleset_id'). Supported at every level.\n" +
							"- 'list': List all rulesets. Supported at every level.\n" +
							"- 'get_rules_for_branch': Get all rules that apply to a branch (requires 'branch'). Repository level only.\n" +
							"- 'list_rule_suites': List rule suites, the evaluations of rules against pushes. Repository and organization levels only.\n" +
							"- 'get_rule_suite': Get a specific rule suite by ID (requires 'rule_suite_id'). Repository and organization levels only.",
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner. Required when level is 'repository'.",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. Required when level is 'repository'.",
					},
					"org": {
						Type:        "string",
						Description: "Organization name. Required when level is 'organization'.",
					},
					"enterprise": {
						Type:        "string",
						Description: "Enterprise slug. Required when level is 'enterprise'.",
					},
					"ruleset_id": {
						Type:        "number",
						Description: "Ruleset ID. Required for the 'get' method.",
					},
					"includes_parents": {
						Type:        "boolean",
						Description: "Include rulesets configured at higher levels that also apply. Defaults to true. Used by the 'get' and 'list' methods at the repository level.",
					},
					"branch": {
						Type:        "string",
						Description: "Branch name. Required for the 'get_rules_for_branch' method.",
					},
					"ref": {
						Type:        "string",
						Description: "The name of the ref (branch, tag, etc.) to filter rule suites by. Used by the 'list_rule_suites' method.",
					},
					"time_period": {
						Type:        "string",
						Enum:        []any{"hour", "day", "week", "month"},
						Description: "The time period to filter rule suites by. Used by the 'list_rule_suites' method.",
					},
					"actor_name": {
						Type:        "string",
						Description: "The handle for the GitHub user account to filter rule suites on. Used by the 'list_rule_suites' method.",
					},
					"repository_name": {
						Type:        "string",
						Description: "Repository name to filter rule suites by. Used by the 'list_rule_suites' method at the organization level.",
					},
					"rule_suite_result": {
						Type:        "string",
						Enum:        []any{"pass", "fail", "bypass", "all"},
						Description: "The rule suite result to filter by. Used by the 'list_rule_suites' method.",
					},
					"evaluate_status": {
						Type:        "string",
						Enum:        []any{"all", "active", "evaluate"},
						Description: "Filter rule suites by ruleset evaluation mode. Used by the 'list_rule_suites' method.",
					},
					"rule_suite_id": {
						Type:        "number",
						Description: "Rule suite ID. Required for the 'get_rule_suite' method.",
					},
				},
				Required: []string{"level", "method"},
			}),
		},
		rulesetReadScopeAccess(),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			level, err := RequiredParam[string](args, "level")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch level {
			case "repository":
				return repositoryRulesetReadRepository(ctx, client, strings.ToLower(method), args)
			case "organization":
				return repositoryRulesetReadOrganization(ctx, client, strings.ToLower(method), args)
			case "enterprise":
				return repositoryRulesetReadEnterprise(ctx, client, strings.ToLower(method), args)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown level: %q (expected 'repository', 'organization', or 'enterprise')", level)), nil, nil
			}
		},
	)
}

// repositoryRulesetReadRepository handles repository_ruleset_read calls with level="repository".
func repositoryRulesetReadRepository(ctx context.Context, client *github.Client, method string, args map[string]any) (*mcp.CallToolResult, any, error) {
	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	switch method {
	case "get":
		rulesetID, err := RequiredBigInt(args, "ruleset_id")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		// GetRuleset always sends includes_parents, so preserve GitHub's true default.
		includesParents := true
		if _, ok := args["includes_parents"]; ok {
			includesParents, err = OptionalParam[bool](args, "includes_parents")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
		}
		result, err := GetRepositoryRuleset(ctx, client, owner, repo, rulesetID, includesParents)
		return result, nil, err
	case "list":
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		var includesParents *bool
		if _, ok := args["includes_parents"]; ok {
			v, err := OptionalParam[bool](args, "includes_parents")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			includesParents = &v
		}
		result, err := ListRepositoryRulesets(ctx, client, owner, repo, includesParents, pagination)
		return result, nil, err
	case "get_rules_for_branch":
		branch, err := RequiredParam[string](args, "branch")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := GetRepositoryRulesForBranch(ctx, client, owner, repo, branch, pagination)
		return result, nil, err
	case "list_rule_suites":
		filters, err := ruleSuiteFiltersFromArgs(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := ListRepositoryRuleSuites(ctx, client, owner, repo, filters, pagination)
		return result, nil, err
	case "get_rule_suite":
		ruleSuiteID, err := RequiredBigInt(args, "rule_suite_id")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := GetRepositoryRuleSuite(ctx, client, owner, repo, ruleSuiteID)
		return result, nil, err
	default:
		return utils.NewToolResultError(fmt.Sprintf("unknown method: %q", method)), nil, nil
	}
}

// repositoryRulesetReadOrganization handles repository_ruleset_read calls with level="organization".
func repositoryRulesetReadOrganization(ctx context.Context, client *github.Client, method string, args map[string]any) (*mcp.CallToolResult, any, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	switch method {
	case "get":
		rulesetID, err := RequiredBigInt(args, "ruleset_id")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := GetOrganizationRepositoryRuleset(ctx, client, org, rulesetID)
		return result, nil, err
	case "list":
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := ListOrganizationRepositoryRulesets(ctx, client, org, pagination)
		return result, nil, err
	case "list_rule_suites":
		filters, err := ruleSuiteFiltersFromArgs(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := ListOrganizationRuleSuites(ctx, client, org, filters, pagination)
		return result, nil, err
	case "get_rule_suite":
		ruleSuiteID, err := RequiredBigInt(args, "rule_suite_id")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := GetOrganizationRuleSuite(ctx, client, org, ruleSuiteID)
		return result, nil, err
	default:
		return utils.NewToolResultError(fmt.Sprintf("method %q is not supported for level \"organization\"; supported methods: get, list, list_rule_suites, get_rule_suite", method)), nil, nil
	}
}

// repositoryRulesetReadEnterprise handles repository_ruleset_read calls with level="enterprise".
func repositoryRulesetReadEnterprise(ctx context.Context, client *github.Client, method string, args map[string]any) (*mcp.CallToolResult, any, error) {
	enterprise, err := RequiredParam[string](args, "enterprise")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	switch method {
	case "get":
		rulesetID, err := RequiredBigInt(args, "ruleset_id")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := GetEnterpriseRepositoryRuleset(ctx, client, enterprise, rulesetID)
		return result, nil, err
	case "list":
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		result, err := ListEnterpriseRepositoryRulesets(ctx, client, enterprise, pagination)
		return result, nil, err
	default:
		return utils.NewToolResultError(fmt.Sprintf("method %q is not supported for level \"enterprise\"; supported methods: get, list", method)), nil, nil
	}
}

// GetRepositoryRuleset gets a specific repository ruleset by ID.
func GetRepositoryRuleset(ctx context.Context, client *github.Client, owner, repo string, rulesetID int64, includesParents bool) (*mcp.CallToolResult, error) {
	ruleset, resp, err := client.Repositories.GetRuleset(ctx, owner, repo, rulesetID, includesParents)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository ruleset", resp, err), nil
	}

	return MarshalledTextResult(ruleset), nil
}

// ListRepositoryRulesets lists all rulesets for a repository. When
// includesParents is nil GitHub's default behaviour (include parents) is used.
func ListRepositoryRulesets(ctx context.Context, client *github.Client, owner, repo string, includesParents *bool, pagination PaginationParams) (*mcp.CallToolResult, error) {
	opts := &github.RepositoryListRulesetsOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
		IncludesParents: includesParents,
	}

	rulesets, resp, err := client.Repositories.GetAllRulesets(ctx, owner, repo, opts)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list repository rulesets", resp, err), nil
	}

	return MarshalledTextResult(rulesets), nil
}

// GetRepositoryRulesForBranch gets all rules that apply to a specific branch.
func GetRepositoryRulesForBranch(ctx context.Context, client *github.Client, owner, repo, branch string, pagination PaginationParams) (*mcp.CallToolResult, error) {
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	branchRules, resp, err := client.Repositories.ListRulesForBranch(ctx, owner, repo, url.PathEscape(branch), opts)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository rules for branch", resp, err), nil
	}

	return MarshalledTextResult(branchRules), nil
}

// RuleSuiteFilters holds the optional filters for listing rule suites.
type RuleSuiteFilters struct {
	Ref             string
	RepositoryName  string
	TimePeriod      string
	ActorName       string
	RuleSuiteResult string
	EvaluateStatus  string
}

func ruleSuiteFiltersFromArgs(args map[string]any) (RuleSuiteFilters, error) {
	ref, err := OptionalParam[string](args, "ref")
	if err != nil {
		return RuleSuiteFilters{}, err
	}
	repositoryName, err := OptionalParam[string](args, "repository_name")
	if err != nil {
		return RuleSuiteFilters{}, err
	}
	timePeriod, err := OptionalParam[string](args, "time_period")
	if err != nil {
		return RuleSuiteFilters{}, err
	}
	actorName, err := OptionalParam[string](args, "actor_name")
	if err != nil {
		return RuleSuiteFilters{}, err
	}
	ruleSuiteResult, err := OptionalParam[string](args, "rule_suite_result")
	if err != nil {
		return RuleSuiteFilters{}, err
	}
	evaluateStatus, err := OptionalParam[string](args, "evaluate_status")
	if err != nil {
		return RuleSuiteFilters{}, err
	}
	return RuleSuiteFilters{
		Ref:             ref,
		RepositoryName:  repositoryName,
		TimePeriod:      timePeriod,
		ActorName:       actorName,
		RuleSuiteResult: ruleSuiteResult,
		EvaluateStatus:  evaluateStatus,
	}, nil
}

// ListRepositoryRuleSuites lists rule suites (evaluations of rules against
// pushes) for a repository. Rule suites are not supported by go-github, so the
// request is issued directly.
func ListRepositoryRuleSuites(ctx context.Context, client *github.Client, owner, repo string, filters RuleSuiteFilters, pagination PaginationParams) (*mcp.CallToolResult, error) {
	apiURL := fmt.Sprintf("repos/%s/%s/rulesets/rule-suites", url.PathEscape(owner), url.PathEscape(repo))
	return listRuleSuites(ctx, client, apiURL, "failed to list repository rule suites", filters, false, pagination)
}

// ListOrganizationRuleSuites lists rule suites for an organization.
func ListOrganizationRuleSuites(ctx context.Context, client *github.Client, org string, filters RuleSuiteFilters, pagination PaginationParams) (*mcp.CallToolResult, error) {
	apiURL := fmt.Sprintf("orgs/%s/rulesets/rule-suites", url.PathEscape(org))
	return listRuleSuites(ctx, client, apiURL, "failed to list organization rule suites", filters, true, pagination)
}

func listRuleSuites(ctx context.Context, client *github.Client, apiURL, errorMessage string, filters RuleSuiteFilters, includeRepositoryName bool, pagination PaginationParams) (*mcp.CallToolResult, error) {
	query := url.Values{}
	if filters.Ref != "" {
		query.Set("ref", filters.Ref)
	}
	if includeRepositoryName && filters.RepositoryName != "" {
		query.Set("repository_name", filters.RepositoryName)
	}
	if filters.TimePeriod != "" {
		query.Set("time_period", filters.TimePeriod)
	}
	if filters.ActorName != "" {
		query.Set("actor_name", filters.ActorName)
	}
	if filters.RuleSuiteResult != "" {
		query.Set("rule_suite_result", filters.RuleSuiteResult)
	}
	if filters.EvaluateStatus != "" {
		query.Set("evaluate_status", filters.EvaluateStatus)
	}
	if pagination.Page > 0 {
		query.Set("page", strconv.Itoa(pagination.Page))
	}
	if pagination.PerPage > 0 {
		query.Set("per_page", strconv.Itoa(pagination.PerPage))
	}
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}

	req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to create request", err), nil
	}

	var ruleSuites any
	resp, err := client.Do(req, &ruleSuites)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, errorMessage, resp, err), nil
	}

	return MarshalledTextResult(ruleSuites), nil
}

// GetRepositoryRuleSuite gets details of a specific repository rule suite.
func GetRepositoryRuleSuite(ctx context.Context, client *github.Client, owner, repo string, ruleSuiteID int64) (*mcp.CallToolResult, error) {
	apiURL := fmt.Sprintf("repos/%s/%s/rulesets/rule-suites/%d", url.PathEscape(owner), url.PathEscape(repo), ruleSuiteID)
	return getRuleSuite(ctx, client, apiURL, "failed to get repository rule suite")
}

// GetOrganizationRuleSuite gets details of a specific organization rule suite.
func GetOrganizationRuleSuite(ctx context.Context, client *github.Client, org string, ruleSuiteID int64) (*mcp.CallToolResult, error) {
	apiURL := fmt.Sprintf("orgs/%s/rulesets/rule-suites/%d", url.PathEscape(org), ruleSuiteID)
	return getRuleSuite(ctx, client, apiURL, "failed to get organization rule suite")
}

func getRuleSuite(ctx context.Context, client *github.Client, apiURL, errorMessage string) (*mcp.CallToolResult, error) {
	req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to create request", err), nil
	}

	var ruleSuite any
	resp, err := client.Do(req, &ruleSuite)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, errorMessage, resp, err), nil
	}

	return MarshalledTextResult(ruleSuite), nil
}

// GetOrganizationRepositoryRuleset gets a specific organization repository
// ruleset by ID.
func GetOrganizationRepositoryRuleset(ctx context.Context, client *github.Client, org string, rulesetID int64) (*mcp.CallToolResult, error) {
	ruleset, resp, err := client.Organizations.GetRepositoryRuleset(ctx, org, rulesetID)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get organization repository ruleset", resp, err), nil
	}

	return MarshalledTextResult(ruleset), nil
}

// ListOrganizationRepositoryRulesets lists all repository rulesets for an
// organization.
func ListOrganizationRepositoryRulesets(ctx context.Context, client *github.Client, org string, pagination PaginationParams) (*mcp.CallToolResult, error) {
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	rulesets, resp, err := client.Organizations.ListAllRepositoryRulesets(ctx, org, opts)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list organization repository rulesets", resp, err), nil
	}

	return MarshalledTextResult(rulesets), nil
}

// GetEnterpriseRepositoryRuleset gets a specific enterprise repository
// ruleset by ID.
func GetEnterpriseRepositoryRuleset(ctx context.Context, client *github.Client, enterprise string, rulesetID int64) (*mcp.CallToolResult, error) {
	ruleset, resp, err := client.Enterprise.GetRepositoryRuleset(ctx, enterprise, rulesetID)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get enterprise repository ruleset", resp, err), nil
	}

	return MarshalledTextResult(ruleset), nil
}

type enterpriseRepositoryRulesetsResponse struct {
	TotalCount int                         `json:"total_count"`
	Rulesets   []*github.RepositoryRuleset `json:"rulesets"`
}

// ListEnterpriseRepositoryRulesets lists all repository rulesets for an
// enterprise. Listing enterprise rulesets is not supported by go-github, so
// the request is issued directly.
func ListEnterpriseRepositoryRulesets(ctx context.Context, client *github.Client, enterprise string, pagination PaginationParams) (*mcp.CallToolResult, error) {
	apiURL := fmt.Sprintf("enterprises/%s/rulesets", url.PathEscape(enterprise))
	query := url.Values{}
	if pagination.Page > 0 {
		query.Set("page", strconv.Itoa(pagination.Page))
	}
	if pagination.PerPage > 0 {
		query.Set("per_page", strconv.Itoa(pagination.PerPage))
	}
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}

	req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to create request", err), nil
	}

	var result enterpriseRepositoryRulesetsResponse
	resp, err := client.Do(req, &result)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list enterprise repository rulesets", resp, err), nil
	}

	return MarshalledTextResult(result), nil
}

// CreateRepositoryRuleset creates a tool to create a new repository ruleset
// at the repository, organization, or enterprise level. The level is
// selected with the "level" parameter.
func CreateRepositoryRuleset(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := rulesetWriteProperties()
	properties["level"] = &jsonschema.Schema{
		Type:        "string",
		Enum:        []any{"repository", "organization", "enterprise"},
		Description: rulesetLevelDescription,
	}
	properties["owner"] = &jsonschema.Schema{Type: "string", Description: "Repository owner. Required when level is 'repository'."}
	properties["repo"] = &jsonschema.Schema{Type: "string", Description: "Repository name. Required when level is 'repository'."}
	properties["org"] = &jsonschema.Schema{Type: "string", Description: "Organization name. Required when level is 'organization'."}
	properties["enterprise"] = &jsonschema.Schema{Type: "string", Description: "Enterprise slug. Required when level is 'enterprise'."}

	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "create_repository_ruleset",
			Description: t("TOOL_CREATE_REPOSITORY_RULESET_DESCRIPTION", "Create a new ruleset at the repository, organization, or enterprise level"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_REPOSITORY_RULESET_USER_TITLE", "Create repository ruleset"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
				Properties:           properties,
				Required:             []string{"level", "name", "enforcement", "rules"},
			},
		},
		rulesetWriteScopeAccess(),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			level, err := RequiredParam[string](args, "level")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ruleset, errResult := buildRepositoryRulesetFromArgs(args)
			if errResult != nil {
				return errResult, nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch level {
			case "repository":
				owner, err := RequiredParam[string](args, "owner")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				repo, err := RequiredParam[string](args, "repo")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				created, resp, err := client.Repositories.CreateRuleset(ctx, owner, repo, ruleset)
				if resp != nil {
					defer func() { _ = resp.Body.Close() }()
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create repository ruleset", resp, err), nil, nil
				}
				return MarshalledTextResult(created), nil, nil
			case "organization":
				org, err := RequiredParam[string](args, "org")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				created, resp, err := client.Organizations.CreateRepositoryRuleset(ctx, org, ruleset)
				if resp != nil {
					defer func() { _ = resp.Body.Close() }()
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create organization repository ruleset", resp, err), nil, nil
				}
				return MarshalledTextResult(created), nil, nil
			case "enterprise":
				enterprise, err := RequiredParam[string](args, "enterprise")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				created, resp, err := client.Enterprise.CreateRepositoryRuleset(ctx, enterprise, ruleset)
				if resp != nil {
					defer func() { _ = resp.Body.Close() }()
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create enterprise repository ruleset", resp, err), nil, nil
				}
				return MarshalledTextResult(created), nil, nil
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown level: %q (expected 'repository', 'organization', or 'enterprise')", level)), nil, nil
			}
		},
	)
}

// rulesetWriteProperties returns the shared input schema properties for the
// ruleset creation tool. Callers add the level-specific identifier
// properties (owner/repo, org, or enterprise).
func rulesetWriteProperties() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"name": {
			Type:        "string",
			Description: "The name of the ruleset",
		},
		"enforcement": {
			Type:        "string",
			Enum:        []any{"disabled", "active", "evaluate"},
			Description: "The enforcement level of the ruleset. 'evaluate' allows admins to test rules before enforcing them",
		},
		"target": {
			Type:        "string",
			Enum:        []any{"branch", "tag", "push", "repository"},
			Description: "The target of the ruleset. Defaults to 'branch'. 'repository' is only valid for 'organization' and 'enterprise' level rulesets.",
		},
		"rules": {
			Type:        "array",
			Description: "An array of rules within the ruleset. Each rule is an object with a 'type' (e.g. 'creation', 'deletion', 'non_fast_forward', 'required_signatures', 'pull_request', 'required_status_checks') and, for rules that need configuration, a 'parameters' object",
			Items: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
				Properties: map[string]*jsonschema.Schema{
					"type": {
						Type:        "string",
						Description: "The type of rule, e.g. 'creation', 'deletion', 'non_fast_forward', 'required_signatures', 'pull_request', 'required_status_checks'",
					},
					"parameters": {
						Type:        "object",
						Description: "Parameters for rule types that require additional configuration",
					},
				},
				Required: []string{"type"},
			},
		},
		"conditions": {
			Type:        "object",
			Description: "Conditions for when this ruleset applies, e.g. {\"ref_name\": {\"include\": [\"refs/heads/main\"], \"exclude\": []}}",
		},
		"bypass_actors": {
			Type:        "array",
			Description: "The actors that can bypass the rules in this ruleset",
			Items: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
				Properties: map[string]*jsonschema.Schema{
					"actor_id": {
						Type:        "number",
						Description: "The ID of the actor that can bypass a ruleset",
					},
					"actor_type": {
						Type:        "string",
						Enum:        []any{"Integration", "OrganizationAdmin", "RepositoryRole", "Team", "DeployKey", "User", "EnterpriseOwner", "EnterpriseRole"},
						Description: "The type of actor that can bypass a ruleset. 'EnterpriseOwner' and 'EnterpriseRole' are only valid for 'enterprise' level rulesets.",
					},
					"bypass_mode": {
						Type:        "string",
						Enum:        []any{"always", "pull_request", "exempt"},
						Description: "When the specified actor can bypass the ruleset. 'pull_request' only applies to branch rulesets and is not valid for the 'DeployKey' actor type. 'exempt' means rules are not run for that actor and no bypass audit entry is created.",
					},
				},
				Required: []string{"actor_type", "bypass_mode"},
			},
		},
	}
}

// buildRepositoryRulesetFromArgs assembles a github.RepositoryRuleset from the
// shared ruleset creation arguments. It returns a non-nil *mcp.CallToolResult
// describing the problem when the arguments are invalid.
func buildRepositoryRulesetFromArgs(args map[string]any) (github.RepositoryRuleset, *mcp.CallToolResult) {
	name, err := RequiredParam[string](args, "name")
	if err != nil {
		return github.RepositoryRuleset{}, utils.NewToolResultError(err.Error())
	}
	enforcement, err := RequiredParam[string](args, "enforcement")
	if err != nil {
		return github.RepositoryRuleset{}, utils.NewToolResultError(err.Error())
	}
	target, err := OptionalParam[string](args, "target")
	if err != nil {
		return github.RepositoryRuleset{}, utils.NewToolResultError(err.Error())
	}

	rules, ok := args["rules"].([]any)
	if !ok {
		return github.RepositoryRuleset{}, utils.NewToolResultError("rules parameter must be an array of rule objects")
	}

	requestedRuleTypes := make([]string, 0, len(rules))
	requestedRuleParameters := make(map[string]map[string]any, len(rules))
	seenRuleTypes := make(map[string]bool, len(rules))
	for i, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			return github.RepositoryRuleset{}, utils.NewToolResultError("each rule must be an object with a 'type' field")
		}
		for key := range ruleMap {
			if key != "type" && key != "parameters" {
				return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("rules[%d]: unsupported or unrecognized key: %q", i, key))
			}
		}
		ruleType, ok := ruleMap["type"].(string)
		if !ok || ruleType == "" {
			return github.RepositoryRuleset{}, utils.NewToolResultError("each rule must have a non-empty string 'type' field")
		}
		if seenRuleTypes[ruleType] {
			// go-github stores only one rule per type.
			return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("duplicate rule type: %q (a ruleset may only have one rule of each type)", ruleType))
		}
		seenRuleTypes[ruleType] = true
		requestedRuleTypes = append(requestedRuleTypes, ruleType)
		if parameters, exists := ruleMap["parameters"]; exists && parameters != nil {
			parametersMap, ok := parameters.(map[string]any)
			if !ok {
				return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("rule %q: parameters must be an object", ruleType))
			}
			requestedRuleParameters[ruleType] = parametersMap
		}
	}

	payload := map[string]any{
		"name":        name,
		"enforcement": enforcement,
		"rules":       rules,
	}
	if target != "" {
		payload["target"] = target
	}
	if conditions, exists := args["conditions"]; exists && conditions != nil {
		conditionsMap, ok := conditions.(map[string]any)
		if !ok {
			return github.RepositoryRuleset{}, utils.NewToolResultError("conditions parameter must be an object")
		}
		payload["conditions"] = conditionsMap
	}
	if bypassActors, exists := args["bypass_actors"]; exists && bypassActors != nil {
		bypassActorsArr, ok := bypassActors.([]any)
		if !ok {
			return github.RepositoryRuleset{}, utils.NewToolResultError("bypass_actors parameter must be an array of objects")
		}
		for i, actor := range bypassActorsArr {
			actorMap, ok := actor.(map[string]any)
			if !ok {
				return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("bypass_actors[%d] must be an object", i))
			}
			for key := range actorMap {
				if key != "actor_id" && key != "actor_type" && key != "bypass_mode" {
					return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("bypass_actors[%d]: unsupported or unrecognized key: %q", i, key))
				}
			}
			bypassMode, ok := actorMap["bypass_mode"].(string)
			if !ok {
				return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("bypass_actors[%d].bypass_mode is required and must be a string", i))
			}
			switch github.BypassMode(bypassMode) {
			case github.BypassModeAlways, github.BypassModePullRequest, github.BypassModeExempt:
			default:
				return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("bypass_actors[%d].bypass_mode must be one of \"always\", \"pull_request\", or \"exempt\"", i))
			}
		}
		payload["bypass_actors"] = bypassActorsArr
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return github.RepositoryRuleset{}, utils.NewToolResultErrorFromErr("failed to build ruleset request", err)
	}
	var ruleset github.RepositoryRuleset
	if err := json.Unmarshal(raw, &ruleset); err != nil {
		return github.RepositoryRuleset{}, utils.NewToolResultErrorFromErr("failed to parse ruleset request", err)
	}

	// Reject fields silently discarded by go-github's custom ruleset unmarshaler.
	appliedRules, errResult := rulesetAppliedRules(ruleset.Rules)
	if errResult != nil {
		return github.RepositoryRuleset{}, errResult
	}
	for _, ruleType := range requestedRuleTypes {
		appliedParameters, ok := appliedRules[ruleType]
		if !ok {
			return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("unsupported or unrecognized rule type: %q", ruleType))
		}
		if requested := requestedRuleParameters[ruleType]; requested != nil {
			if droppedPath := droppedKeyPath(requested, appliedParameters); droppedPath != "" {
				return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("rule %q: unsupported or unrecognized parameter: %q", ruleType, droppedPath))
			}
		}
	}

	if requestedConditions, ok := payload["conditions"].(map[string]any); ok {
		appliedConditions, errResult := rulesetAppliedConditions(ruleset.Conditions)
		if errResult != nil {
			return github.RepositoryRuleset{}, errResult
		}
		if droppedPath := droppedKeyPath(requestedConditions, appliedConditions); droppedPath != "" {
			return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("conditions: unsupported or unrecognized key: %q", droppedPath))
		}
	}

	return ruleset, nil
}

// droppedKeyPath returns the first caller-supplied field omitted by the local
// go-github JSON round-trip. JSON zero values may disappear through omitempty.
func droppedKeyPath(requested, applied map[string]any) string {
	for key, requestedValue := range requested {
		appliedValue, ok := applied[key]
		if !ok {
			if isZeroJSONValue(requestedValue) {
				continue
			}
			return key
		}
		if nested := droppedValuePath(requestedValue, appliedValue); nested != "" {
			return key + nested
		}
	}
	return ""
}

func droppedValuePath(requested, applied any) string {
	switch requestedTyped := requested.(type) {
	case map[string]any:
		appliedMap, ok := applied.(map[string]any)
		if !ok {
			return ""
		}
		if nested := droppedKeyPath(requestedTyped, appliedMap); nested != "" {
			return "." + nested
		}
		return ""
	case []any:
		appliedArr, ok := applied.([]any)
		if !ok {
			return ""
		}
		for i, requestedElem := range requestedTyped {
			if i >= len(appliedArr) {
				if isZeroJSONValue(requestedElem) {
					continue
				}
				return fmt.Sprintf("[%d]", i)
			}
			if nested := droppedValuePath(requestedElem, appliedArr[i]); nested != "" {
				return fmt.Sprintf("[%d]%s", i, nested)
			}
		}
		return ""
	default:
		return ""
	}
}

func isZeroJSONValue(v any) bool {
	switch value := v.(type) {
	case nil:
		return true
	case bool:
		return !value
	case float64:
		return value == 0
	case string:
		return value == ""
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	default:
		return false
	}
}

// rulesetAppliedRules marshals the parsed rules back to the API's array form
// and returns, for each rule type that was actually retained, the parameter
// object the corresponding go-github struct recognized.
func rulesetAppliedRules(rules *github.RepositoryRulesetRules) (map[string]map[string]any, *mcp.CallToolResult) {
	applied := map[string]map[string]any{}
	if rules == nil {
		return applied, nil
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to validate ruleset rules", err)
	}
	var ruleObjects []struct {
		Type       string         `json:"type"`
		Parameters map[string]any `json:"parameters"`
	}
	if err := json.Unmarshal(raw, &ruleObjects); err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to validate ruleset rules", err)
	}
	for _, rule := range ruleObjects {
		parameters := rule.Parameters
		if parameters == nil {
			parameters = map[string]any{}
		}
		applied[rule.Type] = parameters
	}
	return applied, nil
}

// rulesetAppliedConditions marshals the parsed conditions back to the API's
// object form and returns the keys that were actually retained.
func rulesetAppliedConditions(conditions *github.RepositoryRulesetConditions) (map[string]any, *mcp.CallToolResult) {
	if conditions == nil {
		return map[string]any{}, nil
	}
	raw, err := json.Marshal(conditions)
	if err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to validate ruleset conditions", err)
	}
	var applied map[string]any
	if err := json.Unmarshal(raw, &applied); err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to validate ruleset conditions", err)
	}
	if applied == nil {
		applied = map[string]any{}
	}
	return applied, nil
}
