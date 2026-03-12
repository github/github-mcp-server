package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLabel(t *testing.T) {
	t.Parallel()

	// Verify tool definition
	serverTool := GetLabel(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_label", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_label tool should be read-only")

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful label retrieval",
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"name":  "bug",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Label struct {
								ID          githubv4.ID
								Name        githubv4.String
								Color       githubv4.String
								Description githubv4.String
							} `graphql:"label(name: $name)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"name":  githubv4.String("bug"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"label": map[string]any{
								"id":          githubv4.ID("test-label-id"),
								"name":        githubv4.String("bug"),
								"color":       githubv4.String("d73a4a"),
								"description": githubv4.String("Something isn't working"),
							},
						},
					}),
				),
			),
			expectToolError: false,
		},
		{
			name: "label not found",
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"name":  "nonexistent",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Label struct {
								ID          githubv4.ID
								Name        githubv4.String
								Color       githubv4.String
								Description githubv4.String
							} `graphql:"label(name: $name)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"name":  githubv4.String("nonexistent"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"label": map[string]any{
								"id":          githubv4.ID(""),
								"name":        githubv4.String(""),
								"color":       githubv4.String(""),
								"description": githubv4.String(""),
							},
						},
					}),
				),
			),
			expectToolError:    true,
			expectedToolErrMsg: "label 'nonexistent' not found in owner/repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := githubv4.NewClient(tc.mockedClient)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			assert.NotNil(t, result)

			if tc.expectToolError {
				assert.True(t, result.IsError)
				if tc.expectedToolErrMsg != "" {
					textContent := getErrorResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				}
			} else {
				assert.False(t, result.IsError)
			}
		})
	}
}

func TestListLabels(t *testing.T) {
	t.Parallel()

	// Verify tool definition
	serverTool := ListLabels(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_label", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_label tool should be read-only")

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful repository labels listing",
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Labels struct {
								Nodes []struct {
									ID          githubv4.ID
									Name        githubv4.String
									Color       githubv4.String
									Description githubv4.String
								}
								TotalCount githubv4.Int
							} `graphql:"labels(first: 100)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"labels": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":          githubv4.ID("label-1"),
										"name":        githubv4.String("bug"),
										"color":       githubv4.String("d73a4a"),
										"description": githubv4.String("Something isn't working"),
									},
									map[string]any{
										"id":          githubv4.ID("label-2"),
										"name":        githubv4.String("enhancement"),
										"color":       githubv4.String("a2eeef"),
										"description": githubv4.String("New feature or request"),
									},
								},
								"totalCount": githubv4.Int(2),
							},
						},
					}),
				),
			),
			expectToolError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := githubv4.NewClient(tc.mockedClient)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			assert.NotNil(t, result)

			if tc.expectToolError {
				assert.True(t, result.IsError)
				if tc.expectedToolErrMsg != "" {
					textContent := getErrorResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				}
			} else {
				assert.False(t, result.IsError)
			}
		})
	}
}

func TestWriteLabel(t *testing.T) {
	t.Parallel()

	// Verify tool definition
	serverTool := LabelWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "label_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "label_write tool should not be read-only")

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful label creation",
			requestArgs: map[string]any{
				"method":      "create",
				"owner":       "owner",
				"repo":        "repo",
				"name":        "new-label",
				"color":       "f29513",
				"description": "A new test label",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							ID githubv4.ID
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"id": githubv4.ID("test-repo-id"),
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						CreateLabel struct {
							Label struct {
								Name githubv4.String
								ID   githubv4.ID
							}
						} `graphql:"createLabel(input: $input)"`
					}{},
					githubv4.CreateLabelInput{
						RepositoryID: githubv4.ID("test-repo-id"),
						Name:         githubv4.String("new-label"),
						Color:        githubv4.String("f29513"),
						Description:  func() *githubv4.String { s := githubv4.String("A new test label"); return &s }(),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"createLabel": map[string]any{
							"label": map[string]any{
								"id":   githubv4.ID("new-label-id"),
								"name": githubv4.String("new-label"),
							},
						},
					}),
				),
			),
			expectToolError: false,
		},
		{
			name: "create label without color",
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"name":   "new-label",
			},
			mockedClient:       githubv4mock.NewMockedHTTPClient(),
			expectToolError:    true,
			expectedToolErrMsg: "color is required for create",
		},
		{
			name: "successful label update",
			requestArgs: map[string]any{
				"method":   "update",
				"owner":    "owner",
				"repo":     "repo",
				"name":     "bug",
				"new_name": "defect",
				"color":    "ff0000",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Label struct {
								ID   githubv4.ID
								Name githubv4.String
							} `graphql:"label(name: $name)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"name":  githubv4.String("bug"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"label": map[string]any{
								"id":   githubv4.ID("bug-label-id"),
								"name": githubv4.String("bug"),
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						UpdateLabel struct {
							Label struct {
								Name githubv4.String
								ID   githubv4.ID
							}
						} `graphql:"updateLabel(input: $input)"`
					}{},
					githubv4.UpdateLabelInput{
						ID:    githubv4.ID("bug-label-id"),
						Name:  func() *githubv4.String { s := githubv4.String("defect"); return &s }(),
						Color: func() *githubv4.String { s := githubv4.String("ff0000"); return &s }(),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"updateLabel": map[string]any{
							"label": map[string]any{
								"id":   githubv4.ID("bug-label-id"),
								"name": githubv4.String("defect"),
							},
						},
					}),
				),
			),
			expectToolError: false,
		},
		{
			name: "update label without any changes",
			requestArgs: map[string]any{
				"method": "update",
				"owner":  "owner",
				"repo":   "repo",
				"name":   "bug",
			},
			mockedClient:       githubv4mock.NewMockedHTTPClient(),
			expectToolError:    true,
			expectedToolErrMsg: "at least one of new_name, color, or description must be provided for update",
		},
		{
			name: "successful label deletion",
			requestArgs: map[string]any{
				"method": "delete",
				"owner":  "owner",
				"repo":   "repo",
				"name":   "bug",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Label struct {
								ID   githubv4.ID
								Name githubv4.String
							} `graphql:"label(name: $name)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"name":  githubv4.String("bug"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"label": map[string]any{
								"id":   githubv4.ID("bug-label-id"),
								"name": githubv4.String("bug"),
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						DeleteLabel struct {
							ClientMutationID githubv4.String
						} `graphql:"deleteLabel(input: $input)"`
					}{},
					githubv4.DeleteLabelInput{
						ID: githubv4.ID("bug-label-id"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"deleteLabel": map[string]any{
							"clientMutationId": githubv4.String("test-mutation-id"),
						},
					}),
				),
			),
			expectToolError: false,
		},
		{
			name: "invalid method",
			requestArgs: map[string]any{
				"method": "invalid",
				"owner":  "owner",
				"repo":   "repo",
				"name":   "bug",
			},
			mockedClient:       githubv4mock.NewMockedHTTPClient(),
			expectToolError:    true,
			expectedToolErrMsg: "unknown method: invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := githubv4.NewClient(tc.mockedClient)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			assert.NotNil(t, result)

			if tc.expectToolError {
				assert.True(t, result.IsError)
				if tc.expectedToolErrMsg != "" {
					textContent := getErrorResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				}
			} else {
				assert.False(t, result.IsError)
			}
		})
	}
}
