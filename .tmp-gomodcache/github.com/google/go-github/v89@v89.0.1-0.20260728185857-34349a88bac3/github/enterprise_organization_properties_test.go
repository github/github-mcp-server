// Copyright 2025 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_GetOrganizationCustomPropertySchema(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/org-properties/schema", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"properties": [{
				"property_name": "team",
				"value_type": "string",
				"description": "Team name"
			}]
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.GetOrganizationCustomPropertySchema(ctx, "e")
	if err != nil {
		t.Fatalf("Enterprise.GetOrganizationCustomPropertySchema returned error: %v", err)
	}

	want := &EnterpriseCustomPropertySchema{
		Properties: []*CustomProperty{
			{
				PropertyName: Ptr("team"),
				ValueType:    PropertyValueTypeString,
				Description:  Ptr("Team name"),
			},
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.GetOrganizationCustomPropertySchema = %+v, want %+v", got, want)
	}

	const methodName = "GetOrganizationCustomPropertySchema"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.GetOrganizationCustomPropertySchema(ctx, "\n")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetOrganizationCustomPropertySchema(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateOrUpdateOrganizationCustomPropertySchema(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/org-properties/schema", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{}`)
	})

	ctx := t.Context()
	schema := EnterpriseCustomPropertySchema{
		Properties: []*CustomProperty{{PropertyName: Ptr("team")}},
	}
	_, err := client.Enterprise.CreateOrUpdateOrganizationCustomPropertySchema(ctx, "e", schema)
	if err != nil {
		t.Errorf("Enterprise.CreateOrUpdateOrganizationCustomPropertySchema returned error: %v", err)
	}

	const methodName = "CreateOrUpdateOrganizationCustomPropertySchema"
	testBadOptions(t, methodName, func() error {
		_, err := client.Enterprise.CreateOrUpdateOrganizationCustomPropertySchema(ctx, "\n", schema)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.CreateOrUpdateOrganizationCustomPropertySchema(ctx, "e", schema)
	})
}

func TestEnterpriseService_GetOrganizationCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/org-properties/schema/prop", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"property_name": "team",
			"value_type": "string",
			"description": "Team name"
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.GetOrganizationCustomProperty(ctx, "e", "prop")
	if err != nil {
		t.Fatalf("Enterprise.GetOrganizationCustomProperty returned error: %v", err)
	}

	want := &CustomProperty{
		PropertyName: Ptr("team"),
		ValueType:    PropertyValueTypeString,
		Description:  Ptr("Team name"),
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.GetOrganizationCustomProperty = %+v, want %+v", got, want)
	}

	const methodName = "GetOrganizationCustomProperty"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.GetOrganizationCustomProperty(ctx, "\n", "prop")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetOrganizationCustomProperty(ctx, "e", "prop")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateOrUpdateOrganizationCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/org-properties/schema/prop", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{}`)
	})

	ctx := t.Context()
	property := CustomProperty{PropertyName: Ptr("team")}
	_, err := client.Enterprise.CreateOrUpdateOrganizationCustomProperty(ctx, "e", "prop", property)
	if err != nil {
		t.Errorf("Enterprise.CreateOrUpdateOrganizationCustomProperty returned error: %v", err)
	}

	const methodName = "CreateOrUpdateOrganizationCustomProperty"
	testBadOptions(t, methodName, func() error {
		_, err := client.Enterprise.CreateOrUpdateOrganizationCustomProperty(ctx, "\n", "prop", property)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.CreateOrUpdateOrganizationCustomProperty(ctx, "e", "prop", property)
	})
}

func TestEnterpriseService_DeleteOrganizationCustomProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/org-properties/schema/prop", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteOrganizationCustomProperty(ctx, "e", "prop")
	if err != nil {
		t.Errorf("Enterprise.DeleteOrganizationCustomProperty returned error: %v", err)
	}

	const methodName = "DeleteOrganizationCustomProperty"
	testBadOptions(t, methodName, func() error {
		_, err := client.Enterprise.DeleteOrganizationCustomProperty(ctx, "\n", "prop")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteOrganizationCustomProperty(ctx, "e", "prop")
	})
}

func TestEnterpriseService_ListOrganizationCustomPropertyValues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/org-properties/values", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{
			"organization_id": 1,
			"organization_login": "org1",
			"properties": [
				{"property_name": "team", "value": "core"}
			]
		}]`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 10}
	got, _, err := client.Enterprise.ListOrganizationCustomPropertyValues(ctx, "e", opts)
	if err != nil {
		t.Fatalf("Enterprise.ListOrganizationCustomPropertyValues returned error: %v", err)
	}

	want := []*EnterpriseCustomPropertiesValues{
		{
			OrganizationID:    Ptr(int64(1)),
			OrganizationLogin: Ptr("org1"),
			Properties: []*CustomPropertyValue{
				{PropertyName: "team", Value: "core"},
			},
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.ListOrganizationCustomPropertyValues = %+v, want %+v", got, want)
	}

	const methodName = "ListEnterpriseCustomPropertyValues"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.ListOrganizationCustomPropertyValues(ctx, "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListOrganizationCustomPropertyValues(ctx, "e", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateOrUpdateOrganizationCustomPropertyValues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/org-properties/values", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{}`)
	})

	ctx := t.Context()
	values := []*CustomPropertyValue{{PropertyName: "team", Value: Ptr("core")}}
	orgs := []string{"org1"}

	opts := EnterpriseCustomPropertyValuesRequest{
		OrganizationLogin: orgs,
		Properties:        values,
	}
	_, err := client.Enterprise.CreateOrUpdateOrganizationCustomPropertyValues(ctx, "e", opts)
	if err != nil {
		t.Errorf("Enterprise.CreateOrUpdateOrganizationCustomPropertyValues returned error: %v", err)
	}

	const methodName = "CreateOrUpdateOrganizationCustomPropertyValues"
	testBadOptions(t, methodName, func() error {
		_, err := client.Enterprise.CreateOrUpdateOrganizationCustomPropertyValues(ctx, "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.CreateOrUpdateOrganizationCustomPropertyValues(ctx, "e", opts)
	})
}
