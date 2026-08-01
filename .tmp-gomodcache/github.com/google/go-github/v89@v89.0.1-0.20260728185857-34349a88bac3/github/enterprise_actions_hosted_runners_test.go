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

func TestEnterpriseService_ListHostedRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 2,
			"runners": [
				{
					"id": 5,
					"name": "My hosted ubuntu runner",
					"runner_group_id": 2,
					"platform": "linux-x64",
					"image_details": {
						"id": "ubuntu-20.04",
						"size_gb": 86
					},
					"machine_size_details": {
						"id": "4-core",
						"cpu_cores": 4,
						"memory_gb": 16,
						"storage_gb": 150
					},
					"status": "Ready",
					"maximum_runners": 10,
					"public_ip_enabled": true,
					"public_ips": [
						{
							"enabled": true,
							"prefix": "20.80.208.150",
							"length": 31
						}
					],
					"last_active_on": `+referenceTimeStr+`
				},
				{
					"id": 7,
					"name": "My hosted Windows runner",
					"runner_group_id": 2,
					"platform": "win-x64",
					"image_details": {
						"id": "windows-latest",
						"size_gb": 256
					},
					"machine_size_details": {
						"id": "8-core",
						"cpu_cores": 8,
						"memory_gb": 32,
						"storage_gb": 300
					},
					"status": "Ready",
					"maximum_runners": 20,
					"public_ip_enabled": false,
					"public_ips": [],
					"last_active_on": `+referenceTimeStr+`
				}
			]
		}`)
	})
	opts := &ListOptions{Page: 1, PerPage: 1}
	ctx := t.Context()
	hostedRunners, _, err := client.Enterprise.ListHostedRunners(ctx, "o", opts)
	if err != nil {
		t.Errorf("Enterprise.ListHostedRunners returned error: %v", err)
	}

	want := &HostedRunners{
		TotalCount: 2,
		Runners: []*HostedRunner{
			{
				ID:            Ptr(int64(5)),
				Name:          Ptr("My hosted ubuntu runner"),
				RunnerGroupID: Ptr(int64(2)),
				Platform:      Ptr("linux-x64"),
				ImageDetails: &HostedRunnerImageDetail{
					ID:     Ptr("ubuntu-20.04"),
					SizeGB: Ptr(int64(86)),
				},
				MachineSizeDetails: &HostedRunnerMachineSpec{
					ID:        "4-core",
					CPUCores:  4,
					MemoryGB:  16,
					StorageGB: 150,
				},
				Status:          Ptr("Ready"),
				MaximumRunners:  Ptr(int64(10)),
				PublicIPEnabled: Ptr(true),
				PublicIPs: []*HostedRunnerPublicIP{
					{
						Enabled: true,
						Prefix:  "20.80.208.150",
						Length:  31,
					},
				},
				LastActiveOn: &referenceTimestamp,
			},
			{
				ID:            Ptr(int64(7)),
				Name:          Ptr("My hosted Windows runner"),
				RunnerGroupID: Ptr(int64(2)),
				Platform:      Ptr("win-x64"),
				ImageDetails: &HostedRunnerImageDetail{
					ID:     Ptr("windows-latest"),
					SizeGB: Ptr(int64(256)),
				},
				MachineSizeDetails: &HostedRunnerMachineSpec{
					ID:        "8-core",
					CPUCores:  8,
					MemoryGB:  32,
					StorageGB: 300,
				},
				Status:          Ptr("Ready"),
				MaximumRunners:  Ptr(int64(20)),
				PublicIPEnabled: Ptr(false),
				PublicIPs:       []*HostedRunnerPublicIP{},
				LastActiveOn:    &referenceTimestamp,
			},
		},
	}
	if !cmp.Equal(hostedRunners, want) {
		t.Errorf("Enterprise.ListHostedRunners returned %+v, want %+v", hostedRunners, want)
	}

	const methodName = "ListHostedRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListHostedRunners(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListHostedRunners(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateHostedRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"id": 5,
			"name": "My hosted ubuntu runner",
			"runner_group_id": 2,
			"platform": "linux-x64",
			"image_details": {
				"id": "ubuntu-20.04",
				"size_gb": 86
			},
			"machine_size_details": {
				"id": "4-core",
				"cpu_cores": 4,
				"memory_gb": 16,
				"storage_gb": 150
			},
			"status": "Ready",
			"maximum_runners": 10,
			"public_ip_enabled": true,
			"public_ips": [
				{
					"enabled": true,
					"prefix": "20.80.208.150",
					"length": 31
				}
			],
			"last_active_on": `+referenceTimeStr+`
			}`)
	})

	ctx := t.Context()
	req := CreateHostedRunnerRequest{
		Name: "My Hosted runner",
		Image: HostedRunnerImage{
			ID:      "ubuntu-latest",
			Source:  "github",
			Version: Ptr("latest"),
		},
		RunnerGroupID:  1,
		Size:           "4-core",
		MaximumRunners: Ptr(int64(50)),
		EnableStaticIP: Ptr(false),
		ImageGen:       Ptr(true),
	}
	hostedRunner, _, err := client.Enterprise.CreateHostedRunner(ctx, "o", req)
	if err != nil {
		t.Errorf("Enterprise.CreateHostedRunner returned error: %v", err)
	}

	want := &HostedRunner{
		ID:            Ptr(int64(5)),
		Name:          Ptr("My hosted ubuntu runner"),
		RunnerGroupID: Ptr(int64(2)),
		Platform:      Ptr("linux-x64"),
		ImageDetails: &HostedRunnerImageDetail{
			ID:     Ptr("ubuntu-20.04"),
			SizeGB: Ptr(int64(86)),
		},
		MachineSizeDetails: &HostedRunnerMachineSpec{
			ID:        "4-core",
			CPUCores:  4,
			MemoryGB:  16,
			StorageGB: 150,
		},
		Status:          Ptr("Ready"),
		MaximumRunners:  Ptr(int64(10)),
		PublicIPEnabled: Ptr(true),
		PublicIPs: []*HostedRunnerPublicIP{
			{
				Enabled: true,
				Prefix:  "20.80.208.150",
				Length:  31,
			},
		},
		LastActiveOn: &referenceTimestamp,
	}

	if !cmp.Equal(hostedRunner, want) {
		t.Errorf("Enterprise.CreateHostedRunner returned %+v, want %+v", hostedRunner, want)
	}

	// Validation tests
	testCases := []struct {
		name          string
		request       CreateHostedRunnerRequest
		expectedError string
	}{
		{
			name: "Missing Size",
			request: CreateHostedRunnerRequest{
				Name: "My Hosted runner",
				Image: HostedRunnerImage{
					ID:      "ubuntu-latest",
					Source:  "github",
					Version: Ptr("latest"),
				},
				RunnerGroupID: 1,
			},
			expectedError: "validation failed: size is required for creating a hosted runner",
		},
		{
			name: "Missing Image",
			request: CreateHostedRunnerRequest{
				Name:          "My Hosted runner",
				RunnerGroupID: 1,
				Size:          "4-core",
			},
			expectedError: "validation failed: image is required for creating a hosted runner",
		},
		{
			name: "Missing Name",
			request: CreateHostedRunnerRequest{
				Image: HostedRunnerImage{
					ID:      "ubuntu-latest",
					Source:  "github",
					Version: Ptr("latest"),
				},
				RunnerGroupID: 1,
				Size:          "4-core",
			},
			expectedError: "validation failed: name is required for creating a hosted runner",
		},
		{
			name: "Missing RunnerGroupID",
			request: CreateHostedRunnerRequest{
				Name: "My Hosted runner",
				Image: HostedRunnerImage{
					ID:      "ubuntu-latest",
					Source:  "github",
					Version: Ptr("latest"),
				},
				Size: "4-core",
			},
			expectedError: "validation failed: runner group ID is required for creating a hosted runner",
		},
	}

	for _, tt := range testCases {
		_, _, err := client.Enterprise.CreateHostedRunner(ctx, "o", tt.request)
		if err == nil || err.Error() != tt.expectedError {
			t.Errorf("expected error: %v, got: %v", tt.expectedError, err)
		}
	}

	const methodName = "CreateHostedRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.CreateHostedRunner(ctx, "\n", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateHostedRunner(ctx, "o", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunnerGitHubOwnedImages(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners/images/github-owned", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 1,
			"images": [
				{
					"id": "ubuntu-20.04",
					"platform": "linux-x64",
					"size_gb": 86,
					"display_name": "20.04",
					"source": "github"
				}
			]
			}`)
	})

	ctx := t.Context()
	hostedRunnerImages, _, err := client.Enterprise.GetHostedRunnerGitHubOwnedImages(ctx, "o")
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunnerGitHubOwnedImages returned error: %v", err)
	}

	want := &HostedRunnerImages{
		TotalCount: 1,
		Images: []*HostedRunnerImageSpecs{
			{
				ID:          "ubuntu-20.04",
				Platform:    "linux-x64",
				SizeGB:      86,
				DisplayName: "20.04",
				Source:      "github",
			},
		},
	}

	if !cmp.Equal(hostedRunnerImages, want) {
		t.Errorf("Enterprise.GetHostedRunnerGitHubOwnedImages returned %+v, want %+v", hostedRunnerImages, want)
	}

	const methodName = "GetHostedRunnerGitHubOwnedImages"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunnerGitHubOwnedImages(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunnerGitHubOwnedImages(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunnerPartnerImages(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners/images/partner", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 1,
			"images": [
				{
					"id": "ubuntu-20.04",
					"platform": "linux-x64",
					"size_gb": 86,
					"display_name": "20.04",
					"source": "partner"
				}
			]
			}`)
	})

	ctx := t.Context()
	hostedRunnerImages, _, err := client.Enterprise.GetHostedRunnerPartnerImages(ctx, "o")
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunnerPartnerImages returned error: %v", err)
	}

	want := &HostedRunnerImages{
		TotalCount: 1,
		Images: []*HostedRunnerImageSpecs{
			{
				ID:          "ubuntu-20.04",
				Platform:    "linux-x64",
				SizeGB:      86,
				DisplayName: "20.04",
				Source:      "partner",
			},
		},
	}

	if !cmp.Equal(hostedRunnerImages, want) {
		t.Errorf("Enterprise.GetHostedRunnerPartnerImages returned %+v, want %+v", hostedRunnerImages, want)
	}

	const methodName = "GetHostedRunnerPartnerImages"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunnerPartnerImages(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunnerPartnerImages(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunnerLimits(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/enterprises/o/actions/hosted-runners/limits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"public_ips": {
				"current_usage": 17,
				"maximum": 50
			}
		}`)
	})

	ctx := t.Context()
	publicIPLimits, _, err := client.Enterprise.GetHostedRunnerLimits(ctx, "o")
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunnerLimits returned error: %v", err)
	}

	want := &HostedRunnerPublicIPLimits{
		PublicIPs: &PublicIPUsage{
			CurrentUsage: 17,
			Maximum:      50,
		},
	}

	if !cmp.Equal(publicIPLimits, want) {
		t.Errorf("Enterprise.GetHostedRunnerLimits returned %+v, want %+v", publicIPLimits, want)
	}

	const methodName = "GetHostedRunnerLimits"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunnerLimits(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunnerLimits(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunnerMachineSpecs(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners/machine-sizes", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 1,
			"machine_specs": [
				{
					"id": "4-core",
 	 				"cpu_cores": 4,
  					"memory_gb": 16,
  					"storage_gb": 150
				}
			]
			}`)
	})

	ctx := t.Context()
	machineSpecs, _, err := client.Enterprise.GetHostedRunnerMachineSpecs(ctx, "o")
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunnerMachineSpecs returned error: %v", err)
	}
	want := &HostedRunnerMachineSpecs{
		TotalCount: 1,
		MachineSpecs: []*HostedRunnerMachineSpec{
			{
				ID:        "4-core",
				CPUCores:  4,
				MemoryGB:  16,
				StorageGB: 150,
			},
		},
	}

	if !cmp.Equal(machineSpecs, want) {
		t.Errorf("Enterprise.GetHostedRunnerMachineSpecs returned %+v, want %+v", machineSpecs, want)
	}

	const methodName = "GetHostedRunnerMachineSpecs"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunnerMachineSpecs(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunnerMachineSpecs(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunnerPlatforms(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners/platforms", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 1,
  			"platforms": [
    			"linux-x64",
    			"win-x64"
  			]
		}`)
	})

	ctx := t.Context()
	platforms, _, err := client.Enterprise.GetHostedRunnerPlatforms(ctx, "o")
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunnerPlatforms returned error: %v", err)
	}
	want := &HostedRunnerPlatforms{
		TotalCount: 1,
		Platforms: []string{
			"linux-x64",
			"win-x64",
		},
	}

	if !cmp.Equal(platforms, want) {
		t.Errorf("Enterprise.GetHostedRunnerPlatforms returned %+v, want %+v", platforms, want)
	}

	const methodName = "GetHostedRunnerPlatforms"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunnerPlatforms(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunnerPlatforms(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners/23", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 5,
			"name": "My hosted ubuntu runner",
			"runner_group_id": 2,
			"platform": "linux-x64",
			"image_details": {
				"id": "ubuntu-20.04",
				"size_gb": 86
			},
			"machine_size_details": {
				"id": "4-core",
				"cpu_cores": 4,
				"memory_gb": 16,
				"storage_gb": 150
			},
			"status": "Ready",
			"maximum_runners": 10,
			"public_ip_enabled": true,
			"public_ips": [
				{
				"enabled": true,
				"prefix": "20.80.208.150",
				"length": 31
				}
			],
			"last_active_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()
	hostedRunner, _, err := client.Enterprise.GetHostedRunner(ctx, "o", 23)
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunner returned error: %v", err)
	}

	want := &HostedRunner{
		ID:            Ptr(int64(5)),
		Name:          Ptr("My hosted ubuntu runner"),
		RunnerGroupID: Ptr(int64(2)),
		Platform:      Ptr("linux-x64"),
		ImageDetails: &HostedRunnerImageDetail{
			ID:     Ptr("ubuntu-20.04"),
			SizeGB: Ptr(int64(86)),
		},
		MachineSizeDetails: &HostedRunnerMachineSpec{
			ID:        "4-core",
			CPUCores:  4,
			MemoryGB:  16,
			StorageGB: 150,
		},
		Status:          Ptr("Ready"),
		MaximumRunners:  Ptr(int64(10)),
		PublicIPEnabled: Ptr(true),
		PublicIPs: []*HostedRunnerPublicIP{
			{
				Enabled: true,
				Prefix:  "20.80.208.150",
				Length:  31,
			},
		},
		LastActiveOn: &referenceTimestamp,
	}

	if !cmp.Equal(hostedRunner, want) {
		t.Errorf("Enterprise.GetHostedRunner returned %+v, want %+v", hostedRunner, want)
	}

	const methodName = "GetHostedRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunner(ctx, "\n", 23)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunner(ctx, "o", 23)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateHostedRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners/23", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{
			"id": 5,
			"name": "My hosted ubuntu runner",
			"runner_group_id": 2,
			"platform": "linux-x64",
			"image_details": {
				"id": "ubuntu-20.04",
				"size_gb": 86
			},
			"machine_size_details": {
				"id": "4-core",
				"cpu_cores": 4,
				"memory_gb": 16,
				"storage_gb": 150
			},
			"status": "Ready",
			"maximum_runners": 10,
			"public_ip_enabled": true,
			"public_ips": [
				{
					"enabled": true,
					"prefix": "20.80.208.150",
					"length": 31
				}
			],
			"last_active_on": `+referenceTimeStr+`
		}`)
	})

	// Test for a valid update without `Size`
	ctx := t.Context()
	validReq := UpdateHostedRunnerRequest{
		Name:           Ptr("My larger runner"),
		RunnerGroupID:  Ptr(int64(1)),
		MaximumRunners: Ptr(int64(50)),
		EnableStaticIP: Ptr(false),
		ImageVersion:   Ptr("1.0.0"),
	}
	hostedRunner, _, err := client.Enterprise.UpdateHostedRunner(ctx, "o", 23, validReq)
	if err != nil {
		t.Errorf("Enterprise.UpdateHostedRunner returned error: %v", err)
	}

	want := &HostedRunner{
		ID:            Ptr(int64(5)),
		Name:          Ptr("My hosted ubuntu runner"),
		RunnerGroupID: Ptr(int64(2)),
		Platform:      Ptr("linux-x64"),
		ImageDetails: &HostedRunnerImageDetail{
			ID:     Ptr("ubuntu-20.04"),
			SizeGB: Ptr(int64(86)),
		},
		MachineSizeDetails: &HostedRunnerMachineSpec{
			ID:        "4-core",
			CPUCores:  4,
			MemoryGB:  16,
			StorageGB: 150,
		},
		Status:          Ptr("Ready"),
		MaximumRunners:  Ptr(int64(10)),
		PublicIPEnabled: Ptr(true),
		PublicIPs: []*HostedRunnerPublicIP{
			{
				Enabled: true,
				Prefix:  "20.80.208.150",
				Length:  31,
			},
		},
		LastActiveOn: &referenceTimestamp,
	}

	if !cmp.Equal(hostedRunner, want) {
		t.Errorf("Enterprise.UpdateHostedRunner returned %+v, want %+v", hostedRunner, want)
	}

	const methodName = "UpdateHostedRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.UpdateHostedRunner(ctx, "\n", 23, validReq)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateHostedRunner(ctx, "o", 23, validReq)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteHostedRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/o/actions/hosted-runners/23", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		fmt.Fprint(w, `{
			"id": 5,
			"name": "My hosted ubuntu runner",
			"runner_group_id": 2,
			"platform": "linux-x64",
			"image_details": {
				"id": "ubuntu-20.04",
				"size_gb": 86
			},
			"machine_size_details": {
				"id": "4-core",
				"cpu_cores": 4,
				"memory_gb": 16,
				"storage_gb": 150
			},
			"status": "Ready",
			"maximum_runners": 10,
			"public_ip_enabled": true,
			"public_ips": [
				{
				"enabled": true,
				"prefix": "20.80.208.150",
				"length": 31
				}
			],
			"last_active_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()
	hostedRunner, _, err := client.Enterprise.DeleteHostedRunner(ctx, "o", 23)
	if err != nil {
		t.Errorf("Enterprise.DeleteHostedRunner returned error: %v", err)
	}

	want := &HostedRunner{
		ID:            Ptr(int64(5)),
		Name:          Ptr("My hosted ubuntu runner"),
		RunnerGroupID: Ptr(int64(2)),
		Platform:      Ptr("linux-x64"),
		ImageDetails: &HostedRunnerImageDetail{
			ID:     Ptr("ubuntu-20.04"),
			SizeGB: Ptr(int64(86)),
		},
		MachineSizeDetails: &HostedRunnerMachineSpec{
			ID:        "4-core",
			CPUCores:  4,
			MemoryGB:  16,
			StorageGB: 150,
		},
		Status:          Ptr("Ready"),
		MaximumRunners:  Ptr(int64(10)),
		PublicIPEnabled: Ptr(true),
		PublicIPs: []*HostedRunnerPublicIP{
			{
				Enabled: true,
				Prefix:  "20.80.208.150",
				Length:  31,
			},
		},
		LastActiveOn: &referenceTimestamp,
	}

	if !cmp.Equal(hostedRunner, want) {
		t.Errorf("Enterprise.DeleteHostedRunner returned %+v, want %+v", hostedRunner, want)
	}

	const methodName = "DeleteHostedRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.DeleteHostedRunner(ctx, "\n", 23)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.DeleteHostedRunner(ctx, "o", 23)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListHostedRunnerCustomImages(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/hosted-runners/images/custom", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 2,
			"images": [
				{
					"id": 1,
					"platform": "linux-x64",
					"name": "CustomImage1",
					"source": "custom",
					"versions_count": 4,
					"total_versions_size": 200,
					"latest_version": "1.3.0",
					"state": "Ready"
				},
				{
					"id": 2,
					"platform": "linux-x64",
					"name": "CustomImage2",
					"source": "custom",
					"versions_count": 2,
					"total_versions_size": 150,
					"latest_version": "1.0.0",
					"state": "Ready"
				}
			]
		}`)
	})

	ctx := t.Context()
	images, _, err := client.Enterprise.ListHostedRunnerCustomImages(ctx, "e")
	if err != nil {
		t.Errorf("Enterprise.ListHostedRunnerCustomImages returned error: %v", err)
	}

	want := &HostedRunnerCustomImages{
		TotalCount: 2,
		Images: []*HostedRunnerCustomImage{
			{
				ID:                1,
				Platform:          "linux-x64",
				Name:              "CustomImage1",
				Source:            "custom",
				VersionsCount:     4,
				TotalVersionsSize: 200,
				LatestVersion:     "1.3.0",
				State:             "Ready",
			},
			{
				ID:                2,
				Platform:          "linux-x64",
				Name:              "CustomImage2",
				Source:            "custom",
				VersionsCount:     2,
				TotalVersionsSize: 150,
				LatestVersion:     "1.0.0",
				State:             "Ready",
			},
		},
	}

	if !cmp.Equal(images, want) {
		t.Errorf("Enterprise.ListHostedRunnerCustomImages returned %+v, want %+v", images, want)
	}

	const methodName = "ListHostedRunnerCustomImages"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListHostedRunnerCustomImages(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListHostedRunnerCustomImages(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunnerCustomImage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/hosted-runners/images/custom/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 1,
			"platform": "linux-x64",
			"name": "CustomImage",
			"source": "custom",
			"versions_count": 4,
			"total_versions_size": 200,
			"latest_version": "1.3.0",
			"state": "Ready"
		}`)
	})

	ctx := t.Context()
	image, _, err := client.Enterprise.GetHostedRunnerCustomImage(ctx, "e", 1)
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunnerCustomImage returned error: %v", err)
	}

	want := &HostedRunnerCustomImage{
		ID:                1,
		Platform:          "linux-x64",
		Name:              "CustomImage",
		Source:            "custom",
		VersionsCount:     4,
		TotalVersionsSize: 200,
		LatestVersion:     "1.3.0",
		State:             "Ready",
	}

	if !cmp.Equal(image, want) {
		t.Errorf("Enterprise.GetHostedRunnerCustomImage returned %+v, want %+v", image, want)
	}

	const methodName = "GetHostedRunnerCustomImage"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunnerCustomImage(ctx, "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunnerCustomImage(ctx, "e", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteHostedRunnerCustomImage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/hosted-runners/images/custom/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteHostedRunnerCustomImage(ctx, "e", 1)
	if err != nil {
		t.Errorf("Enterprise.DeleteHostedRunnerCustomImage returned error: %v", err)
	}

	const methodName = "DeleteHostedRunnerCustomImage"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.DeleteHostedRunnerCustomImage(ctx, "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteHostedRunnerCustomImage(ctx, "e", 1)
	})
}

func TestEnterpriseService_ListHostedRunnerCustomImageVersions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/hosted-runners/images/custom/1/versions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 2,
			"image_versions": [
				{
					"version": "1.1.0",
					"size_gb": 75,
					"state": "Ready",
					"state_details": "None",
					"created_on": `+referenceTimeStr+`
				},
				{
					"version": "1.0.0",
					"size_gb": 75,
					"state": "Ready",
					"state_details": "None",
					"created_on": `+referenceTimeStr+`
				}
			]
		}`)
	})

	ctx := t.Context()
	versions, _, err := client.Enterprise.ListHostedRunnerCustomImageVersions(ctx, "e", 1)
	if err != nil {
		t.Errorf("Enterprise.ListHostedRunnerCustomImageVersions returned error: %v", err)
	}

	want := &HostedRunnerCustomImageVersions{
		TotalCount: 2,
		ImageVersions: []*HostedRunnerCustomImageVersion{
			{
				Version:      "1.1.0",
				SizeGB:       75,
				State:        "Ready",
				StateDetails: "None",
				CreatedOn:    referenceTimestamp,
			},
			{
				Version:      "1.0.0",
				SizeGB:       75,
				State:        "Ready",
				StateDetails: "None",
				CreatedOn:    referenceTimestamp,
			},
		},
	}

	if !cmp.Equal(versions, want) {
		t.Errorf("Enterprise.ListHostedRunnerCustomImageVersions returned %+v, want %+v", versions, want)
	}

	const methodName = "ListHostedRunnerCustomImageVersions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListHostedRunnerCustomImageVersions(ctx, "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListHostedRunnerCustomImageVersions(ctx, "e", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetHostedRunnerCustomImageVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/hosted-runners/images/custom/1/versions/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"version": "1.0.0",
			"size_gb": 75,
			"state": "Ready",
			"state_details": "None",
			"created_on": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()
	version, _, err := client.Enterprise.GetHostedRunnerCustomImageVersion(ctx, "e", 1, "1.0.0")
	if err != nil {
		t.Errorf("Enterprise.GetHostedRunnerCustomImageVersion returned error: %v", err)
	}

	want := &HostedRunnerCustomImageVersion{
		Version:      "1.0.0",
		SizeGB:       75,
		State:        "Ready",
		StateDetails: "None",
		CreatedOn:    referenceTimestamp,
	}

	if !cmp.Equal(version, want) {
		t.Errorf("Enterprise.GetHostedRunnerCustomImageVersion returned %+v, want %+v", version, want)
	}

	const methodName = "GetHostedRunnerCustomImageVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetHostedRunnerCustomImageVersion(ctx, "\n", 1, "1.0.0")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetHostedRunnerCustomImageVersion(ctx, "e", 1, "1.0.0")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteHostedRunnerCustomImageVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/hosted-runners/images/custom/1/versions/1.0.0", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteHostedRunnerCustomImageVersion(ctx, "e", 1, "1.0.0")
	if err != nil {
		t.Errorf("Enterprise.DeleteHostedRunnerCustomImageVersion returned error: %v", err)
	}

	const methodName = "DeleteHostedRunnerCustomImageVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.DeleteHostedRunnerCustomImageVersion(ctx, "\n", 1, "1.0.0")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteHostedRunnerCustomImageVersion(ctx, "e", 1, "1.0.0")
	})
}
