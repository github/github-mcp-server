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
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RepositoryRulesetRead creates a tool for read operations on a repository's
// rulesets and rule suites. The operation is selected with the "method"
// parameter.
func RepositoryRulesetRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "repository_ruleset_read",
			Description: t("TOOL_REPOSITORY_RULESET_READ_DESCRIPTION", "Read a repository's rulesets and rule suites. Select the operation with the 'method' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_REPOSITORY_RULESET_READ_USER_TITLE", "Read repository rulesets"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Enum: []any{"get", "list", "get_rules_for_branch", "list_rule_suites", "get_rule_suite"},
						Description: "Operation to perform:\n" +
							"- 'get': Get a specific ruleset by ID (requires 'ruleset_id').\n" +
							"- 'list': List all rulesets for the repository.\n" +
							"- 'get_rules_for_branch': Get all rules that apply to a branch (requires 'branch').\n" +
							"- 'list_rule_suites': List rule suites, the evaluations of rules against pushes.\n" +
							"- 'get_rule_suite': Get a specific rule suite by ID (requires 'rule_suite_id').",
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"ruleset_id": {
						Type:        "number",
						Description: "Ruleset ID. Required for the 'get' method.",
					},
					"includes_parents": {
						Type:        "boolean",
						Description: "Include rulesets configured at higher levels that also apply. Defaults to true. Used by the 'get' and 'list' methods.",
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
					"rule_suite_result": {
						Type:        "string",
						Enum:        []any{"pass", "fail", "bypass", "all"},
						Description: "The rule suite result to filter by. Used by the 'list_rule_suites' method.",
					},
					"rule_suite_id": {
						Type:        "number",
						Description: "Rule suite ID. Required for the 'get_rule_suite' method.",
					},
				},
				Required: []string{"method", "owner", "repo"},
			}),
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch strings.ToLower(method) {
			case "get":
				rulesetID, err := RequiredBigInt(args, "ruleset_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				// GetRuleset always sends includes_parents; default to the
				// GitHub API default of true when the caller omits it.
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
		},
	)
}

// OrganizationRepositoryRulesetRead creates a tool for read operations on an
// organization's repository rulesets. The operation is selected with the
// "method" parameter.
func OrganizationRepositoryRulesetRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "organization_repository_ruleset_read",
			Description: t("TOOL_ORGANIZATION_REPOSITORY_RULESET_READ_DESCRIPTION", "Read an organization's repository rulesets. Select the operation with the 'method' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ORGANIZATION_REPOSITORY_RULESET_READ_USER_TITLE", "Read organization repository rulesets"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Enum: []any{"get", "list"},
						Description: "Operation to perform:\n" +
							"- 'get': Get a specific repository ruleset by ID (requires 'ruleset_id').\n" +
							"- 'list': List all repository rulesets for the organization.",
					},
					"org": {
						Type:        "string",
						Description: "Organization name",
					},
					"ruleset_id": {
						Type:        "number",
						Description: "Ruleset ID. Required for the 'get' method.",
					},
				},
				Required: []string{"method", "org"},
			}),
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch strings.ToLower(method) {
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
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %q", method)), nil, nil
			}
		},
	)
}

// GetRepositoryRuleset gets a specific repository ruleset by ID.
func GetRepositoryRuleset(ctx context.Context, client *github.Client, owner, repo string, rulesetID int64, includesParents bool) (*mcp.CallToolResult, error) {
	ruleset, resp, err := client.Repositories.GetRuleset(ctx, owner, repo, rulesetID, includesParents)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository ruleset", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

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
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list repository rulesets", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	return MarshalledTextResult(rulesets), nil
}

// GetRepositoryRulesForBranch gets all rules that apply to a specific branch.
func GetRepositoryRulesForBranch(ctx context.Context, client *github.Client, owner, repo, branch string, pagination PaginationParams) (*mcp.CallToolResult, error) {
	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	branchRules, resp, err := client.Repositories.ListRulesForBranch(ctx, owner, repo, branch, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository rules for branch", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	return MarshalledTextResult(branchRules), nil
}

// ruleSuiteFilters holds the optional filters for listing rule suites.
type ruleSuiteFilters struct {
	Ref             string
	TimePeriod      string
	ActorName       string
	RuleSuiteResult string
}

func ruleSuiteFiltersFromArgs(args map[string]any) (ruleSuiteFilters, error) {
	ref, err := OptionalParam[string](args, "ref")
	if err != nil {
		return ruleSuiteFilters{}, err
	}
	timePeriod, err := OptionalParam[string](args, "time_period")
	if err != nil {
		return ruleSuiteFilters{}, err
	}
	actorName, err := OptionalParam[string](args, "actor_name")
	if err != nil {
		return ruleSuiteFilters{}, err
	}
	ruleSuiteResult, err := OptionalParam[string](args, "rule_suite_result")
	if err != nil {
		return ruleSuiteFilters{}, err
	}
	return ruleSuiteFilters{
		Ref:             ref,
		TimePeriod:      timePeriod,
		ActorName:       actorName,
		RuleSuiteResult: ruleSuiteResult,
	}, nil
}

// ListRepositoryRuleSuites lists rule suites (evaluations of rules against
// pushes) for a repository. Rule suites are not supported by go-github, so the
// request is issued directly.
func ListRepositoryRuleSuites(ctx context.Context, client *github.Client, owner, repo string, filters ruleSuiteFilters, pagination PaginationParams) (*mcp.CallToolResult, error) {
	apiURL := fmt.Sprintf("repos/%s/%s/rulesets/rule-suites", owner, repo)
	query := url.Values{}
	if filters.Ref != "" {
		query.Set("ref", filters.Ref)
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
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list repository rule suites", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	return MarshalledTextResult(ruleSuites), nil
}

// GetRepositoryRuleSuite gets details of a specific repository rule suite,
// including the evaluation results for each rule. Rule suites are not supported
// by go-github, so the request is issued directly.
func GetRepositoryRuleSuite(ctx context.Context, client *github.Client, owner, repo string, ruleSuiteID int64) (*mcp.CallToolResult, error) {
	apiURL := fmt.Sprintf("repos/%s/%s/rulesets/rule-suites/%d", owner, repo, ruleSuiteID)
	req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to create request", err), nil
	}

	var ruleSuite any
	resp, err := client.Do(req, &ruleSuite)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository rule suite", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	return MarshalledTextResult(ruleSuite), nil
}

// GetOrganizationRepositoryRuleset gets a specific organization repository
// ruleset by ID.
func GetOrganizationRepositoryRuleset(ctx context.Context, client *github.Client, org string, rulesetID int64) (*mcp.CallToolResult, error) {
	ruleset, resp, err := client.Organizations.GetRepositoryRuleset(ctx, org, rulesetID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get organization repository ruleset", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

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
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list organization repository rulesets", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	return MarshalledTextResult(rulesets), nil
}

// CreateRepositoryRuleset creates a tool to create a new repository ruleset.
func CreateRepositoryRuleset(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := rulesetWriteProperties([]any{"branch", "tag", "push"})
	properties["owner"] = &jsonschema.Schema{Type: "string", Description: "Repository owner"}
	properties["repo"] = &jsonschema.Schema{Type: "string", Description: "Repository name"}

	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "create_repository_ruleset",
			Description: t("TOOL_CREATE_REPOSITORY_RULESET_DESCRIPTION", "Create a new ruleset for a repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_REPOSITORY_RULESET_USER_TITLE", "Create repository ruleset"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"owner", "repo", "name", "enforcement", "rules"},
			},
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
			ruleset, errResult := buildRepositoryRulesetFromArgs(args)
			if errResult != nil {
				return errResult, nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			created, resp, err := client.Repositories.CreateRuleset(ctx, owner, repo, ruleset)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create repository ruleset", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(created), nil, nil
		},
	)
}

// CreateOrganizationRepositoryRuleset creates a tool to create a new organization repository ruleset.
func CreateOrganizationRepositoryRuleset(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := rulesetWriteProperties([]any{"branch", "tag", "push", "repository"})
	properties["org"] = &jsonschema.Schema{Type: "string", Description: "Organization name"}

	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "create_organization_repository_ruleset",
			Description: t("TOOL_CREATE_ORGANIZATION_REPOSITORY_RULESET_DESCRIPTION", "Create a new repository ruleset for an organization"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_ORGANIZATION_REPOSITORY_RULESET_USER_TITLE", "Create organization repository ruleset"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"org", "name", "enforcement", "rules"},
			},
		},
		[]scopes.Scope{scopes.AdminOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
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

			created, resp, err := client.Organizations.CreateRepositoryRuleset(ctx, org, ruleset)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create organization repository ruleset", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(created), nil, nil
		},
	)
}

// CreateEnterpriseRepositoryRuleset creates a tool to create a new enterprise repository ruleset.
func CreateEnterpriseRepositoryRuleset(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := rulesetWriteProperties([]any{"branch", "tag", "push", "repository"})
	properties["enterprise"] = &jsonschema.Schema{Type: "string", Description: "Enterprise slug"}

	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "create_enterprise_repository_ruleset",
			Description: t("TOOL_CREATE_ENTERPRISE_REPOSITORY_RULESET_DESCRIPTION", "Create a new repository ruleset for an enterprise"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_ENTERPRISE_REPOSITORY_RULESET_USER_TITLE", "Create enterprise repository ruleset"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"enterprise", "name", "enforcement", "rules"},
			},
		},
		[]scopes.Scope{scopes.AdminEnterprise},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			enterprise, err := RequiredParam[string](args, "enterprise")
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

			created, resp, err := client.Enterprise.CreateRepositoryRuleset(ctx, enterprise, ruleset)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create enterprise repository ruleset", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(created), nil, nil
		},
	)
}

// rulesetWriteProperties returns the shared input schema properties for the
// ruleset creation tools. Callers pass the target values valid for the API
// level and add the owner/repo, org, or enterprise identifier properties.
func rulesetWriteProperties(targets []any) map[string]*jsonschema.Schema {
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
			Enum:        targets,
			Description: "The target of the ruleset. Defaults to 'branch'",
		},
		"rules": {
			Type:        "array",
			Description: "An array of rules within the ruleset. Each rule is an object with a 'type' (e.g. 'creation', 'deletion', 'non_fast_forward', 'required_signatures', 'pull_request', 'required_status_checks') and, for rules that need configuration, a 'parameters' object",
			Items: &jsonschema.Schema{
				Type: "object",
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
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"actor_id": {
						Type:        "number",
						Description: "The ID of the actor that can bypass a ruleset",
					},
					"actor_type": {
						Type:        "string",
						Enum:        []any{"Integration", "OrganizationAdmin", "RepositoryRole", "Team", "DeployKey"},
						Description: "The type of actor that can bypass a ruleset",
					},
					"bypass_mode": {
						Type:        "string",
						Enum:        []any{"always", "pull_request"},
						Description: "When the specified actor can bypass the ruleset",
					},
				},
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
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			return github.RepositoryRuleset{}, utils.NewToolResultError("each rule must be an object with a 'type' field")
		}
		ruleType, ok := ruleMap["type"].(string)
		if !ok || ruleType == "" {
			return github.RepositoryRuleset{}, utils.NewToolResultError("each rule must have a non-empty string 'type' field")
		}
		requestedRuleTypes = append(requestedRuleTypes, ruleType)
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

	// github.RepositoryRulesetRules.UnmarshalJSON silently discards rule types it
	// does not recognize, which would let a typo create a weaker ruleset than the
	// caller requested. Verify every requested rule type survived the round-trip.
	appliedRuleTypes, errResult := rulesetAppliedRuleTypes(ruleset.Rules)
	if errResult != nil {
		return github.RepositoryRuleset{}, errResult
	}
	for _, ruleType := range requestedRuleTypes {
		if !appliedRuleTypes[ruleType] {
			return github.RepositoryRuleset{}, utils.NewToolResultError(fmt.Sprintf("unsupported or unrecognized rule type: %q", ruleType))
		}
	}

	return ruleset, nil
}

// rulesetAppliedRuleTypes marshals the parsed rules back to the API's array form
// and returns the set of rule types that were actually retained.
func rulesetAppliedRuleTypes(rules *github.RepositoryRulesetRules) (map[string]bool, *mcp.CallToolResult) {
	applied := map[string]bool{}
	if rules == nil {
		return applied, nil
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to validate ruleset rules", err)
	}
	var ruleObjects []struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &ruleObjects); err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to validate ruleset rules", err)
	}
	for _, rule := range ruleObjects {
		applied[rule.Type] = true
	}
	return applied, nil
}
