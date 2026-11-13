package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func multiSelectField() IssueField {
	return IssueField{
		ID:       "IFMS_1",
		Name:     "Components",
		DataType: "MULTI_SELECT",
		Options: []IssueSingleSelectFieldOption{
			{ID: "OPT_AUTH", Name: "Auth"},
			{ID: "OPT_BILL", Name: "Billing"},
			{ID: "OPT_API", Name: "API"},
		},
	}
}

func Test_resolveFieldFilters_MultiSelect(t *testing.T) {
	t.Parallel()

	t.Run("matches canonical option names", func(t *testing.T) {
		out, err := resolveFieldFilters([]rawFieldFilter{{
			Name:      "Components",
			Values:    []string{"  auth  ", "BILLING"},
			HasValues: true,
		}}, []IssueField{multiSelectField()})
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.NotNil(t, out[0].MultiSelectOptionValues)
		assert.Equal(t, []githubv4.String{"Auth", "Billing"}, *out[0].MultiSelectOptionValues)
	})

	tests := []struct {
		name    string
		fields  []IssueField
		filter  rawFieldFilter
		wantErr string
	}{
		{
			name:    "rejects unknown option",
			fields:  []IssueField{multiSelectField()},
			filter:  rawFieldFilter{Name: "Components", Values: []string{"Nope"}, HasValues: true},
			wantErr: "\"Nope\" is not a valid option",
		},
		{
			name:    "rejects scalar value",
			fields:  []IssueField{multiSelectField()},
			filter:  rawFieldFilter{Name: "Components", Value: "Auth", HasValue: true},
			wantErr: "is multi_select, use 'values'",
		},
		{
			name: "rejects values for single select",
			fields: []IssueField{{
				Name:     "Priority",
				DataType: "SINGLE_SELECT",
				Options:  []IssueSingleSelectFieldOption{{Name: "P1"}},
			}},
			filter:  rawFieldFilter{Name: "Priority", Values: []string{"P1"}, HasValues: true},
			wantErr: "is single_select, use 'value'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveFieldFilters([]rawFieldFilter{tt.filter}, tt.fields)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func Test_fragmentToMinimalFieldValue_MultiSelect(t *testing.T) {
	t.Parallel()

	fv := IssueFieldValueFragment{TypeName: "IssueFieldMultiSelectValue"}
	fv.MultiSelectValue.Field.MultiSelect.Name = "Components"
	fv.MultiSelectValue.Options = []struct {
		Name githubv4.String
	}{{Name: "Auth"}, {Name: "Billing"}}

	value, ok := fragmentToMinimalFieldValue(fv)
	require.True(t, ok)
	assert.Equal(t, MinimalFieldValue{
		Field:  "Components",
		Values: []string{"Auth", "Billing"},
	}, value)
}

func Test_optionalIssueWriteFields_MultiSelect(t *testing.T) {
	t.Parallel()

	fields, err := optionalIssueWriteFields(map[string]any{
		"issue_fields": []any{map[string]any{
			"field_name":         "Components",
			"field_option_names": []any{"Auth", "Billing"},
		}},
	})
	require.NoError(t, err)
	require.Len(t, fields, 1)
	assert.Equal(t, []string{"Auth", "Billing"}, fields[0].FieldOptionNames)

	tests := []struct {
		name    string
		item    map[string]any
		wantErr string
	}{
		{
			name:    "rejects empty options",
			item:    map[string]any{"field_name": "Components", "field_option_names": []any{}},
			wantErr: "use 'delete: true'",
		},
		{
			name: "rejects multiple setters",
			item: map[string]any{
				"field_name":         "Components",
				"value":              "Auth",
				"field_option_names": []any{"Auth"},
			},
			wantErr: "cannot specify more than one",
		},
		{
			name: "rejects delete with options",
			item: map[string]any{
				"field_name":         "Components",
				"delete":             true,
				"field_option_names": []any{"Auth"},
			},
			wantErr: "cannot specify 'delete' together with",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := optionalIssueWriteFields(map[string]any{"issue_fields": []any{tt.item}})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func Test_resolveIssueRequestFieldValues_MultiSelect(t *testing.T) {
	t.Parallel()

	newClient := func() *githubv4.Client {
		response := githubv4mock.DataResponse(map[string]any{
			"repository": map[string]any{
				"issueFields": map[string]any{
					"nodes": []any{map[string]any{
						"__typename":     "IssueFieldMultiSelect",
						"fullDatabaseId": "101",
						"name":           "Components",
						"dataType":       "multi_select",
						"options": []any{
							map[string]any{"name": "Auth"},
							map[string]any{"name": "Billing"},
						},
					}},
				},
			},
		})
		return githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				issueFieldWriteMetadataQuery{},
				map[string]any{
					"owner": githubv4.String("owner"),
					"repo":  githubv4.String("repo"),
				},
				response,
			),
		))
	}

	values, _, err := resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", []issueWriteFieldInput{{
		FieldName:        "Components",
		FieldOptionNames: []string{"auth", "BILLING"},
	}})
	require.NoError(t, err)
	require.Len(t, values, 1)
	assert.Equal(t, []string{"Auth", "Billing"}, values[0].Value)

	_, _, err = resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", []issueWriteFieldInput{{
		FieldName:        "Components",
		FieldOptionNames: []string{"Nope"},
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nope")

	_, _, err = resolveIssueRequestFieldValues(context.Background(), newClient(), "owner", "repo", []issueWriteFieldInput{{
		FieldName:       "Components",
		FieldOptionName: "Auth",
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field_option_names")
}
