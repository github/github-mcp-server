package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func hasFilter(query, filterType string) bool {
	// Match filter at start of string, after whitespace, or after non-word characters like '('
	pattern := fmt.Sprintf(`(^|\s|\W)%s:\S+`, regexp.QuoteMeta(filterType))
	matched, _ := regexp.MatchString(pattern, query)
	return matched
}

func hasSpecificFilter(query, filterType, filterValue string) bool {
	// Match specific filter:value at start, after whitespace, or after non-word characters
	// End with word boundary, whitespace, or non-word characters like ')'
	pattern := fmt.Sprintf(`(^|\s|\W)%s:%s($|\s|\W)`, regexp.QuoteMeta(filterType), regexp.QuoteMeta(filterValue))
	matched, _ := regexp.MatchString(pattern, query)
	return matched
}

func hasRepoFilter(query string) bool {
	return hasFilter(query, "repo")
}

func hasTypeFilter(query string) bool {
	return hasFilter(query, "type")
}

// searchPostProcessFn is invoked after a successful search response, before
// the call result is returned. It may attach additional metadata (such as IFC
// labels) to the call result based on the search payload.
type searchPostProcessFn func(ctx context.Context, result *github.IssuesSearchResult, callResult *mcp.CallToolResult)

type searchConfig struct {
	postProcess searchPostProcessFn
}

type searchOption func(*searchConfig)

// withSearchPostProcess registers a callback invoked after a successful search
// response. The callback may mutate the call result (e.g. to attach _meta.ifc).
func withSearchPostProcess(fn searchPostProcessFn) searchOption {
	return func(c *searchConfig) { c.postProcess = fn }
}

// prepareSearchArgs resolves the search query string and REST search options from the tool args,
// applying the standard is:<type> / repo:<owner>/<repo> munging shared by search_issues and
// search_pull_requests.
func prepareSearchArgs(args map[string]any, searchType string) (string, *github.SearchOptions, error) {
	query, err := RequiredParam[string](args, "query")
	if err != nil {
		return "", nil, err
	}

	if !hasSpecificFilter(query, "is", searchType) {
		query = fmt.Sprintf("is:%s %s", searchType, query)
	}

	owner, err := OptionalParam[string](args, "owner")
	if err != nil {
		return "", nil, err
	}

	repo, err := OptionalParam[string](args, "repo")
	if err != nil {
		return "", nil, err
	}

	if owner != "" && repo != "" && !hasRepoFilter(query) {
		query = fmt.Sprintf("repo:%s/%s %s", owner, repo, query)
	}

	sort, err := OptionalParam[string](args, "sort")
	if err != nil {
		return "", nil, err
	}
	order, err := OptionalParam[string](args, "order")
	if err != nil {
		return "", nil, err
	}
	pagination, err := OptionalPaginationParams(args)
	if err != nil {
		return "", nil, err
	}

	opts := &github.SearchOptions{
		Sort:  sort,
		Order: order,
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	// field.<name>:<value> qualifiers require the advanced search API.
	if strings.Contains(query, "field.") {
		opts.AdvancedSearch = github.Ptr(true)
	}

	return query, opts, nil
}

func searchHandler(
	ctx context.Context,
	getClient GetClientFn,
	args map[string]any,
	searchType string,
	errorPrefix string,
	options ...searchOption,
) (*mcp.CallToolResult, error) {
	cfg := searchConfig{}
	for _, opt := range options {
		opt(&cfg)
	}
	query, opts, err := prepareSearchArgs(args, searchType)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	client, err := getClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix+": failed to get GitHub client", err), nil
	}
	result, resp, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr(errorPrefix+": failed to read response body", err), nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, errorPrefix, resp, body), nil
	}

	r, err := json.Marshal(result)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix+": failed to marshal response", err), nil
	}

	callResult := utils.NewToolResultText(string(r))
	if cfg.postProcess != nil {
		cfg.postProcess(ctx, result, callResult)
	}
	return callResult, nil
}
