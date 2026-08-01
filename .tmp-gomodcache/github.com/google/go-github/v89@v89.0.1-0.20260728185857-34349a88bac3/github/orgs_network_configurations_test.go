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

func TestOrganizationsService_ListOrgsNetworkConfigurations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/settings/network-configurations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "1", "per_page": "3"})
		fmt.Fprint(w, `{
		  "total_count": 3,
		  "network_configurations": [
			{
			  "id": "123456789ABCDEF",
			  "name": "Network Configuration One",
			  "compute_service": "actions",
			  "network_settings_ids": [
				"23456789ABDCEF1",
				"3456789ABDCEF12"
			  ],
			  "created_on": `+referenceTimeStr+`
			},
			{
			  "id": "456789ABDCEF123",
			  "name": "Network Configuration Two",
			  "compute_service": "none",
			  "network_settings_ids": [
				"56789ABDCEF1234",
				"6789ABDCEF12345"
			  ],
			  "created_on": `+referenceTimeStr+`
			},
			{
			  "id": "789ABDCEF123456",
			  "name": "Network Configuration Three",
			  "compute_service": "codespaces",
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

	opts := &ListOptions{Page: 1, PerPage: 3}
	configurations, _, err := client.Organizations.ListNetworkConfigurations(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListNetworkConfigurations returned error %v", err)
	}
	want := &NetworkConfigurations{
		TotalCount: Ptr(int64(3)),
		NetworkConfigurations: []*NetworkConfiguration{
			{
				ID:             Ptr("123456789ABCDEF"),
				Name:           Ptr("Network Configuration One"),
				ComputeService: Ptr(ComputeService("actions")),
				NetworkSettingsIDs: []string{
					"23456789ABDCEF1",
					"3456789ABDCEF12",
				},
				CreatedOn: &referenceTimestamp,
			},
			{
				ID:             Ptr("456789ABDCEF123"),
				Name:           Ptr("Network Configuration Two"),
				ComputeService: Ptr(ComputeService("none")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
					"6789ABDCEF12345",
				},
				CreatedOn: &referenceTimestamp,
			},
			{
				ID:             Ptr("789ABDCEF123456"),
				Name:           Ptr("Network Configuration Three"),
				ComputeService: Ptr(ComputeService("codespaces")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
					"6789ABDCEF12345",
				},
				CreatedOn: &referenceTimestamp,
			},
		},
	}
	if !cmp.Equal(want, configurations) {
		t.Errorf("Organizations.ListNetworkConfigurations mismatch (-want +got):\n%v", cmp.Diff(want, configurations))
	}

	const methodName = "ListNetworkConfigurations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListNetworkConfigurations(ctx, "\no", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListNetworkConfigurations(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateOrgsNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/settings/network-configurations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
		  "id": "456789ABDCEF123",
		  "name": "network-configuration-two",
		  "compute_service": "none",
		  "network_settings_ids": [
			"56789ABDCEF1234"
		  ],
		  "created_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()

	req := NetworkConfigurationRequest{
		Name:           Ptr("network-configuration-two"),
		ComputeService: Ptr(ComputeService("none")),
		NetworkSettingsIDs: []string{
			"56789ABDCEF1234",
		},
	}

	configuration, _, err := client.Organizations.CreateNetworkConfiguration(ctx, "o", req)
	if err != nil {
		t.Errorf("Organizations.CreateNetworkConfiguration returned error %v", err)
	}

	want := &NetworkConfiguration{
		ID:             Ptr("456789ABDCEF123"),
		Name:           Ptr("network-configuration-two"),
		ComputeService: Ptr(ComputeService("none")),
		NetworkSettingsIDs: []string{
			"56789ABDCEF1234",
		},
		CreatedOn: &referenceTimestamp,
	}

	if !cmp.Equal(want, configuration) {
		t.Errorf("Organizations.CreateNetworkConfiguration mismatch (-want +got):\n%v", cmp.Diff(want, configuration))
	}

	validationTests := []struct {
		name    string
		request NetworkConfigurationRequest
		want    string
	}{
		{
			name: "invalid network configuration name length",
			request: NetworkConfigurationRequest{
				Name:           Ptr(""),
				ComputeService: Ptr(ComputeService("none")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
				},
			},
			want: "validation failed: must be between 1 and 100 characters",
		},
		{
			name: "invalid network configuration name",
			// may only contain upper and lowercase letters a-z, numbers 0-9, '.', '-', and '_'.
			request: NetworkConfigurationRequest{
				Name: Ptr("network configuration two"),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
				},
			},
			want: "validation failed: may only contain upper and lowercase letters a-z, numbers 0-9, '.', '-', and '_'",
		},
		{
			name: "invalid network settings ids",
			request: NetworkConfigurationRequest{
				Name:           Ptr("network-configuration-two"),
				ComputeService: Ptr(ComputeService("none")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
					"3456789ABDCEF12",
				},
			},
			want: "validation failed: exactly one network settings id must be specified",
		},
		{
			name: "invalid compute service",
			request: NetworkConfigurationRequest{
				Name:           Ptr("network-configuration-two"),
				ComputeService: Ptr(ComputeService("codespaces")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
				},
			},
			want: "validation failed: compute service can only be one of: none, actions",
		},
	}

	for _, tc := range validationTests {
		_, _, err := client.Organizations.CreateNetworkConfiguration(ctx, "o", tc.request)
		if err == nil || err.Error() != tc.want {
			t.Errorf("expected error to be %v, got %v", tc.want, err)
		}
	}

	const methodName = "CreateNetworkConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateNetworkConfiguration(ctx, "\no", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateNetworkConfiguration(ctx, "o", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetOrgsNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/settings/network-configurations/789ABDCEF123456", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			 "id": "789ABDCEF123456",
			  "name": "Network Configuration Three",
			  "compute_service": "codespaces",
			  "network_settings_ids": [
				"56789ABDCEF1234",
				"6789ABDCEF12345"
			  ],
			  "created_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()

	configuration, _, err := client.Organizations.GetNetworkConfiguration(ctx, "o", "789ABDCEF123456")
	if err != nil {
		t.Errorf("Organizations.GetNetworkConfiguration returned error: %v", err)
	}
	want := &NetworkConfiguration{
		ID:             Ptr("789ABDCEF123456"),
		Name:           Ptr("Network Configuration Three"),
		ComputeService: Ptr(ComputeService("codespaces")),
		NetworkSettingsIDs: []string{
			"56789ABDCEF1234",
			"6789ABDCEF12345",
		},
		CreatedOn: &referenceTimestamp,
	}
	if !cmp.Equal(want, configuration) {
		t.Errorf("Organizations.GetNetworkConfiguration mismatch (-want +got):\n%v", cmp.Diff(want, configuration))
	}

	const methodName = "GetNetworkConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetNetworkConfiguration(ctx, "\no", "789ABDCEF123456")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetNetworkConfiguration(ctx, "o", "789ABDCEF123456")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_UpdateOrgsNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/settings/network-configurations/789ABDCEF123456", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{
		  "id": "789ABDCEF123456",
		  "name": "Network Configuration Three Update",
		  "compute_service": "actions",
		  "network_settings_ids": [
			"56789ABDCEF1234",
			"6789ABDCEF12345"
		  ],
		  "created_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()

	req := NetworkConfigurationRequest{
		Name:           Ptr("network-configuration-three-update"),
		ComputeService: Ptr(ComputeService("actions")),
		NetworkSettingsIDs: []string{
			"56789ABDCEF1234",
		},
	}
	configuration, _, err := client.Organizations.UpdateNetworkConfiguration(ctx, "o", "789ABDCEF123456", req)
	if err != nil {
		t.Errorf("Organizations.UpdateNetworkConfiguration returned error: %v", err)
	}

	want := &NetworkConfiguration{
		ID:             Ptr("789ABDCEF123456"),
		Name:           Ptr("Network Configuration Three Update"),
		ComputeService: Ptr(ComputeService("actions")),
		NetworkSettingsIDs: []string{
			"56789ABDCEF1234",
			"6789ABDCEF12345",
		},
		CreatedOn: &referenceTimestamp,
	}
	if !cmp.Equal(want, configuration) {
		t.Errorf("Organizations.UpdateNetworkConfiguration mismatch (-want +got):\n%v", cmp.Diff(want, configuration))
	}

	validationTests := []struct {
		name    string
		request NetworkConfigurationRequest
		want    string
	}{
		{
			name: "invalid network configuration name length",
			request: NetworkConfigurationRequest{
				Name:           Ptr(""),
				ComputeService: Ptr(ComputeService("none")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
				},
			},
			want: "validation failed: must be between 1 and 100 characters",
		},
		{
			name: "invalid network configuration name",
			// may only contain upper and lowercase letters a-z, numbers 0-9, '.', '-', and '_'.
			request: NetworkConfigurationRequest{
				Name: Ptr("network configuration three update"),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
				},
			},
			want: "validation failed: may only contain upper and lowercase letters a-z, numbers 0-9, '.', '-', and '_'",
		},
		{
			name: "invalid network settings ids",
			request: NetworkConfigurationRequest{
				Name:           Ptr("network-configuration-three-update"),
				ComputeService: Ptr(ComputeService("none")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
					"3456789ABDCEF12",
				},
			},
			want: "validation failed: exactly one network settings id must be specified",
		},
		{
			name: "invalid compute service",
			request: NetworkConfigurationRequest{
				Name:           Ptr("network-configuration-three-update"),
				ComputeService: Ptr(ComputeService("codespaces")),
				NetworkSettingsIDs: []string{
					"56789ABDCEF1234",
				},
			},
			want: "validation failed: compute service can only be one of: none, actions",
		},
	}

	for _, tc := range validationTests {
		_, _, err := client.Organizations.UpdateNetworkConfiguration(ctx, "o", "789ABDCEF123456", tc.request)
		if err == nil || err.Error() != tc.want {
			t.Errorf("expected error to be %v, got %v", tc.want, err)
		}
	}

	const methodName = "UpdateNetworkConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.UpdateNetworkConfiguration(ctx, "\no", "789ABDCEF123456", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.UpdateNetworkConfiguration(ctx, "o", "789ABDCEF123456", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_DeleteOrgsNetworkConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/settings/network-configurations/789ABDCEF123456", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.DeleteNetworkConfigurations(ctx, "o", "789ABDCEF123456")
	if err != nil {
		t.Errorf("Organizations.DeleteNetworkConfigurations returned error %v", err)
	}

	const methodName = "DeleteNetworkConfigurations"
	testBadOptions(t, methodName, func() error {
		_, err = client.Organizations.DeleteNetworkConfigurations(ctx, "\ne", "123456789ABCDEF")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.DeleteNetworkConfigurations(ctx, "e", "123456789ABCDEF")
	})
}

func TestOrganizationsService_GetOrgsNetworkConfigurationResource(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/settings/network-settings/789ABDCEF123456", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
		  "id": "220F78DACB92BBFBC5E6F22DE1CCF52309D",
		  "network_configuration_id": "934E208B3EE0BD60CF5F752C426BFB53562",
		  "name": "my_network_settings",
		  "subnet_id": "/subscriptions/14839728-3ad9-43ab-bd2b-fa6ad0f75e2a/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet",
		  "region": "germanywestcentral"
		}
		`)
	})

	ctx := t.Context()

	resource, _, err := client.Organizations.GetNetworkConfigurationResource(ctx, "o", "789ABDCEF123456")
	if err != nil {
		t.Errorf("Organizations.GetNetworkConfigurationResource returned error %v", err)
	}

	want := &NetworkSettingsResource{
		ID:                     Ptr("220F78DACB92BBFBC5E6F22DE1CCF52309D"),
		Name:                   Ptr("my_network_settings"),
		NetworkConfigurationID: Ptr("934E208B3EE0BD60CF5F752C426BFB53562"),
		SubnetID:               Ptr("/subscriptions/14839728-3ad9-43ab-bd2b-fa6ad0f75e2a/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
		Region:                 Ptr("germanywestcentral"),
	}

	if !cmp.Equal(want, resource) {
		t.Errorf("Organizations.GetNetworkConfigurationResource mismatch (-want +got):\n%v", cmp.Diff(want, resource))
	}

	const methodName = "GetNetworkConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetNetworkConfigurationResource(ctx, "\no", "789ABDCEF123456")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetNetworkConfigurationResource(ctx, "o", "789ABDCEF123456")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
