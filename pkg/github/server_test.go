package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/google/go-github/v79/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
)

func stubGetClientFn(client *github.Client) GetClientFn {
	return func(_ context.Context) (*github.Client, error) {
		return client, nil
	}
}

func stubGetClientFromHTTPFn(client *http.Client) GetClientFn {
	return func(_ context.Context) (*github.Client, error) {
		return github.NewClient(client), nil
	}
}

func stubGetClientFnErr(err string) GetClientFn {
	return func(_ context.Context) (*github.Client, error) {
		return nil, errors.New(err)
	}
}

func stubGetGQLClientFn(client *githubv4.Client) GetGQLClientFn {
	return func(_ context.Context) (*githubv4.Client, error) {
		return client, nil
	}
}

func stubRepoAccessCache(client *githubv4.Client, ttl time.Duration) *lockdown.RepoAccessCache {
	cacheName := fmt.Sprintf("repo-access-cache-test-%d", time.Now().UnixNano())
	return lockdown.GetInstance(client, lockdown.WithTTL(ttl), lockdown.WithCacheName(cacheName))
}

func stubFeatureFlags(enabledFlags map[string]bool) FeatureFlags {
	return FeatureFlags{
		LockdownMode: enabledFlags["lockdown-mode"],
	}
}

func stubGetRawClientFn(client *raw.Client) raw.GetRawClientFn {
	return func(_ context.Context) (*raw.Client, error) {
		return client, nil
	}
}

func badRequestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		structuredErrorResponse := github.ErrorResponse{
			Message: msg,
		}

		b, err := json.Marshal(structuredErrorResponse)
		if err != nil {
			http.Error(w, "failed to marshal error response", http.StatusInternalServerError)
		}

		http.Error(w, string(b), http.StatusBadRequest)
	}
}

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
		params      map[string]interface{}
		paramName   string
		expected    string
		expectError bool
	}{
		{
			name:        "valid string parameter",
			params:      map[string]interface{}{"name": "test-value"},
			paramName:   "name",
			expected:    "test-value",
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]interface{}{},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty string parameter",
			params:      map[string]interface{}{"name": ""},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]interface{}{"name": 123},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := RequiredParam[string](request, tc.paramName)

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
		params      map[string]interface{}
		paramName   string
		expected    string
		expectError bool
	}{
		{
			name:        "valid string parameter",
			params:      map[string]interface{}{"name": "test-value"},
			paramName:   "name",
			expected:    "test-value",
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]interface{}{},
			paramName:   "name",
			expected:    "",
			expectError: false,
		},
		{
			name:        "empty string parameter",
			params:      map[string]interface{}{"name": ""},
			paramName:   "name",
			expected:    "",
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]interface{}{"name": 123},
			paramName:   "name",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := OptionalParam[string](request, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func Test_RequiredInt(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		paramName   string
		expected    int
		expectError bool
	}{
		{
			name:        "valid number parameter",
			params:      map[string]interface{}{"count": float64(42)},
			paramName:   "count",
			expected:    42,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]interface{}{},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]interface{}{"count": "not-a-number"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := RequiredInt(request, tc.paramName)

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
		params      map[string]interface{}
		paramName   string
		expected    int
		expectError bool
	}{
		{
			name:        "valid number parameter",
			params:      map[string]interface{}{"count": float64(42)},
			paramName:   "count",
			expected:    42,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]interface{}{},
			paramName:   "count",
			expected:    0,
			expectError: false,
		},
		{
			name:        "zero value",
			params:      map[string]interface{}{"count": float64(0)},
			paramName:   "count",
			expected:    0,
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]interface{}{"count": "not-a-number"},
			paramName:   "count",
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := OptionalIntParam(request, tc.paramName)

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
		params      map[string]interface{}
		paramName   string
		defaultVal  int
		expected    int
		expectError bool
	}{
		{
			name:        "valid number parameter",
			params:      map[string]interface{}{"count": float64(42)},
			paramName:   "count",
			defaultVal:  10,
			expected:    42,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]interface{}{},
			paramName:   "count",
			defaultVal:  10,
			expected:    10,
			expectError: false,
		},
		{
			name:        "zero value",
			params:      map[string]interface{}{"count": float64(0)},
			paramName:   "count",
			defaultVal:  10,
			expected:    10,
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]interface{}{"count": "not-a-number"},
			paramName:   "count",
			defaultVal:  10,
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := OptionalIntParamWithDefault(request, tc.paramName, tc.defaultVal)

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
		params      map[string]interface{}
		paramName   string
		expected    bool
		expectError bool
	}{
		{
			name:        "true value",
			params:      map[string]interface{}{"flag": true},
			paramName:   "flag",
			expected:    true,
			expectError: false,
		},
		{
			name:        "false value",
			params:      map[string]interface{}{"flag": false},
			paramName:   "flag",
			expected:    false,
			expectError: false,
		},
		{
			name:        "missing parameter",
			params:      map[string]interface{}{},
			paramName:   "flag",
			expected:    false,
			expectError: false,
		},
		{
			name:        "wrong type parameter",
			params:      map[string]interface{}{"flag": "not-a-boolean"},
			paramName:   "flag",
			expected:    false,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := OptionalParam[bool](request, tc.paramName)

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
		params      map[string]interface{}
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
			request := createMCPRequest(tc.params)
			result, err := OptionalStringArrayParam(request, tc.paramName)

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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := OptionalPaginationParams(request)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
