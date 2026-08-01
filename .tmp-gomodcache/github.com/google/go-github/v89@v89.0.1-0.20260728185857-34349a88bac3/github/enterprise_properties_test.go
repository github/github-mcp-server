// Copyright 2024 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEnterpriseService_GetAllCustomProperties(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/properties/schema", func(w http.ResponseWriter, r *http.Request) {
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
          "property_name": "service",
          "value_type": "string"
        },
        {
          "property_name": "team",
          "value_type": "string",
          "description": "Team owning the repository"
        }
        ]`)
	})

	ctx := t.Context()
	properties, _, err := client.Enterprise.GetAllCustomProperties(ctx, "e")
	if err != nil {
		t.Errorf("Enterprise.GetAllCustomProperties returned error: %v", err)
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
			PropertyName: Ptr("service"),
			ValueType:    PropertyValueTypeString,
		},
		{
			PropertyName: Ptr("team"),
			ValueType:    PropertyValueTypeString,
			Description:  Ptr("Team owning the repository"),
		},
	}
	if !cmp.Equal(properties, want) {
		t.Errorf("Enterprise.GetAllCustomProperties returned %+v, want %+v", properties, want)
	}

	const methodName = "GetAllCustomProperties"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetAllCustomProperties(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateOrUpdateCustomProperties(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	properties := []*CustomProperty{
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

	mux.HandleFunc("/enterprises/e/properties/schema", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, struct {
			Properties []*CustomProperty `json:"properties"`
		}{
			Properties: properties,
		})
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
	properties, _, err := client.Enterprise.CreateOrUpdateCustomProperties(ctx, "e", properties)
	if err != nil {
		t.Errorf("Enterprise.CreateOrUpdateCustomProperties returned error: %v", err)
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
		t.Errorf("Enterprise.CreateOrUpdateCustomProperties returned %+v, want %+v", properties, want)
	}

	const methodName = "CreateOrUpdateCustomProperties"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateOrUpdateCustomProperties(ctx, "e", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/properties/schema/name", func(w http.ResponseWriter, r *http.Request) {
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
	property, _, err := client.Enterprise.GetCustomProperty(ctx, "e", "name")
	if err != nil {
		t.Errorf("Enterprise.GetCustomProperty returned error: %v", err)
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
		t.Errorf("Enterprise.GetCustomProperty returned %+v, want %+v", property, want)
	}

	const methodName = "GetCustomProperty"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetCustomProperty(ctx, "e", "name")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateOrUpdateCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/properties/schema/name", func(w http.ResponseWriter, r *http.Request) {
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
	property, _, err := client.Enterprise.CreateOrUpdateCustomProperty(ctx, "e", "name", &CustomProperty{
		ValueType:        PropertyValueTypeSingleSelect,
		Required:         Ptr(true),
		DefaultValue:     "production",
		Description:      Ptr("Prod or dev environment"),
		AllowedValues:    []string{"production", "development"},
		ValuesEditableBy: Ptr("org_actors"),
	})
	if err != nil {
		t.Errorf("Enterprise.CreateOrUpdateCustomProperty returned error: %v", err)
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
		t.Errorf("Enterprise.CreateOrUpdateCustomProperty returned %+v, want %+v", property, want)
	}

	const methodName = "CreateOrUpdateCustomProperty"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateOrUpdateCustomProperty(ctx, "e", "name", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_RemoveCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/properties/schema/name", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Enterprise.RemoveCustomProperty(ctx, "e", "name")
	if err != nil {
		t.Errorf("Enterprise.RemoveCustomProperty returned error: %v", err)
	}

	const methodName = "RemoveCustomProperty"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.RemoveCustomProperty(ctx, "e", "name")
	})
}
