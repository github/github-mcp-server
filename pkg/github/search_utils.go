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
	// fields, when non-empty, restricts each result item to the requested
	// subset of fields. fieldsTool and fieldsDeps identify the calling tool and
	// its dependencies so fields telemetry can be recorded.
	fields     []string
	fieldsTool string
	fieldsDeps ToolDependencies
}

type searchOption func(*searchConfig)

// withSearchPostProcess registers a callback invoked after a successful search
// response. The callback may mutate the call result (e.g. to attach _meta.ifc).
func withSearchPostProcess(fn searchPostProcessFn) searchOption {
	return func(c *searchConfig) { c.postProcess = fn }
}

// withFieldsFiltering enables the optional `fields` response filtering for a
// search tool. When fields is non-empty, each result item is reduced to the
// requested subset while the total_count / incomplete_results wrapper is
// preserved. tool and deps identify the caller so fields telemetry (adoption and
// realized savings) can be recorded.
func withFieldsFiltering(deps ToolDependencies, tool string, fields []string) searchOption {
	return func(c *searchConfig) {
		c.fieldsDeps = deps
		c.fieldsTool = tool
		c.fields = fields
	}
}

// searchMode selects the engine used to run a search. It maps to the endpoint's
// search_type parameter.
type searchMode int

const (
	// searchModeLexical is the API default, so search_type can be omitted.
	searchModeLexical searchMode = iota
	searchModeSemantic
)

// prepareSearchArgs resolves the search query string and REST search options from the tool args,
// applying the standard is:<type> / repo:<owner>/<repo> munging shared by search_issues and
// search_pull_requests.
func prepareSearchArgs(args map[string]any, targetType string, mode searchMode) (string, *github.SearchOptions, error) {
	query, err := RequiredParam[string](args, "query")
	if err != nil {
		return "", nil, err
	}

	if !hasSpecificFilter(query, "is", targetType) {
		query = fmt.Sprintf("is:%s %s", targetType, query)
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

	// Lexical is the API default, so it leaves search_type unset.
	if mode == searchModeSemantic {
		query = applySemanticSearch(query, opts)
	}

	return query, opts, nil
}

// qualifierQuotePattern matches a quoted qualifier value, e.g. label:"needs
// triage". The quotes there are meaningful — they delimit a value containing
// spaces — so they must survive stripFreeTextQuotes.
var qualifierQuotePattern = regexp.MustCompile(`([-\w.]+:)"([^"]*)"`)

// stripFreeTextQuotes removes quotes around free text while preserving them
// around qualifier values — since these delimit a value containing spaces.
func stripFreeTextQuotes(query string) string {
	const sentinel = "\x00"

	// Hide qualifier quotes behind a sentinel that cannot appear in a query,
	// strip what remains, then restore them.
	protected := qualifierQuotePattern.ReplaceAllString(query, "${1}"+sentinel+"${2}"+sentinel)
	stripped := strings.ReplaceAll(protected, `"`, "")
	return strings.ReplaceAll(stripped, sentinel, `"`)
}

// applySemanticSearch switches the request to the semantic index.
func applySemanticSearch(query string, opts *github.SearchOptions) string {
	opts.SearchType = "semantic"
	return stripFreeTextQuotes(query)
}

func searchHandler(
	ctx context.Context,
	getClient GetClientFn,
	args map[string]any,
	targetType string,
	errorPrefix string,
	options ...searchOption,
) (*mcp.CallToolResult, error) {
	cfg := searchConfig{}
	for _, opt := range options {
		opt(&cfg)
	}
	query, opts, err := prepareSearchArgs(args, targetType, searchModeLexical)
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

	// result.Issues are raw *github.Issue objects marshaled directly below rather than through
	// a convertToMinimal* helper (see minimal_types.go), so Title/Body must be sanitized here.
	for _, iss := range result.Issues {
		sanitizeIssueTitleAndBody(iss)
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
