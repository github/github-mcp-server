package github

import (
	"fmt"
	"math"
	"testing"

	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_IsAcceptedError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectAccepted bool
	}{
		{
			name:           "github AcceptedError",
			err:            &github.AcceptedError{},
			expectAccepted: true,
		},
		{
			name:           "regular error",
			err:            fmt.Errorf("some other error"),
			expectAccepted: false,
		},
		{
			name:           "nil error",
			err:            nil,
			expectAccepted: false,
		},
		{
			name:           "wrapped AcceptedError",
			err:            fmt.Errorf("wrapped: %w", &github.AcceptedError{}),
			expectAccepted: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isAcceptedError(tc.err)
			assert.Equal(t, tc.expectAccepted, result)
		})
	}
}

func Test_RequiredStringParam(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		paramName   string
		expected    string
		expectError bool
	}{
		{
			name:        "valid string parameter",
			params:      map[string]any{"name": "test-value"},
			paramName:   "name",
			expected:    "test-value",
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]any{},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty string parameter",
			params:      map[string]any{"name": ""},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]any{"name": 123},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := RequiredParam[string](tc.params, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func Test_OptionalStringParam(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		paramName   string
		expected    string
		expectError bool
	}{
		{
			name:        "valid string parameter",
			params:      map[string]any{"name": "test-value"},
			paramName:   "name",
			expected:    "test-value",
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]any{},
			paramName:   "name",
			expected:    "",
			expectError: false,
		},
		{
			name:        "empty string parameter",
			params:      map[string]any{"name": ""},
			paramName:   "name",
			expected:    "",
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]any{"name": 123},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := OptionalParam[string](tc.params, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestOptionalNullableStringParam(t *testing.T) {
	tests := []struct {
		name         string
		params       map[string]any
		want         string
		wantProvided bool
		wantError    string
	}{
		{name: "omitted", params: map[string]any{}},
		{name: "null", params: map[string]any{"type": nil}, wantProvided: true},
		{name: "string", params: map[string]any{"type": "Bug"}, want: "Bug", wantProvided: true},
		{name: "empty", params: map[string]any{"type": ""}, wantProvided: true, wantError: "parameter type must not be empty"},
		{name: "wrong type", params: map[string]any{"type": float64(1)}, wantProvided: true, wantError: "parameter type is not of type string or null, is float64"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, provided, err := OptionalNullableStringParam(tc.params, "type")

			assert.Equal(t, tc.wantProvided, provided)
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			if tc.want == "" {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tc.want, *got)
			}
		})
	}
}

func Test_inSignedIntRange(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		bitSize int
		want    bool
	}{
		{"int64 min", math.MinInt64, 64, true},
		{"largest float64 below 2^63", math.Nextafter(math.Ldexp(1, 63), 0), 64, true},
		{"2^63", math.Ldexp(1, 63), 64, false},
		{"MaxInt64 rounds up to 2^63", float64(math.MaxInt64), 64, false},
		{"below int64 min", math.Nextafter(math.MinInt64, math.Inf(-1)), 64, false},
		{"int32 max", math.MaxInt32, 32, true},
		{"2^31", math.MaxInt32 + 1, 32, false},
		{"int32 min", math.MinInt32, 32, true},
		{"below int32 min", math.MinInt32 - 1, 32, false},
		{"NaN", math.NaN(), 64, false},
		{"positive infinity", math.Inf(1), 64, false},
		{"negative infinity", math.Inf(-1), 64, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, inSignedIntRange(tc.value, tc.bitSize))
		})
	}
}

func Test_RequiredBigInt(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		expected    int64
		expectedErr string
	}{
		{"valid number", float64(42), 42, ""},
		{"valid string", "42", 42, ""},
		{"MinInt64 string", "-9223372036854775808", math.MinInt64, ""},
		{"largest float64 below 2^63", math.Nextafter(math.Ldexp(1, 63), 0), 9223372036854774784, ""},
		{"2^63 string", "9223372036854775808", 0, "too large to fit in int64"},
		{"MaxInt64 string rounds up to 2^63", "9223372036854775807", 0, "too large to fit in int64"},
		{"2^63 float64", math.Ldexp(1, 63), 0, "too large to fit in int64"},
		{"below MinInt64", "-9223372036854777856", 0, "too large to fit in int64"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := RequiredBigInt(map[string]any{"id": tc.value}, "id")

			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_RequiredInt(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		paramName   string
		expected    int
		expectError bool
	}{
		{
			name:        "valid number parameter",
			params:      map[string]any{"count": float64(42)},
			paramName:   "count",
			expected:    42,
			expectError: false,
		},
		{
			name:        "valid string number parameter",
			params:      map[string]any{"count": "42"},
			paramName:   "count",
			expected:    42,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]any{},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "zero string parameter",
			params:      map[string]any{"count": "0"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]any{"count": "not-a-number"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "boolean type parameter",
			params:      map[string]any{"count": true},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "NaN string",
			params:      map[string]any{"count": "NaN"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "Inf string",
			params:      map[string]any{"count": "Inf"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "negative Inf string",
			params:      map[string]any{"count": "-Inf"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "fractional string",
			params:      map[string]any{"count": "1.5"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "fractional float64",
			params:      map[string]any{"count": float64(1.5)},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "NaN float64",
			params:      map[string]any{"count": math.NaN()},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "Inf float64",
			params:      map[string]any{"count": math.Inf(1)},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "MaxFloat64",
			params:      map[string]any{"count": math.MaxFloat64},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := RequiredInt(tc.params, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
func Test_OptionalIntParam(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		paramName   string
		expected    int
		expectError bool
	}{
		{
			name:        "valid number parameter",
			params:      map[string]any{"count": float64(42)},
			paramName:   "count",
			expected:    42,
			expectError: false,
		},
		{
			name:        "valid string number parameter",
			params:      map[string]any{"count": "42"},
			paramName:   "count",
			expected:    42,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]any{},
			paramName:   "count",
			expected:    0,
			expectError: false,
		},
		{
			name:        "zero value",
			params:      map[string]any{"count": float64(0)},
			paramName:   "count",
			expected:    0,
			expectError: false,
		},
		{
			name:        "zero string value",
			params:      map[string]any{"count": "0"},
			paramName:   "count",
			expected:    0,
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]any{"count": "not-a-number"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "NaN string",
			params:      map[string]any{"count": "NaN"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "fractional string",
			params:      map[string]any{"count": "1.5"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "fractional float64",
			params:      map[string]any{"count": float64(1.5)},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "2^63 string parameter",
			params:      map[string]any{"count": "9223372036854775808"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "MaxInt64 string parameter rounds up to 2^63",
			params:      map[string]any{"count": "9223372036854775807"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "2^63 float64 parameter",
			params:      map[string]any{"count": math.Ldexp(1, 63)},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "MinInt float64 parameter",
			params:      map[string]any{"count": float64(math.MinInt)},
			paramName:   "count",
			expected:    math.MinInt,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := OptionalIntParam(tc.params, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func Test_OptionalNumberParamWithDefault(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		paramName   string
		defaultVal  int
		expected    int
		expectError bool
	}{
		{
			name:        "valid number parameter",
			params:      map[string]any{"count": float64(42)},
			paramName:   "count",
			defaultVal:  10,
			expected:    42,
			expectError: false,
		},
		{
			name:        "valid string number parameter",
			params:      map[string]any{"count": "42"},
			paramName:   "count",
			defaultVal:  10,
			expected:    42,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]any{},
			paramName:   "count",
			defaultVal:  10,
			expected:    10,
			expectError: false,
		},
		{
			name:        "zero value",
			params:      map[string]any{"count": float64(0)},
			paramName:   "count",
			defaultVal:  10,
			expected:    10,
			expectError: false,
		},
		{
			name:        "zero string value uses default",
			params:      map[string]any{"count": "0"},
			paramName:   "count",
			defaultVal:  10,
			expected:    10,
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]any{"count": "not-a-number"},
			paramName:   "count",
			defaultVal:  10,
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := OptionalIntParamWithDefault(tc.params, tc.paramName, tc.defaultVal)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func Test_OptionalBooleanParam(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		paramName   string
		expected    bool
		expectError bool
	}{
		{
			name:        "true value",
			params:      map[string]any{"flag": true},
			paramName:   "flag",
			expected:    true,
			expectError: false,
		},
		{
			name:        "false value",
			params:      map[string]any{"flag": false},
			paramName:   "flag",
			expected:    false,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]any{},
			paramName:   "flag",
			expected:    false,
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]any{"flag": "not-a-boolean"},
			paramName:   "flag",
			expected:    false,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := OptionalParam[bool](tc.params, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestOptionalStringArrayParam(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		paramName   string
		expected    []string
		expectError bool
	}{
		{
			name:        "parameter not in request",
			params:      map[string]any{},
			paramName:   "flag",
			expected:    []string{},
			expectError: false,
		},
		{
			name: "valid any array parameter",
			params: map[string]any{
				"flag": []any{"v1", "v2"},
			},
			paramName:   "flag",
			expected:    []string{"v1", "v2"},
			expectError: false,
		},
		{
			name: "valid string array parameter",
			params: map[string]any{
				"flag": []string{"v1", "v2"},
			},
			paramName:   "flag",
			expected:    []string{"v1", "v2"},
			expectError: false,
		},
		{
			name: "wrong type parameter",
			params: map[string]any{
				"flag": 1,
			},
			paramName:   "flag",
			expected:    []string{},
			expectError: true,
		},
		{
			name: "wrong slice type parameter",
			params: map[string]any{
				"flag": []any{"foo", 2},
			},
			paramName:   "flag",
			expected:    []string{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := OptionalStringArrayParam(tc.params, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestOptionalPaginationParams(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		expected    PaginationParams
		expectError bool
	}{
		{
			name:   "no pagination parameters, default values",
			params: map[string]any{},
			expected: PaginationParams{
				Page:    1,
				PerPage: 30,
			},
			expectError: false,
		},
		{
			name: "page parameter, default perPage",
			params: map[string]any{
				"page": float64(2),
			},
			expected: PaginationParams{
				Page:    2,
				PerPage: 30,
			},
			expectError: false,
		},
		{
			name: "perPage parameter, default page",
			params: map[string]any{
				"perPage": float64(50),
			},
			expected: PaginationParams{
				Page:    1,
				PerPage: 50,
			},
			expectError: false,
		},
		{
			name: "page and perPage parameters",
			params: map[string]any{
				"page":    float64(2),
				"perPage": float64(50),
			},
			expected: PaginationParams{
				Page:    2,
				PerPage: 50,
			},
			expectError: false,
		},
		{
			name: "invalid page parameter",
			params: map[string]any{
				"page": "not-a-number",
			},
			expected:    PaginationParams{},
			expectError: true,
		},
		{
			name: "invalid perPage parameter",
			params: map[string]any{
				"perPage": "not-a-number",
			},
			expected:    PaginationParams{},
			expectError: true,
		},
		{
			name: "string page and perPage parameters",
			params: map[string]any{
				"page":    "3",
				"perPage": "25",
			},
			expected: PaginationParams{
				Page:    3,
				PerPage: 25,
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := OptionalPaginationParams(tc.params)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestCursorPaginationParams_ToGraphQLParams(t *testing.T) {
	ptr := func(v int32) *int32 { return &v }
	strPtr := func(v string) *string { return &v }

	tests := []struct {
		name        string
		params      CursorPaginationParams
		expected    *GraphQLPaginationParams
		expectError bool
		errMsg      string
	}{
		{
			name:     "perPage 1",
			params:   CursorPaginationParams{PerPage: 1},
			expected: &GraphQLPaginationParams{First: ptr(1)},
		},
		{
			name:     "perPage 100",
			params:   CursorPaginationParams{PerPage: 100},
			expected: &GraphQLPaginationParams{First: ptr(100)},
		},
		{
			name:     "perPage 50 with after",
			params:   CursorPaginationParams{PerPage: 50, After: "cursor123"},
			expected: &GraphQLPaginationParams{First: ptr(50), After: strPtr("cursor123")},
		},
		{
			name:        "perPage 0",
			params:      CursorPaginationParams{PerPage: 0},
			expectError: true,
			errMsg:      "perPage value 0 is below minimum of 1",
		},
		{
			name:        "perPage -1",
			params:      CursorPaginationParams{PerPage: -1},
			expectError: true,
			errMsg:      "perPage value -1 is below minimum of 1",
		},
		{
			name:        "perPage 101",
			params:      CursorPaginationParams{PerPage: 101},
			expectError: true,
			errMsg:      "perPage value 101 exceeds maximum of 100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.params.ToGraphQLParams()
			if tc.expectError {
				require.Error(t, err)
				assert.Equal(t, tc.errMsg, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
