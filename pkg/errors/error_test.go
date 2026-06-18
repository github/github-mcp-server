package errors

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubErrorContext(t *testing.T) {
	t.Run("API errors can be added to context and retrieved", func(t *testing.T) {
		// Given a context with GitHub error tracking enabled
		ctx := ContextWithGitHubErrors(context.Background())

		// Create a mock GitHub response
		resp := &github.Response{
			Response: &http.Response{
				StatusCode: 404,
				Status:     "404 Not Found",
			},
		}
		originalErr := fmt.Errorf("resource not found")

		// When we add an API error to the context
		updatedCtx, err := NewGitHubAPIErrorToCtx(ctx, "failed to fetch resource", resp, originalErr)
		require.NoError(t, err)

		// Then we should be able to retrieve the error from the updated context
		apiErrors, err := GetGitHubAPIErrors(updatedCtx)
		require.NoError(t, err)
		require.Len(t, apiErrors, 1)

		apiError := apiErrors[0]
		assert.Equal(t, "failed to fetch resource", apiError.Message)
		assert.Equal(t, resp, apiError.Response)
		assert.Equal(t, originalErr, apiError.Err)
		assert.Equal(t, "failed to fetch resource: resource not found", apiError.Error())
	})

	t.Run("GraphQL errors can be added to context and retrieved", func(t *testing.T) {
		// Given a context with GitHub error tracking enabled
		ctx := ContextWithGitHubErrors(context.Background())

		originalErr := fmt.Errorf("GraphQL query failed")

		// When we add a GraphQL error to the context
		graphQLErr := newGitHubGraphQLError("failed to execute mutation", originalErr)
		updatedCtx, err := addGitHubGraphQLErrorToContext(ctx, graphQLErr)
		require.NoError(t, err)

		// Then we should be able to retrieve the error from the updated context
		gqlErrors, err := GetGitHubGraphQLErrors(updatedCtx)
		require.NoError(t, err)
		require.Len(t, gqlErrors, 1)

		gqlError := gqlErrors[0]
		assert.Equal(t, "failed to execute mutation", gqlError.Message)
		assert.Equal(t, originalErr, gqlError.Err)
		assert.Equal(t, "failed to execute mutation: GraphQL query failed", gqlError.Error())
	})

	t.Run("Raw API errors can be added to context and retrieved", func(t *testing.T) {
		// Given a context with GitHub error tracking enabled
		ctx := ContextWithGitHubErrors(context.Background())

		// Create a mock HTTP response
		resp := &http.Response{
			StatusCode: 404,
			Status:     "404 Not Found",
		}
		originalErr := fmt.Errorf("raw content not found")

		// When we add a raw API error to the context
		rawAPIErr := newGitHubRawAPIError("failed to fetch raw content", resp, originalErr)
		updatedCtx, err := addRawAPIErrorToContext(ctx, rawAPIErr)
		require.NoError(t, err)

		// Then we should be able to retrieve the error from the updated context
		rawErrors, err := GetGitHubRawAPIErrors(updatedCtx)
		require.NoError(t, err)
		require.Len(t, rawErrors, 1)

		rawError := rawErrors[0]
		assert.Equal(t, "failed to fetch raw content", rawError.Message)
		assert.Equal(t, resp, rawError.Response)
		assert.Equal(t, originalErr, rawError.Err)
	})

	t.Run("multiple errors can be accumulated in context", func(t *testing.T) {
		// Given a context with GitHub error tracking enabled
		ctx := ContextWithGitHubErrors(context.Background())

		// When we add multiple API errors
		resp1 := &github.Response{Response: &http.Response{StatusCode: 404}}
		resp2 := &github.Response{Response: &http.Response{StatusCode: 403}}

		ctx, err := NewGitHubAPIErrorToCtx(ctx, "first error", resp1, fmt.Errorf("not found"))
		require.NoError(t, err)

		ctx, err = NewGitHubAPIErrorToCtx(ctx, "second error", resp2, fmt.Errorf("forbidden"))
		require.NoError(t, err)

		// And add a GraphQL error
		gqlErr := newGitHubGraphQLError("graphql error", fmt.Errorf("query failed"))
		ctx, err = addGitHubGraphQLErrorToContext(ctx, gqlErr)
		require.NoError(t, err)

		// And add a raw API error
		rawErr := newGitHubRawAPIError("raw error", &http.Response{StatusCode: 404}, fmt.Errorf("not found"))
		ctx, err = addRawAPIErrorToContext(ctx, rawErr)
		require.NoError(t, err)

		// Then we should be able to retrieve all errors
		apiErrors, err := GetGitHubAPIErrors(ctx)
		require.NoError(t, err)
		assert.Len(t, apiErrors, 2)

		gqlErrors, err := GetGitHubGraphQLErrors(ctx)
		require.NoError(t, err)
		assert.Len(t, gqlErrors, 1)

		rawErrors, err := GetGitHubRawAPIErrors(ctx)
		require.NoError(t, err)
		assert.Len(t, rawErrors, 1)

		// Verify error details
		assert.Equal(t, "first error", apiErrors[0].Message)
		assert.Equal(t, "second error", apiErrors[1].Message)
		assert.Equal(t, "graphql error", gqlErrors[0].Message)
		assert.Equal(t, "raw error", rawErrors[0].Message)
	})

	t.Run("context pointer sharing allows middleware to inspect errors without context propagation", func(t *testing.T) {
		// This test demonstrates the key behavior: even when the context itself
		// isn't propagated through function calls, the pointer to the error slice
		// is shared, allowing middleware to inspect errors that were added later.

		// Given a context with GitHub error tracking enabled
		originalCtx := ContextWithGitHubErrors(context.Background())

		// Simulate a middleware that captures the context early
		var middlewareCtx context.Context

		// Middleware function that captures the context
		middleware := func(ctx context.Context) {
			middlewareCtx = ctx // Middleware saves the context reference
		}

		// Call middleware with the original context
		middleware(originalCtx)

		// Simulate some business logic that adds errors to the context
		// but doesn't propagate the updated context back to middleware
		businessLogic := func(ctx context.Context) {
			resp := &github.Response{Response: &http.Response{StatusCode: 500}}

			// Add an error to the context (this modifies the shared pointer)
			_, err := NewGitHubAPIErrorToCtx(ctx, "business logic failed", resp, fmt.Errorf("internal error"))
			require.NoError(t, err)

			// Add another error
			_, err = NewGitHubAPIErrorToCtx(ctx, "second failure", resp, fmt.Errorf("another error"))
			require.NoError(t, err)
		}

		// Execute business logic - note that we don't propagate the returned context
		businessLogic(originalCtx)

		// Then the middleware should be able to see the errors that were added
		// even though it only has a reference to the original context
		apiErrors, err := GetGitHubAPIErrors(middlewareCtx)
		require.NoError(t, err)
		assert.Len(t, apiErrors, 2, "Middleware should see errors added after it captured the context")

		assert.Equal(t, "business logic failed", apiErrors[0].Message)
		assert.Equal(t, "second failure", apiErrors[1].Message)
	})

	t.Run("context without GitHub errors returns error", func(t *testing.T) {
		// Given a regular context without GitHub error tracking
		ctx := context.Background()

		// When we try to retrieve errors
		apiErrors, err := GetGitHubAPIErrors(ctx)

		// Then it should return an error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context does not contain GitHubCtxErrors")
		assert.Nil(t, apiErrors)

		// Same for GraphQL errors
		gqlErrors, err := GetGitHubGraphQLErrors(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context does not contain GitHubCtxErrors")
		assert.Nil(t, gqlErrors)

		// Same for raw API errors
		rawErrors, err := GetGitHubRawAPIErrors(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context does not contain GitHubCtxErrors")
		assert.Nil(t, rawErrors)
	})

	t.Run("ContextWithGitHubErrors resets existing errors", func(t *testing.T) {
		// Given a context with existing errors
		ctx := ContextWithGitHubErrors(context.Background())
		resp := &github.Response{Response: &http.Response{StatusCode: 404}}
		ctx, err := NewGitHubAPIErrorToCtx(ctx, "existing error", resp, fmt.Errorf("error"))
		require.NoError(t, err)

		// Add a raw API error too
		rawErr := newGitHubRawAPIError("existing raw error", &http.Response{StatusCode: 404}, fmt.Errorf("error"))
		ctx, err = addRawAPIErrorToContext(ctx, rawErr)
		require.NoError(t, err)

		// Verify errors exist
		apiErrors, err := GetGitHubAPIErrors(ctx)
		require.NoError(t, err)
		assert.Len(t, apiErrors, 1)

		rawErrors, err := GetGitHubRawAPIErrors(ctx)
		require.NoError(t, err)
		assert.Len(t, rawErrors, 1)

		// When we call ContextWithGitHubErrors again
		resetCtx := ContextWithGitHubErrors(ctx)

		// Then all errors should be cleared
		apiErrors, err = GetGitHubAPIErrors(resetCtx)
		require.NoError(t, err)
		assert.Len(t, apiErrors, 0, "API errors should be reset")

		rawErrors, err = GetGitHubRawAPIErrors(resetCtx)
		require.NoError(t, err)
		assert.Len(t, rawErrors, 0, "Raw API errors should be reset")
	})

	t.Run("NewGitHubAPIErrorResponse creates MCP error result and stores context error", func(t *testing.T) {
		// Given a context with GitHub error tracking enabled
		ctx := ContextWithGitHubErrors(context.Background())

		resp := &github.Response{Response: &http.Response{StatusCode: 422}}
		originalErr := fmt.Errorf("validation failed")

		// When we create an API error response
		result := NewGitHubAPIErrorResponse(ctx, "API call failed", resp, originalErr)

		// Then it should return an MCP error result
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		// And the error should be stored in the context
		apiErrors, err := GetGitHubAPIErrors(ctx)
		require.NoError(t, err)
		require.Len(t, apiErrors, 1)

		apiError := apiErrors[0]
		assert.Equal(t, "API call failed", apiError.Message)
		assert.Equal(t, resp, apiError.Response)
		assert.Equal(t, originalErr, apiError.Err)
	})

	t.Run("NewGitHubGraphQLErrorResponse creates MCP error result and stores context error", func(t *testing.T) {
		// Given a context with GitHub error tracking enabled
		ctx := ContextWithGitHubErrors(context.Background())

		originalErr := fmt.Errorf("mutation failed")

		// When we create a GraphQL error response
		result := NewGitHubGraphQLErrorResponse(ctx, "GraphQL call failed", originalErr)

		// Then it should return an MCP error result
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		// And the error should be stored in the context
		gqlErrors, err := GetGitHubGraphQLErrors(ctx)
		require.NoError(t, err)
		require.Len(t, gqlErrors, 1)

		gqlError := gqlErrors[0]
		assert.Equal(t, "GraphQL call failed", gqlError.Message)
		assert.Equal(t, originalErr, gqlError.Err)
	})

	t.Run("NewGitHubAPIStatusErrorResponse creates MCP error result from status code", func(t *testing.T) {
		// Given a context with GitHub error tracking enabled
		ctx := ContextWithGitHubErrors(context.Background())

		resp := &github.Response{Response: &http.Response{StatusCode: 422}}
		body := []byte(`{"message": "Validation Failed"}`)

		// When we create a status error response
		result := NewGitHubAPIStatusErrorResponse(ctx, "failed to create issue", resp, body)

		// Then it should return an MCP error result
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		// And the error should be stored in the context
		apiErrors, err := GetGitHubAPIErrors(ctx)
		require.NoError(t, err)
		require.Len(t, apiErrors, 1)

		apiError := apiErrors[0]
		assert.Equal(t, "failed to create issue", apiError.Message)
		assert.Equal(t, resp, apiError.Response)
		// The synthetic error should contain the status code and body
		assert.Contains(t, apiError.Err.Error(), "unexpected status 422")
		assert.Contains(t, apiError.Err.Error(), "Validation Failed")
	})

	t.Run("NewGitHubAPIErrorToCtx with uninitialized context does not error", func(t *testing.T) {
		// Given a regular context without GitHub error tracking initialized
		ctx := context.Background()

		// Create a mock GitHub response
		resp := &github.Response{
			Response: &http.Response{
				StatusCode: 500,
				Status:     "500 Internal Server Error",
			},
		}
		originalErr := fmt.Errorf("internal server error")

		// When we try to add an API error to an uninitialized context
		updatedCtx, err := NewGitHubAPIErrorToCtx(ctx, "failed operation", resp, originalErr)

		// Then it should not return an error (graceful handling)
		assert.NoError(t, err, "NewGitHubAPIErrorToCtx should handle uninitialized context gracefully")
		assert.Equal(t, ctx, updatedCtx, "Context should be returned unchanged when not initialized")

		// And attempting to retrieve errors should still return an error since context wasn't initialized
		apiErrors, err := GetGitHubAPIErrors(updatedCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context does not contain GitHubCtxErrors")
		assert.Nil(t, apiErrors)
	})

	t.Run("NewGitHubAPIErrorToCtx with nil context does not error", func(t *testing.T) {
		// Given a nil context
		var ctx context.Context

		// Create a mock GitHub response
		resp := &github.Response{
			Response: &http.Response{
				StatusCode: 400,
				Status:     "400 Bad Request",
			},
		}
		originalErr := fmt.Errorf("bad request")

		// When we try to add an API error to a nil context
		updatedCtx, err := NewGitHubAPIErrorToCtx(ctx, "failed with nil context", resp, originalErr)

		// Then it should not return an error (graceful handling)
		assert.NoError(t, err, "NewGitHubAPIErrorToCtx should handle nil context gracefully")
		assert.Nil(t, updatedCtx, "Context should remain nil when passed as nil")
	})
}

func TestGitHubErrorTypes(t *testing.T) {
	t.Run("GitHubAPIError implements error interface", func(t *testing.T) {
		resp := &github.Response{Response: &http.Response{StatusCode: 404}}
		originalErr := fmt.Errorf("not found")

		apiErr := newGitHubAPIError("test message", resp, originalErr)

		// Should implement error interface
		var err error = apiErr
		assert.Equal(t, "test message: not found", err.Error())
	})

	t.Run("GitHubGraphQLError implements error interface", func(t *testing.T) {
		originalErr := fmt.Errorf("query failed")

		gqlErr := newGitHubGraphQLError("test message", originalErr)

		// Should implement error interface
		var err error = gqlErr
		assert.Equal(t, "test message: query failed", err.Error())
	})
}

// TestMiddlewareScenario demonstrates a realistic middleware scenario
func TestMiddlewareScenario(t *testing.T) {
	t.Run("realistic middleware error collection scenario", func(t *testing.T) {
		// Simulate a realistic HTTP middleware scenario

		// 1. Request comes in, middleware sets up error tracking
		ctx := ContextWithGitHubErrors(context.Background())

		// 2. Middleware stores reference to context for later inspection
		var middlewareCtx context.Context
		setupMiddleware := func(ctx context.Context) context.Context {
			middlewareCtx = ctx
			return ctx
		}

		// 3. Setup middleware
		ctx = setupMiddleware(ctx)

		// 4. Simulate multiple service calls that add errors
		simulateServiceCall1 := func(ctx context.Context) {
			resp := &github.Response{Response: &http.Response{StatusCode: 403}}
			_, err := NewGitHubAPIErrorToCtx(ctx, "insufficient permissions", resp, fmt.Errorf("forbidden"))
			require.NoError(t, err)
		}

		simulateServiceCall2 := func(ctx context.Context) {
			resp := &github.Response{Response: &http.Response{StatusCode: 404}}
			_, err := NewGitHubAPIErrorToCtx(ctx, "resource not found", resp, fmt.Errorf("not found"))
			require.NoError(t, err)
		}

		simulateGraphQLCall := func(ctx context.Context) {
			gqlErr := newGitHubGraphQLError("mutation failed", fmt.Errorf("invalid input"))
			_, err := addGitHubGraphQLErrorToContext(ctx, gqlErr)
			require.NoError(t, err)
		}

		// 5. Execute service calls (without context propagation)
		simulateServiceCall1(ctx)
		simulateServiceCall2(ctx)
		simulateGraphQLCall(ctx)

		// 6. Middleware inspects errors at the end of request processing
		finalizeMiddleware := func(ctx context.Context) ([]string, []string) {
			var apiErrorMessages []string
			var gqlErrorMessages []string

			if apiErrors, err := GetGitHubAPIErrors(ctx); err == nil {
				for _, apiErr := range apiErrors {
					apiErrorMessages = append(apiErrorMessages, apiErr.Message)
				}
			}

			if gqlErrors, err := GetGitHubGraphQLErrors(ctx); err == nil {
				for _, gqlErr := range gqlErrors {
					gqlErrorMessages = append(gqlErrorMessages, gqlErr.Message)
				}
			}

			return apiErrorMessages, gqlErrorMessages
		}

		// 7. Middleware can see all errors that were added during request processing
		apiMessages, gqlMessages := finalizeMiddleware(middlewareCtx)

		// Verify all errors were captured
		assert.Len(t, apiMessages, 2)
		assert.Contains(t, apiMessages, "insufficient permissions")
		assert.Contains(t, apiMessages, "resource not found")

		assert.Len(t, gqlMessages, 1)
		assert.Contains(t, gqlMessages, "mutation failed")
	})
}
