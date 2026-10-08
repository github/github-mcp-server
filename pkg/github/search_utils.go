package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
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

type SearchIssuesInput struct {
	Query   string   `json:"query"`
	Owner   string   `json:"owner,omitempty"`
	Repo    string   `json:"repo,omitempty"`
	Sort    string   `json:"sort,omitempty"`
	Order   string   `json:"order,omitempty"`
	Fields  []string `json:"fields,omitempty"`
	Page    *int     `json:"page,omitempty"`
	PerPage *int     `json:"perPage,omitempty"`
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
func prepareSearchArgs(input SearchIssuesInput, targetType string, mode searchMode) (string, *github.SearchOptions, error) {
	query := input.Query
	if query == "" {
		return "", nil, fmt.Errorf("missing required parameter: query")
	}

	if !hasSpecificFilter(query, "is", targetType) {
		query = fmt.Sprintf("is:%s %s", targetType, query)
	}

	if input.Owner != "" && input.Repo != "" && !hasRepoFilter(query) {
		query = fmt.Sprintf("repo:%s/%s %s", input.Owner, input.Repo, query)
	}

	pagination := PaginationParams{Page: 1, PerPage: 30}
	if input.Page != nil {
		pagination.Page = *input.Page
	}
	if input.PerPage != nil {
		pagination.PerPage = *input.PerPage
	}

	opts := &github.SearchOptions{
		Sort:  input.Sort,
		Order: input.Order,
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	// field.<name>:<value> qualifiers require the advanced search API.
	if strings.Contains(query, "field.") {
		opts.AdvancedSearch = new(true)
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
	input SearchIssuesInput,
	targetType string,
	errorPrefix string,
	options ...searchOption,
) (*mcp.CallToolResult, SearchIssuesResponse, error) {
	var output SearchIssuesResponse
	cfg := searchConfig{}
	for _, opt := range options {
		opt(&cfg)
	}
	query, opts, err := prepareSearchArgs(input, targetType, searchModeLexical)
	if err != nil {
		return utils.NewToolResultError(err.Error()), output, nil
	}

	client, err := getClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix+": failed to get GitHub client", err), output, nil
	}
	result, resp, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix, err), output, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr(errorPrefix+": failed to read response body", err), output, nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, errorPrefix, resp, body), output, nil
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
			return utils.NewToolResultErrorFromErr(errorPrefix+": failed to filter results", err), output, nil
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
		return utils.NewToolResultErrorFromErr(errorPrefix+": failed to marshal response", err), output, nil
	}

	if cfg.fieldsTool != "" {
		recordFieldsUsageFor(ctx, cfg.fieldsDeps, cfg.fieldsTool, result, filtered, len(r))
	}

	callResult := utils.NewToolResultText(string(r))
	if cfg.postProcess != nil {
		cfg.postProcess(ctx, result, callResult)
	}
	output = SearchIssuesResponse{
		Total:             result.Total,
		IncompleteResults: result.IncompleteResults,
		Items:             make([]SearchIssueResult, 0, len(result.Issues)),
	}
	for _, issue := range result.Issues {
		output.Items = append(output.Items, SearchIssueResult{Issue: issue})
	}
	return callResult, output, nil
}
