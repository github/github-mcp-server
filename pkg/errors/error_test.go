package errors

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v82/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubErrorTypes(t *testing.T) {
	t.Run("GitHubAPIError implements error interface", func(t *testing.T) {
		resp := &github.Response{Response: &http.Response{StatusCode: 404}}
		originalErr := fmt.Errorf("not found")

		apiErr := NewGitHubAPIError("test message", resp, originalErr)

		// Should implement error interface
		var err error = apiErr
		assert.Equal(t, "test message: not found", err.Error())
	})

	t.Run("GitHubAPIError supports Unwrap", func(t *testing.T) {
		originalErr := fmt.Errorf("not found")
		apiErr := NewGitHubAPIError("test message", nil, originalErr)

		assert.True(t, errors.Is(apiErr, originalErr))
	})

	t.Run("GitHubGraphQLError implements error interface", func(t *testing.T) {
		originalErr := fmt.Errorf("query failed")

		gqlErr := NewGitHubGraphQLError("test message", originalErr)

		// Should implement error interface
		var err error = gqlErr
		assert.Equal(t, "test message: query failed", err.Error())
	})

	t.Run("GitHubGraphQLError supports Unwrap", func(t *testing.T) {
		originalErr := fmt.Errorf("query failed")
		gqlErr := NewGitHubGraphQLError("test message", originalErr)

		assert.True(t, errors.Is(gqlErr, originalErr))
	})

	t.Run("GitHubRawAPIError implements error interface", func(t *testing.T) {
		resp := &http.Response{StatusCode: 500}
		originalErr := fmt.Errorf("server error")

		rawErr := NewGitHubRawAPIError("test message", resp, originalErr)

		var err error = rawErr
		assert.Equal(t, "test message: server error", err.Error())
	})

	t.Run("GitHubRawAPIError supports Unwrap", func(t *testing.T) {
		originalErr := fmt.Errorf("server error")
		rawErr := NewGitHubRawAPIError("test message", nil, originalErr)

		assert.True(t, errors.Is(rawErr, originalErr))
	})
}

func TestNewGitHubAPIErrorResponse(t *testing.T) {
	t.Run("creates CallToolResult with error set via SetError", func(t *testing.T) {
		resp := &github.Response{Response: &http.Response{StatusCode: 422}}
		originalErr := fmt.Errorf("validation failed")

		result := NewGitHubAPIErrorResponse(context.Background(), "API call failed", resp, originalErr)

		require.NotNil(t, result)
		assert.True(t, result.IsError)

		// The typed error should be accessible via GetError
		gotErr := result.GetError()
		require.NotNil(t, gotErr)

		var apiErr *GitHubAPIError
		require.True(t, errors.As(gotErr, &apiErr))
		assert.Equal(t, "API call failed", apiErr.Message)
		assert.Equal(t, resp, apiErr.Response)
		assert.Equal(t, originalErr, apiErr.Err)
	})

	t.Run("error text is set in content", func(t *testing.T) {
		resp := &github.Response{Response: &http.Response{StatusCode: 404}}
		originalErr := fmt.Errorf("not found")

		result := NewGitHubAPIErrorResponse(context.Background(), "failed to get issue", resp, originalErr)

		require.NotNil(t, result)
		assert.True(t, result.IsError)
		// SetError sets content to err.Error()
		assert.Equal(t, "failed to get issue: not found", result.GetError().Error())
	})
}

func TestNewGitHubGraphQLErrorResponse(t *testing.T) {
	t.Run("creates CallToolResult with error set via SetError", func(t *testing.T) {
		originalErr := fmt.Errorf("mutation failed")

		result := NewGitHubGraphQLErrorResponse(context.Background(), "GraphQL call failed", originalErr)

		require.NotNil(t, result)
		assert.True(t, result.IsError)

		// The typed error should be accessible via GetError
		gotErr := result.GetError()
		require.NotNil(t, gotErr)

		var gqlErr *GitHubGraphQLError
		require.True(t, errors.As(gotErr, &gqlErr))
		assert.Equal(t, "GraphQL call failed", gqlErr.Message)
		assert.Equal(t, originalErr, gqlErr.Err)
	})
}

func TestNewGitHubRawAPIErrorResponse(t *testing.T) {
	t.Run("creates CallToolResult with error set via SetError", func(t *testing.T) {
		resp := &http.Response{StatusCode: 500}
		originalErr := fmt.Errorf("server error")

		result := NewGitHubRawAPIErrorResponse(context.Background(), "raw API failed", resp, originalErr)

		require.NotNil(t, result)
		assert.True(t, result.IsError)

		// The typed error should be accessible via GetError
		gotErr := result.GetError()
		require.NotNil(t, gotErr)

		var rawErr *GitHubRawAPIError
		require.True(t, errors.As(gotErr, &rawErr))
		assert.Equal(t, "raw API failed", rawErr.Message)
		assert.Equal(t, resp, rawErr.Response)
		assert.Equal(t, originalErr, rawErr.Err)
	})
}

func TestNewGitHubAPIStatusErrorResponse(t *testing.T) {
	t.Run("creates MCP error result from status code", func(t *testing.T) {
		resp := &github.Response{Response: &http.Response{StatusCode: 422}}
		body := []byte(`{"message": "Validation Failed"}`)

		result := NewGitHubAPIStatusErrorResponse(context.Background(), "failed to create issue", resp, body)

		require.NotNil(t, result)
		assert.True(t, result.IsError)

		// The typed error should be accessible via GetError
		gotErr := result.GetError()
		require.NotNil(t, gotErr)

		var apiErr *GitHubAPIError
		require.True(t, errors.As(gotErr, &apiErr))
		assert.Equal(t, "failed to create issue", apiErr.Message)
		assert.Equal(t, resp, apiErr.Response)
		// The synthetic error should contain the status code and body
		assert.Contains(t, apiErr.Err.Error(), "unexpected status 422")
		assert.Contains(t, apiErr.Err.Error(), "Validation Failed")
	})
}

func TestErrorTypesCanBeExtractedWithErrorsAs(t *testing.T) {
	t.Run("GitHubAPIError can be extracted from CallToolResult", func(t *testing.T) {
		resp := &github.Response{Response: &http.Response{StatusCode: 403}}
		originalErr := fmt.Errorf("forbidden")

		result := NewGitHubAPIErrorResponse(context.Background(), "access denied", resp, originalErr)

		gotErr := result.GetError()
		require.NotNil(t, gotErr)

		// Can extract typed error
		var apiErr *GitHubAPIError
		require.True(t, errors.As(gotErr, &apiErr))
		assert.Equal(t, 403, apiErr.Response.StatusCode)

		// Can also unwrap to original error
		assert.True(t, errors.Is(gotErr, originalErr))
	})

	t.Run("GitHubGraphQLError can be extracted from CallToolResult", func(t *testing.T) {
		originalErr := fmt.Errorf("invalid query")

		result := NewGitHubGraphQLErrorResponse(context.Background(), "query failed", originalErr)

		gotErr := result.GetError()
		require.NotNil(t, gotErr)

		var gqlErr *GitHubGraphQLError
		require.True(t, errors.As(gotErr, &gqlErr))
		assert.Equal(t, "query failed", gqlErr.Message)
		assert.True(t, errors.Is(gotErr, originalErr))
	})

	t.Run("GitHubRawAPIError can be extracted from CallToolResult", func(t *testing.T) {
		resp := &http.Response{StatusCode: 500}
		originalErr := fmt.Errorf("internal server error")

		result := NewGitHubRawAPIErrorResponse(context.Background(), "raw call failed", resp, originalErr)

		gotErr := result.GetError()
		require.NotNil(t, gotErr)

		var rawErr *GitHubRawAPIError
		require.True(t, errors.As(gotErr, &rawErr))
		assert.Equal(t, 500, rawErr.Response.StatusCode)
		assert.True(t, errors.Is(gotErr, originalErr))
	})
}
