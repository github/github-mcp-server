package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
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

func searchHandler(
	ctx context.Context,
	getClient GetClientFn,
	args map[string]any,
	searchType string,
	errorPrefix string,
) (*mcp.CallToolResult, error) {
	query, err := RequiredParam[string](args, "query")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	if !hasSpecificFilter(query, "is", searchType) {
		query = fmt.Sprintf("is:%s %s", searchType, query)
	}

	owner, err := OptionalParam[string](args, "owner")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	repo, err := OptionalParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	if owner != "" && repo != "" && !hasRepoFilter(query) {
		query = fmt.Sprintf("repo:%s/%s %s", owner, repo, query)
	}

	sort, err := OptionalParam[string](args, "sort")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	order, err := OptionalParam[string](args, "order")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	pagination, err := OptionalPaginationParams(args)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	opts := &github.SearchOptions{
		// Default to "created" if no sort is provided, as it's a common use case.
		Sort:  sort,
		Order: order,
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
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

	return utils.NewToolResultText(string(r)), nil
}
