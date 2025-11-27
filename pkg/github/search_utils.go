package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
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
	request mcp.CallToolRequest,
	searchType string,
	errorPrefix string,
) (*mcp.CallToolResult, error) {
	query, err := RequiredParam[string](request, "query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if !hasSpecificFilter(query, "is", searchType) {
		query = fmt.Sprintf("is:%s %s", searchType, query)
	}

	owner, err := OptionalParam[string](request, "owner")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	repo, err := OptionalParam[string](request, "repo")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if owner != "" && repo != "" && !hasRepoFilter(query) {
		query = fmt.Sprintf("repo:%s/%s %s", owner, repo, query)
	}

	sort, err := OptionalParam[string](request, "sort")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	order, err := OptionalParam[string](request, "order")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	pagination, err := OptionalFixedCursorPaginationParams(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// WithFixedCursorPagination: fetch exactly pageSize items, use TotalCount to determine if there's more
	pageSize := pagination.PerPage
	// Determine current page from After cursor
	page := 1
	if pagination.After != "" {
		decoded, err := decodePageCursor(pagination.After)
		if err == nil && decoded > 0 {
			page = decoded
		}
	}
	opts := &github.SearchOptions{
		Sort:  sort,
		Order: order,
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: pageSize,
		},
	}

	client, err := getClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get GitHub client: %w", errorPrefix, err)
	}
	result, resp, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to read response body: %w", errorPrefix, err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", errorPrefix, string(body))), nil
	}

	// Prepare paginated results
	items := result.Issues
	totalCount := result.GetTotal()

	// Calculate if there's a next page based on total count and current position
	currentItemCount := len(items)
	itemsSeenSoFar := (page-1)*pageSize + currentItemCount
	hasNextPage := itemsSeenSoFar < totalCount

	nextCursor := ""
	if hasNextPage {
		nextPage := page + 1
		nextCursor = encodePageCursor(nextPage)
	}

	pageInfo := struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor,omitempty"`
	}{
		HasNextPage: hasNextPage,
		EndCursor:   nextCursor,
	}

	response := struct {
		TotalCount        int             `json:"totalCount"`
		IncompleteResults bool            `json:"incompleteResults"`
		Items             []*github.Issue `json:"items"`
		PageInfo          interface{}     `json:"pageInfo"`
	}{
		TotalCount:        totalCount,
		IncompleteResults: result.GetIncompleteResults(),
		Items:             items,
		PageInfo:          pageInfo,
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal response: %w", errorPrefix, err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// encodePageCursor encodes the page number as a base64 string
func encodePageCursor(page int) string {
	s := fmt.Sprintf("page=%d", page)
	return b64Encode(s)
}

// decodePageCursor decodes a base64 cursor and extracts the page number
func decodePageCursor(cursor string) (int, error) {
	data, err := b64Decode(cursor)
	if err != nil {
		return 1, err
	}
	var page int
	n, err := fmt.Sscanf(data, "page=%d", &page)
	if err != nil || n != 1 {
		return 1, fmt.Errorf("invalid cursor format")
	}
	return page, nil
}

// b64Encode encodes a string to base64
func b64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// b64Decode decodes a base64 string
func b64Decode(s string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
