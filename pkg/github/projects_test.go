package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	gh "github.com/google/go-github/v74/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListProjects(t *testing.T) {
	// Verify tool definition and schema once
	mockClient := gh.NewClient(nil)
	tool, _ := ListProjects(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_projects", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "owner_type")
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "before")
	assert.Contains(t, tool.InputSchema.Properties, "after")
	assert.Contains(t, tool.InputSchema.Properties, "per_page")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "owner_type"})

	// Minimal project objects (fields chosen to likely exist on ProjectV2; test only asserts round-trip JSON array length)
	orgProjects := []map[string]any{{"id": 1, "title": "Org Project"}}
	userProjects := []map[string]any{{"id": 2, "title": "User Project"}}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLength int
		expectedErrMsg string
	}{
		{
			name: "success organization",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, orgProjects),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "organization",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name: "success user",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{username}/projectsV2", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, userProjects),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octocat",
				"owner_type": "user",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name: "success organization with pagination & query",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						q := r.URL.Query()
						// Assert query params present
						if q.Get("after") == "cursor123" && q.Get("per_page") == "50" && q.Get("q") == "roadmap" {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(mock.MustMarshal(orgProjects))
							return
						}
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message":"unexpected query params"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "organization",
				"after":      "cursor123",
				"per_page":   float64(50),
				"query":      "roadmap",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "organization",
			},
			expectError:    true,
			expectedErrMsg: "failed to list projects",
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner_type": "organization",
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "octo-org",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			_, handler := ListProjects(stubGetClientFn(client), translations.NullTranslationHelper)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				// Parameter missing cases
				if tc.name == "missing owner" {
					assert.Contains(t, text, "missing required parameter: owner")
				}
				if tc.name == "missing owner_type" {
					assert.Contains(t, text, "missing required parameter: owner_type")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var arr []map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &arr)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedLength, len(arr))
		})
	}
}
