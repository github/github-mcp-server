package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListIssueFields(t *testing.T) {
	// Verify tool definition
	serverTool := ListIssueFields(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_issue_fields", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner"})
	assert.ElementsMatch(t, serverTool.RequiredScopes, []string{"repo", "read:org"})
	assert.ElementsMatch(t, serverTool.AcceptedScopes, []string{"repo", "read:org", "write:org", "admin:org"})

	queryStruct := issueFieldsRepoQuery{}
	defaultVars := map[string]any{
		"owner": githubv4.String("testowner"),
		"name":  githubv4.String("testrepo"),
	}
	orgQueryStruct := issueFieldsOrgQuery{}
	defaultOrgVars := map[string]any{
		"login": githubv4.String("testowner"),
	}

	tests := []struct {
		name            string
		requestArgs     map[string]any
		mockQueryStruct any
		mockVars        map[string]any
		gqlResponse     githubv4mock.GQLResponse
		expectError     bool
		expectedFields  []IssueField
		expectedErrMsg  string
	}{
		{
			name: "no fields returns empty list",
			requestArgs: map[string]any{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			gqlResponse: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issueFields": map[string]any{
						"nodes": []any{},
					},
				},
			}),
			expectedFields: []IssueField{},
		},
		{
			name: "text field returned",
			requestArgs: map[string]any{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			gqlResponse: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issueFields": map[string]any{
						"nodes": []any{
							map[string]any{
								"__typename":  "IssueFieldText",
								"id":          "IFT_1",
								"name":        "DRI",
								"description": "Directly responsible individual",
								"dataType":    "TEXT",
								"visibility":  "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{
					ID:          "IFT_1",
					Name:        "DRI",
					Description: "Directly responsible individual",
					DataType:    "TEXT",
					Visibility:  "ORG_ONLY",
				},
			},
		},
		{
			name: "single_select field with options returned",
			requestArgs: map[string]any{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			gqlResponse: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issueFields": map[string]any{
						"nodes": []any{
							map[string]any{
								"__typename":  "IssueFieldSingleSelect",
								"id":          "IFSS_1",
								"name":        "Priority",
								"description": "Level of importance",
								"dataType":    "SINGLE_SELECT",
								"visibility":  "ALL",
								"options": []any{
									map[string]any{
										"id":    "OPT_1",
										"name":  "High",
										"color": "red",
									},
									map[string]any{
										"id":    "OPT_2",
										"name":  "Low",
										"color": "blue",
									},
								},
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{
					ID:          "IFSS_1",
					Name:        "Priority",
					Description: "Level of importance",
					DataType:    "SINGLE_SELECT",
					Visibility:  "ALL",
					Options: []IssueSingleSelectFieldOption{
						{ID: "OPT_1", Name: "High", Color: "red"},
						{ID: "OPT_2", Name: "Low", Color: "blue"},
					},
				},
			},
		},
		{
			name: "missing owner parameter",
			requestArgs: map[string]any{
				"repo": "testrepo",
			},
			gqlResponse:    githubv4mock.DataResponse(map[string]any{}),
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name: "no repo returns org-level fields",
			requestArgs: map[string]any{
				"owner": "testowner",
			},
			mockQueryStruct: orgQueryStruct,
			mockVars:        defaultOrgVars,
			gqlResponse: githubv4mock.DataResponse(map[string]any{
				"organization": map[string]any{
					"issueFields": map[string]any{
						"nodes": []any{
							map[string]any{
								"__typename": "IssueFieldText",
								"id":         "IFT_1",
								"name":       "DRI",
								"dataType":   "TEXT",
								"visibility": "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{ID: "IFT_1", Name: "DRI", DataType: "TEXT", Visibility: "ORG_ONLY"},
			},
		},
		{
			name: "number field returned",
			requestArgs: map[string]any{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			gqlResponse: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issueFields": map[string]any{
						"nodes": []any{
							map[string]any{
								"__typename": "IssueFieldNumber",
								"id":         "IFN_1",
								"name":       "Engineering Staffing",
								"dataType":   "NUMBER",
								"visibility": "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{ID: "IFN_1", Name: "Engineering Staffing", DataType: "NUMBER", Visibility: "ORG_ONLY"},
			},
		},
		{
			name: "date field returned",
			requestArgs: map[string]any{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			gqlResponse: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issueFields": map[string]any{
						"nodes": []any{
							map[string]any{
								"__typename": "IssueFieldDate",
								"id":         "IFD_1",
								"name":       "Target Date",
								"dataType":   "DATE",
								"visibility": "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{ID: "IFD_1", Name: "Target Date", DataType: "DATE", Visibility: "ORG_ONLY"},
			},
		},
		{
			name: "graphql error returns failure",
			requestArgs: map[string]any{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			gqlResponse:    githubv4mock.ErrorResponse("boom"),
			expectError:    true,
			expectedErrMsg: "failed to list issue fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			qs := tc.mockQueryStruct
			if qs == nil {
				qs = queryStruct
			}
			vars := tc.mockVars
			if vars == nil {
				vars = defaultVars
			}
			mockedHTTPClient := githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(qs, vars, tc.gqlResponse),
			)
			gqlClient := githubv4.NewClient(mockedHTTPClient)
			deps := BaseDeps{GQLClient: gqlClient}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
					return
				}
				require.NotNil(t, result)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			var returnedFields []IssueField
			err = json.Unmarshal([]byte(textContent.Text), &returnedFields)
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedFields), len(returnedFields))
			for i, expected := range tc.expectedFields {
				assert.Equal(t, expected.ID, returnedFields[i].ID)
				assert.Equal(t, expected.Name, returnedFields[i].Name)
				assert.Equal(t, expected.DataType, returnedFields[i].DataType)
				assert.Equal(t, expected.Visibility, returnedFields[i].Visibility)
				if expected.Options != nil {
					require.Equal(t, len(expected.Options), len(returnedFields[i].Options))
					for j, opt := range expected.Options {
						assert.Equal(t, opt.Name, returnedFields[i].Options[j].Name)
						assert.Equal(t, opt.Color, returnedFields[i].Options[j].Color)
					}
				}
			}
		})
	}
}
