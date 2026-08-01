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

func TestEnterpriseService_ListEnterpriseNetworkConfigurations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc(" /enterprises/e/network-configurations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "3", "per_page": "2"})
		fmt.Fprint(w, `{
		  "total_count": 2,
		  "network_configurations": [
			{
			  "id": "123456789ABCDEF",
			  "name": "configuration one",
			  "compute_service": "actions",
			  "network_settings_ids": [
				"23456789ABDCEF1",
				"3456789ABDCEF12"
			  ],
			  "created_on": `+referenceTimeStr+`
			},
			{
			  "id": "456789ABDCEF123",
			  "name": "configuration two",
			  "compute_service": "none",
			  "network_settings_ids": [
				"56789ABDCEF1234",
				"6789ABDCEF12345"
			  ],
			  "created_on": `+referenceTimeStr+`
			}
		  ]
		}`)
	})

	ctx := t.Context()

	opts := &ListOptions{Page: 3, PerPage: 2}
	configurations, _, err := client.Enterprise.ListEnterpriseNetworkConfigurations(ctx, "e", opts)
	if err != nil {
		t.Errorf("Enterprise.ListEnterpriseNetworkConfigurations returned error: %v", err)
	}

	want := &NetworkConfigurations{
		TotalCount: Ptr(int64(2)),
		NetworkConfigurations: []*NetworkConfiguration{
			{
				ID:                 Ptr("123456789ABCDEF"),
				Name:               Ptr("configuration one"),
				ComputeService:     Ptr(ComputeService("actions")),
				NetworkSettingsIDs: []string{"23456789ABDCEF1", "3456789ABDCEF12"},
				CreatedOn:          &referenceTimestamp,
			},
			{
				ID:                 Ptr("456789ABDCEF123"),
				Name:               Ptr("configuration two"),
				ComputeService:     Ptr(ComputeService("none")),
				NetworkSettingsIDs: []string{"56789ABDCEF1234", "6789ABDCEF12345"},
				CreatedOn:          &referenceTimestamp,
			},
		},
	}
	if !cmp.Equal(configurations, want) {
		t.Errorf("Enterprise.ListEnterpriseNetworkConfigurations mismatch (-want +got):\n%v", cmp.Diff(want, configurations))
	}

	const methodName = "ListEnterpriseNetworkConfigurations"
	testBadOptions(t, methodName, func() error {
		_, _, err = client.Enterprise.ListEnterpriseNetworkConfigurations(ctx, "\ne", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListEnterpriseNetworkConfigurations(ctx, "e", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateEnterpriseNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/network-configurations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
		  "id": "123456789ABCDEF",
		  "name": "configuration one",
		  "compute_service": "actions",
		  "network_settings_ids": [
			"23456789ABDCEF1"
		  ],
		  "created_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()

	req := NetworkConfigurationRequest{
		Name:           Ptr("configuration-one"),
		ComputeService: Ptr(ComputeService("actions")),
		NetworkSettingsIDs: []string{
			"23456789ABDCEF1",
		},
	}
	configuration, _, err := client.Enterprise.CreateEnterpriseNetworkConfiguration(ctx, "e", req)
	if err != nil {
		t.Errorf("Enterprise.CreateEnterpriseNetworkConfiguration returned error: %v", err)
	}

	want := &NetworkConfiguration{
		ID:                 Ptr("123456789ABCDEF"),
		Name:               Ptr("configuration one"),
		ComputeService:     Ptr(ComputeService("actions")),
		NetworkSettingsIDs: []string{"23456789ABDCEF1"},
		CreatedOn:          &referenceTimestamp,
	}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Enterprise.CreateEnterpriseNetworkConfiguration mismatch (-want +got):\n%v", cmp.Diff(want, configuration))
	}

	validationTest := []struct {
		name    string
		request NetworkConfigurationRequest
		want    string
	}{
		{
			name: "invalid network settings id",
			request: NetworkConfigurationRequest{
				Name:               Ptr(""),
				NetworkSettingsIDs: []string{"56789ABDCEF1234"},
			},
			want: "validation failed: must be between 1 and 100 characters",
		},
		{
			name: "invalid network settings id",
			request: NetworkConfigurationRequest{
				Name: Ptr("updated-configuration-one"),
			},
			want: "validation failed: exactly one network settings id must be specified",
		},
		{
			name: "invalid compute service",
			request: NetworkConfigurationRequest{
				Name:               Ptr("updated-configuration-one"),
				ComputeService:     Ptr(ComputeService("")),
				NetworkSettingsIDs: []string{"56789ABDCEF1234"},
			},
			want: "validation failed: compute service can only be one of: none, actions",
		},
	}
	for _, tc := range validationTest {
		_, _, err = client.Enterprise.CreateEnterpriseNetworkConfiguration(ctx, "e", tc.request)
		if err == nil || err.Error() != tc.want {
			t.Errorf("expected error to be %v, got %v", tc.want, err)
		}
	}

	const methodName = "CreateEnterpriseNetworkConfiguration"
	testBadOptions(t, methodName, func() error {
		_, _, err = client.Enterprise.CreateEnterpriseNetworkConfiguration(ctx, "\ne", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateEnterpriseNetworkConfiguration(ctx, "e", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetEnterpriseNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/network-configurations/123456789ABCDEF", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
		  "id": "123456789ABCDEF",
		  "name": "configuration one",
		  "compute_service": "actions",
		  "network_settings_ids": [
			"23456789ABDCEF1",
			"3456789ABDCEF12"
		  ],
		  "created_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()
	configuration, _, err := client.Enterprise.GetEnterpriseNetworkConfiguration(ctx, "e", "123456789ABCDEF")
	if err != nil {
		t.Errorf("Enterprise.GetEnterpriseNetworkConfiguration returned err: %v", err)
	}

	want := &NetworkConfiguration{
		ID:                 Ptr("123456789ABCDEF"),
		Name:               Ptr("configuration one"),
		ComputeService:     Ptr(ComputeService("actions")),
		NetworkSettingsIDs: []string{"23456789ABDCEF1", "3456789ABDCEF12"},
		CreatedOn:          &referenceTimestamp,
	}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Enterprise.GetEnterpriseNetworkConfiguration mismatch (-want +got):\n%v", cmp.Diff(want, configuration))
	}

	const methodName = "GetEnterpriseNetworkConfiguration"
	testBadOptions(t, methodName, func() error {
		_, _, err = client.Enterprise.GetEnterpriseNetworkConfiguration(ctx, "\ne", "123456789ABCDEF")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetEnterpriseNetworkConfiguration(ctx, "e", "123456789ABCDEF")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateEnterpriseNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/network-configurations/123456789ABCDEF", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{
		  "id": "123456789ABCDEF",
		  "name": "updated configuration one",
		  "compute_service": "none",
		  "network_settings_ids": [
			"456789ABDCEF123"
		  ],
		  "created_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()
	req := NetworkConfigurationRequest{
		Name: Ptr("updated-configuration-one"),
		NetworkSettingsIDs: []string{
			"456789ABDCEF123",
		},
		ComputeService: Ptr(ComputeService("none")),
	}
	configuration, _, err := client.Enterprise.UpdateEnterpriseNetworkConfiguration(ctx, "e", "123456789ABCDEF", req)
	if err != nil {
		t.Errorf("Enterprise.UpdateEnterpriseNetworkConfiguration returned error %v", err)
	}

	want := &NetworkConfiguration{
		ID:                 Ptr("123456789ABCDEF"),
		Name:               Ptr("updated configuration one"),
		ComputeService:     Ptr(ComputeService("none")),
		NetworkSettingsIDs: []string{"456789ABDCEF123"},
		CreatedOn:          &referenceTimestamp,
	}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Enterprise.UpdateEnterpriseNetworkConfiguration mismatch (-want +get)\n%v", cmp.Diff(want, configuration))
	}

	validationTest := []struct {
		name    string
		request NetworkConfigurationRequest
		want    string
	}{
		{
			name: "invalid network settings id",
			request: NetworkConfigurationRequest{
				Name:               Ptr(""),
				NetworkSettingsIDs: []string{"56789ABDCEF1234"},
			},
			want: "validation failed: must be between 1 and 100 characters",
		},
		{
			name: "invalid network settings id",
			request: NetworkConfigurationRequest{
				Name: Ptr("updated-configuration-one"),
			},
			want: "validation failed: exactly one network settings id must be specified",
		},
		{
			name: "invalid compute service",
			request: NetworkConfigurationRequest{
				Name:               Ptr("updated-configuration-one"),
				ComputeService:     Ptr(ComputeService("something")),
				NetworkSettingsIDs: []string{"56789ABDCEF1234"},
			},
			want: "validation failed: compute service can only be one of: none, actions",
		},
	}
	for _, tc := range validationTest {
		_, _, err = client.Enterprise.UpdateEnterpriseNetworkConfiguration(ctx, "e", "123456789ABCDEF", tc.request)
		if err == nil || err.Error() != tc.want {
			t.Errorf("expected error to be %v, got %v", tc.want, err)
		}
	}

	const methodName = "UpdateEnterpriseNetworkConfiguration"
	testBadOptions(t, methodName, func() error {
		_, _, err = client.Enterprise.UpdateEnterpriseNetworkConfiguration(ctx, "\ne", "123456789ABCDEF", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateEnterpriseNetworkConfiguration(ctx, "e", "123456789ABCDEF", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteEnterpriseNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/network-configurations/123456789ABCDEF", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteEnterpriseNetworkConfiguration(ctx, "e", "123456789ABCDEF")
	if err != nil {
		t.Errorf("Enterprise.DeleteEnterpriseNetworkConfiguration returned error %v", err)
	}

	const methodName = "DeleteEnterpriseNetworkConfiguration"
	testBadOptions(t, methodName, func() error {
		_, err = client.Enterprise.DeleteEnterpriseNetworkConfiguration(ctx, "\ne", "123456789ABCDEF")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteEnterpriseNetworkConfiguration(ctx, "e", "123456789ABCDEF")
	})
}

func TestEnterpriseService_GetEnterpriseNetworkSettingsResource(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/network-settings/123456789ABCDEF", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": "220F78DACB92BBFBC5E6F22DE1CCF52309D",
			"network_configuration_id": "934E208B3EE0BD60CF5F752C426BFB53562",
			"name": "my_network_settings",
			"subnet_id": "/subscriptions/14839728-3ad9-43ab-bd2b-fa6ad0f75e2a/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet",
			"region": "germanywestcentral"
		}`)
	})

	ctx := t.Context()
	resource, _, err := client.Enterprise.GetEnterpriseNetworkSettingsResource(ctx, "e", "123456789ABCDEF")
	if err != nil {
		t.Errorf("Enterprise.GetEnterpriseNetworkSettingsResource returned error %v", err)
	}

	want := &NetworkSettingsResource{
		ID:                     Ptr("220F78DACB92BBFBC5E6F22DE1CCF52309D"),
		NetworkConfigurationID: Ptr("934E208B3EE0BD60CF5F752C426BFB53562"),
		Name:                   Ptr("my_network_settings"),
		SubnetID:               Ptr("/subscriptions/14839728-3ad9-43ab-bd2b-fa6ad0f75e2a/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
		Region:                 Ptr("germanywestcentral"),
	}
	if !cmp.Equal(resource, want) {
		t.Errorf("Enterprise.GetEnterpriseNetworkSettingsResource mismatch (-want +got):\n%v", cmp.Diff(want, resource))
	}

	const methodName = "GetEnterpriseNetworkSettingsResource"
	testBadOptions(t, methodName, func() error {
		_, _, err = client.Enterprise.GetEnterpriseNetworkSettingsResource(ctx, "\ne", "123456789ABCDEF")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetEnterpriseNetworkSettingsResource(ctx, "e", "123456789ABCDEF")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
