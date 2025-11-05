package github

import (
	"encoding/json"
	"errors"
	"fmt"

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

// WithPagination adds REST API pagination parameters to a tool.
// https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api
// WithPagination adds cursor-based pagination parameters to a tool.
// This replaces the old page/perPage approach with a simple cursor string.
func WithPagination() mcp.ToolOption {
	return func(tool *mcp.Tool) {
		mcp.WithString("cursor",
			mcp.Description("Pagination cursor. Leave empty for the first page. Use the nextCursor value from the previous response to get the next page."),
		)(tool)
	}
}

// WithUnifiedPagination is deprecated and now redirects to WithPagination
// for consistency. All pagination is now cursor-based.
func WithUnifiedPagination() mcp.ToolOption {
	return WithPagination()
}

// WithCursorPagination adds cursor-based pagination parameters to a tool.
// This is now the same as WithPagination for consistency.
func WithCursorPagination() mcp.ToolOption {
	return WithPagination()
}

type PaginationParams struct {
	Page    int
	PerPage int
	After   string
}

// OptionalPaginationParams returns the "page", "perPage", and "after" parameters from the request,
// or their default values if not present, "page" default is 1, "perPage" default is 30.
// In future, we may want to make the default values configurable, or even have this
// function returned from `withPagination`, where the defaults are provided alongside
// the min/max values.
func OptionalPaginationParams(r mcp.CallToolRequest) (PaginationParams, error) {
	page, err := OptionalIntParamWithDefault(r, "page", 1)
	if err != nil {
		return PaginationParams{}, err
	}
	perPage, err := OptionalIntParamWithDefault(r, "perPage", 30)
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

// GetCursorBasedParams extracts and decodes the cursor parameter from the request.
// Returns decoded pagination parameters with a fixed page size of 10.
func GetCursorBasedParams(r mcp.CallToolRequest) (*DecodedCursor, error) {
	cursor, err := OptionalParam[string](r, "cursor")
	if err != nil {
		return nil, err
	}
	
	decoded, err := DecodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	
	// Always use page size of 10 as per requirements
	decoded.PerPage = 10
	
	return decoded, nil
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

// CursorBasedPaginationParams represents the cursor-based pagination input
type CursorBasedPaginationParams struct {
	Cursor string
}

// DecodedCursor represents the decoded cursor information
type DecodedCursor struct {
	Page    int
	PerPage int
	After   string // For GraphQL cursors
}

// DecodeCursor decodes a cursor string into pagination parameters
// Cursor format: "page=2;perPage=10" for REST or just a GraphQL cursor string
func DecodeCursor(cursor string) (*DecodedCursor, error) {
	if cursor == "" {
		// Empty cursor means first page with default size
		return &DecodedCursor{
			Page:    1,
			PerPage: 10,
		}, nil
	}

	// Check if this is a GraphQL cursor (doesn't contain '=')
	if !contains(cursor, "=") {
		// This is a GraphQL cursor, return it as-is in After field
		return &DecodedCursor{
			Page:    1,
			PerPage: 10,
			After:   cursor,
		}, nil
	}

	// Parse REST API cursor format: "page=2;perPage=10"
	decoded := &DecodedCursor{
		Page:    1,
		PerPage: 10,
	}

	parts := splitString(cursor, ";")
	for _, part := range parts {
		kv := splitString(part, "=")
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		value := kv[1]

		switch key {
		case "page":
			var p int
			_, err := fmt.Sscanf(value, "%d", &p)
			if err == nil {
				decoded.Page = p
			}
		case "perPage":
			var pp int
			_, err := fmt.Sscanf(value, "%d", &pp)
			if err == nil {
				decoded.PerPage = pp
			}
		}
	}

	return decoded, nil
}

// EncodeCursor encodes pagination parameters into a cursor string
func EncodeCursor(page int, perPage int) string {
	return fmt.Sprintf("page=%d;perPage=%d", page, perPage)
}

// EncodeGraphQLCursor returns the GraphQL cursor as-is
func EncodeGraphQLCursor(cursor string) string {
	return cursor
}

// PaginatedResponse wraps a response with pagination metadata
type PaginatedResponse struct {
	Data        interface{} `json:"data"`
	MoreResults bool        `json:"moreResults"`
	NextCursor  string      `json:"nextCursor,omitempty"`
}

// NewPaginatedRESTResponse creates a paginated response for REST API results.
// It fetches one extra item to determine if there are more results.
func NewPaginatedRESTResponse(data interface{}, currentPage int, pageSize int, hasMore bool) *PaginatedResponse {
	resp := &PaginatedResponse{
		Data:        data,
		MoreResults: hasMore,
	}
	
	if hasMore {
		resp.NextCursor = EncodeCursor(currentPage+1, pageSize)
	}
	
	return resp
}

// NewPaginatedGraphQLResponse creates a paginated response for GraphQL API results.
func NewPaginatedGraphQLResponse(data interface{}, hasNextPage bool, endCursor string) *PaginatedResponse {
	resp := &PaginatedResponse{
		Data:        data,
		MoreResults: hasNextPage,
	}
	
	if hasNextPage && endCursor != "" {
		resp.NextCursor = EncodeGraphQLCursor(endCursor)
	}
	
	return resp
}

// MarshalPaginatedResponse marshals a paginated response to JSON text result
func MarshalPaginatedResponse(resp *PaginatedResponse) *mcp.CallToolResult {
	data, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to marshal paginated response", err)
	}
	return mcp.NewToolResultText(string(data))
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function to split a string by a delimiter
func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	if sep == "" {
		return []string{s}
	}

	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func MarshalledTextResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to marshal text result to json", err)
	}

	return mcp.NewToolResultText(string(data))
}
