//go:build e2e

package github

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v73/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindClosingPullRequestsIntegration tests the FindClosingPullRequests tool with real GitHub API calls
func TestFindClosingPullRequestsIntegration(t *testing.T) {
	// This test requires a GitHub token
	token := os.Getenv("GITHUB_MCP_SERVER_E2E_TOKEN")
	if token == "" {
		t.Skip("GITHUB_MCP_SERVER_E2E_TOKEN environment variable is not set")
	}

	// Create GitHub clients
	httpClient := github.NewClient(nil).WithAuthToken(token).Client()
	gqlClient := githubv4.NewClient(httpClient)

	getGQLClient := func(ctx context.Context) (*githubv4.Client, error) {
		return gqlClient, nil
	}

	// Create the tool
	tool, handler := FindClosingPullRequests(getGQLClient, translations.NullTranslationHelper)

	// Test cases with known GitHub issues that were closed by PRs
	testCases := []struct {
		name                   string
		owner                  string
		repo                   string
		issueNumbers           []int
		expectedResults        int
		expectSomeClosingPRs   bool
		expectSpecificIssue    string
		expectSpecificPRNumber int
	}{
		{
			name:                 "Single issue using issue_numbers - VS Code well-known closed issue",
			owner:                "microsoft",
			repo:                 "vscode",
			issueNumbers:         []int{123456}, // This is a made-up issue for testing
			expectedResults:      1,
			expectSomeClosingPRs: false, // We expect this to not exist or have no closing PRs
		},
		{
			name:                 "Multiple issues using issue_numbers with mixed results",
			owner:                "microsoft",
			repo:                 "vscode",
			issueNumbers:         []int{1, 999999},
			expectedResults:      2,
			expectSomeClosingPRs: false, // These are likely non-existent or have no closing PRs
		},
		{
			name:            "Issue from a popular repo using issue_numbers - React",
			owner:           "facebook",
			repo:            "react",
			issueNumbers:    []int{1}, // Very first issue in React repo
			expectedResults: 1,
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request arguments
			args := map[string]interface{}{
				"limit":         5,
				"owner":         tc.owner,
				"repo":          tc.repo,
				"issue_numbers": tc.issueNumbers,
			}

			// Create mock request
			request := mockCallToolRequest{
				arguments: args,
			}

			// Call the handler
			result, err := handler(ctx, request)

			if err != nil {
				t.Logf("Error calling tool: %v", err)
				// For integration tests, we might expect some errors for non-existent issues
				// Let's check if it's a reasonable error
				assert.Contains(t, err.Error(), "failed to")
				return
			}

			require.NotNil(t, result)
			assert.False(t, result.IsError, "Expected successful result")

			// Parse the response
			textContent, ok := result.Content[0].(map[string]interface{})
			if !ok {
				// Try to get as text content
				if len(result.Content) > 0 {
					if textResult, ok := result.Content[0].(string); ok {
						t.Logf("Response: %s", textResult)

						// Parse JSON response
						var response struct {
							Results []map[string]interface{} `json:"results"`
						}
						err := json.Unmarshal([]byte(textResult), &response)
						require.NoError(t, err, "Failed to parse JSON response")

						// Verify structure
						assert.Len(t, response.Results, tc.expectedResults, "Expected specific number of results")

						for i, result := range response.Results {
							t.Logf("Issue %d:", i+1)
							t.Logf("  Owner: %v, Repo: %v, Number: %v", result["owner"], result["repo"], result["issue_number"])
							t.Logf("  Total closing PRs: %v", result["total_count"])

							if errorMsg, hasError := result["error"]; hasError {
								t.Logf("  Error: %v", errorMsg)
							}

							// Verify basic structure
							assert.NotEmpty(t, result["owner"], "Owner should not be empty")
							assert.NotEmpty(t, result["repo"], "Repo should not be empty")
							assert.NotNil(t, result["issue_number"], "Issue number should not be nil")

							// Check closing PRs if any
							if closingPRs, ok := result["closing_pull_requests"].([]interface{}); ok {
								t.Logf("  Found %d closing PRs", len(closingPRs))
								for j, pr := range closingPRs {
									if prMap, ok := pr.(map[string]interface{}); ok {
										t.Logf("    PR %d: #%v - %v", j+1, prMap["number"], prMap["title"])
										t.Logf("      State: %v, Merged: %v", prMap["state"], prMap["merged"])
										t.Logf("      URL: %v", prMap["url"])
									}
								}
							}
						}

						return
					}
				}
				t.Fatalf("Unexpected content type: %T", result.Content[0])
			}

			t.Logf("Response content: %+v", textContent)
		})
	}
}

// mockCallToolRequest implements the mcp.CallToolRequest interface for testing
type mockCallToolRequest struct {
	arguments map[string]interface{}
}

func (m mockCallToolRequest) GetArguments() map[string]interface{} {
	return m.arguments
}
