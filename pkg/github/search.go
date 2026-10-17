package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

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

// SearchRepositories creates a tool to search for GitHub repositories.
func SearchRepositories(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Repository search query. Examples: 'machine learning in:name stars:>1000 language:python', 'topic:react', 'user:facebook'. Supports advanced search syntax for precise filtering.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort repositories by field, defaults to best match",
				Enum:        []any{"stars", "forks", "help-wanted-issues", "updated"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
			"minimal_output": {
				Type:        "boolean",
				Description: "Return minimal repository information (default: true). When false, modern clients receive additional curated repository details, not the complete GitHub API object. Legacy clients retain full GitHub API repository objects.",
				Default:     json.RawMessage(`true`),
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool[SearchRepositoriesInput, *SearchRepositoriesOutput](
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "search_repositories",
			Description: t("TOOL_SEARCH_REPOSITORIES_DESCRIPTION", "Find GitHub repositories by name, description, readme, topics, or other metadata. Perfect for discovering projects, finding examples, or locating specific repositories across GitHub."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_REPOSITORIES_USER_TITLE", "Search repositories"),
				ReadOnlyHint: true,
			},
			InputSchema:  schema,
			OutputSchema: searchRepositoriesOutputSchema(),
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input SearchRepositoriesInput) (*mcp.CallToolResult, *SearchRepositoriesOutput, error) {
			var output *SearchRepositoriesOutput
			if input.Query == "" {
				return utils.NewToolResultError("missing required parameter: query"), output, nil
			}
			query := input.Query
			pagination := repositoryPagination(input.Page, input.PerPage)
			minimalOutput := input.MinimalOutput == nil || *input.MinimalOutput
			opts := &github.SearchOptions{
				Sort:  input.Sort,
				Order: input.Order,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), output, nil
			}
			result, resp, err := client.Search.Repositories(ctx, query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search repositories with query '%s'", query),
					resp,
					err,
				), output, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), output, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to search repositories", resp, body), output, nil
			}

			// Return either minimal or full response based on parameter
			var r []byte
			output = &SearchRepositoriesOutput{
				TotalCount:        result.GetTotal(),
				IncompleteResults: result.GetIncompleteResults(),
				Items:             make([]SearchRepository, 0, len(result.Repositories)),
			}
			if minimalOutput {
				minimalRepos := make([]MinimalRepository, 0, len(result.Repositories))
				for _, repo := range result.Repositories {
					minimalRepo := minimalSearchRepository(repo)
					minimalRepos = append(minimalRepos, minimalRepo)
					output.Items = append(output.Items, SearchRepository{MinimalRepository: minimalRepo})
				}

				minimalResult := &MinimalSearchRepositoriesResult{
					TotalCount:        result.GetTotal(),
					IncompleteResults: result.GetIncompleteResults(),
					Items:             minimalRepos,
				}

				r, err = json.Marshal(minimalResult)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to marshal minimal response", err), nil, nil
				}
			} else {
				r, err = json.Marshal(result)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to marshal full response", err), nil, nil
				}
				for _, repo := range result.Repositories {
					output.Items = append(output.Items, fullSearchRepository(repo))
				}
			}

			callResult := utils.NewToolResultText(string(r))
			attachSearchRepositoriesIFCLabel(ctx, deps, result.Repositories, callResult)
			return callResult, output, nil
		},
		normalizeSearchArguments(true),
	)
}

// attachSearchRepositoriesIFCLabel joins per-repository IFC labels across
// every matched repository and attaches the result to callResult when IFC
// labels are enabled. Visibility is read directly from the search response —
// no extra API call. The join math is shared with search_issues via
// ifc.LabelSearchIssues: public-only results stay public-untrusted,
// mixed-visibility results become private-untrusted, and all-private results
// become private-trusted. The
// feature-flag check is centralized here (mirroring the attach* helpers in
// ifc_labels.go) so the handler can call this unconditionally.
func attachSearchRepositoriesIFCLabel(ctx context.Context, deps ToolDependencies, repos []*github.Repository, callResult *mcp.CallToolResult) {
	if callResult == nil || callResult.IsError || !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
		return
	}

	visibilities := make([]bool, 0, len(repos))
	for _, repo := range repos {
		visibilities = append(visibilities, repo.GetPrivate())
	}

	setIFCLabel(callResult, ifc.LabelSearchIssues(visibilities))
}

// SearchCode creates a tool to search for code across GitHub repositories.
type SearchCodeInput struct {
	Query   string   `json:"query"`
	Sort    string   `json:"sort,omitempty"`
	Order   string   `json:"order,omitempty"`
	Fields  []string `json:"fields,omitempty"`
	Page    *int     `json:"page,omitempty"`
	PerPage *int     `json:"perPage,omitempty"`
}

type SearchCodeOutput struct {
	TotalCount        int                    `json:"total_count" jsonschema:"Total number of matching code results."`
	IncompleteResults bool                   `json:"incomplete_results" jsonschema:"Whether GitHub returned an incomplete result set."`
	Items             []SearchCodeOutputItem `json:"items"`
}

type SearchCodeOutputItem struct {
	Name        *string                `json:"name,omitempty"`
	Path        *string                `json:"path,omitempty"`
	SHA         *string                `json:"sha,omitempty"`
	Repository  *string                `json:"repository,omitempty"`
	TextMatches []*SearchCodeTextMatch `json:"text_matches,omitempty" jsonschema:"Matching code fragments returned by GitHub."`
}

type SearchCodeTextMatch struct {
	ObjectURL  *string                   `json:"object_url,omitempty" jsonschema:"API URL of the matched file or object."`
	ObjectType *string                   `json:"object_type,omitempty" jsonschema:"GitHub object type containing the match."`
	Property   *string                   `json:"property,omitempty" jsonschema:"Property containing the matched text."`
	Fragment   *string                   `json:"fragment,omitempty" jsonschema:"Source fragment containing the match."`
	Matches    []*SearchCodeTextMatchHit `json:"matches,omitempty" jsonschema:"Matched text and its offsets in the fragment."`
}

type SearchCodeTextMatchHit struct {
	Text    *string `json:"text,omitempty" jsonschema:"Matched text."`
	Indices []int   `json:"indices,omitempty" jsonschema:"Start and end offsets of the match in the fragment."`
}

func structuredSearchCodeOutput(result MinimalCodeSearchResult, fields []string) SearchCodeOutput {
	output := SearchCodeOutput{
		TotalCount:        result.TotalCount,
		IncompleteResults: result.IncompleteResults,
		Items:             make([]SearchCodeOutputItem, 0, len(result.Items)),
	}
	for _, item := range result.Items {
		selected := func(field string) bool {
			return len(fields) == 0 || slices.Contains(fields, field)
		}
		projected := SearchCodeOutputItem{}
		if selected("name") {
			projected.Name = new(item.Name)
		}
		if selected("path") {
			projected.Path = new(item.Path)
		}
		if selected("sha") {
			projected.SHA = new(item.SHA)
		}
		if selected("repository") {
			projected.Repository = new(item.Repository)
		}
		if selected("text_matches") && item.TextMatches != nil {
			projected.TextMatches = make([]*SearchCodeTextMatch, 0, len(item.TextMatches))
			for _, match := range item.TextMatches {
				if match == nil {
					projected.TextMatches = append(projected.TextMatches, nil)
					continue
				}
				projectedMatch := &SearchCodeTextMatch{
					ObjectURL:  match.ObjectURL,
					ObjectType: match.ObjectType,
					Property:   match.Property,
					Fragment:   match.Fragment,
				}
				if match.Matches != nil {
					projectedMatch.Matches = make([]*SearchCodeTextMatchHit, 0, len(match.Matches))
					for _, hit := range match.Matches {
						if hit == nil {
							projectedMatch.Matches = append(projectedMatch.Matches, nil)
							continue
						}
						projectedMatch.Matches = append(projectedMatch.Matches, &SearchCodeTextMatchHit{
							Text:    hit.Text,
							Indices: hit.Indices,
						})
					}
				}
				projected.TextMatches = append(projected.TextMatches, projectedMatch)
			}
		}
		output.Items = append(output.Items, projected)
	}
	return output
}

func SearchCode(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Search query (GitHub code search REST). Implicit AND between terms; supports `OR`, `NOT`, and `\"quoted phrase\"` for exact match. Qualifiers: `repo:owner/repo`, `org:`, `user:`, `language:`, `path:dir` (prefix match), `filename:exact.ext`, `extension:`, `in:file`, `in:path`, `size:`, `is:archived`, `is:fork`. Max 256 chars. Examples: `WithContext language:go org:github`; `\"package main\" repo:o/r`; `func extension:go path:cmd repo:o/r`; `NOT TODO language:go repo:o/r`.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field ('indexed' only)",
			},
			"order": {
				Type:        "string",
				Description: "Sort order for results",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	schema.Properties["fields"] = fieldsSchemaProperty(
		"Subset of fields to return for each code search result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'repository' and 'text_matches' in particular drops the largest per-result data.",
		codeSearchItemFieldEnum,
	)
	WithPagination(schema)

	return NewTool[SearchCodeInput, SearchCodeOutput](
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "search_code",
			Description: t("TOOL_SEARCH_CODE_DESCRIPTION", "Fast and precise code search across ALL GitHub repositories using GitHub's native search engine. Best for finding exact symbols, functions, classes, or specific code patterns."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_CODE_USER_TITLE", "Search code"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input SearchCodeInput) (*mcp.CallToolResult, SearchCodeOutput, error) {
			var output SearchCodeOutput
			if input.Query == "" {
				return utils.NewToolResultError("missing required parameter: query"), output, nil
			}
			pagination := PaginationParams{Page: 1, PerPage: 30}
			if input.Page != nil {
				pagination.Page = *input.Page
			}
			if input.PerPage != nil {
				pagination.PerPage = *input.PerPage
			}
			query := input.Query

			opts := &github.SearchOptions{
				Sort:      input.Sort,
				Order:     input.Order,
				TextMatch: true,
				ListOptions: github.ListOptions{
					PerPage: pagination.PerPage,
					Page:    pagination.Page,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), output, nil
			}

			result, resp, err := client.Search.Code(ctx, query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search code with query '%s'", query),
					resp,
					err,
				), output, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), output, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to search code", resp, body), output, nil
			}

			minimalItems := make([]MinimalCodeResult, 0, len(result.CodeResults))
			for _, code := range result.CodeResults {
				item := MinimalCodeResult{
					Name:        code.GetName(),
					Path:        code.GetPath(),
					SHA:         code.GetSHA(),
					TextMatches: code.TextMatches,
				}
				if code.Repository != nil {
					item.Repository = code.Repository.GetFullName()
				}
				minimalItems = append(minimalItems, item)
			}

			minimalResult := &MinimalCodeSearchResult{
				TotalCount:        result.GetTotal(),
				IncompleteResults: result.GetIncompleteResults(),
				Items:             minimalItems,
			}

			filtered := false
			var payload any = minimalResult
			if len(input.Fields) > 0 {
				filteredItems, err := filterEachField(minimalItems, input.Fields)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to filter code search results", err), output, nil
				}
				payload = map[string]any{
					"total_count":        minimalResult.TotalCount,
					"incomplete_results": minimalResult.IncompleteResults,
					"items":              filteredItems,
				}
				filtered = true
			}

			r, err := json.Marshal(payload)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), output, nil
			}

			recordSearchCodeFieldsUsage(ctx, deps, minimalResult, filtered, len(r))

			callResult := utils.NewToolResultText(string(r))
			// Code search spans repositories; the IFC label is the conservative
			// join across every matched repository's visibility, read directly
			// from the search response.
			visibilities := make([]bool, 0, len(result.CodeResults))
			for _, code := range result.CodeResults {
				if code.Repository != nil {
					visibilities = append(visibilities, code.Repository.GetPrivate())
				}
			}
			callResult = attachJoinedIFCLabel(ctx, deps, callResult, visibilities, ifc.LabelSearchIssues)
			output = structuredSearchCodeOutput(*minimalResult, input.Fields)
			return callResult, output, nil
		},
		normalizeTypedReadArguments(nil, false),
	)
}

// recordSearchCodeFieldsUsage emits fields telemetry for a search_code call.
// sentBytes is the size of the payload actually returned.
func recordSearchCodeFieldsUsage(ctx context.Context, deps ToolDependencies, full *MinimalCodeSearchResult, filtered bool, sentBytes int) {
	recordFieldsUsageFor(ctx, deps, "search_code", full, filtered, sentBytes)
}

func userOrOrgHandler(ctx context.Context, accountType string, deps ToolDependencies, input SearchAccountsInput) (*mcp.CallToolResult, *MinimalSearchUsersResult, error) {
	if input.Query == "" {
		return utils.NewToolResultError("missing required parameter: query"), nil, nil
	}
	query := input.Query
	pagination := repositoryPagination(input.Page, input.PerPage)

	opts := &github.SearchOptions{
		Sort:  input.Sort,
		Order: input.Order,
		ListOptions: github.ListOptions{
			PerPage: pagination.PerPage,
			Page:    pagination.Page,
		},
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	searchQuery := query
	if !hasTypeFilter(query) {
		searchQuery = "type:" + accountType + " " + query
	}
	result, resp, err := client.Search.Users(ctx, searchQuery, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to search %ss with query '%s'", accountType, query),
			resp,
			err,
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, fmt.Sprintf("failed to search %ss", accountType), resp, body), nil, nil
	}

	minimalUsers := make([]MinimalUser, 0, len(result.Users))

	for _, user := range result.Users {
		if user.Login != nil {
			mu := MinimalUser{
				Login:      user.GetLogin(),
				ID:         user.GetID(),
				ProfileURL: user.GetHTMLURL(),
				AvatarURL:  user.GetAvatarURL(),
			}
			minimalUsers = append(minimalUsers, mu)
		}
	}
	minimalResp := &MinimalSearchUsersResult{
		TotalCount:        result.GetTotal(),
		IncompleteResults: result.GetIncompleteResults(),
		Items:             minimalUsers,
	}
	if result.Total != nil {
		minimalResp.TotalCount = *result.Total
	}
	if result.IncompleteResults != nil {
		minimalResp.IncompleteResults = *result.IncompleteResults
	}

	r, err := json.Marshal(minimalResp)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
	}
	callResult := utils.NewToolResultText(string(r))
	// User and organization search returns public profile information that is
	// authored by the account holders themselves, so it is public-untrusted.
	callResult = attachStaticIFCLabel(ctx, deps, callResult, ifc.PublicUntrusted())
	return callResult, minimalResp, nil
}

// SearchUsers creates a tool to search for GitHub users.
func SearchUsers(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "User search query. Examples: 'john smith', 'location:seattle', 'followers:>100'. Search is automatically scoped to type:user.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort users by number of followers or repositories, or when the person joined GitHub.",
				Enum:        []any{"followers", "repositories", "joined"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool[SearchAccountsInput, *MinimalSearchUsersResult](
		ToolsetMetadataUsers,
		mcp.Tool{
			Name:        "search_users",
			Description: t("TOOL_SEARCH_USERS_DESCRIPTION", "Find GitHub users by username, real name, or other profile information. Useful for locating developers, contributors, or team members."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_USERS_USER_TITLE", "Search users"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input SearchAccountsInput) (*mcp.CallToolResult, *MinimalSearchUsersResult, error) {
			return userOrOrgHandler(ctx, "user", deps, input)
		},
		normalizeSearchArguments(false),
	)
}

// SearchOrgs creates a tool to search for GitHub organizations.
func SearchOrgs(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Organization search query. Examples: 'microsoft', 'location:california', 'created:>=2025-01-01'. Search is automatically scoped to type:org.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field by category",
				Enum:        []any{"followers", "repositories", "joined"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool[SearchAccountsInput, *MinimalSearchUsersResult](
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name:        "search_orgs",
			Description: t("TOOL_SEARCH_ORGS_DESCRIPTION", "Find GitHub organizations by name, location, or other organization metadata. Ideal for discovering companies, open source foundations, or teams."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_ORGS_USER_TITLE", "Search organizations"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.ReadOrg),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input SearchAccountsInput) (*mcp.CallToolResult, *MinimalSearchUsersResult, error) {
			return userOrOrgHandler(ctx, "org", deps, input)
		},
		normalizeSearchArguments(false),
	)
}

// SearchCommits creates a tool to search for commits across GitHub repositories.
func SearchCommits(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Commit search query (GitHub commit search REST). Searches commit messages on the default branch only. Scope the search with `repo:owner/repo`, `org:`, or `user:` (queries without a scope qualifier match across all of GitHub and are usually not what you want). Other qualifiers: `author:`, `committer:`, `author-name:`, `committer-name:`, `author-email:`, `committer-email:`, `author-date:`, `committer-date:` (supports `>`, `<`, `>=`, `<=`, and `YYYY-MM-DD..YYYY-MM-DD` ranges), `merge:true|false`, `hash:`, `tree:`, `parent:`, `is:public`. Examples: `repo:owner/repo fix panic`; `org:github author:defunkt committer-date:>=2024-01-01`; `\"refactor cache\" repo:o/r`; `hash:abc1234 repo:o/r`.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort by author or committer date (defaults to best match)",
				Enum:        []any{"author-date", "committer-date"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool[SearchCommitsInput, *MinimalSearchCommitsResult](
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "search_commits",
			Description: t("TOOL_SEARCH_COMMITS_DESCRIPTION", "Search for commits across GitHub repositories using GitHub's commit search syntax. Useful for finding specific changes, authors, or messages across one or many repositories. Searches the default branch only."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_COMMITS_USER_TITLE", "Search commits"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input SearchCommitsInput) (*mcp.CallToolResult, *MinimalSearchCommitsResult, error) {
			if input.Query == "" {
				return utils.NewToolResultError("missing required parameter: query"), nil, nil
			}
			query := input.Query
			pagination := repositoryPagination(input.Page, input.PerPage)

			opts := &github.SearchOptions{
				Sort:  input.Sort,
				Order: input.Order,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			result, resp, err := client.Search.Commits(ctx, query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search commits with query '%s'", query),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to search commits", resp, body), nil, nil
			}

			minimalCommits := make([]MinimalCommitSearchItem, 0, len(result.Commits))
			for _, commit := range result.Commits {
				minimalCommits = append(minimalCommits, convertCommitResultToMinimalCommit(commit))
			}

			minimalResult := &MinimalSearchCommitsResult{
				TotalCount:        result.GetTotal(),
				IncompleteResults: result.GetIncompleteResults(),
				Items:             minimalCommits,
			}

			r, err := json.Marshal(minimalResult)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			callResult := utils.NewToolResultText(string(r))
			// Commit search spans repositories; the IFC label is the conservative
			// join across every matched repository's visibility, read directly
			// from the search response.
			visibilities := make([]bool, 0, len(result.Commits))
			for _, commit := range result.Commits {
				if commit.Repository != nil {
					visibilities = append(visibilities, commit.Repository.GetPrivate())
				}
			}
			callResult = attachJoinedIFCLabel(ctx, deps, callResult, visibilities, ifc.LabelSearchIssues)
			return callResult, minimalResult, nil
		},
		normalizeSearchArguments(false),
	)
}
