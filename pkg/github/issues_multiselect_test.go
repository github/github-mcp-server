package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
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

	t.Run("trims surrounding whitespace when matching options", func(t *testing.T) {
		raw := []rawFieldFilter{{
			Name:      "Components",
			Values:    []string{"  auth  ", " Billing"},
			HasValues: true,
		}}
		out, err := resolveFieldFilters(raw, fields)
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.NotNil(t, out[0].MultiSelectOptionValues)
		assert.Equal(t, []githubv4.String{"Auth", "Billing"}, *out[0].MultiSelectOptionValues)
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
		assert.Equal(t, int64(101), deletions[0],
			"the resolver returns the database field ID; the REST DELETE endpoint takes this integer in its URL path")
	})
}
