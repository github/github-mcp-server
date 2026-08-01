package github

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
)

// OptionalParamOK is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It 返回 值, 一个boolean indicating 如果参数 was present, 和一个错误 如果type is wrong.
func OptionalParamOK[T any, A map[string]any](args A, p string) (value T, ok bool, err error) {
	// Check 如果参数 is present 在请求
	val, exists := args[p]
	if !exists {
		// 不present, 返回 zero 值, 假, no 错误
		return
	}

	// Check 如果参数 is 的expected type
	value, ok = val.(T)
	if !ok {
		// Present 但wrong type
		err = fmt.Errorf("parameter %s is not of type %T, is %T", p, value, val)
		ok = true // Set ok to 真 因为the 参数 *was* present, even if wrong type
		return
	}

	// Present 和correct type
	ok = true
	return
}

// isAcceptedErr或检查s 如果错误 is 一个accepted 错误.
func isAcceptedError(err error) bool {
	var acceptedError *github.AcceptedError
	return errors.As(err, &acceptedError)
}

// toInt converts 一个值 to int, handling both float64 和string representations.
// Some MCP 客户端s send numeric 值 as strings. It rejects NaN, ±Inf,
// fractional 值, 和值 outside int 范围.
func toInt(val any) (int, error) {
	var f float64
	switch v := val.(type) {
	case float64:
		f = v
	case string:
		var err error
		f, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric value: %s", v)
		}
	default:
		return 0, fmt.Errorf("expected number, got %T", val)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("non-finite numeric value")
	}
	if f != math.Trunc(f) {
		return 0, fmt.Errorf("non-integer numeric value: %v", f)
	}
	if f > math.MaxInt || f < math.MinInt {
		return 0, fmt.Errorf("numeric value out of int range: %v", f)
	}
	return int(f), nil
}

// toInt64 converts 一个值 to int64, handling both float64 和string representations.
// Some MCP 客户端s send numeric 值 as strings. It rejects NaN, ±Inf,
// fractional 值, 和值 that lose precision 在float64→int64 conversion.
func toInt64(val any) (int64, error) {
	var f float64
	switch v := val.(type) {
	case float64:
		f = v
	case string:
		var err error
		f, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric value: %s", v)
		}
	default:
		return 0, fmt.Errorf("expected number, got %T", val)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("non-finite numeric value")
	}
	if f != math.Trunc(f) {
		return 0, fmt.Errorf("non-integer numeric value: %v", f)
	}
	result := int64(f)
	// Check round-trip to detect precision loss f或large int64 值
	if float64(result) != f {
		return 0, fmt.Errorf("numeric value %v is too large to fit in int64", f)
	}
	return result, nil
}

// RequiredParam is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It does following 检查s:
// 1. Checks 如果参数 is present 在请求.
// 2. Checks 如果参数 is 的expected type.
// 3. Checks 如果参数 is 不空, i.e: non-zero 值
func RequiredParam[T comparable](args map[string]any, p string) (T, error) {
	var zero T

	// Check 如果参数 is present 在请求
	if _, ok := args[p]; !ok {
		return zero, fmt.Errorf("missing required parameter: %s", p)
	}

	// Check 如果参数 is 的expected type
	val, ok := args[p].(T)
	if !ok {
		return zero, fmt.Errorf("parameter %s is not of type %T", p, zero)
	}

	if val == zero {
		return zero, fmt.Errorf("missing required parameter: %s", p)
	}

	return val, nil
}

// RequiredInt is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It does following 检查s:
// 1. Checks 如果参数 is present 在请求.
// 2. Checks 如果参数 is 的expected type (float64 或numeric string).
// 3. Checks 如果参数 is 不空, i.e: non-zero 值
func RequiredInt(args map[string]any, p string) (int, error) {
	v, ok := args[p]
	if !ok {
		return 0, fmt.Errorf("missing required parameter: %s", p)
	}

	result, err := toInt(v)
	if err != nil {
		return 0, fmt.Errorf("parameter %s is not a valid number: %w", p, err)
	}

	if result == 0 {
		return 0, fmt.Errorf("missing required parameter: %s", p)
	}

	return result, nil
}

// RequiredBigInt is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It does following 检查s:
// 1. Checks 如果参数 is present 在请求.
// 2. Checks 如果参数 is 的expected type (float64 或numeric string).
// 3. Checks 如果参数 is 不空, i.e: non-zero 值.
// 4. Validates that float64 值 可以 safely converted to int64 without truncation.
func RequiredBigInt(args map[string]any, p string) (int64, error) {
	val, ok := args[p]
	if !ok {
		return 0, fmt.Errorf("missing required parameter: %s", p)
	}

	result, err := toInt64(val)
	if err != nil {
		return 0, fmt.Errorf("parameter %s is not a valid number: %w", p, err)
	}

	if result == 0 {
		return 0, fmt.Errorf("missing required parameter: %s", p)
	}

	return result, nil
}

// OptionalParam is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It does following 检查s:
// 1. Checks 如果参数 is present 在请求, if not, it 返回 its zero-值
// 2. If it is present, it 检查s 如果参数 is 的expected type 和返回 it
func OptionalParam[T any](args map[string]any, p string) (T, error) {
	var zero T

	// Check 如果参数 is present 在请求
	if _, ok := args[p]; !ok {
		return zero, nil
	}

	// Check 如果参数 is 的expected type
	if _, ok := args[p].(T); !ok {
		return zero, fmt.Errorf("parameter %s is not of type %T, is %T", p, zero, args[p])
	}

	return args[p].(T), nil
}

// OptionalIntParam is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It does following 检查s:
// 1. Checks 如果参数 is present 在请求, if not, it 返回 its zero-值
// 2. If it is present, it 检查s 如果参数 is 的expected type (float64 或numeric string) 和返回 it
func OptionalIntParam(args map[string]any, p string) (int, error) {
	val, ok := args[p]
	if !ok {
		return 0, nil
	}

	result, err := toInt(val)
	if err != nil {
		return 0, fmt.Errorf("parameter %s is not a valid number: %w", p, err)
	}

	return result, nil
}

// OptionalIntParamWith默认is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求
// similar to 可选IntParam, 但it 也takes 一个默认值.
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

// OptionalBoolParamWith默认is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求
// similar to 可选BoolParam, 但it 也takes 一个默认值.
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

// OptionalStringArrayParam is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It does following 检查s:
// 1. Checks 如果参数 is present 在请求, if not, it 返回 its zero-值
// 2. If it is present, iterates elements 和检查s 每个is 一个string
func OptionalStringArrayParam(args map[string]any, p string) ([]string, error) {
	// Check 如果参数 is present 在请求
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

// OptionalBigIntArrayParam is 一个helper 函数 that 可以 用于 fetch 一个请求ed 参数 来自请求.
// It does following 检查s:
// 1. Checks 如果参数 is present 在请求, if not, it 返回 一个空 slice
// 2. If it is present, iterates elements, 检查s 每个is 一个string, 和converts them to int64 值
func OptionalBigIntArrayParam(args map[string]any, p string) ([]int64, error) {
	// Check 如果参数 is present 在请求
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

// WithPagination adds REST API pagination 参数 to 一个工具.
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

// WithUnifiedPagination adds REST API pagination 参数 to 一个工具.
// GraphQL 工具 will use this 和convert 页/perPage to GraphQL curs或参数 internally.
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

// WithCursorPagination adds 仅cursor-based pagination 参数 to 一个工具 (no 页 参数).
func WithCursorPagination(schema *jsonschema.Schema) *jsonschema.Schema {
	schema.Properties["perPage"] = &jsonschema.Schema{
		Type:        "number",
		Description: "Results per page for pagination (min 1, max 100)",
		Minimum:     jsonschema.Ptr(1.0),
		Maximum:     jsonschema.Ptr(100.0),
	}

	schema.Properties["after"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Cursor for pagination. Use the cursor from the previous response.",
	}

	return schema
}

type PaginationParams struct {
	Page    int
	PerPage int
	After   string
}

// OptionalPaginationParams 返回 "页", "perPage", 和"after" 参数 来自请求,
// 或their 默认值 if 不present, "页" 默认is 1, "perPage" 默认is 30.
// In future, we may want to make 默认值 configurable, 或even have this
// 函数 返回ed from `withPagination`, 其中defaults are provided alongside
// min/max 值.
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

// OptionalCursorPaginationParams 返回 "perPage" 和"after" 参数 来自请求,
// without "页" 参数, suitable f或cursor-based pagination only.
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

type pageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	NextCursor      string `json:"nextCursor,omitempty"`
	PrevCursor      string `json:"prevCursor,omitempty"`
}

func buildPageInfo(resp *github.Response) pageInfo {
	return pageInfo{
		HasNextPage:     resp.After != "",
		HasPreviousPage: resp.Before != "",
		NextCursor:      resp.After,
		PrevCursor:      resp.Before,
	}
}

// ToGraphQLParams converts curs或pagination 参数 to GraphQL-specific 参数.
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

// ToGraphQLParams converts REST API pagination 参数 to GraphQL-specific 参数.
// 此converts 页/perPage to 第一个 参数 f或GraphQL queries.
// If After is provided, it takes precedence over 页-based pagination.
func (p PaginationParams) ToGraphQLParams() (*GraphQLPaginationParams, error) {
	// Convert to CursorPaginationParams 和delegate to avoid duplication
	cursor := CursorPaginationParams{
		PerPage: p.PerPage,
		After:   p.After,
	}
	return cursor.ToGraphQLParams()
}
