package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates a new GitHub MCP server with the specified GH client and logger.

func NewServer(version string, opts *mcp.ServerOptions) *mcp.Server {
	if opts == nil {
		opts = &mcp.ServerOptions{}
	}

	// Create a new MCP server
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "github-mcp-server",
		Title:   "GitHub MCP Server",
		Version: version,
		Icons:   octicons.Icons("mark-github"),
	}, opts)

	return s
}

func CompletionsHandler(getClient GetClientFn) func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		switch req.Params.Ref.Type {
		case "ref/resource":
			if strings.HasPrefix(req.Params.Ref.URI, "repo://") {
				return RepositoryResourceCompletionHandler(getClient)(ctx, req)
			}
			return nil, fmt.Errorf("unsupported resource URI: %s", req.Params.Ref.URI)
		case "ref/prompt":
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported ref type: %s", req.Params.Ref.Type)
		}
	}
}

// OptionalParamOK is a helper function that can be used to fetch a requested parameter from the request.
// It returns the value, a boolean indicating if the parameter was present, and an error if the type is wrong.
func OptionalParamOK[T any, A map[string]any](args A, p string) (value T, ok bool, err error) {
	// Check if the parameter is present in the request
	val, exists := args[p]
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
func RequiredParam[T comparable](args map[string]any, p string) (T, error) {
	var zero T

	// Check if the parameter is present in the request
	if _, ok := args[p]; !ok {
		return zero, fmt.Errorf("missing required parameter: %s", p)
	}

	// Check if the parameter is of the expected type
	val, ok := args[p].(T)
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
func RequiredInt(args map[string]any, p string) (int, error) {
	v, err := RequiredParam[float64](args, p)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// RequiredBigInt is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request.
// 2. Checks if the parameter is of the expected type (float64).
// 3. Checks if the parameter is not empty, i.e: non-zero value.
// 4. Validates that the float64 value can be safely converted to int64 without truncation.
func RequiredBigInt(args map[string]any, p string) (int64, error) {
	v, err := RequiredParam[float64](args, p)
	if err != nil {
		return 0, err
	}

	result := int64(v)
	// Check if converting back produces the same value to avoid silent truncation
	if float64(result) != v {
		return 0, fmt.Errorf("parameter %s value %f is too large to fit in int64", p, v)
	}
	return result, nil
}

// OptionalParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns its zero-value
// 2. If it is present, it checks if the parameter is of the expected type and returns it
func OptionalParam[T any](args map[string]any, p string) (T, error) {
	var zero T

	// Check if the parameter is present in the request
	if _, ok := args[p]; !ok {
		return zero, nil
	}

	// Check if the parameter is of the expected type
	if _, ok := args[p].(T); !ok {
		return zero, fmt.Errorf("parameter %s is not of type %T, is %T", p, zero, args[p])
	}

	return args[p].(T), nil
}

// OptionalIntParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns its zero-value
// 2. If it is present, it checks if the parameter is of the expected type and returns it
func OptionalIntParam(args map[string]any, p string) (int, error) {
	v, err := OptionalParam[float64](args, p)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// OptionalIntParamWithDefault is a helper function that can be used to fetch a requested parameter from the request
// similar to optionalIntParam, but it also takes a default value.
func OptionalIntParamWithDefault(args map[string]any, p string, d int) (int, error) {
	v, err := OptionalIntParam(args, p)
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
func OptionalBoolParamWithDefault(args map[string]any, p string, d bool) (bool, error) {
	_, ok := args[p]
	v, err := OptionalParam[bool](args, p)
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
func OptionalStringArrayParam(args map[string]any, p string) ([]string, error) {
	// Check if the parameter is present in the request
	if _, ok := args[p]; !ok {
		return []string{}, nil
	}

	switch v := args[p].(type) {
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
		return []string{}, fmt.Errorf("parameter %s could not be coerced to []string, is %T", p, args[p])
	}
}

func convertStringSliceToBigIntSlice(s []string) ([]int64, error) {
	int64Slice := make([]int64, len(s))
	for i, str := range s {
		val, err := convertStringToBigInt(str, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to convert element %d (%s) to int64: %w", i, str, err)
		}
		int64Slice[i] = val
	}
	return int64Slice, nil
}

func convertStringToBigInt(s string, def int64) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def, fmt.Errorf("failed to convert string %s to int64: %w", s, err)
	}
	return v, nil
}

// OptionalBigIntArrayParam is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns an empty slice
// 2. If it is present, iterates the elements, checks each is a string, and converts them to int64 values
func OptionalBigIntArrayParam(args map[string]any, p string) ([]int64, error) {
	// Check if the parameter is present in the request
	if _, ok := args[p]; !ok {
		return []int64{}, nil
	}

	switch v := args[p].(type) {
	case nil:
		return []int64{}, nil
	case []string:
		return convertStringSliceToBigIntSlice(v)
	case []any:
		int64Slice := make([]int64, len(v))
		for i, v := range v {
			s, ok := v.(string)
			if !ok {
				return []int64{}, fmt.Errorf("parameter %s is not of type string, is %T", p, v)
			}
			val, err := convertStringToBigInt(s, 0)
			if err != nil {
				return []int64{}, fmt.Errorf("parameter %s: failed to convert element %d (%s) to int64: %w", p, i, s, err)
			}
			int64Slice[i] = val
		}
		return int64Slice, nil
	default:
		return []int64{}, fmt.Errorf("parameter %s could not be coerced to []int64, is %T", p, args[p])
	}
}

// WithPagination adds REST API pagination parameters to a tool.
// https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api
func WithPagination(schema *jsonschema.Schema) *jsonschema.Schema {
	schema.Properties["page"] = &jsonschema.Schema{
		Type:        "number",
		Description: "Page number for pagination (min 1)",
		Minimum:     jsonschema.Ptr(1.0),
	}

	schema.Properties["perPage"] = &jsonschema.Schema{
		Type:        "number",
		Description: "Results per page for pagination (min 1, max 100)",
		Minimum:     jsonschema.Ptr(1.0),
		Maximum:     jsonschema.Ptr(100.0),
	}

	return schema
}

// WithUnifiedPagination adds REST API pagination parameters to a tool.
// GraphQL tools will use this and convert page/perPage to GraphQL cursor parameters internally.
func WithUnifiedPagination(schema *jsonschema.Schema) *jsonschema.Schema {
	schema.Properties["page"] = &jsonschema.Schema{
		Type:        "number",
		Description: "Page number for pagination (min 1)",
		Minimum:     jsonschema.Ptr(1.0),
	}

	schema.Properties["perPage"] = &jsonschema.Schema{
		Type:        "number",
		Description: "Results per page for pagination (min 1, max 100)",
		Minimum:     jsonschema.Ptr(1.0),
		Maximum:     jsonschema.Ptr(100.0),
	}

	schema.Properties["after"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs.",
	}

	return schema
}

// WithCursorPagination adds only cursor-based pagination parameters to a tool (no page parameter).
func WithCursorPagination(schema *jsonschema.Schema) *jsonschema.Schema {
	schema.Properties["perPage"] = &jsonschema.Schema{
		Type:        "number",
		Description: "Results per page for pagination (min 1, max 100)",
		Minimum:     jsonschema.Ptr(1.0),
		Maximum:     jsonschema.Ptr(100.0),
	}

	schema.Properties["after"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Cursor for pagination. Use the endCursor from the previous page's PageInfo for GraphQL APIs.",
	}

	return schema
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
func OptionalPaginationParams(args map[string]any) (PaginationParams, error) {
	page, err := OptionalIntParamWithDefault(args, "page", 1)
	if err != nil {
		return PaginationParams{}, err
	}
	perPage, err := OptionalIntParamWithDefault(args, "perPage", 30)
	if err != nil {
		return PaginationParams{}, err
	}
	after, err := OptionalParam[string](args, "after")
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
func OptionalCursorPaginationParams(args map[string]any) (CursorPaginationParams, error) {
	perPage, err := OptionalIntParamWithDefault(args, "perPage", 30)
	if err != nil {
		return CursorPaginationParams{}, err
	}
	after, err := OptionalParam[string](args, "after")
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
		return utils.NewToolResultErrorFromErr("failed to marshal text result to json", err)
	}

	return utils.NewToolResultText(string(data))
}
