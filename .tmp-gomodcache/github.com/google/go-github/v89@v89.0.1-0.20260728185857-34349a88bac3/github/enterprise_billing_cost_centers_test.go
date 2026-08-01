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

func TestEnterpriseService_ListCostCenters(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/settings/billing/cost-centers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"state": "active",
		})
		fmt.Fprint(w, `{
			"costCenters": [
				{
					"id": "2eeb8ffe-6903-11ee-8c99-0242ac120002",
					"name": "Cost Center Name",
					"state": "active",
					"azure_subscription": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
					"resources": [
						{
							"type": "User",
							"name": "Monalisa"
						},
						{
							"type": "Repo",
							"name": "octocat/hello-world"
						}
					]
				},
				{
					"id": "3ffb9ffe-6903-11ee-8c99-0242ac120003",
					"name": "Another Cost Center",
					"state": "active",
					"resources": [
						{
							"type": "User",
							"name": "Octocat"
						}
					]
				}
			]
		}`)
	})

	ctx := t.Context()
	opts := &ListCostCenterOptions{
		State: Ptr("active"),
	}
	costCenters, _, err := client.Enterprise.ListCostCenters(ctx, "e", opts)
	if err != nil {
		t.Errorf("Enterprise.ListCostCenters returned error: %v", err)
	}

	want := &CostCenters{
		CostCenters: []*CostCenter{
			{
				ID:                "2eeb8ffe-6903-11ee-8c99-0242ac120002",
				Name:              "Cost Center Name",
				State:             Ptr("active"),
				AzureSubscription: Ptr("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
				Resources: []*CostCenterResource{
					{
						Type: "User",
						Name: "Monalisa",
					},
					{
						Type: "Repo",
						Name: "octocat/hello-world",
					},
				},
			},
			{
				ID:    "3ffb9ffe-6903-11ee-8c99-0242ac120003",
				Name:  "Another Cost Center",
				State: Ptr("active"),
				Resources: []*CostCenterResource{
					{
						Type: "User",
						Name: "Octocat",
					},
				},
			},
		},
	}
	if !cmp.Equal(costCenters, want) {
		t.Errorf("Enterprise.ListCostCenters returned %+v, want %+v", costCenters, want)
	}

	const methodName = "ListCostCenters"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListCostCenters(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListCostCenters(ctx, "e", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListCostCenters_invalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.ListCostCenters(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestEnterpriseService_CreateCostCenter(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := CostCenterRequest{
		Name: "Engineering Team",
	}

	mux.HandleFunc("/enterprises/e/settings/billing/cost-centers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, req)
		fmt.Fprint(w, `{
			"id": "abc123",
			"name": "Engineering Team",
			"resources": []
		}`)
	})

	ctx := t.Context()
	costCenter, _, err := client.Enterprise.CreateCostCenter(ctx, "e", req)
	if err != nil {
		t.Errorf("Enterprise.CreateCostCenter returned error: %v", err)
	}

	want := &CostCenter{
		ID:        "abc123",
		Name:      "Engineering Team",
		Resources: []*CostCenterResource{},
	}
	if !cmp.Equal(costCenter, want) {
		t.Errorf("Enterprise.CreateCostCenter returned %+v, want %+v", costCenter, want)
	}

	const methodName = "CreateCostCenter"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.CreateCostCenter(ctx, "\n", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateCostCenter(ctx, "e", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateCostCenter_invalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.CreateCostCenter(ctx, "%", CostCenterRequest{})
	testURLParseError(t, err)
}

func TestEnterpriseService_GetCostCenter(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/settings/billing/cost-centers/2eeb8ffe-6903-11ee-8c99-0242ac120002", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": "2eeb8ffe-6903-11ee-8c99-0242ac120002",
			"name": "Cost Center Name",
			"state": "active",
			"azure_subscription": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"resources": [
				{
					"type": "User",
					"name": "Monalisa"
				},
				{
					"type": "Repo",
					"name": "octocat/hello-world"
				}
			]
		}`)
	})

	ctx := t.Context()
	costCenter, _, err := client.Enterprise.GetCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
	if err != nil {
		t.Errorf("Enterprise.GetCostCenter returned error: %v", err)
	}

	want := &CostCenter{
		ID:                "2eeb8ffe-6903-11ee-8c99-0242ac120002",
		Name:              "Cost Center Name",
		State:             Ptr("active"),
		AzureSubscription: Ptr("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		Resources: []*CostCenterResource{
			{
				Type: "User",
				Name: "Monalisa",
			},
			{
				Type: "Repo",
				Name: "octocat/hello-world",
			},
		},
	}
	if !cmp.Equal(costCenter, want) {
		t.Errorf("Enterprise.GetCostCenter returned %+v, want %+v", costCenter, want)
	}

	const methodName = "GetCostCenter"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetCostCenter(ctx, "\n", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetCostCenter_invalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.GetCostCenter(ctx, "%", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
	testURLParseError(t, err)
}

func TestEnterpriseService_UpdateCostCenter(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := CostCenterRequest{
		Name: "Updated Cost Center Name",
	}

	mux.HandleFunc("/enterprises/e/settings/billing/cost-centers/2eeb8ffe-6903-11ee-8c99-0242ac120002", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, req)
		fmt.Fprint(w, `{
			"id": "2eeb8ffe-6903-11ee-8c99-0242ac120002",
			"name": "Updated Cost Center Name",
			"state": "active",
			"azure_subscription": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"resources": [
				{
					"type": "User",
					"name": "Monalisa"
				},
				{
					"type": "Repo",
					"name": "octocat/hello-world"
				}
			]
		}`)
	})

	ctx := t.Context()
	costCenter, _, err := client.Enterprise.UpdateCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
	if err != nil {
		t.Errorf("Enterprise.UpdateCostCenter returned error: %v", err)
	}

	want := &CostCenter{
		ID:                "2eeb8ffe-6903-11ee-8c99-0242ac120002",
		Name:              "Updated Cost Center Name",
		State:             Ptr("active"),
		AzureSubscription: Ptr("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
		Resources: []*CostCenterResource{
			{
				Type: "User",
				Name: "Monalisa",
			},
			{
				Type: "Repo",
				Name: "octocat/hello-world",
			},
		},
	}
	if !cmp.Equal(costCenter, want) {
		t.Errorf("Enterprise.UpdateCostCenter returned %+v, want %+v", costCenter, want)
	}

	const methodName = "UpdateCostCenter"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.UpdateCostCenter(ctx, "\n", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateCostCenter_invalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.UpdateCostCenter(ctx, "%", "2eeb8ffe-6903-11ee-8c99-0242ac120002", CostCenterRequest{})
	testURLParseError(t, err)
}

func TestEnterpriseService_DeleteCostCenter(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/settings/billing/cost-centers/2eeb8ffe-6903-11ee-8c99-0242ac120002", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		fmt.Fprint(w, `{
			"message": "Cost center successfully deleted.",
			"id": "2eeb8ffe-6903-11ee-8c99-0242ac120002",
			"name": "Engineering Team",
			"costCenterState": "CostCenterArchived"
		}`)
	})

	ctx := t.Context()
	result, _, err := client.Enterprise.DeleteCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
	if err != nil {
		t.Errorf("Enterprise.DeleteCostCenter returned error: %v", err)
	}

	want := &DeleteCostCenterResponse{
		Message:         "Cost center successfully deleted.",
		ID:              "2eeb8ffe-6903-11ee-8c99-0242ac120002",
		Name:            "Engineering Team",
		CostCenterState: "CostCenterArchived",
	}
	if !cmp.Equal(result, want) {
		t.Errorf("Enterprise.DeleteCostCenter returned %+v, want %+v", result, want)
	}

	const methodName = "DeleteCostCenter"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.DeleteCostCenter(ctx, "\n", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.DeleteCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteCostCenter_invalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.DeleteCostCenter(ctx, "%", "2eeb8ffe-6903-11ee-8c99-0242ac120002")
	testURLParseError(t, err)
}

func TestEnterpriseService_AddResourcesToCostCenter(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := CostCenterResourceRequest{
		Users: []string{"monalisa"},
	}

	mux.HandleFunc("/enterprises/e/settings/billing/cost-centers/2eeb8ffe-6903-11ee-8c99-0242ac120002/resource", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, req)
		fmt.Fprint(w, `{
			"message": "Resources successfully added to the cost center.",
			"reassigned_resources": [
				{
					"resource_type": "user",
					"name": "monalisa",
					"previous_cost_center": "old-cost-center"
				},
				{
					"resource_type": "organization",
					"name": "octo-org",
					"previous_cost_center": "another-cost-center"
				},
				{
					"resource_type": "repository",
					"name": "octo-repo",
					"previous_cost_center": "yet-another-cost-center"
				}
			]
		}`)
	})

	ctx := t.Context()
	result, _, err := client.Enterprise.AddResourcesToCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
	if err != nil {
		t.Errorf("Enterprise.AddResourcesToCostCenter returned error: %v", err)
	}

	want := &AddResourcesToCostCenterResponse{
		Message: Ptr("Resources successfully added to the cost center."),
		ReassignedResources: []*ReassignedResource{
			{
				ResourceType:       Ptr("user"),
				Name:               Ptr("monalisa"),
				PreviousCostCenter: Ptr("old-cost-center"),
			},
			{
				ResourceType:       Ptr("organization"),
				Name:               Ptr("octo-org"),
				PreviousCostCenter: Ptr("another-cost-center"),
			},
			{
				ResourceType:       Ptr("repository"),
				Name:               Ptr("octo-repo"),
				PreviousCostCenter: Ptr("yet-another-cost-center"),
			},
		},
	}
	if !cmp.Equal(result, want) {
		t.Errorf("Enterprise.AddResourcesToCostCenter returned %+v, want %+v", result, want)
	}

	const methodName = "AddResourcesToCostCenter"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.AddResourcesToCostCenter(ctx, "\n", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.AddResourcesToCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_AddResourcesToCostCenter_invalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.AddResourcesToCostCenter(ctx, "%", "2eeb8ffe-6903-11ee-8c99-0242ac120002", CostCenterResourceRequest{})
	testURLParseError(t, err)
}

func TestEnterpriseService_RemoveResourcesFromCostCenter(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := CostCenterResourceRequest{
		Users: []string{"monalisa"},
	}

	mux.HandleFunc("/enterprises/e/settings/billing/cost-centers/2eeb8ffe-6903-11ee-8c99-0242ac120002/resource", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, req)
		fmt.Fprint(w, `{
			"message": "Resources successfully removed from the cost center."
		}`)
	})

	ctx := t.Context()
	result, _, err := client.Enterprise.RemoveResourcesFromCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
	if err != nil {
		t.Errorf("Enterprise.RemoveResourcesFromCostCenter returned error: %v", err)
	}

	want := &RemoveResourcesFromCostCenterResponse{
		Message: Ptr("Resources successfully removed from the cost center."),
	}
	if !cmp.Equal(result, want) {
		t.Errorf("Enterprise.RemoveResourcesFromCostCenter returned %+v, want %+v", result, want)
	}

	const methodName = "RemoveResourcesFromCostCenter"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.RemoveResourcesFromCostCenter(ctx, "\n", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.RemoveResourcesFromCostCenter(ctx, "e", "2eeb8ffe-6903-11ee-8c99-0242ac120002", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_RemoveResourcesFromCostCenter_invalidEnterprise(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Enterprise.RemoveResourcesFromCostCenter(ctx, "%", "2eeb8ffe-6903-11ee-8c99-0242ac120002", CostCenterResourceRequest{})
	testURLParseError(t, err)
}
