package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/v2/pkg/translations"
)

func Test_GetCodeQualityFinding(t *testing.T) {
	// Verify tool definition once
	toolDef := GetCodeQualityFinding(translations.NullTranslationHelper)
	testSecurityToolSnapshot(t, toolDef.Tool)

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
		Number: new(42),
		State:  new("open"),
		Rule: &codeQualityRule{
			ID:          new("test-rule"),
			Description: new("Test Rule Description"),
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

func Test_ListCodeQualityFindings(t *testing.T) {
	// Verify tool definition once
	toolDef := ListCodeQualityFindings(translations.NullTranslationHelper)
	tool := toolDef.Tool
	testSecurityToolSnapshot(t, tool)

	assert.Equal(t, "list_code_quality_findings", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_code_quality_findings tool should be read-only")

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "state")
	assert.Contains(t, schema.Properties, "direction")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Contains(t, schema.Properties, "after")
	assert.NotContains(t, schema.Properties, "page", "the API only supports cursor pagination")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})
	assert.Equal(t, []any{"open", "dismissed"}, schema.Properties["state"].Enum)
	assert.Equal(t, []any{"asc", "desc"}, schema.Properties["direction"].Enum)

	openFinding := json.RawMessage(`{
		"number": 116,
		"state": "open",
		"url": "https://api.github.com/repos/owner/repo/code-quality/findings/116",
		"rule": {"id": "js/conditional-comment", "title": "Conditional comments", "severity": "warning", "category": "reliability"},
		"location": {"path": "src/app.js", "start_line": 21, "end_line": 21},
		"message": {"text": "Do not use conditional comments.", "markdown": "Do not use conditional comments."},
		"created_at": "2026-09-08T18:16:36Z"
	}`)
	dismissedFinding := json.RawMessage(`{
		"number": 7,
		"state": "dismissed",
		"url": "https://api.github.com/repos/owner/repo/code-quality/findings/7",
		"rule": {"id": "js/unused-local-variable", "title": "Unused variable", "severity": "note", "category": "maintainability"},
		"location": {"path": "src/util.js", "start_line": 3},
		"message": {"text": "Unused variable x.", "markdown": "Unused variable x."},
		"created_at": "2026-09-01T10:00:00Z"
	}`)

	writeFindings := func(t *testing.T, link string, findings ...json.RawMessage) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			if link != "" {
				w.Header().Set("Link", link)
			}
			w.WriteHeader(http.StatusOK)
			b, err := json.Marshal(findings)
			require.NoError(t, err)
			_, _ = w.Write(b)
		}
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectError        bool
		expectedNumbers    []int
		expectedStates     []string
		expectedNextCursor string
		expectedErrMsg     string
	}{
		{
			name: "successful listing with defaults",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"per_page": "30",
				}).andThen(writeFindings(t, "", openFinding)),
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectedNumbers: []int{116},
			expectedStates:  []string{"open"},
		},
		{
			name: "successful dismissed listing in ascending order",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"state":     "dismissed",
					"direction": "asc",
					"per_page":  "30",
				}).andThen(writeFindings(t, "", dismissedFinding)),
			}),
			requestArgs: map[string]any{
				"owner":     "owner",
				"repo":      "repo",
				"state":     "dismissed",
				"direction": "asc",
			},
			expectedNumbers: []int{7},
			expectedStates:  []string{"dismissed"},
		},
		{
			name: "successful listing passes cursor and page size",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"after":    "Y3Vyc29yOnYyOpK5",
					"per_page": "100",
				}).andThen(writeFindings(t, "", openFinding)),
			}),
			requestArgs: map[string]any{
				"owner":   "owner",
				"repo":    "repo",
				"after":   "Y3Vyc29yOnYyOpK5",
				"perPage": float64(100),
			},
			expectedNumbers: []int{116},
			expectedStates:  []string{"open"},
		},
		{
			name: "successful listing surfaces next cursor and ignores empty last cursor",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"per_page": "2",
				}).andThen(writeFindings(t,
					`<https://api.github.com/repositories/1/code-quality/findings?per_page=2&after=nextcursor123>; rel="next", `+
						`<https://api.github.com/repositories/1/code-quality/findings?per_page=2&before=>; rel="last"`,
					openFinding, dismissedFinding,
				)),
			}),
			requestArgs: map[string]any{
				"owner":   "owner",
				"repo":    "repo",
				"perPage": float64(2),
			},
			expectedNumbers:    []int{116, 7},
			expectedStates:     []string{"open", "dismissed"},
			expectedNextCursor: "nextcursor123",
		},
		{
			name: "listing forbidden",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepo: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Code quality is not enabled for this repository."}`))
				},
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list findings for repository 'owner/repo'",
		},
		{
			name: "listing not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeQualityFindingsByOwnerByRepo: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				},
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "Not Found",
		},
		{
			name:         "missing owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"repo": "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing repo",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner": "owner",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			textContent := getTextResult(t, result)

			var returnedResult struct {
				Findings []struct {
					Number int    `json:"number"`
					State  string `json:"state"`
					URL    string `json:"url"`
				} `json:"findings"`
				PageInfo struct {
					HasNextPage     bool   `json:"hasNextPage"`
					HasPreviousPage bool   `json:"hasPreviousPage"`
					NextCursor      string `json:"nextCursor"`
					PrevCursor      string `json:"prevCursor"`
				} `json:"pageInfo"`
			}
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &returnedResult))
			require.Len(t, returnedResult.Findings, len(tc.expectedNumbers))
			for i, finding := range returnedResult.Findings {
				assert.Equal(t, tc.expectedNumbers[i], finding.Number)
				assert.Equal(t, tc.expectedStates[i], finding.State)
				assert.NotEmpty(t, finding.URL, "legacy text should return findings as the API returns them")
			}
			assert.Equal(t, tc.expectedNextCursor != "", returnedResult.PageInfo.HasNextPage)
			assert.Equal(t, tc.expectedNextCursor, returnedResult.PageInfo.NextCursor)
			assert.False(t, returnedResult.PageInfo.HasPreviousPage, "an empty before cursor must not be surfaced")
			assert.Empty(t, returnedResult.PageInfo.PrevCursor)
		})
	}
}
