package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
)

func Test_GetCodeQualityFinding(t *testing.T) {
	// Verify tool definition once
	toolDef := GetCodeQualityFinding(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_code_quality_finding", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)

	// InputSchema is of type any, need to cast to *jsonschema.Schema
	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "findingNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "findingNumber"})

	type codeQualityRule struct {
		ID          *string `json:"id,omitempty"`
		Title       *string `json:"title,omitempty"`
		Description *string `json:"description,omitempty"`
		Help        *string `json:"help,omitempty"`
		Severity    *string `json:"severity,omitempty"`
		Category    *string `json:"category,omitempty"`
	}

	type codeQualityLocation struct {
		Path        *string `json:"path,omitempty"`
		StartLine   *int    `json:"start_line,omitempty"`
		StartColumn *int    `json:"start_column,omitempty"`
		EndLine     *int    `json:"end_line,omitempty"`
		EndColumn   *int    `json:"end_column,omitempty"`
	}

	type codeQualityMessage struct {
		Text     string `json:"text"`
		Markdown string `json:"markdown"`
	}

	type codeQualityFinding struct {
		Number    *int                 `json:"number,omitempty"`
		State     *string              `json:"state,omitempty"`
		URL       *string              `json:"url,omitempty"`
		Rule      *codeQualityRule     `json:"rule,omitempty"`
		Location  *codeQualityLocation `json:"location,omitempty"`
		Message   *codeQualityMessage  `json:"message,omitempty"`
		CreatedAt *github.Timestamp    `json:"created_at,omitempty"`
	}

	// Setup mock finding for success case
	mockFinding := &codeQualityFinding{
		Number: github.Ptr(42),
		State:  github.Ptr("open"),
		Rule: &codeQualityRule{
			ID:          github.Ptr("test-rule"),
			Description: github.Ptr("Test Rule Description"),
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]any
		expectError     bool
		expectedFinding *codeQualityFinding
		expectedErrMsg  string
	}{
		{
			name: "successful finding fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber: mockResponse(t, http.StatusOK, mockFinding),
			}),
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"findingNumber": float64(42),
			},
			expectError:     false,
			expectedFinding: mockFinding,
		},
		{
			name: "finding fetch fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				},
			}),
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"findingNumber": float64(9999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get finding",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler with new signature
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedFinding codeQualityFinding
			err = json.Unmarshal([]byte(textContent.Text), &returnedFinding)
			assert.NoError(t, err)
			assert.Equal(t, *tc.expectedFinding.Number, *returnedFinding.Number)
			assert.Equal(t, *tc.expectedFinding.State, *returnedFinding.State)
			assert.Equal(t, *tc.expectedFinding.Rule.ID, *returnedFinding.Rule.ID)

		})
	}
}
