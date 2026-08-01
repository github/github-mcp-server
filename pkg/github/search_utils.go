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
	// Match 筛选 at start of string, after whitespace, 或after non-word characters like '('
	pattern := fmt.Sprintf(`(^|\s|\W)%s:\S+`, regexp.QuoteMeta(filterType))
	matched, _ := regexp.MatchString(pattern, query)
	return matched
}

func hasSpecificFilter(query, filterType, filterValue string) bool {
	// Match specific 筛选:值 at start, after whitespace, 或after non-word characters
	// End with word boundary, whitespace, 或non-word characters like ')'
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

// searchPostProcessFn is invoked after 一个成功ful search 响应, before
// c所有结果 is 返回ed. It may attach additional 元数据 (such as IFC
// labels) 到c所有结果 based 在search payload.
type searchPostProcessFn func(ctx context.Context, result *github.IssuesSearchResult, callResult *mcp.CallToolResult)

type searchConfig struct {
	postProcess searchPostProcessFn
	// fields, when non-空, restricts 每个结果 item 到请求ed
	// subset of fields. fieldsTool 和fieldsDeps identify 调用ing 工具 and
	// its dependencies so fields telemetry 可以 recorded.
	fields     []string
	fieldsTool string
	fieldsDeps ToolDependencies
}

type searchOption func(*searchConfig)

// withSearchPostProcess registers 一个调用back invoked after 一个成功ful search
// 响应. 调用back may mutate c所有结果 (e.g. to attach _meta.ifc).
func withSearchPostProcess(fn searchPostProcessFn) searchOption {
	return func(c *searchConfig) { c.postProcess = fn }
}

// withFieldsFiltering 启用s 可选 `fields` 响应 筛选ing f或a
// search 工具. When fields is non-空, 每个结果 item is reduced to the
// 请求ed subset 当the total_count / incomplete_结果 wrapper is
// preserved. 工具 和deps identify 调用er so fields telemetry (adoption and
// realized savings) 可以 recorded.
func withFieldsFiltering(deps ToolDependencies, tool string, fields []string) searchOption {
	return func(c *searchConfig) {
		c.fieldsDeps = deps
		c.fieldsTool = tool
		c.fields = fields
	}
}

// prepareSearchArgs resolves search query string 和REST search options 来自工具 args,
// applying standard is:<type> / repo:<owner>/<repo> munging shared by search_议题 and
// search_pull_请求s.
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

	// field.<name>:<值> qualifiers require advanced search API.
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

	filtered := false
	var payload any = result
	if len(cfg.fields) > 0 {
		filteredItems, err := filterEachField(result.Issues, cfg.fields)
		if err != nil {
			return utils.NewToolResultErrorFromErr(errorPrefix+": failed to filter results", err), nil
		}
		payload = map[string]any{
			"total_count":        result.Total,
			"incomplete_results": result.IncompleteResults,
			"items":              filteredItems,
		}
		filtered = true
	}

	r, err := json.Marshal(payload)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix+": failed to marshal response", err), nil
	}

	if cfg.fieldsTool != "" {
		recordFieldsUsageFor(ctx, cfg.fieldsDeps, cfg.fieldsTool, result, filtered, len(r))
	}

	callResult := utils.NewToolResultText(string(r))
	if cfg.postProcess != nil {
		cfg.postProcess(ctx, result, callResult)
	}
	return callResult, nil
}
