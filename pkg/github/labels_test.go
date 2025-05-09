package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListLabels(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListLabels(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_labels", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

	// Setup mock labels for success case
	mockLabels := []*github.Label{
		{
			ID:          github.Ptr(int64(1)),
			Name:        github.Ptr("bug"),
			Description: github.Ptr("Something isn't working"),
			Color:       github.Ptr("f29513"),
			URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/bug"),
			Default:     github.Ptr(true),
		},
		{
			ID:          github.Ptr(int64(2)),
			Name:        github.Ptr("enhancement"),
			Description: github.Ptr("New feature or request"),
			Color:       github.Ptr("a2eeef"),
			URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/enhancement"),
			Default:     github.Ptr(false),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLabels []*github.Label
		expectedErrMsg string
	}{
		{
			name: "successful labels listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposLabelsByOwnerByRepo,
					mockLabels,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    false,
			expectedLabels: mockLabels,
		},
		{
			name: "labels listing with pagination",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposLabelsByOwnerByRepo,
					expectQueryParams(t, map[string]string{
						"page":     "2",
						"per_page": "10",
					}).andThen(
						mockResponse(t, http.StatusOK, mockLabels),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"page":    float64(2),
				"perPage": float64(10),
			},
			expectError:    false,
			expectedLabels: mockLabels,
		},
		{
			name: "labels listing fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposLabelsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list labels",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListLabels(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedLabels []*github.Label
			err = json.Unmarshal([]byte(textContent.Text), &returnedLabels)
			require.NoError(t, err)
			assert.Len(t, returnedLabels, len(tc.expectedLabels))

			for i, label := range returnedLabels {
				assert.Equal(t, *tc.expectedLabels[i].Name, *label.Name)
				assert.Equal(t, *tc.expectedLabels[i].Color, *label.Color)
				assert.Equal(t, *tc.expectedLabels[i].Description, *label.Description)
				assert.Equal(t, *tc.expectedLabels[i].Default, *label.Default)
			}
		})
	}
}

func Test_GetLabel(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetLabel(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_label", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "name"})

	// Setup mock label for success case
	mockLabel := &github.Label{
		ID:          github.Ptr(int64(1)),
		Name:        github.Ptr("bug"),
		Description: github.Ptr("Something isn't working"),
		Color:       github.Ptr("f29513"),
		URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/bug"),
		Default:     github.Ptr(true),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLabel  *github.Label
		expectedErrMsg string
	}{
		{
			name: "successful label retrieval",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposLabelsByOwnerByRepoByName,
					mockLabel,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"name":  "bug",
			},
			expectError:   false,
			expectedLabel: mockLabel,
		},
		{
			name: "label retrieval fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposLabelsByOwnerByRepoByName,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Label not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"name":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to get label",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetLabel(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedLabel *github.Label
			err = json.Unmarshal([]byte(textContent.Text), &returnedLabel)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedLabel.Name, *returnedLabel.Name)
			assert.Equal(t, *tc.expectedLabel.Color, *returnedLabel.Color)
			assert.Equal(t, *tc.expectedLabel.Description, *returnedLabel.Description)
			assert.Equal(t, *tc.expectedLabel.Default, *returnedLabel.Default)
		})
	}
}

func Test_CreateLabel(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CreateLabel(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_label", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.Contains(t, tool.InputSchema.Properties, "color")
	assert.Contains(t, tool.InputSchema.Properties, "description")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "name", "color"})

	// Setup mock created label for success case
	mockLabel := &github.Label{
		ID:          github.Ptr(int64(3)),
		Name:        github.Ptr("documentation"),
		Description: github.Ptr("Improvements or additions to documentation"),
		Color:       github.Ptr("0075ca"),
		URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/documentation"),
		Default:     github.Ptr(false),
	}

	labelRequest := map[string]interface{}{
		"name":        "documentation",
		"description": "Improvements or additions to documentation",
		"color":       "0075ca",
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLabel  *github.Label
		expectedErrMsg string
	}{
		{
			name: "successful label creation",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposLabelsByOwnerByRepo,
					expectRequestBody(t, labelRequest).andThen(
						mockResponse(t, http.StatusCreated, mockLabel),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"name":        "documentation",
				"color":       "0075ca",
				"description": "Improvements or additions to documentation",
			},
			expectError:   false,
			expectedLabel: mockLabel,
		},
		{
			name: "label creation fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposLabelsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Validation failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"name":        "documentation",
				"color":       "invalid-color",
				"description": "Improvements or additions to documentation",
			},
			expectError:    true,
			expectedErrMsg: "failed to create label",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := CreateLabel(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedLabel *github.Label
			err = json.Unmarshal([]byte(textContent.Text), &returnedLabel)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedLabel.Name, *returnedLabel.Name)
			assert.Equal(t, *tc.expectedLabel.Color, *returnedLabel.Color)
			assert.Equal(t, *tc.expectedLabel.Description, *returnedLabel.Description)
		})
	}
}

func Test_UpdateLabel(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := UpdateLabel(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "update_label", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.Contains(t, tool.InputSchema.Properties, "new_name")
	assert.Contains(t, tool.InputSchema.Properties, "color")
	assert.Contains(t, tool.InputSchema.Properties, "description")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "name"})

	// Setup mock updated label for success case
	mockLabel := &github.Label{
		ID:          github.Ptr(int64(1)),
		Name:        github.Ptr("bug :bug:"),
		Description: github.Ptr("Small bug fix required"),
		Color:       github.Ptr("b01f26"),
		URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/bug%20:bug:"),
		Default:     github.Ptr(true),
	}

	labelRequest := map[string]interface{}{
		"name":        "bug :bug:",
		"description": "Small bug fix required",
		"color":       "b01f26",
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLabel  *github.Label
		expectedErrMsg string
	}{
		{
			name: "successful label update",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchReposLabelsByOwnerByRepoByName,
					expectRequestBody(t, labelRequest).andThen(
						mockResponse(t, http.StatusOK, mockLabel),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"name":        "bug",
				"new_name":    "bug :bug:",
				"color":       "b01f26",
				"description": "Small bug fix required",
			},
			expectError:   false,
			expectedLabel: mockLabel,
		},
		{
			name: "label update fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchReposLabelsByOwnerByRepoByName,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Label not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"name":        "nonexistent",
				"new_name":    "bug :bug:",
				"color":       "b01f26",
				"description": "Small bug fix required",
			},
			expectError:    true,
			expectedErrMsg: "failed to update label",
		},
		{
			name:         "no update parameters provided",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"name":  "bug",
			},
			expectError:    false,
			expectedErrMsg: "No update parameters provided.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := UpdateLabel(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Special case for no update parameters - we return a tool result error, not a Go error
			if tc.name == "no update parameters provided" {
				require.NoError(t, err)
				require.NotNil(t, result)
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			// Verify results for other cases
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedLabel *github.Label
			err = json.Unmarshal([]byte(textContent.Text), &returnedLabel)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedLabel.Name, *returnedLabel.Name)
			assert.Equal(t, *tc.expectedLabel.Color, *returnedLabel.Color)
			assert.Equal(t, *tc.expectedLabel.Description, *returnedLabel.Description)
		})
	}
}

func Test_DeleteLabel(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := DeleteLabel(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "delete_label", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "name"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult string
		expectedErrMsg string
	}{
		{
			name: "successful label deletion",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteReposLabelsByOwnerByRepoByName,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"name":  "bug",
			},
			expectError:    false,
			expectedResult: "successfully deleted",
		},
		{
			name: "label deletion fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteReposLabelsByOwnerByRepoByName,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Label not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"name":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to delete label",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := DeleteLabel(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Verify the result
			assert.Contains(t, textContent.Text, tc.expectedResult)
		})
	}
}

func Test_ListLabelsForIssue(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListLabelsForIssue(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_labels_for_issue", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "issue_number"})

	// Setup mock labels for success case
	mockLabels := []*github.Label{
		{
			ID:          github.Ptr(int64(1)),
			Name:        github.Ptr("bug"),
			Description: github.Ptr("Something isn't working"),
			Color:       github.Ptr("f29513"),
			URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/bug"),
			Default:     github.Ptr(true),
		},
		{
			ID:          github.Ptr(int64(2)),
			Name:        github.Ptr("enhancement"),
			Description: github.Ptr("New feature or request"),
			Color:       github.Ptr("a2eeef"),
			URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/enhancement"),
			Default:     github.Ptr(false),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLabels []*github.Label
		expectedErrMsg string
	}{
		{
			name: "successful labels listing for issue",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposIssuesLabelsByOwnerByRepoByIssueNumber,
					mockLabels,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedLabels: mockLabels,
		},
		{
			name: "labels listing for issue with pagination",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposIssuesLabelsByOwnerByRepoByIssueNumber,
					expectQueryParams(t, map[string]string{
						"page":     "2",
						"per_page": "10",
					}).andThen(
						mockResponse(t, http.StatusOK, mockLabels),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"page":         float64(2),
				"perPage":      float64(10),
			},
			expectError:    false,
			expectedLabels: mockLabels,
		},
		{
			name: "labels listing for issue fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposIssuesLabelsByOwnerByRepoByIssueNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Issue not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to list labels for issue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListLabelsForIssue(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedLabels []*github.Label
			err = json.Unmarshal([]byte(textContent.Text), &returnedLabels)
			require.NoError(t, err)
			assert.Len(t, returnedLabels, len(tc.expectedLabels))

			for i, label := range returnedLabels {
				assert.Equal(t, *tc.expectedLabels[i].Name, *label.Name)
				assert.Equal(t, *tc.expectedLabels[i].Color, *label.Color)
				assert.Equal(t, *tc.expectedLabels[i].Description, *label.Description)
				assert.Equal(t, *tc.expectedLabels[i].Default, *label.Default)
			}
		})
	}
}

func Test_AddLabelsToIssue(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := AddLabelsToIssue(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "add_labels_to_issue", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.Properties, "labels")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "issue_number", "labels"})

	// Setup mock labels for success case
	mockLabels := []*github.Label{
		{
			ID:          github.Ptr(int64(1)),
			Name:        github.Ptr("bug"),
			Description: github.Ptr("Something isn't working"),
			Color:       github.Ptr("f29513"),
			URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/bug"),
			Default:     github.Ptr(true),
		},
		{
			ID:          github.Ptr(int64(2)),
			Name:        github.Ptr("enhancement"),
			Description: github.Ptr("New feature or request"),
			Color:       github.Ptr("a2eeef"),
			URL:         github.Ptr("https://api.github.com/repos/octocat/Hello-World/labels/enhancement"),
			Default:     github.Ptr(false),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLabels []*github.Label
		expectedErrMsg string
	}{
		{
			name: "successful labels addition to issue",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposIssuesLabelsByOwnerByRepoByIssueNumber,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// Return success status and expected labels
						w.WriteHeader(http.StatusOK)
						data, _ := json.Marshal(mockLabels)
						_, _ = w.Write(data)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"labels":       []interface{}{"bug", "enhancement"},
			},
			expectError:    false,
			expectedLabels: mockLabels,
		},
		{
			name: "labels addition to issue fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposIssuesLabelsByOwnerByRepoByIssueNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Validation failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"labels":       []interface{}{"invalid-label"},
			},
			expectError:    true,
			expectedErrMsg: "failed to add labels to issue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := AddLabelsToIssue(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedLabels []*github.Label
			err = json.Unmarshal([]byte(textContent.Text), &returnedLabels)
			require.NoError(t, err)
			assert.Len(t, returnedLabels, len(tc.expectedLabels))

			for i, label := range returnedLabels {
				assert.Equal(t, *tc.expectedLabels[i].Name, *label.Name)
				assert.Equal(t, *tc.expectedLabels[i].Color, *label.Color)
				assert.Equal(t, *tc.expectedLabels[i].Description, *label.Description)
				assert.Equal(t, *tc.expectedLabels[i].Default, *label.Default)
			}
		})
	}
}

func Test_RemoveLabelFromIssue(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := RemoveLabelFromIssue(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "remove_label_from_issue", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "issue_number", "name"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult string
		expectedErrMsg string
	}{
		{
			name: "successful label removal from issue",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteReposIssuesLabelsByOwnerByRepoByIssueNumberByName,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`[]`)) // GitHub returns an empty array on successful removal
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"name":         "bug",
			},
			expectError:    false,
			expectedResult: "successfully removed",
		},
		{
			name: "label removal from issue fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteReposIssuesLabelsByOwnerByRepoByIssueNumberByName,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Label or issue not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
				"name":         "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to remove label from issue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := RemoveLabelFromIssue(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Verify the result
			assert.Contains(t, textContent.Text, tc.expectedResult)
		})
	}
}

func Test_RequiredStringArrayParam(t *testing.T) {
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
			expected:    nil,
			expectError: true,
		},
		{
			name: "empty any array parameter",
			params: map[string]any{
				"flag": []any{},
			},
			paramName:   "flag",
			expected:    nil,
			expectError: true,
		},
		{
			name: "empty string array parameter",
			params: map[string]any{
				"flag": []string{},
			},
			paramName:   "flag",
			expected:    nil,
			expectError: true,
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
			name: "nil parameter",
			params: map[string]any{
				"flag": nil,
			},
			paramName:   "flag",
			expected:    nil,
			expectError: true,
		},
		{
			name: "wrong type parameter",
			params: map[string]any{
				"flag": 1,
			},
			paramName:   "flag",
			expected:    nil,
			expectError: true,
		},
		{
			name: "wrong slice type parameter",
			params: map[string]any{
				"flag": []any{"foo", 2},
			},
			paramName:   "flag",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.params)
			result, err := RequiredStringArrayParam(request, tc.paramName)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
