package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v73/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGlobalSecurityAdvisories(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListGlobalSecurityAdvisories(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_global_security_advisories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "ghsa_id")
	assert.Contains(t, tool.InputSchema.Properties, "severity")
	assert.Contains(t, tool.InputSchema.Properties, "ecosystem")

	// Mock advisory data
	mockAdvisories := []*github.GlobalSecurityAdvisory{
		{
			ID: github.Ptr(int64(123)),
		},
		{
			ID: github.Ptr(int64(456)),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedCount  int
		expectedErrMsg string
	}{
		{
			name: "successful listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/advisories",
						Method:  "GET",
					},
					mockResponse(t, http.StatusOK, mockAdvisories),
				),
			),
			requestArgs:   map[string]interface{}{},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "with severity filter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/advisories",
						Method:  "GET",
					},
					expectQueryParams(t, map[string]string{"severity": "high"}).andThen(
						mockResponse(t, http.StatusOK, []*github.GlobalSecurityAdvisory{mockAdvisories[0]}),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"severity": "high",
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "with ecosystem filter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/advisories",
						Method:  "GET",
					},
					expectQueryParams(t, map[string]string{"ecosystem": "go"}).andThen(
						mockResponse(t, http.StatusOK, mockAdvisories),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"ecosystem": "go",
			},
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListGlobalSecurityAdvisories(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)

			// Parse and verify the response
			textContent := getTextResult(t, result)
			var advisories []*github.GlobalSecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &advisories)
			require.NoError(t, err)
			assert.Len(t, advisories, tc.expectedCount)
		})
	}
}

func TestGetGlobalSecurityAdvisory(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetGlobalSecurityAdvisory(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_global_security_advisory", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "ghsa_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"ghsa_id"})

	// Mock advisory data
	mockAdvisory := &github.GlobalSecurityAdvisory{
		ID: github.Ptr(int64(123)),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedID     int64
		expectedErrMsg string
	}{
		{
			name: "successful get",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/advisories/GHSA-xxxx-xxxx-xxxx",
						Method:  "GET",
					},
					mockResponse(t, http.StatusOK, mockAdvisory),
				),
			),
			requestArgs: map[string]interface{}{
				"ghsa_id": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError: false,
			expectedID:  123,
		},
		{
			name: "missing ghsa_id parameter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/advisories/GHSA-xxxx-xxxx-xxxx",
						Method:  "GET",
					},
					mockResponse(t, http.StatusOK, mockAdvisory),
				),
			),
			requestArgs: map[string]interface{}{},
			expectError: true,
		},
		{
			name: "advisory not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/advisories/GHSA-nonexistent",
						Method:  "GET",
					},
					mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
				),
			),
			requestArgs: map[string]interface{}{
				"ghsa_id": "GHSA-nonexistent",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetGlobalSecurityAdvisory(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				if tc.expectedErrMsg != "" {
					if err != nil {
						assert.Contains(t, err.Error(), tc.expectedErrMsg)
					} else {
						assert.True(t, result.IsError)
						textContent := getErrorResult(t, result)
						assert.Contains(t, textContent.Text, tc.expectedErrMsg)
					}
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)

			// Parse and verify the response
			textContent := getTextResult(t, result)
			var advisory *github.GlobalSecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &advisory)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedID, *advisory.ID)
		})
	}
}
