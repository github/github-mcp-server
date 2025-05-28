package github

import (
	"context"
	"testing"

	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryCompletionHandler(t *testing.T) {
	// Mock client function that returns an error to test error handling
	errorGetClient := func(_ context.Context) (*github.Client, error) {
		return nil, assert.AnError
	}

	tests := []struct {
		name          string
		request       mcp.CompleteRequest
		getClient     GetClientFn
		expectedError bool
	}{
		{
			name: "invalid ref type - should return error",
			request: mcp.CompleteRequest{
				Params: struct {
					Ref      any `json:"ref"`
					Argument struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"argument"`
				}{
					Ref: "invalid",
					Argument: struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						Name:  "owner",
						Value: "test",
					},
				},
			},
			getClient:     errorGetClient,
			expectedError: true,
		},
		{
			name: "unsupported ref type - should return error",
			request: mcp.CompleteRequest{
				Params: struct {
					Ref      any `json:"ref"`
					Argument struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"argument"`
				}{
					Ref: map[string]interface{}{
						"type": "ref/prompt",
						"name": "some_prompt",
					},
					Argument: struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						Name:  "param",
						Value: "test",
					},
				},
			},
			getClient:     errorGetClient,
			expectedError: true,
		},
		{
			name: "missing uri in resource reference - should return error",
			request: mcp.CompleteRequest{
				Params: struct {
					Ref      any `json:"ref"`
					Argument struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"argument"`
				}{
					Ref: map[string]interface{}{
						"type": "ref/resource",
					},
					Argument: struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						Name:  "owner",
						Value: "test",
					},
				},
			},
			getClient:     errorGetClient,
			expectedError: true,
		},
		{
			name: "non-repo URI - should return empty completion",
			request: mcp.CompleteRequest{
				Params: struct {
					Ref      any `json:"ref"`
					Argument struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"argument"`
				}{
					Ref: map[string]interface{}{
						"type": "ref/resource",
						"uri":  "file:///some/path",
					},
					Argument: struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						Name:  "param",
						Value: "test",
					},
				},
			},
			getClient:     errorGetClient,
			expectedError: false,
		},
		{
			name: "unsupported argument - should return empty completion",
			request: mcp.CompleteRequest{
				Params: struct {
					Ref      any `json:"ref"`
					Argument struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"argument"`
				}{
					Ref: map[string]interface{}{
						"type": "ref/resource",
						"uri":  "repo://{owner}/{repo}/contents{/path*}",
					},
					Argument: struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						Name:  "unsupported",
						Value: "test",
					},
				},
			},
			getClient:     errorGetClient,
			expectedError: false,
		},
		{
			name: "client error - should return error",
			request: mcp.CompleteRequest{
				Params: struct {
					Ref      any `json:"ref"`
					Argument struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"argument"`
				}{
					Ref: map[string]interface{}{
						"type": "ref/resource",
						"uri":  "repo://{owner}/{repo}/contents{/path*}",
					},
					Argument: struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						Name:  "owner",
						Value: "test",
					},
				},
			},
			getClient:     errorGetClient,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RepositoryCompletionHandler(tt.getClient)
			result, err := handler(context.Background(), tt.request)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			// For non-repo URIs and unsupported arguments, we should get empty completion
			assert.Equal(t, []string{}, result.Completion.Values)
		})
	}
}

func TestUtilityFunctions(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "extract owner from basic repo URI",
			uri:      "repo://octocat/Hello-World/contents",
			expected: "octocat",
		},
		{
			name:     "extract owner from template URI",
			uri:      "repo://{owner}/{repo}/contents{/path*}",
			expected: "",
		},
		{
			name:     "extract owner from non-repo URI",
			uri:      "file:///some/path",
			expected: "",
		},
		{
			name:     "empty URI",
			uri:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractOwnerFromURI(tt.uri)
			assert.Equal(t, tt.expected, result)
		})
	}

	ownerRepoTests := []struct {
		name          string
		uri           string
		expectedOwner string
		expectedRepo  string
	}{
		{
			name:          "extract owner and repo from basic URI",
			uri:           "repo://octocat/Hello-World/contents",
			expectedOwner: "octocat",
			expectedRepo:  "Hello-World",
		},
		{
			name:          "extract from template URI",
			uri:           "repo://{owner}/{repo}/contents{/path*}",
			expectedOwner: "",
			expectedRepo:  "",
		},
		{
			name:          "extract from branch URI",
			uri:           "repo://octocat/Hello-World/refs/heads/main/contents",
			expectedOwner: "octocat",
			expectedRepo:  "Hello-World",
		},
		{
			name:          "extract from commit URI",
			uri:           "repo://octocat/Hello-World/sha/abc123/contents",
			expectedOwner: "octocat",
			expectedRepo:  "Hello-World",
		},
		{
			name:          "extract from tag URI",
			uri:           "repo://octocat/Hello-World/refs/tags/v1.0/contents",
			expectedOwner: "octocat",
			expectedRepo:  "Hello-World",
		},
		{
			name:          "extract from PR URI",
			uri:           "repo://octocat/Hello-World/refs/pull/123/head/contents",
			expectedOwner: "octocat",
			expectedRepo:  "Hello-World",
		},
		{
			name:          "non-repo URI",
			uri:           "file:///some/path",
			expectedOwner: "",
			expectedRepo:  "",
		},
		{
			name:          "empty URI",
			uri:           "",
			expectedOwner: "",
			expectedRepo:  "",
		},
	}

	for _, tt := range ownerRepoTests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo := extractOwnerRepoFromURI(tt.uri)
			assert.Equal(t, tt.expectedOwner, owner)
			assert.Equal(t, tt.expectedRepo, repo)
		})
	}

	refTests := []struct {
		name        string
		uri         string
		expectedRef string
	}{
		{
			name:        "extract branch ref",
			uri:         "repo://octocat/Hello-World/refs/heads/main/contents",
			expectedRef: "refs/heads/main",
		},
		{
			name:        "extract commit ref",
			uri:         "repo://octocat/Hello-World/sha/abc123/contents",
			expectedRef: "abc123",
		},
		{
			name:        "extract tag ref",
			uri:         "repo://octocat/Hello-World/refs/tags/v1.0/contents",
			expectedRef: "refs/tags/v1.0",
		},
		{
			name:        "extract PR ref",
			uri:         "repo://octocat/Hello-World/refs/pull/123/head/contents",
			expectedRef: "refs/pull/123/head",
		},
		{
			name:        "basic repo URI - no ref",
			uri:         "repo://octocat/Hello-World/contents",
			expectedRef: "",
		},
		{
			name:        "template URI - no ref",
			uri:         "repo://{owner}/{repo}/contents{/path*}",
			expectedRef: "",
		},
		{
			name:        "template branch URI - no ref",
			uri:         "repo://{owner}/{repo}/refs/heads/{branch}/contents{/path*}",
			expectedRef: "",
		},
	}

	for _, tt := range refTests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRefFromURI(tt.uri)
			assert.Equal(t, tt.expectedRef, result)
		})
	}
}