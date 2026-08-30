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

func TestListUserLists(t *testing.T) {
	t.Parallel()

	serverTool := ListUserLists(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_user_lists", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_user_lists tool should be read-only")

	// scope gating: PublicRead(ReadUser)
	assert.Equal(t, []string{"read:user"}, serverTool.ScopeAccess.Scopes)
	assert.NotNil(t, serverTool.ScopeAccess.Visible)
	assert.NotNil(t, serverTool.ScopeAccess.Challenge)
	assert.True(t, serverTool.ScopeAccess.Visible(nil))

	tests := []struct {
		name            string
		requestArgs     map[string]any
		mockedClient    *http.Client
		expectToolError bool
	}{
		{
			name: "list user lists without items",
			requestArgs: map[string]any{
				"include_items": false,
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Viewer struct {
							Lists struct {
								Nodes []struct {
									ID          githubv4.ID
									Name        githubv4.String
									Description githubv4.String
									IsPrivate   githubv4.Boolean
								}
								TotalCount githubv4.Int
							} `graphql:"lists(first: 100)"`
						}
					}{},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"viewer": map[string]any{
							"lists": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":          githubv4.ID("list-1"),
										"name":        githubv4.String("My list"),
										"description": githubv4.String("A list"),
										"isPrivate":   githubv4.Boolean(true),
									},
								},
								"totalCount": githubv4.Int(1),
							},
						},
					}),
				),
			),
			expectToolError: false,
		},
		{
			name: "list user lists with items",
			requestArgs: map[string]any{
				"include_items": true,
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Viewer struct {
							Lists struct {
								Nodes []struct {
									ID          githubv4.ID
									Name        githubv4.String
									Description githubv4.String
									IsPrivate   githubv4.Boolean
								}
								TotalCount githubv4.Int
							} `graphql:"lists(first: 100)"`
						}
					}{},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"viewer": map[string]any{
							"lists": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":          githubv4.ID("list-1"),
										"name":        githubv4.String("My list"),
										"description": githubv4.String("A list"),
										"isPrivate":   githubv4.Boolean(false),
									},
								},
								"totalCount": githubv4.Int(1),
							},
						},
					}),
				),
				githubv4mock.NewQueryMatcher(
					struct {
						Node struct {
							UserList struct {
								Items struct {
									Nodes []struct {
										Repository struct {
											NameWithOwner githubv4.String
										}
									}
								} `graphql:"items(first: 100)"`
							} `graphql:"... on UserList"`
						} `graphql:"node(id: $id)"`
					}{},
					map[string]any{
						"id": githubv4.ID("list-1"),
					},
					githubv4mock.DataResponse(map[string]any{
						"node": map[string]any{
							"items": map[string]any{
								"nodes": []any{
									map[string]any{
										"repository": map[string]any{
											"nameWithOwner": githubv4.String("owner/repo"),
										},
									},
								},
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
			} else {
				assert.False(t, result.IsError)
			}
		})
	}
}

func TestCreateUserList(t *testing.T) {
	t.Parallel()

	serverTool := CreateUserList(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "create_user_list", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.False(t, *tool.Annotations.DestructiveHint)

	// scope gating: RequireAll(User)
	assert.Equal(t, []string{"user"}, serverTool.ScopeAccess.Scopes)
	assert.NotNil(t, serverTool.ScopeAccess.Visible)
	assert.NotNil(t, serverTool.ScopeAccess.Challenge)

	tests := []struct {
		name            string
		requestArgs     map[string]any
		mockedClient    *http.Client
		expectToolError bool
	}{
		{
			name: "create list",
			requestArgs: map[string]any{
				"name":        "My list",
				"description": "A list",
				"is_private":  true,
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewMutationMatcher(
					struct {
						CreateUserList struct {
							List struct {
								ID   githubv4.ID
								Name githubv4.String
							}
						} `graphql:"createUserList(input: $input)"`
					}{},
					githubv4.CreateUserListInput{
						Name:        githubv4.String("My list"),
						Description: func() *githubv4.String { s := githubv4.String("A list"); return &s }(),
						IsPrivate:   func() *githubv4.Boolean { b := githubv4.Boolean(true); return &b }(),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"createUserList": map[string]any{
							"list": map[string]any{
								"id":   githubv4.ID("list-1"),
								"name": githubv4.String("My list"),
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
			assert.Equal(t, tc.expectToolError, result.IsError)
		})
	}
}

func TestUpdateUserList(t *testing.T) {
	t.Parallel()

	serverTool := UpdateUserList(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "update_user_list", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.False(t, *tool.Annotations.DestructiveHint)
	assert.Equal(t, []string{"user"}, serverTool.ScopeAccess.Scopes)

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "update list name",
			requestArgs: map[string]any{
				"name":     "Old name",
				"new_name": "New name",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Viewer struct {
							Lists struct {
								Nodes []struct {
									ID   githubv4.ID
									Name githubv4.String
								}
							} `graphql:"lists(first: 100)"`
						}
					}{},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"viewer": map[string]any{
							"lists": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":   githubv4.ID("list-1"),
										"name": githubv4.String("Old name"),
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						UpdateUserList struct {
							List struct {
								Name githubv4.String
							}
						} `graphql:"updateUserList(input: $input)"`
					}{},
					githubv4.UpdateUserListInput{
						ListID: githubv4.ID("list-1"),
						Name:   func() *githubv4.String { s := githubv4.String("New name"); return &s }(),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"updateUserList": map[string]any{
							"list": map[string]any{
								"name": githubv4.String("New name"),
							},
						},
					}),
				),
			),
			expectToolError: false,
		},
		{
			name: "update list not found",
			requestArgs: map[string]any{
				"name":     "Missing",
				"new_name": "New name",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Viewer struct {
							Lists struct {
								Nodes []struct {
									ID   githubv4.ID
									Name githubv4.String
								}
							} `graphql:"lists(first: 100)"`
						}
					}{},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"viewer": map[string]any{
							"lists": map[string]any{
								"nodes": []any{},
							},
						},
					}),
				),
			),
			expectToolError:    true,
			expectedToolErrMsg: "list 'Missing' not found",
		},
		{
			name: "update without changes",
			requestArgs: map[string]any{
				"name": "My list",
			},
			mockedClient:       githubv4mock.NewMockedHTTPClient(),
			expectToolError:    true,
			expectedToolErrMsg: "at least one of new_name, description, or is_private must be provided for update",
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

func TestDeleteUserList(t *testing.T) {
	t.Parallel()

	serverTool := DeleteUserList(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "delete_user_list", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.True(t, *tool.Annotations.DestructiveHint)
	assert.Equal(t, []string{"user"}, serverTool.ScopeAccess.Scopes)

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "delete list",
			requestArgs: map[string]any{
				"name": "My list",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Viewer struct {
							Lists struct {
								Nodes []struct {
									ID   githubv4.ID
									Name githubv4.String
								}
							} `graphql:"lists(first: 100)"`
						}
					}{},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"viewer": map[string]any{
							"lists": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":   githubv4.ID("list-1"),
										"name": githubv4.String("My list"),
									},
								},
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						DeleteUserList struct {
							ClientMutationID githubv4.String
						} `graphql:"deleteUserList(input: $input)"`
					}{},
					githubv4.DeleteUserListInput{
						ListID: githubv4.ID("list-1"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"deleteUserList": map[string]any{
							"clientMutationId": githubv4.String("test-mutation-id"),
						},
					}),
				),
			),
			expectToolError: false,
		},
		{
			name: "delete list not found",
			requestArgs: map[string]any{
				"name": "Missing",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Viewer struct {
							Lists struct {
								Nodes []struct {
									ID   githubv4.ID
									Name githubv4.String
								}
							} `graphql:"lists(first: 100)"`
						}
					}{},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"viewer": map[string]any{
							"lists": map[string]any{
								"nodes": []any{},
							},
						},
					}),
				),
			),
			expectToolError:    true,
			expectedToolErrMsg: "list 'Missing' not found",
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

func TestAddRepositoryToList(t *testing.T) {
	t.Parallel()

	serverTool := AddRepositoryToList(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "add_repository_to_list", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.False(t, *tool.Annotations.DestructiveHint)
	assert.Equal(t, []string{"user"}, serverTool.ScopeAccess.Scopes)

	// Repository currently in lists "A" and "B"; adding to "C" must resubmit all
	// three because updateUserListsForItem REPLACES membership (does not append).
	mockedClient := githubv4mock.NewMockedHTTPClient(
		// 1. resolve list "C" -> list-c
		githubv4mock.NewQueryMatcher(
			struct {
				Viewer struct {
					Lists struct {
						Nodes []struct {
							ID   githubv4.ID
							Name githubv4.String
						}
					} `graphql:"lists(first: 100)"`
				}
			}{},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"viewer": map[string]any{
					"lists": map[string]any{
						"nodes": []any{
							map[string]any{"id": githubv4.ID("list-c"), "name": githubv4.String("C")},
						},
					},
				},
			}),
		),
		// 2. resolve repository -> repo-id
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
					"id": githubv4.ID("repo-id"),
				},
			}),
		),
		// 3. read current list membership -> A, B
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Lists struct {
						Nodes []struct {
							ID githubv4.ID
						}
					} `graphql:"lists(first: 100)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner": githubv4.String("owner"),
				"repo":  githubv4.String("repo"),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"lists": map[string]any{
						"nodes": []any{
							map[string]any{"id": githubv4.ID("list-a")},
							map[string]any{"id": githubv4.ID("list-b")},
						},
					},
				},
			}),
		),
		// 4. resubmit full set A, B, C
		githubv4mock.NewMutationMatcher(
			struct {
				UpdateUserListsForItem struct {
					ClientMutationID githubv4.String
				} `graphql:"updateUserListsForItem(input: $input)"`
			}{},
			githubv4.UpdateUserListsForItemInput{
				ItemID:  githubv4.ID("repo-id"),
				ListIDs: []githubv4.ID{"list-a", "list-b", "list-c"},
			},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"updateUserListsForItem": map[string]any{
					"clientMutationId": githubv4.String("test-mutation-id"),
				},
			}),
		),
	)

	client := githubv4.NewClient(mockedClient)
	deps := BaseDeps{
		GQLClient: client,
	}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":     "owner",
		"repo":      "repo",
		"list_name": "C",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestRemoveRepositoryFromList(t *testing.T) {
	t.Parallel()

	serverTool := RemoveRepositoryFromList(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "remove_repository_from_list", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.False(t, *tool.Annotations.DestructiveHint)
	assert.Equal(t, []string{"user"}, serverTool.ScopeAccess.Scopes)

	// Repository currently in lists "A" and "B"; removing from "B" must resubmit
	// only the remainder ("A").
	mockedClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Viewer struct {
					Lists struct {
						Nodes []struct {
							ID   githubv4.ID
							Name githubv4.String
						}
					} `graphql:"lists(first: 100)"`
				}
			}{},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"viewer": map[string]any{
					"lists": map[string]any{
						"nodes": []any{
							map[string]any{"id": githubv4.ID("list-b"), "name": githubv4.String("B")},
						},
					},
				},
			}),
		),
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
					"id": githubv4.ID("repo-id"),
				},
			}),
		),
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Lists struct {
						Nodes []struct {
							ID githubv4.ID
						}
					} `graphql:"lists(first: 100)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner": githubv4.String("owner"),
				"repo":  githubv4.String("repo"),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"lists": map[string]any{
						"nodes": []any{
							map[string]any{"id": githubv4.ID("list-a")},
							map[string]any{"id": githubv4.ID("list-b")},
						},
					},
				},
			}),
		),
		githubv4mock.NewMutationMatcher(
			struct {
				UpdateUserListsForItem struct {
					ClientMutationID githubv4.String
				} `graphql:"updateUserListsForItem(input: $input)"`
			}{},
			githubv4.UpdateUserListsForItemInput{
				ItemID:  githubv4.ID("repo-id"),
				ListIDs: []githubv4.ID{"list-a"},
			},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"updateUserListsForItem": map[string]any{
					"clientMutationId": githubv4.String("test-mutation-id"),
				},
			}),
		),
	)

	client := githubv4.NewClient(mockedClient)
	deps := BaseDeps{
		GQLClient: client,
	}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":     "owner",
		"repo":      "repo",
		"list_name": "B",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestAddRepositoryToListListNotFound(t *testing.T) {
	t.Parallel()

	serverTool := AddRepositoryToList(translations.NullTranslationHelper)
	mockedClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Viewer struct {
					Lists struct {
						Nodes []struct {
							ID   githubv4.ID
							Name githubv4.String
						}
					} `graphql:"lists(first: 100)"`
				}
			}{},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"viewer": map[string]any{
					"lists": map[string]any{
						"nodes": []any{},
					},
				},
			}),
		),
	)

	client := githubv4.NewClient(mockedClient)
	deps := BaseDeps{
		GQLClient: client,
	}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":     "owner",
		"repo":      "repo",
		"list_name": "Missing",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	textContent := getErrorResult(t, result)
	assert.Contains(t, textContent.Text, "list 'Missing' not found")
}
