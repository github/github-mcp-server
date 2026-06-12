package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	gogithub "github.com/google/go-github/v87/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multi_select fixture reused across these tests.
func multiSelectField() IssueField {
	return IssueField{
		ID:       "IFMS_1",
		Name:     "Components",
		DataType: "MULTI_SELECT",
		Options: []IssueSingleSelectFieldOption{
			{ID: "OPT_AUTH", Name: "Auth", Color: "red"},
			{ID: "OPT_BILL", Name: "Billing", Color: "yellow"},
			{ID: "OPT_API", Name: "API", Color: "blue"},
		},
	}
}

func Test_parseRawFieldFilters_MultiSelect(t *testing.T) {
	t.Parallel()

	t.Run("accepts values as []any of strings", func(t *testing.T) {
		filters, err := parseRawFieldFilters(map[string]any{
			"field_filters": []any{
				map[string]any{
					"field_name": "Components",
					"values":     []any{"Auth", "Billing"},
				},
			},
		})
		require.NoError(t, err)
		require.Len(t, filters, 1)
		assert.Equal(t, "Components", filters[0].Name)
		assert.False(t, filters[0].HasValue)
		assert.True(t, filters[0].HasValues)
		assert.Equal(t, []string{"Auth", "Billing"}, filters[0].Values)
	})

	t.Run("accepts values as []string", func(t *testing.T) {
		filters, err := parseRawFieldFilters(map[string]any{
			"field_filters": []map[string]any{
				{"field_name": "Components", "values": []string{"Auth"}},
			},
		})
		require.NoError(t, err)
		require.Len(t, filters, 1)
		assert.Equal(t, []string{"Auth"}, filters[0].Values)
	})

	t.Run("rejects both value and values on the same entry", func(t *testing.T) {
		_, err := parseRawFieldFilters(map[string]any{
			"field_filters": []any{
				map[string]any{
					"field_name": "Components",
					"value":      "Auth",
					"values":     []any{"Auth"},
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "provide either 'value' or 'values', not both")
	})

	t.Run("rejects entry with neither value nor values", func(t *testing.T) {
		_, err := parseRawFieldFilters(map[string]any{
			"field_filters": []any{
				map[string]any{"field_name": "Components"},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing 'value'")
	})

	t.Run("rejects values items that are not strings", func(t *testing.T) {
		_, err := parseRawFieldFilters(map[string]any{
			"field_filters": []any{
				map[string]any{
					"field_name": "Components",
					"values":     []any{"Auth", 7},
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "values must be an array of strings")
	})
}

func Test_resolveFieldFilters_MultiSelect(t *testing.T) {
	t.Parallel()

	fields := []IssueField{multiSelectField()}

	t.Run("matches options case-insensitively and sets MultiSelectOptionValues", func(t *testing.T) {
		raw := []rawFieldFilter{{
			Name:      "Components",
			Values:    []string{"auth", "BILLING"},
			HasValues: true,
		}}
		out, err := resolveFieldFilters(raw, fields)
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.NotNil(t, out[0].MultiSelectOptionValues)
		got := *out[0].MultiSelectOptionValues
		assert.Equal(t, []githubv4.String{"Auth", "Billing"}, got)
		assert.Nil(t, out[0].SingleSelectOptionValue)
	})

	t.Run("rejects unknown option and lists valid ones", func(t *testing.T) {
		raw := []rawFieldFilter{{
			Name:      "Components",
			Values:    []string{"Auth", "Nope"},
			HasValues: true,
		}}
		_, err := resolveFieldFilters(raw, fields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "\"Nope\" is not a valid option")
		assert.Contains(t, err.Error(), "Auth, Billing, API")
	})

	t.Run("rejects scalar value on multi_select field", func(t *testing.T) {
		raw := []rawFieldFilter{{
			Name:     "Components",
			Value:    "Auth",
			HasValue: true,
		}}
		_, err := resolveFieldFilters(raw, fields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is multi_select, use 'values'")
	})

	t.Run("rejects empty values slice", func(t *testing.T) {
		raw := []rawFieldFilter{{
			Name:      "Components",
			Values:    []string{},
			HasValues: true,
		}}
		_, err := resolveFieldFilters(raw, fields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least one value")
	})

	t.Run("rejects values array on single_select field", func(t *testing.T) {
		ssFields := []IssueField{{
			Name:     "Priority",
			DataType: "SINGLE_SELECT",
			Options:  []IssueSingleSelectFieldOption{{Name: "P1"}, {Name: "P2"}},
		}}
		raw := []rawFieldFilter{{
			Name:      "Priority",
			Values:    []string{"P1"},
			HasValues: true,
		}}
		_, err := resolveFieldFilters(raw, ssFields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is single_select, use 'value'")
	})
}

func Test_fragmentToMinimalFieldValue_MultiSelect(t *testing.T) {
	t.Parallel()

	fv := IssueFieldValueFragment{
		TypeName: "IssueFieldMultiSelectValue",
	}
	fv.MultiSelectValue.Field.MultiSelect.Name = "Components"
	fv.MultiSelectValue.Field.MultiSelect.FullDatabaseID = "42"
	fv.MultiSelectValue.Options = []struct {
		Name githubv4.String
	}{
		{Name: "Auth"},
		{Name: "Billing"},
	}

	m, ok := fragmentToMinimalFieldValue(fv)
	require.True(t, ok)
	assert.Equal(t, "Components", m.Field)
	assert.Equal(t, []string{"Auth", "Billing"}, m.Values)
	assert.Empty(t, m.Value)
}

func Test_IssueFieldRef_Name_MultiSelect(t *testing.T) {
	t.Parallel()

	var ref IssueFieldRef
	ref.MultiSelect.Name = "Components"
	ref.MultiSelect.FullDatabaseID = "99"

	assert.Equal(t, "Components", ref.Name())
	assert.Equal(t, "99", ref.FullDatabaseIDStr())
}

func Test_optionalIssueWriteFields_MultiSelect(t *testing.T) {
	t.Parallel()

	t.Run("accepts field_option_names as []any of strings", func(t *testing.T) {
		fields, err := optionalIssueWriteFields(map[string]any{
			"issue_fields": []any{
				map[string]any{
					"field_name":         "Components",
					"field_option_names": []any{"Auth", "Billing"},
				},
			},
		})
		require.NoError(t, err)
		require.Len(t, fields, 1)
		assert.Equal(t, "Components", fields[0].FieldName)
		assert.Equal(t, []string{"Auth", "Billing"}, fields[0].FieldOptionNames)
		assert.Empty(t, fields[0].FieldOptionName)
		assert.Nil(t, fields[0].Value)
	})

	t.Run("accepts field_option_names as []string", func(t *testing.T) {
		fields, err := optionalIssueWriteFields(map[string]any{
			"issue_fields": []any{
				map[string]any{
					"field_name":         "Components",
					"field_option_names": []string{"Auth"},
				},
			},
		})
		require.NoError(t, err)
		require.Len(t, fields, 1)
		assert.Equal(t, []string{"Auth"}, fields[0].FieldOptionNames)
	})

	t.Run("rejects empty field_option_names slice", func(t *testing.T) {
		_, err := optionalIssueWriteFields(map[string]any{
			"issue_fields": []any{
				map[string]any{
					"field_name":         "Components",
					"field_option_names": []any{},
				},
			},
		})
		require.Error(t, err)
		// An empty slice is a common "clear the field" guess; nudge callers to delete:true
		// so the GraphQL deletion mutation runs instead of an unintentional no-op.
		assert.Contains(t, err.Error(), "empty field_option_names")
		assert.Contains(t, err.Error(), "delete: true")
	})

	t.Run("rejects field_option_names + value combo", func(t *testing.T) {
		_, err := optionalIssueWriteFields(map[string]any{
			"issue_fields": []any{
				map[string]any{
					"field_name":         "Components",
					"value":              "Auth",
					"field_option_names": []any{"Auth"},
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify more than one of value, field_option_name, or field_option_names")
	})

	t.Run("rejects field_option_names + field_option_name combo", func(t *testing.T) {
		_, err := optionalIssueWriteFields(map[string]any{
			"issue_fields": []any{
				map[string]any{
					"field_name":         "Components",
					"field_option_name":  "Auth",
					"field_option_names": []any{"Auth"},
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify more than one of")
	})

	t.Run("rejects field_option_names + delete combo", func(t *testing.T) {
		_, err := optionalIssueWriteFields(map[string]any{
			"issue_fields": []any{
				map[string]any{
					"field_name":         "Components",
					"delete":             true,
					"field_option_names": []any{"Auth"},
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify 'delete' together with")
	})
}

func Test_resolveIssueRequestFieldValues_MultiSelect(t *testing.T) {
	t.Parallel()

	// Mocked metadata GraphQL response — one IssueFieldMultiSelect field with three options.
	metaResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issueFields": map[string]any{
				"nodes": []any{
					map[string]any{
						"__typename":     "IssueFieldMultiSelect",
						"id":             "IFMS_node_101",
						"fullDatabaseId": "101",
						"name":           "Components",
						"dataType":       "multi_select",
						"options": []any{
							map[string]any{"fullDatabaseId": "9001", "name": "Auth"},
							map[string]any{"fullDatabaseId": "9002", "name": "Billing"},
							map[string]any{"fullDatabaseId": "9003", "name": "API"},
						},
					},
				},
			},
		},
	})

	newClient := func() *githubv4.Client {
		return githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				issueFieldWriteMetadataQuery{},
				map[string]any{
					"owner": githubv4.String("owner"),
					"repo":  githubv4.String("repo"),
				},
				metaResponse,
			),
		))
	}

	t.Run("resolves option names to []string payload", func(t *testing.T) {
		in := []issueWriteFieldInput{{
			FieldName:        "Components",
			FieldOptionNames: []string{"Auth", "Billing"},
		}}
		vals, _, err := resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", in)
		require.NoError(t, err)
		require.Len(t, vals, 1)
		require.NotNil(t, vals[0].Value)
		// REST IssueField#build_value_attributes expects an array of option names for multi_select.
		got, ok := vals[0].Value.([]string)
		require.True(t, ok, "value should be []string, got %T", vals[0].Value)
		assert.Equal(t, []string{"Auth", "Billing"}, got)
	})

	t.Run("matches options case-insensitively and returns canonical names", func(t *testing.T) {
		in := []issueWriteFieldInput{{
			FieldName:        "Components",
			FieldOptionNames: []string{"auth", "BILLING"},
		}}
		vals, _, err := resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", in)
		require.NoError(t, err)
		got, ok := vals[0].Value.([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"Auth", "Billing"}, got)
	})

	t.Run("rejects unknown option name and lists valid ones", func(t *testing.T) {
		in := []issueWriteFieldInput{{
			FieldName:        "Components",
			FieldOptionNames: []string{"Auth", "Nope"},
		}}
		_, _, err := resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Nope")
	})

	t.Run("rejects scalar field_option_name on multi_select field", func(t *testing.T) {
		in := []issueWriteFieldInput{{
			FieldName:       "Components",
			FieldOptionName: "Auth",
		}}
		_, _, err := resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field_option_name cannot be used")
		assert.Contains(t, err.Error(), "field_option_names")
	})

	t.Run("rejects raw value on multi_select field", func(t *testing.T) {
		in := []issueWriteFieldInput{{
			FieldName: "Components",
			Value:     "Auth,Billing",
		}}
		_, _, err := resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multi_select")
	})

	t.Run("captures node ID for delete:true so the GraphQL mutation can clear the field", func(t *testing.T) {
		in := []issueWriteFieldInput{{
			FieldName: "Components",
			Delete:    true,
		}}
		vals, deletions, err := resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", in)
		require.NoError(t, err)
		assert.Empty(t, vals)
		require.Len(t, deletions, 1)
		assert.Equal(t, int64(101), deletions[0].DatabaseID)
		// The GraphQL node ID is what deleteIssueFieldValue needs — without it the
		// REST PATCH would silently no-op (Go's omitempty strips an empty array).
		assert.Equal(t, githubv4.ID("IFMS_node_101"), deletions[0].NodeID)
	})
}

// Regression test for the delete:true bug: REST PATCH alone can't clear an issue
// field value (an empty issue_field_values array gets stripped by omitempty, so the
// field's old value sticks around). UpdateIssue has to follow up with a
// deleteIssueFieldValue GraphQL mutation per deleted field.
func Test_UpdateIssue_DeleteFieldValueRunsGraphQLMutation(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  gogithub.Ptr(42),
		Title:   gogithub.Ptr("Test issue"),
		State:   gogithub.Ptr("open"),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/42"),
	}

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
	}))

	// (1) fetchExistingIssueFieldValues for the merge step. Returning the field that's
	// about to be deleted is the worst case — it makes the kept list empty, which is
	// when omitempty bites and the GraphQL mutation is the only thing that can clear it.
	existingFieldsResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"issueFieldValues": map[string]any{
					"nodes": []any{
						map[string]any{
							"__typename": "IssueFieldMultiSelectValue",
							"field": map[string]any{
								"fullDatabaseId": "101",
								"name":           "Components",
							},
							"options": []any{
								map[string]any{"name": "Auth"},
								map[string]any{"name": "Billing"},
							},
						},
					},
				},
			},
		},
	})

	// (2) fetchIssueIDs lookup so UpdateIssue can address the issue from GraphQL.
	issueIDResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{"id": "I_node_42"},
		},
	})

	// (3) The actual delete mutation — this is the thing that fixes the bug. The matcher
	// will fail the test if the handler skips it or sends the wrong issueId/fieldId.
	deleteResponse := githubv4mock.DataResponse(map[string]any{
		"deleteIssueFieldValue": map[string]any{
			"issue": map[string]any{"number": 42},
		},
	})

	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						IssueFieldValues struct {
							Nodes []IssueFieldValueFragment
						} `graphql:"issueFieldValues(first: 25)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":  githubv4.String("owner"),
				"repo":   githubv4.String("repo"),
				"number": githubv4.Int(42),
			},
			existingFieldsResponse,
		),
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						ID githubv4.ID
					} `graphql:"issue(number: $issueNumber)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":       githubv4.String("owner"),
				"repo":        githubv4.String("repo"),
				"issueNumber": githubv4.Int(42),
			},
			issueIDResponse,
		),
		githubv4mock.NewMutationMatcher(
			struct {
				DeleteIssueFieldValue struct {
					Issue struct {
						Number githubv4.Int
					}
				} `graphql:"deleteIssueFieldValue(input: $input)"`
			}{},
			DeleteIssueFieldValueInput{
				IssueID: githubv4.ID("I_node_42"),
				FieldID: githubv4.ID("IFMS_node_101"),
			},
			nil,
			deleteResponse,
		),
	))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		gqlClient,
		"owner", "repo", 42,
		"", "", nil, nil, 0, "",
		nil,
		[]fieldDeletion{{DatabaseID: 101, NodeID: githubv4.ID("IFMS_node_101")}},
		"", "", 0,
	)
	require.NoError(t, err)
	if result.IsError {
		t.Fatalf("expected non-error result, got: %s", getTextResult(t, result).Text)
	}
}

// (intentionally no further helpers)

// Test_UpdateIssue_DeleteAndSetFieldsInSameCall verifies that a single UpdateIssue
// can delete one issue field while setting another in the same request: the REST PATCH
// must carry only the set operation (no null clearing for the deleted field), and the
// deleteIssueFieldValue GraphQL mutation must fire for the deletion.
func Test_UpdateIssue_DeleteAndSetFieldsInSameCall(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  gogithub.Ptr(42),
		Title:   gogithub.Ptr("Test issue"),
		State:   gogithub.Ptr("open"),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/42"),
	}

	// Verify the PATCH body carries only the kept set operation. Crucially, it must
	// NOT include the deleted Components field (omitting != sending null), and it
	// must NOT double-clear the deleted field — the GraphQL mutation owns that path.
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"issue_field_values": []any{
				map[string]any{"field_id": float64(202), "value": "P2"},
			},
		}).andThen(
			mockResponse(t, http.StatusOK, mockIssue),
		),
	}))

	// Existing issue carries both fields. After merge + deletion filter, only the
	// Priority "set" entry should remain in the REST payload.
	existingFieldsResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"issueFieldValues": map[string]any{
					"nodes": []any{
						map[string]any{
							"__typename": "IssueFieldMultiSelectValue",
							"field": map[string]any{
								"fullDatabaseId": "101",
								"name":           "Components",
							},
							"options": []any{
								map[string]any{"name": "Auth"},
								map[string]any{"name": "Billing"},
							},
						},
						map[string]any{
							"__typename": "IssueFieldSingleSelectValue",
							"field": map[string]any{
								"fullDatabaseId": "202",
								"name":           "Priority",
							},
							"value": "P1",
						},
					},
				},
			},
		},
	})

	issueIDResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{"id": "I_node_42"},
		},
	})

	// The matcher fails the test if the mutation doesn't fire with the right
	// (issueId, fieldId) pair — proves the delete side of the combined update ran.
	deleteResponse := githubv4mock.DataResponse(map[string]any{
		"deleteIssueFieldValue": map[string]any{
			"issue": map[string]any{"number": 42},
		},
	})

	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						IssueFieldValues struct {
							Nodes []IssueFieldValueFragment
						} `graphql:"issueFieldValues(first: 25)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":  githubv4.String("owner"),
				"repo":   githubv4.String("repo"),
				"number": githubv4.Int(42),
			},
			existingFieldsResponse,
		),
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						ID githubv4.ID
					} `graphql:"issue(number: $issueNumber)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":       githubv4.String("owner"),
				"repo":        githubv4.String("repo"),
				"issueNumber": githubv4.Int(42),
			},
			issueIDResponse,
		),
		githubv4mock.NewMutationMatcher(
			struct {
				DeleteIssueFieldValue struct {
					Issue struct {
						Number githubv4.Int
					}
				} `graphql:"deleteIssueFieldValue(input: $input)"`
			}{},
			DeleteIssueFieldValueInput{
				IssueID: githubv4.ID("I_node_42"),
				FieldID: githubv4.ID("IFMS_node_101"),
			},
			nil,
			deleteResponse,
		),
	))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		gqlClient,
		"owner", "repo", 42,
		"", "", nil, nil, 0, "",
		[]*gogithub.IssueRequestFieldValue{
			{FieldID: 202, Value: "P2"},
		},
		[]fieldDeletion{{DatabaseID: 101, NodeID: githubv4.ID("IFMS_node_101")}},
		"", "", 0,
	)
	require.NoError(t, err)
	if result.IsError {
		t.Fatalf("expected non-error result, got: %s", getTextResult(t, result).Text)
	}
}
