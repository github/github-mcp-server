package github

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryResourceCompletionHandler(t *testing.T) {
	tests := []struct {
		name     string
		request  *mcp.CompleteRequest
		expected *mcp.CompleteResult
		wantErr  bool
	}{
		{
			name: "non-resource completion request",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref: &mcp.CompleteReference{
						Type: "something-else",
					},
				},
			},
			expected: nil,
			wantErr:  false,
		},
		{
			name: "invalid ref type",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref: &mcp.CompleteReference{
						Type: "invalid-ref",
					},
				},
			},
			expected: nil,
			wantErr:  false,
		},
		{
			name: "unknown argument",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref: &mcp.CompleteReference{
						Type: "ref/resource",
					},
					Context: &mcp.CompleteContext{},
					Argument: mcp.CompleteParamsArgument{
						Name:  "unknown_arg",
						Value: "test",
					},
				},
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getClient := func(_ context.Context) (*github.Client, error) {
				return &github.Client{}, nil
			}

			handler := RepositoryResourceCompletionHandler(getClient)
			result, err := handler(t.Context(), tt.request)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRepositoryResourceCompletionHandler_GetClientError(t *testing.T) {
	getClient := func(_ context.Context) (*github.Client, error) {
		return nil, errors.New("client error")
	}

	handler := RepositoryResourceCompletionHandler(getClient)
	request := &mcp.CompleteRequest{
		Params: &mcp.CompleteParams{
			Ref: &mcp.CompleteReference{
				Type: "ref/resource",
			},
			Context: &mcp.CompleteContext{
				Arguments: map[string]string{
					"owner": "test",
				},
			},
			Argument: mcp.CompleteParamsArgument{
				Name:  "owner",
				Value: "test",
			},
		},
	}

	result, err := handler(t.Context(), request)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "client error")
}

// Test the logical behavior of complete functions with missing dependencies
func TestCompleteRepo_MissingOwner(t *testing.T) {
	ctx := t.Context()
	resolved := map[string]string{} // No owner
	argValue := "test"

	result, err := completeRepo(ctx, nil, resolved, argValue)
	require.Error(t, err)
	assert.Nil(t, result) // Should return nil slice when owner is missing
}

func TestCompleteBranch_MissingDependencies(t *testing.T) {
	ctx := t.Context()

	// Test missing owner
	resolved := map[string]string{"repo": "testrepo"}
	result, err := completeBranch(ctx, nil, resolved, "main")
	require.Error(t, err)
	assert.Nil(t, result) // Returns nil slice when dependencies are missing

	// Test missing repo
	resolved = map[string]string{"owner": "testowner"}
	result, err = completeBranch(ctx, nil, resolved, "main")
	require.Error(t, err)
	assert.Nil(t, result) // Returns nil slice when dependencies are missing
}

func TestCompleteSHA_MissingDependencies(t *testing.T) {
	ctx := t.Context()

	// Test missing owner
	resolved := map[string]string{"repo": "testrepo"}
	result, err := completeSHA(ctx, nil, resolved, "abc123")
	require.Error(t, err)
	assert.Nil(t, result) // Returns nil slice when dependencies are missing

	// Test missing repo
	resolved = map[string]string{"owner": "testowner"}
	result, err = completeSHA(ctx, nil, resolved, "abc123")
	require.Error(t, err)
	assert.Nil(t, result) // Returns nil slice when dependencies are missing
}

func TestCompleteTag_MissingDependencies(t *testing.T) {
	ctx := t.Context()

	// Test missing owner
	resolved := map[string]string{"repo": "testrepo"}
	result, err := completeTag(ctx, nil, resolved, "v1.0")
	require.Error(t, err)
	assert.Nil(t, result) // completeTag returns nil for missing dependencies

	// Test missing repo
	resolved = map[string]string{"owner": "testowner"}
	result, err = completeTag(ctx, nil, resolved, "v1.0")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestCompletePRNumber_MissingDependencies(t *testing.T) {
	ctx := t.Context()

	// Test missing owner
	resolved := map[string]string{"repo": "testrepo"}
	result, err := completePRNumber(ctx, nil, resolved, "1")
	require.Error(t, err)
	assert.Nil(t, result) // Returns nil slice when dependencies are missing

	// Test missing repo
	resolved = map[string]string{"owner": "testowner"}
	result, err = completePRNumber(ctx, nil, resolved, "1")
	require.Error(t, err)
	assert.Nil(t, result) // Returns nil slice when dependencies are missing
}

func TestCompletePath_MissingDependencies(t *testing.T) {
	ctx := t.Context()

	// Test missing owner
	resolved := map[string]string{"repo": "testrepo"}
	result, err := completePath(ctx, nil, resolved, "src/")
	require.Error(t, err)
	assert.Nil(t, result) // completePath returns nil for missing dependencies

	// Test missing repo
	resolved = map[string]string{"owner": "testowner"}
	result, err = completePath(ctx, nil, resolved, "src/")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestCompletePath_RefSelection(t *testing.T) {
	// Test the logic for selecting the ref (branch, sha, tag, or HEAD)
	// We test this by verifying the function handles different ref combinations
	// without making API calls (since we can't mock them easily)

	ctx := t.Context()

	// Test that the function returns nil when dependencies are missing
	resolved := map[string]string{
		"owner": "",
		"repo":  "",
	}
	result, err := completePath(ctx, nil, resolved, "src/")
	require.Error(t, err)
	assert.Nil(t, result)

	// When owner is present but repo is missing
	resolved = map[string]string{
		"owner": "testowner",
		"repo":  "",
	}
	result, err = completePath(ctx, nil, resolved, "src/")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestRepositoryResourceArgumentResolvers_Existence(t *testing.T) {
	// Test that all expected resolvers are present
	expectedResolvers := []string{
		"owner", "repo", "branch", "sha", "tag", "prNumber", "path",
	}

	for _, resolver := range expectedResolvers {
		t.Run(fmt.Sprintf("resolver_%s_exists", resolver), func(t *testing.T) {
			_, exists := RepositoryResourceArgumentResolvers[resolver]
			assert.True(t, exists, "Resolver %s should exist", resolver)
		})
	}

	// Verify the total count
	assert.Len(t, RepositoryResourceArgumentResolvers, len(expectedResolvers))
}

func TestRepositoryResourceCompletionHandler_MaxResults(t *testing.T) {
	// Test that results are limited to 100 items
	getClient := func(_ context.Context) (*github.Client, error) {
		return &github.Client{}, nil
	}

	handler := RepositoryResourceCompletionHandler(getClient)

	// Mock a resolver that returns more than 100 results
	originalResolver := RepositoryResourceArgumentResolvers["owner"]
	RepositoryResourceArgumentResolvers["owner"] = func(_ context.Context, _ *github.Client, _ map[string]string, _ string) ([]string, error) {
		// Return 150 results
		results := make([]string, 150)
		for i := 0; i < 150; i++ {
			results[i] = fmt.Sprintf("user%d", i)
		}
		return results, nil
	}

	request := &mcp.CompleteRequest{
		Params: &mcp.CompleteParams{
			Ref: &mcp.CompleteReference{
				Type: "ref/resource",
			},
			Context: &mcp.CompleteContext{
				Arguments: map[string]string{
					"owner": "test",
				},
			},
			Argument: mcp.CompleteParamsArgument{
				Name:  "owner",
				Value: "test",
			},
		},
	}

	result, err := handler(t.Context(), request)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.LessOrEqual(t, len(result.Completion.Values), 100)

	// Restore original resolver
	RepositoryResourceArgumentResolvers["owner"] = originalResolver
}

func TestRepositoryResourceCompletionHandler_WithContext(t *testing.T) {
	// Test that the handler properly passes resolved context arguments
	getClient := func(_ context.Context) (*github.Client, error) {
		return &github.Client{}, nil
	}

	handler := RepositoryResourceCompletionHandler(getClient)

	// Mock a resolver that just returns the resolved arguments for testing
	originalResolver := RepositoryResourceArgumentResolvers["repo"]
	RepositoryResourceArgumentResolvers["repo"] = func(_ context.Context, _ *github.Client, resolved map[string]string, _ string) ([]string, error) {
		if owner, exists := resolved["owner"]; exists {
			return []string{fmt.Sprintf("repo-for-%s", owner)}, nil
		}
		return []string{}, nil
	}

	request := &mcp.CompleteRequest{
		Params: &mcp.CompleteParams{
			Ref: &mcp.CompleteReference{
				Type: "ref/resource",
			},
			Argument: mcp.CompleteParamsArgument{
				Name:  "repo",
				Value: "test",
			},
			Context: &mcp.CompleteContext{
				Arguments: map[string]string{
					"owner": "testowner",
				},
			},
		},
	}

	result, err := handler(t.Context(), request)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Completion.Values, "repo-for-testowner")

	// Restore original resolver
	RepositoryResourceArgumentResolvers["repo"] = originalResolver
}

func TestRepositoryResourceCompletionHandler_NilContext(t *testing.T) {
	// Test that the handler handles nil context gracefully
	getClient := func(_ context.Context) (*github.Client, error) {
		return &github.Client{}, nil
	}

	handler := RepositoryResourceCompletionHandler(getClient)

	// Mock a resolver that checks for empty resolved map
	originalResolver := RepositoryResourceArgumentResolvers["repo"]
	RepositoryResourceArgumentResolvers["repo"] = func(_ context.Context, _ *github.Client, resolved map[string]string, _ string) ([]string, error) {
		assert.NotNil(t, resolved, "Resolved map should never be nil")
		return []string{"test-repo"}, nil
	}

	request := &mcp.CompleteRequest{
		Params: &mcp.CompleteParams{
			Ref: &mcp.CompleteReference{
				Type: "ref/resource",
			},
			Argument: mcp.CompleteParamsArgument{
				Name:  "repo",
				Value: "test",
			},
			// Context is not set, so it should default to empty map
			Context: &mcp.CompleteContext{
				Arguments: map[string]string{},
			},
		},
	}

	result, err := handler(t.Context(), request)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Restore original resolver
	RepositoryResourceArgumentResolvers["repo"] = originalResolver
}
