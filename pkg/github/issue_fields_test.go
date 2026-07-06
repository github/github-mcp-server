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
	// Verify tool definitions. The MS-aware variant owns the _ff_<flag> snap;
	// the legacy variant owns the canonical list_issue_fields.snap.
	serverTool := ListIssueFields(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name+"_ff_"+FeatureFlagIssueFieldsMultiSelect, tool))
	assert.Equal(t, FeatureFlagIssueFieldsMultiSelect, serverTool.FeatureFlagEnable, "ListIssueFields is the multi-select-aware variant and must be gated on the FF")
	assert.Contains(t, tool.Description, "multi_select", "the MS-aware description must mention multi_select")

	legacyServerTool := ListIssueFieldsLegacy(translations.NullTranslationHelper)
	legacyTool := legacyServerTool.Tool
	require.NoError(t, toolsnaps.Test(legacyTool.Name, legacyTool))
	assert.Empty(t, legacyServerTool.FeatureFlagEnable, "ListIssueFieldsLegacy must not require any flag to be enabled")
	assert.ElementsMatch(t, []string{FeatureFlagIssueFieldsMultiSelect}, legacyServerTool.FeatureFlagDisable, "ListIssueFieldsLegacy must be hidden when the multi-select flag is on")
	assert.NotContains(t, legacyTool.Description, "multi_select", "the legacy description must not advertise multi_select")

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
								"__typename":     "IssueFieldText",
								"id":             "IFT_1",
								"fullDatabaseId": "42",
								"name":           "DRI",
								"description":    "Directly responsible individual",
								"dataType":       "TEXT",
								"visibility":     "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{
					ID:          "IFT_1",
					DatabaseID:  42,
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
								"__typename":     "IssueFieldSingleSelect",
								"id":             "IFSS_1",
								"fullDatabaseId": "99",
								"name":           "Priority",
								"description":    "Level of importance",
								"dataType":       "SINGLE_SELECT",
								"visibility":     "ALL",
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
					DatabaseID:  99,
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
								"__typename":     "IssueFieldText",
								"id":             "IFT_1",
								"fullDatabaseId": "77",
								"name":           "DRI",
								"dataType":       "TEXT",
								"visibility":     "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{ID: "IFT_1", DatabaseID: 77, Name: "DRI", DataType: "TEXT", Visibility: "ORG_ONLY"},
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
								"__typename":     "IssueFieldNumber",
								"id":             "IFN_1",
								"fullDatabaseId": "101",
								"name":           "Engineering Staffing",
								"dataType":       "NUMBER",
								"visibility":     "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{ID: "IFN_1", DatabaseID: 101, Name: "Engineering Staffing", DataType: "NUMBER", Visibility: "ORG_ONLY"},
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
								"__typename":     "IssueFieldDate",
								"id":             "IFD_1",
								"fullDatabaseId": "202",
								"name":           "Target Date",
								"dataType":       "DATE",
								"visibility":     "ORG_ONLY",
							},
						},
					},
				},
			}),
			expectedFields: []IssueField{
				{ID: "IFD_1", DatabaseID: 202, Name: "Target Date", DataType: "DATE", Visibility: "ORG_ONLY"},
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
				assert.Equal(t, expected.DatabaseID, returnedFields[i].DatabaseID)
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

// Test_ListIssueFields_LegacyDoesNotLeakMSDescriptionUnderRealTranslationHelper
// pins the fix for the translation-key collision bug. The real translation
// helper is first-write-wins on cache hits: if both the MS and legacy variants
// use the same translation key, the second registration inherits the first
// registration's cached description regardless of the fallback passed in.
//
// AllTools() calls the MS variant first, so under the bug the legacy variant
// silently gets the MS description at runtime — leaking "multi_select" into
// what should be a legacy-only surface. This test simulates the real helper's
// cache behaviour and asserts the two descriptions actually differ.
func Test_ListIssueFields_LegacyDoesNotLeakMSDescriptionUnderRealTranslationHelper(t *testing.T) {
	// Mimic the caching behaviour of translations.TranslationHelper (see
	// pkg/translations/translations.go): first-write-wins keyed by key name.
	cache := map[string]string{}
	firstWriteWins := func(key, defaultValue string) string {
		if v, ok := cache[key]; ok {
			return v
		}
		cache[key] = defaultValue
		return defaultValue
	}

	// Call in registration order: MS variant first, legacy second. This is
	// what AllTools() does — see tools.go.
	msDesc := ListIssueFields(firstWriteWins).Tool.Description
	legacyDesc := ListIssueFieldsLegacy(firstWriteWins).Tool.Description

	require.NotEqual(t, msDesc, legacyDesc,
		"the MS and legacy variants MUST use different translation keys so the real TranslationHelper's first-write-wins cache doesn't leak the MS description into the legacy variant")
	assert.Contains(t, msDesc, "multi_select",
		"MS variant description must advertise multi_select")
	assert.NotContains(t, legacyDesc, "multi_select",
		"legacy variant description must not advertise multi_select — that would leak the gated feature")
}
