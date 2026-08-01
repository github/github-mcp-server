// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestOrganizationsService_GetAllCustomProperties(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/properties/schema", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
  {
    "property_name": "name",
    "value_type": "single_select",
    "required": true,
    "default_value": "production",
    "description": "Prod or dev environment",
    "allowed_values":[
      "production",
      "development"
    ],
    "values_editable_by": "org_actors"
  },
  {
    "property_name": "test",
    "value_type": "multi_select",
    "required": true,
    "default_value": [
      "foo",
      "baz"
    ],
    "description": "Prod or dev environment",
    "allowed_values":[
      "foo",
      "bar",
			"baz"
    ],
    "values_editable_by": "org_actors"
  },
  {
    "property_name": "service",
    "value_type": "string"
  },
  {
    "property_name": "team",
    "value_type": "string",
    "description": "Team owning the repository"
  },
  {
    "property_name": "documentation",
    "value_type": "url",
    "required": true,
    "description": "Link to the documentation",
    "default_value": "https://example.com/docs"
  }
]`)
	})

	ctx := t.Context()
	properties, _, err := client.Organizations.GetAllCustomProperties(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.GetAllCustomProperties returned error: %v", err)
	}

	want := []*CustomProperty{
		{
			PropertyName:     Ptr("name"),
			ValueType:        PropertyValueTypeSingleSelect,
			Required:         Ptr(true),
			DefaultValue:     "production",
			Description:      Ptr("Prod or dev environment"),
			AllowedValues:    []string{"production", "development"},
			ValuesEditableBy: Ptr("org_actors"),
		},
		{
			PropertyName:     Ptr("test"),
			ValueType:        PropertyValueTypeMultiSelect,
			Required:         Ptr(true),
			DefaultValue:     []any{"foo", "baz"},
			Description:      Ptr("Prod or dev environment"),
			AllowedValues:    []string{"foo", "bar", "baz"},
			ValuesEditableBy: Ptr("org_actors"),
		},
		{
			PropertyName: Ptr("service"),
			ValueType:    PropertyValueTypeString,
		},
		{
			PropertyName: Ptr("team"),
			ValueType:    PropertyValueTypeString,
			Description:  Ptr("Team owning the repository"),
		},
		{
			PropertyName: Ptr("documentation"),
			ValueType:    PropertyValueTypeURL,
			Required:     Ptr(true),
			Description:  Ptr("Link to the documentation"),
			DefaultValue: "https://example.com/docs",
		},
	}

	const methodName = "GetAllCustomProperties"

	if diff := cmp.Diff(want, properties); diff != "" {
		t.Errorf("Organizations.%v diff mismatch (-want +got):\n%v", methodName, diff)
	}

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetAllCustomProperties(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateOrUpdateCustomProperties(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := struct {
		Properties []*CustomProperty `json:"properties"`
	}{
		Properties: []*CustomProperty{
			{
				PropertyName: Ptr("name"),
				ValueType:    PropertyValueTypeSingleSelect,
				Required:     Ptr(true),
			},
			{
				PropertyName: Ptr("service"),
				ValueType:    PropertyValueTypeString,
			},
		},
	}

	mux.HandleFunc("/orgs/o/properties/schema", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[
		{
          "property_name": "name",
          "value_type": "single_select",
          "required": true
        },
        {
          "property_name": "service",
          "value_type": "string"
        }
        ]`)
	})

	ctx := t.Context()
	properties, _, err := client.Organizations.CreateOrUpdateCustomProperties(ctx, "o", input.Properties)
	if err != nil {
		t.Errorf("Organizations.CreateOrUpdateCustomProperties returned error: %v", err)
	}

	want := []*CustomProperty{
		{
			PropertyName: Ptr("name"),
			ValueType:    PropertyValueTypeSingleSelect,
			Required:     Ptr(true),
		},
		{
			PropertyName: Ptr("service"),
			ValueType:    PropertyValueTypeString,
		},
	}

	if !cmp.Equal(properties, want) {
		t.Errorf("Organizations.CreateOrUpdateCustomProperties returned %+v, want %+v", properties, want)
	}

	const methodName = "CreateOrUpdateCustomProperties"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateOrUpdateCustomProperties(ctx, "o", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/properties/schema/name", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
		"property_name": "name",
		"value_type": "single_select",
		"required": true,
		"default_value": "production",
		"description": "Prod or dev environment",
		"allowed_values":[
		  "production",
		  "development"
		],
		"values_editable_by": "org_actors"
	  }`)
	})

	ctx := t.Context()
	property, _, err := client.Organizations.GetCustomProperty(ctx, "o", "name")
	if err != nil {
		t.Errorf("Organizations.GetCustomProperty returned error: %v", err)
	}

	want := &CustomProperty{
		PropertyName:     Ptr("name"),
		ValueType:        PropertyValueTypeSingleSelect,
		Required:         Ptr(true),
		DefaultValue:     "production",
		Description:      Ptr("Prod or dev environment"),
		AllowedValues:    []string{"production", "development"},
		ValuesEditableBy: Ptr("org_actors"),
	}
	if !cmp.Equal(property, want) {
		t.Errorf("Organizations.GetCustomProperty returned %+v, want %+v", property, want)
	}

	const methodName = "GetCustomProperty"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetCustomProperty(ctx, "o", "name")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateOrUpdateCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/properties/schema/name", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{
		"property_name": "name",
		"value_type": "single_select",
		"required": true,
		"default_value": "production",
		"description": "Prod or dev environment",
		"allowed_values":[
		  "production",
		  "development"
		],
		"values_editable_by": "org_actors"
	  }`)
	})

	ctx := t.Context()
	property, _, err := client.Organizations.CreateOrUpdateCustomProperty(ctx, "o", "name", &CustomProperty{
		ValueType:        PropertyValueTypeSingleSelect,
		Required:         Ptr(true),
		DefaultValue:     "production",
		Description:      Ptr("Prod or dev environment"),
		AllowedValues:    []string{"production", "development"},
		ValuesEditableBy: Ptr("org_actors"),
	})
	if err != nil {
		t.Errorf("Organizations.CreateOrUpdateCustomProperty returned error: %v", err)
	}

	want := &CustomProperty{
		PropertyName:     Ptr("name"),
		ValueType:        PropertyValueTypeSingleSelect,
		Required:         Ptr(true),
		DefaultValue:     "production",
		Description:      Ptr("Prod or dev environment"),
		AllowedValues:    []string{"production", "development"},
		ValuesEditableBy: Ptr("org_actors"),
	}
	if !cmp.Equal(property, want) {
		t.Errorf("Organizations.CreateOrUpdateCustomProperty returned %+v, want %+v", property, want)
	}

	const methodName = "CreateOrUpdateCustomProperty"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateOrUpdateCustomProperty(ctx, "o", "name", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_RemoveCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/properties/schema/name", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.RemoveCustomProperty(ctx, "o", "name")
	if err != nil {
		t.Errorf("Organizations.RemoveCustomProperty returned error: %v", err)
	}

	const methodName = "RemoveCustomProperty"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.RemoveCustomProperty(ctx, "0", "name")
	})
}

func TestOrganizationsService_ListCustomPropertyValues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/properties/values", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":             "1",
			"per_page":         "100",
			"repository_query": "repo:octocat/Hello-World",
		})
		fmt.Fprint(w, `[{
		"repository_id": 1296269,
		"repository_name": "Hello-World",
		"repository_full_name": "octocat/Hello-World",
		"properties": [
		{
          "property_name": "environment",
          "value": "production"
        },
        {
          "property_name": "service",
          "value": "web"
        },
        {
          "property_name": "languages",
          "value": ["Go", "JavaScript"]
        },
        {
          "property_name": "null_property",
          "value": null
        }
		]
        }]`)
	})

	ctx := t.Context()
	repoPropertyValues, _, err := client.Organizations.ListCustomPropertyValues(ctx, "o", &ListCustomPropertyValuesOptions{
		ListOptions: ListOptions{
			Page:    1,
			PerPage: 100,
		},
		RepositoryQuery: "repo:octocat/Hello-World",
	})
	if err != nil {
		t.Errorf("Organizations.ListCustomPropertyValues returned error: %v", err)
	}

	want := []*RepoCustomPropertyValue{
		{
			RepositoryID:       1296269,
			RepositoryName:     "Hello-World",
			RepositoryFullName: "octocat/Hello-World",
			Properties: []*CustomPropertyValue{
				{
					PropertyName: "environment",
					Value:        "production",
				},
				{
					PropertyName: "service",
					Value:        "web",
				},
				{
					PropertyName: "languages",
					Value:        []string{"Go", "JavaScript"},
				},
				{
					PropertyName: "null_property",
					Value:        nil,
				},
			},
		},
	}

	if !cmp.Equal(repoPropertyValues, want) {
		t.Errorf("Organizations.ListCustomPropertyValues returned %+v, want %+v", repoPropertyValues, want)
	}

	const methodName = "ListCustomPropertyValues"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListCustomPropertyValues(ctx, "\n", &ListCustomPropertyValuesOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListCustomPropertyValues(ctx, "o", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCustomPropertyValue_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		data    string
		want    *CustomPropertyValue
		wantErr bool
	}{
		"Invalid JSON": {
			data:    `{`,
			want:    &CustomPropertyValue{},
			wantErr: true,
		},
		"String value": {
			data: `{
				"property_name": "environment",
				"value": "production"
			}`,
			want: &CustomPropertyValue{
				PropertyName: "environment",
				Value:        "production",
			},
			wantErr: false,
		},
		"Array of strings value": {
			data: `{
				"property_name": "languages",
				"value": ["Go", "JavaScript"]
			}`,
			want: &CustomPropertyValue{
				PropertyName: "languages",
				Value:        []string{"Go", "JavaScript"},
			},
			wantErr: false,
		},
		"Non-string value in array": {
			data: `{
				"property_name": "languages",
				"value": ["Go", 42]
			}`,
			want: &CustomPropertyValue{
				PropertyName: "languages",
				Value:        nil,
			},
			wantErr: true,
		},
		"Unexpected value type": {
			data: `{
				"property_name": "environment",
				"value": {"invalid": "type"}
			}`,
			want: &CustomPropertyValue{
				PropertyName: "environment",
				Value:        nil,
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cpv := &CustomPropertyValue{}
			err := cpv.UnmarshalJSON([]byte(tc.data))
			if (err != nil) != tc.wantErr {
				t.Errorf("CustomPropertyValue.UnmarshalJSON error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr && !cmp.Equal(tc.want, cpv) {
				t.Errorf("CustomPropertyValue.UnmarshalJSON expected %+v, got %+v", tc.want, cpv)
			}
		})
	}
}

func TestOrganizationsService_CreateOrUpdateRepoCustomPropertyValues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := struct {
		RepositoryNames []string               `json:"repository_names"`
		Properties      []*CustomPropertyValue `json:"properties"`
	}{
		RepositoryNames: []string{"repo"},
		Properties: []*CustomPropertyValue{
			{
				PropertyName: "service",
				Value:        "string",
			},
		},
	}

	mux.HandleFunc("/orgs/o/properties/values", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
	})

	ctx := t.Context()
	_, err := client.Organizations.CreateOrUpdateRepoCustomPropertyValues(ctx, "o", input.RepositoryNames, input.Properties)
	if err != nil {
		t.Errorf("Organizations.CreateOrUpdateRepoCustomPropertyValues returned error: %v", err)
	}

	const methodName = "CreateOrUpdateRepoCustomPropertyValues"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.CreateOrUpdateRepoCustomPropertyValues(ctx, "o", nil, nil)
	})
}

func TestCustomPropertyDefaultValueString(t *testing.T) {
	t.Parallel()
	for _, d := range []struct {
		testName string
		property *CustomProperty
		ok       bool
		want     string
	}{
		{
			testName: "invalid_type",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeMultiSelect,
				DefaultValue: []string{"a", "b"},
			},
			ok:   false,
			want: "",
		},
		{
			testName: "string_invalid_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeString,
				DefaultValue: []string{"a", "b"},
			},
			ok:   false,
			want: "",
		},
		{
			testName: "string_nil_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeString,
				DefaultValue: nil,
			},
			ok:   false,
			want: "",
		},
		{
			testName: "string_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeString,
				DefaultValue: "test-string",
			},
			ok:   true,
			want: "test-string",
		},
		{
			testName: "single_select_invalid_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeSingleSelect,
				DefaultValue: []string{"a", "b"},
			},
			ok:   false,
			want: "",
		},
		{
			testName: "single_select_nil_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeSingleSelect,
				DefaultValue: nil,
			},
			ok:   false,
			want: "",
		},
		{
			testName: "single_select_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeSingleSelect,
				DefaultValue: "test-string",
			},
			ok:   true,
			want: "test-string",
		},
		{
			testName: "url_invalid_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeURL,
				DefaultValue: []string{"a", "b"},
			},
			ok:   false,
			want: "",
		},
		{
			testName: "url_nil_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeURL,
				DefaultValue: nil,
			},
			ok:   false,
			want: "",
		},
		{
			testName: "url_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeURL,
				DefaultValue: "http://example.com",
			},
			ok:   true,
			want: "http://example.com",
		},
	} {
		t.Run(d.testName, func(t *testing.T) {
			t.Parallel()
			got, ok := d.property.DefaultValueString()

			if ok != d.ok {
				t.Fatalf("CustomProperty.DefaultValueString set ok to %+v, want %+v", ok, d.ok)
			}

			if got != d.want {
				t.Fatalf("CustomProperty.DefaultValueString returned %+v, want %+v", got, d.want)
			}
		})
	}
}

func TestCustomPropertyDefaultValueStrings(t *testing.T) {
	t.Parallel()
	for _, d := range []struct {
		testName string
		property *CustomProperty
		ok       bool
		want     []string
	}{
		{
			testName: "invalid_type",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeString,
				DefaultValue: "test",
			},
			ok:   false,
			want: []string{},
		},
		{
			testName: "invalid_slice",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeString,
				DefaultValue: []any{1, 2, 3},
			},
			ok:   false,
			want: []string{},
		},
		{
			testName: "multi_select_invalid_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeMultiSelect,
				DefaultValue: "test",
			},
			ok:   false,
			want: []string{},
		},
		{
			testName: "multi_select_nil_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeMultiSelect,
				DefaultValue: nil,
			},
			ok:   false,
			want: []string{},
		},
		{
			testName: "multi_select_any_slice_single_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeMultiSelect,
				DefaultValue: []any{"a"},
			},
			ok:   true,
			want: []string{"a"},
		},
		{
			testName: "multi_select_string_slice_single_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeMultiSelect,
				DefaultValue: []string{"a"},
			},
			ok:   true,
			want: []string{"a"},
		},
		{
			testName: "multi_select_any_slice_multi_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeMultiSelect,
				DefaultValue: []any{"a", "b"},
			},
			ok:   true,
			want: []string{"a", "b"},
		},
		{
			testName: "multi_select_string_slice_multi_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeMultiSelect,
				DefaultValue: []string{"a", "b"},
			},
			ok:   true,
			want: []string{"a", "b"},
		},
	} {
		t.Run(d.testName, func(t *testing.T) {
			t.Parallel()
			got, ok := d.property.DefaultValueStrings()

			if ok != d.ok {
				t.Fatalf("CustomProperty.DefaultValueStrings set ok to %+v, want %+v", ok, d.ok)
			}

			if !cmp.Equal(got, d.want, cmpopts.EquateEmpty()) {
				t.Fatalf("CustomProperty.DefaultValueStrings returned %+v, want %+v", got, d.want)
			}
		})
	}
}

func TestCustomPropertyDefaultValueBool(t *testing.T) {
	t.Parallel()
	for _, d := range []struct {
		testName string
		property *CustomProperty
		ok       bool
		want     bool
	}{
		{
			testName: "invalid_type",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeString,
				DefaultValue: "test",
			},
			ok:   false,
			want: false,
		},
		{
			testName: "true_false_invalid_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeTrueFalse,
				DefaultValue: "test",
			},
			ok:   false,
			want: false,
		},
		{
			testName: "true_false_nil_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeTrueFalse,
				DefaultValue: nil,
			},
			ok:   false,
			want: false,
		},
		{
			testName: "true_false_true_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeTrueFalse,
				DefaultValue: "true",
			},
			ok:   true,
			want: true,
		},
		{
			testName: "true_false_false_value",
			property: &CustomProperty{
				ValueType:    PropertyValueTypeTrueFalse,
				DefaultValue: "false",
			},
			ok:   true,
			want: false,
		},
	} {
		t.Run(d.testName, func(t *testing.T) {
			t.Parallel()
			got, ok := d.property.DefaultValueBool()

			if ok != d.ok {
				t.Fatalf("CustomProperty.DefaultValueBool set ok to %+v, want %+v", ok, d.ok)
			}

			if ok != d.ok {
				t.Fatalf("CustomProperty.DefaultValueBool returned %+v, want %+v", got, d.want)
			}
		})
	}
}
