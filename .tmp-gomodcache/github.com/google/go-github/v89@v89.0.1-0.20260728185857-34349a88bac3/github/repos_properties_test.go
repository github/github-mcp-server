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
)

func TestRepositoriesService_GetAllCustomPropertyValues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/properties/values", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
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
		]`)
	})

	ctx := t.Context()
	customPropertyValues, _, err := client.Repositories.GetAllCustomPropertyValues(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetAllCustomPropertyValues returned error: %v", err)
	}

	want := []*CustomPropertyValue{
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
	}

	if !cmp.Equal(customPropertyValues, want) {
		t.Errorf("Repositories.GetAllCustomPropertyValues returned %+v, want %+v", customPropertyValues, want)
	}

	const methodName = "GetAllCustomPropertyValues"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetAllCustomPropertyValues(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateOrUpdateCustomProperties(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/usr/r/properties/values", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	repoCustomProperty := []*CustomPropertyValue{
		{
			PropertyName: "environment",
			Value:        "production",
		},
	}
	_, err := client.Repositories.CreateOrUpdateCustomProperties(ctx, "usr", "r", repoCustomProperty)
	if err != nil {
		t.Errorf("Repositories.CreateOrUpdateCustomProperties returned error: %v", err)
	}

	const methodName = "CreateOrUpdateCustomProperties"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.CreateOrUpdateCustomProperties(ctx, "usr", "r", repoCustomProperty)
	})
}
