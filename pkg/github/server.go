package github

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-github/v74/github"
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

// RequiredIntArrayParam is a helper function that can be used to fetch a required integer array parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request
// 2. Checks if the parameter is an array and each element can be converted to int
// 3. Checks if the array is not empty
func RequiredIntArrayParam(r mcp.CallToolRequest, p string) ([]int, error) {
	// Check if the parameter is present in the request
	if _, ok := r.GetArguments()[p]; !ok {
		return nil, fmt.Errorf("missing required parameter: %s", p)
	}

	switch v := r.GetArguments()[p].(type) {
	case nil:
		return nil, fmt.Errorf("missing required parameter: %s", p)
	case []int:
		if len(v) == 0 {
			return nil, fmt.Errorf("parameter %s cannot be empty", p)
		}
		return v, nil
	case []any:
		if len(v) == 0 {
			return nil, fmt.Errorf("parameter %s cannot be empty", p)
		}
		intSlice := make([]int, len(v))
		for i, elem := range v {
			switch num := elem.(type) {
			case float64:
				intSlice[i] = int(num)
			case int:
				intSlice[i] = num
			case int32:
				intSlice[i] = int(num)
			case int64:
				intSlice[i] = int(num)
			default:
				return nil, fmt.Errorf("parameter %s contains non-numeric value, element %d is %T", p, i, elem)
			}
		}
		return intSlice, nil
	default:
		return nil, fmt.Errorf("parameter %s is not an array, is %T", p, r.GetArguments()[p])
	}
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

// OptionalIntArrayParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns its zero-value
// 2. If it is present, iterates the elements and checks each is a number that can be converted to int
func OptionalIntArrayParam(r mcp.CallToolRequest, p string) ([]int, error) {
	// Check if the parameter is present in the request
	if _, ok := r.GetArguments()[p]; !ok {
		return []int{}, nil
	}

	switch v := r.GetArguments()[p].(type) {
	case nil:
		return []int{}, nil
	case []int:
		return v, nil
	case []any:
		intSlice := make([]int, len(v))
		for i, v := range v {
			switch num := v.(type) {
			case float64:
				intSlice[i] = int(num)
			case int:
				intSlice[i] = num
			case int32:
				intSlice[i] = int(num)
			case int64:
				intSlice[i] = int(num)
			default:
				return []int{}, fmt.Errorf("parameter %s array element at index %d is not of type number, is %T", p, i, v)
			}
		}
		return intSlice, nil
	default:
		return []int{}, fmt.Errorf("parameter %s could not be coerced to []int, is %T", p, r.GetArguments()[p])
	}
}

// WithPagination adds REST API pagination parameters to a tool.
// https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api
func WithPagination() mcp.ToolOption {
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
	}
}

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

func MarshalledTextResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to marshal text result to json", err)
	}

	return mcp.NewToolResultText(string(data))
}
