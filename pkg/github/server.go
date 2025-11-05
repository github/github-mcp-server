package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v76/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates a new GitHub MCP server with the specified GH client and logger.

func NewServer(version string, opts ...server.ServerOption) *server.MCPServer {
	// Add default options
	defaultOpts := []server.ServerOption{
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	}
	opts = append(defaultOpts, opts...)

	// Create a new MCP server
	s := server.NewMCPServer(
		"github-mcp-server",
		version,
		opts...,
	)
	return s
}

// OptionalParamOK is a helper function that can be used to fetch a requested parameter from the request.
// It returns the value, a boolean indicating if the parameter was present, and an error if the type is wrong.
func OptionalParamOK[T any](r mcp.CallToolRequest, p string) (value T, ok bool, err error) {
	// Check if the parameter is present in the request
	val, exists := r.GetArguments()[p]
	if !exists {
		// Not present, return zero value, false, no error
		return
	}

	// Check if the parameter is of the expected type
	value, ok = val.(T)
	if !ok {
		// Present but wrong type
		err = fmt.Errorf("parameter %s is not of type %T, is %T", p, value, val)
		ok = true // Set ok to true because the parameter *was* present, even if wrong type
		return
	}

	// Present and correct type
	ok = true
	return
}

// isAcceptedError checks if the error is an accepted error.
func isAcceptedError(err error) bool {
	var acceptedError *github.AcceptedError
	return errors.As(err, &acceptedError)
}

// RequiredParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request.
// 2. Checks if the parameter is of the expected type.
// 3. Checks if the parameter is not empty, i.e: non-zero value
func RequiredParam[T comparable](r mcp.CallToolRequest, p string) (T, error) {
	var zero T

	// Check if the parameter is present in the request
	if _, ok := r.GetArguments()[p]; !ok {
		return zero, fmt.Errorf("missing required parameter: %s", p)
	}

	// Check if the parameter is of the expected type
	val, ok := r.GetArguments()[p].(T)
	if !ok {
		return zero, fmt.Errorf("parameter %s is not of type %T", p, zero)
	}

	if val == zero {
		return zero, fmt.Errorf("missing required parameter: %s", p)
	}

	return val, nil
}

// RequiredInt is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request.
// 2. Checks if the parameter is of the expected type.
// 3. Checks if the parameter is not empty, i.e: non-zero value
func RequiredInt(r mcp.CallToolRequest, p string) (int, error) {
	v, err := RequiredParam[float64](r, p)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// OptionalParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns its zero-value
// 2. If it is present, it checks if the parameter is of the expected type and returns it
func OptionalParam[T any](r mcp.CallToolRequest, p string) (T, error) {
	var zero T

	// Check if the parameter is present in the request
	if _, ok := r.GetArguments()[p]; !ok {
		return zero, nil
	}

	// Check if the parameter is of the expected type
	if _, ok := r.GetArguments()[p].(T); !ok {
		return zero, fmt.Errorf("parameter %s is not of type %T, is %T", p, zero, r.GetArguments()[p])
	}

	return r.GetArguments()[p].(T), nil
}

// OptionalIntParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns its zero-value
// 2. If it is present, it checks if the parameter is of the expected type and returns it
func OptionalIntParam(r mcp.CallToolRequest, p string) (int, error) {
	v, err := OptionalParam[float64](r, p)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// OptionalIntParamWithDefault is a helper function that can be used to fetch a requested parameter from the request
// similar to optionalIntParam, but it also takes a default value.
func OptionalIntParamWithDefault(r mcp.CallToolRequest, p string, d int) (int, error) {
	v, err := OptionalIntParam(r, p)
	if err != nil {
		return 0, err
	}
	if v == 0 {
		return d, nil
	}
	return v, nil
}

// OptionalBoolParamWithDefault is a helper function that can be used to fetch a requested parameter from the request
// similar to optionalBoolParam, but it also takes a default value.
func OptionalBoolParamWithDefault(r mcp.CallToolRequest, p string, d bool) (bool, error) {
	args := r.GetArguments()
	_, ok := args[p]
	v, err := OptionalParam[bool](r, p)
	if err != nil {
		return false, err
	}
	if !ok {
		return d, nil
	}
	return v, nil
}

// OptionalStringArrayParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns its zero-value
// 2. If it is present, iterates the elements and checks each is a string
func OptionalStringArrayParam(r mcp.CallToolRequest, p string) ([]string, error) {
	// Check if the parameter is present in the request
	if _, ok := r.GetArguments()[p]; !ok {
		return []string{}, nil
	}

	switch v := r.GetArguments()[p].(type) {
	case nil:
		return []string{}, nil
	case []string:
		return v, nil
	case []any:
		strSlice := make([]string, len(v))
		for i, v := range v {
			s, ok := v.(string)
			if !ok {
				return []string{}, fmt.Errorf("parameter %s is not of type string, is %T", p, v)
			}
			strSlice[i] = s
		}
		return strSlice, nil
	default:
		return []string{}, fmt.Errorf("parameter %s could not be coerced to []string, is %T", p, r.GetArguments()[p])
	}
}

// WithPagination adds cursor-based pagination parameter to a tool.
// Page size is fixed at 10 items. The cursor is an opaque string that should be
// passed back to retrieve the next page of results.
func WithPagination() mcp.ToolOption {
	return func(tool *mcp.Tool) {
		mcp.WithString("cursor",
			mcp.Description("Cursor for pagination. Use the cursor value from the previous response's pagination metadata to retrieve the next page. Leave blank for the first page."),
		)(tool)
	}
}

// CursorPageSize is the fixed page size for cursor-based pagination
const CursorPageSize = 10

// CursorFetchSize is the size to fetch from API (one extra to detect if more data exists)
const CursorFetchSize = CursorPageSize + 1

// WithUnifiedPagination adds REST API pagination parameters to a tool.
// GraphQL tools will use this and convert page/perPage to GraphQL cursor parameters internally.
func WithUnifiedPagination() mcp.ToolOption {
	return func(tool *mcp.Tool) {
		mcp.WithNumber("page",
			mcp.Description("Page number for pagination (min 1)"),
			mcp.Min(1),
		)(tool)

		mcp.WithNumber("perPage",
			mcp.Description("Results per page for pagination (min 1, max 100)"),
			mcp.Min(1),
			mcp.Max(100),
		)(tool)

		mcp.WithString("after",
			mcp.Description("Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs."),
		)(tool)
	}
}

// WithCursorPagination adds only cursor-based pagination parameters to a tool (no page parameter).
func WithCursorPagination() mcp.ToolOption {
	return func(tool *mcp.Tool) {
		mcp.WithNumber("perPage",
			mcp.Description("Results per page for pagination (min 1, max 100)"),
			mcp.Min(1),
			mcp.Max(100),
		)(tool)

		mcp.WithString("after",
			mcp.Description("Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs."),
		)(tool)
	}
}

type PaginationParams struct {
	Page    int
	PerPage int
	After   string
}

// ParseCursor parses a cursor string into page and perPage values.
// The cursor format is "page=N" where N is the page number (1-indexed).
// Returns page 1 if cursor is empty or invalid.
func ParseCursor(cursor string) (page int, perPage int) {
	perPage = CursorPageSize
	page = 1

	if cursor == "" {
		return page, perPage
	}

	// Parse cursor format: "page=N"
	parts := strings.Split(cursor, "=")
	if len(parts) == 2 && parts[0] == "page" {
		if parsedPage, err := strconv.Atoi(parts[1]); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	return page, perPage
}

// EncodeCursor creates a cursor string from a page number.
func EncodeCursor(page int) string {
	return fmt.Sprintf("page=%d", page)
}

// OptionalPaginationParams returns pagination parameters from the request.
// This now uses cursor-based pagination where the cursor is parsed into page/perPage.
// For backward compatibility, it still supports page/perPage parameters if provided,
// but the new cursor-based approach is preferred.
func OptionalPaginationParams(r mcp.CallToolRequest) (PaginationParams, error) {
	// First check for cursor parameter (new approach)
	cursor, err := OptionalParam[string](r, "cursor")
	if err != nil {
		return PaginationParams{}, err
	}

	// If cursor is provided, parse it
	if cursor != "" {
		page, perPage := ParseCursor(cursor)
		return PaginationParams{
			Page:    page,
			PerPage: perPage,
			After:   "", // Not used in REST API pagination
		}, nil
	}

	// Fallback to old page/perPage parameters for backward compatibility
	page, err := OptionalIntParamWithDefault(r, "page", 1)
	if err != nil {
		return PaginationParams{}, err
	}
	perPage, err := OptionalIntParamWithDefault(r, "perPage", CursorPageSize)
	if err != nil {
		return PaginationParams{}, err
	}
	after, err := OptionalParam[string](r, "after")
	if err != nil {
		return PaginationParams{}, err
	}
	return PaginationParams{
		Page:    page,
		PerPage: perPage,
		After:   after,
	}, nil
}

// OptionalCursorPaginationParams returns the "perPage" and "after" parameters from the request,
// without the "page" parameter, suitable for cursor-based pagination only.
func OptionalCursorPaginationParams(r mcp.CallToolRequest) (CursorPaginationParams, error) {
	perPage, err := OptionalIntParamWithDefault(r, "perPage", 30)
	if err != nil {
		return CursorPaginationParams{}, err
	}
	after, err := OptionalParam[string](r, "after")
	if err != nil {
		return CursorPaginationParams{}, err
	}
	return CursorPaginationParams{
		PerPage: perPage,
		After:   after,
	}, nil
}

type CursorPaginationParams struct {
	PerPage int
	After   string
}

// ToGraphQLParams converts cursor pagination parameters to GraphQL-specific parameters.
func (p CursorPaginationParams) ToGraphQLParams() (*GraphQLPaginationParams, error) {
	if p.PerPage > 100 {
		return nil, fmt.Errorf("perPage value %d exceeds maximum of 100", p.PerPage)
	}
	if p.PerPage < 0 {
		return nil, fmt.Errorf("perPage value %d cannot be negative", p.PerPage)
	}
	first := int32(p.PerPage)

	var after *string
	if p.After != "" {
		after = &p.After
	}

	return &GraphQLPaginationParams{
		First: &first,
		After: after,
	}, nil
}

type GraphQLPaginationParams struct {
	First *int32
	After *string
}

// ToGraphQLParams converts REST API pagination parameters to GraphQL-specific parameters.
// This converts page/perPage to first parameter for GraphQL queries.
// If After is provided, it takes precedence over page-based pagination.
func (p PaginationParams) ToGraphQLParams() (*GraphQLPaginationParams, error) {
	// Convert to CursorPaginationParams and delegate to avoid duplication
	cursor := CursorPaginationParams{
		PerPage: p.PerPage,
		After:   p.After,
	}
	return cursor.ToGraphQLParams()
}

// PaginatedResponse wraps paginated results with cursor metadata
type PaginatedResponse struct {
	Items    interface{} `json:"items"`
	MoreData bool        `json:"moreData"`
	Cursor   string      `json:"cursor,omitempty"`
}

// CreatePaginatedResponse creates a paginated response with cursor metadata.
// It takes the full results (which may have one extra item), the current page number,
// and returns a response with up to CursorPageSize items, plus metadata about whether
// more data exists and what the next cursor should be.
func CreatePaginatedResponse(items interface{}, currentPage int) (*mcp.CallToolResult, error) {
	// Use reflection or type assertion to handle different slice types
	// For now, we'll use a more generic approach with json marshaling
	data, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	// Parse the JSON to count items
	var itemsArray []interface{}
	if err := json.Unmarshal(data, &itemsArray); err != nil {
		// If it's not an array, return as-is (no pagination metadata)
		return mcp.NewToolResultText(string(data)), nil
	}

	hasMore := len(itemsArray) > CursorPageSize
	itemsToReturn := itemsArray
	if hasMore {
		itemsToReturn = itemsArray[:CursorPageSize]
	}

	response := PaginatedResponse{
		Items:    itemsToReturn,
		MoreData: hasMore,
	}

	if hasMore {
		response.Cursor = EncodeCursor(currentPage + 1)
	}

	resultData, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal paginated response: %w", err)
	}

	return mcp.NewToolResultText(string(resultData)), nil
}

// CreatePaginatedSearchResponse creates a paginated response for search results that have
// structured metadata (like TotalCount). It wraps the Items array with pagination metadata
// while preserving other fields.
func CreatePaginatedSearchResponse(searchResult interface{}, currentPage int) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(searchResult)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search result: %w", err)
	}

	// Parse the search result to extract Items array
	var resultMap map[string]interface{}
	if err := json.Unmarshal(data, &resultMap); err != nil {
		return mcp.NewToolResultText(string(data)), nil
	}

	items, ok := resultMap["items"].([]interface{})
	if !ok {
		// Try "Repositories", "Users", etc.
		if repos, ok := resultMap["repositories"].([]interface{}); ok {
			items = repos
		} else if users, ok := resultMap["users"].([]interface{}); ok {
			items = users
		} else {
			// If we can't find items, return as-is
			return mcp.NewToolResultText(string(data)), nil
		}
	}

	hasMore := len(items) > CursorPageSize
	itemsToReturn := items
	if hasMore {
		itemsToReturn = items[:CursorPageSize]
	}

	// Update the result map with paginated items and add pagination metadata
	resultMap["items"] = itemsToReturn
	resultMap["moreData"] = hasMore
	if hasMore {
		resultMap["cursor"] = EncodeCursor(currentPage + 1)
	}

	resultData, err := json.Marshal(resultMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal paginated search response: %w", err)
	}

	return mcp.NewToolResultText(string(resultData)), nil
}

func MarshalledTextResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to marshal text result to json", err)
	}

	return mcp.NewToolResultText(string(data))
}
