// Copyright 2021 The go-github AUTHORS. All rights reserved.
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

func TestActionsService_ListOrganizationRunnerGroups(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":3,"runner_groups":[{"id":1,"name":"Default","visibility":"all","default":true,"runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/1/runners","inherited":false,"allows_public_repositories":true,"restricted_to_workflows":true,"selected_workflows":["a","b"]},{"id":2,"name":"octo-runner-group","visibility":"selected","default":false,"selected_repositories_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories","runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners","inherited":true,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]},{"id":3,"name":"expensive-hardware","visibility":"private","default":false,"runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/3/runners","inherited":false,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]}]}`)
	})

	opts := &ListOrgRunnerGroupOptions{ListOptions: ListOptions{Page: 2, PerPage: 2}}
	ctx := t.Context()
	groups, _, err := client.Actions.ListOrganizationRunnerGroups(ctx, "o", opts)
	if err != nil {
		t.Errorf("Actions.ListOrganizationRunnerGroups returned error: %v", err)
	}

	want := &RunnerGroups{
		TotalCount: 3,
		RunnerGroups: []*RunnerGroup{
			{ID: Ptr(int64(1)), Name: Ptr("Default"), Visibility: Ptr("all"), Default: Ptr(true), RunnersURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/1/runners"), Inherited: Ptr(false), AllowsPublicRepositories: Ptr(true), RestrictedToWorkflows: Ptr(true), SelectedWorkflows: []string{"a", "b"}},
			{ID: Ptr(int64(2)), Name: Ptr("octo-runner-group"), Visibility: Ptr("selected"), Default: Ptr(false), SelectedRepositoriesURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories"), RunnersURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners"), Inherited: Ptr(true), AllowsPublicRepositories: Ptr(true), RestrictedToWorkflows: Ptr(false), SelectedWorkflows: []string{}},
			{ID: Ptr(int64(3)), Name: Ptr("expensive-hardware"), Visibility: Ptr("private"), Default: Ptr(false), RunnersURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/3/runners"), Inherited: Ptr(false), AllowsPublicRepositories: Ptr(true), RestrictedToWorkflows: Ptr(false), SelectedWorkflows: []string{}},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Actions.ListOrganizationRunnerGroups returned %+v, want %+v", groups, want)
	}

	const methodName = "ListOrganizationRunnerGroups"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListOrganizationRunnerGroups(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListOrganizationRunnerGroups(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListOrganizationRunnerGroupsVisibleToRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2", "visible_to_repository": "github"})
		fmt.Fprint(w, `{"total_count":3,"runner_groups":[{"id":1,"name":"Default","visibility":"all","default":true,"runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/1/runners","inherited":false,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]},{"id":2,"name":"octo-runner-group","visibility":"selected","default":false,"selected_repositories_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories","runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners","inherited":true,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]},{"id":3,"name":"expensive-hardware","visibility":"private","default":false,"runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/3/runners","inherited":false,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]}]}`)
	})

	opts := &ListOrgRunnerGroupOptions{ListOptions: ListOptions{Page: 2, PerPage: 2}, VisibleToRepository: "github"}
	ctx := t.Context()
	groups, _, err := client.Actions.ListOrganizationRunnerGroups(ctx, "o", opts)
	if err != nil {
		t.Errorf("Actions.ListOrganizationRunnerGroups returned error: %v", err)
	}

	want := &RunnerGroups{
		TotalCount: 3,
		RunnerGroups: []*RunnerGroup{
			{ID: Ptr(int64(1)), Name: Ptr("Default"), Visibility: Ptr("all"), Default: Ptr(true), RunnersURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/1/runners"), Inherited: Ptr(false), AllowsPublicRepositories: Ptr(true), RestrictedToWorkflows: Ptr(false), SelectedWorkflows: []string{}},
			{ID: Ptr(int64(2)), Name: Ptr("octo-runner-group"), Visibility: Ptr("selected"), Default: Ptr(false), SelectedRepositoriesURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories"), RunnersURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners"), Inherited: Ptr(true), AllowsPublicRepositories: Ptr(true), RestrictedToWorkflows: Ptr(false), SelectedWorkflows: []string{}},
			{ID: Ptr(int64(3)), Name: Ptr("expensive-hardware"), Visibility: Ptr("private"), Default: Ptr(false), RunnersURL: Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/3/runners"), Inherited: Ptr(false), AllowsPublicRepositories: Ptr(true), RestrictedToWorkflows: Ptr(false), SelectedWorkflows: []string{}},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Actions.ListOrganizationRunnerGroups returned %+v, want %+v", groups, want)
	}

	const methodName = "ListOrganizationRunnerGroups"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListOrganizationRunnerGroups(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListOrganizationRunnerGroups(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetOrganizationRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":2,"name":"octo-runner-group","visibility":"selected","default":false,"selected_repositories_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories","runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners","hosted_runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/hosted-runners","network_configuration_id":"EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA","inherited":false,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]}`)
	})

	ctx := t.Context()
	group, _, err := client.Actions.GetOrganizationRunnerGroup(ctx, "o", 2)
	if err != nil {
		t.Errorf("Actions.GetOrganizationRunnerGroup returned error: %v", err)
	}

	want := &RunnerGroup{
		ID:                       Ptr(int64(2)),
		Name:                     Ptr("octo-runner-group"),
		Visibility:               Ptr("selected"),
		Default:                  Ptr(false),
		SelectedRepositoriesURL:  Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories"),
		RunnersURL:               Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners"),
		HostedRunnersURL:         Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/hosted-runners"),
		NetworkConfigurationID:   Ptr("EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA"),
		Inherited:                Ptr(false),
		AllowsPublicRepositories: Ptr(true),
		RestrictedToWorkflows:    Ptr(false),
		SelectedWorkflows:        []string{},
	}

	if !cmp.Equal(group, want) {
		t.Errorf("Actions.GetOrganizationRunnerGroup returned %+v, want %+v", group, want)
	}

	const methodName = "GetOrganizationRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetOrganizationRunnerGroup(ctx, "\n", 2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetOrganizationRunnerGroup(ctx, "o", 2)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_DeleteOrganizationRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.DeleteOrganizationRunnerGroup(ctx, "o", 2)
	if err != nil {
		t.Errorf("Actions.DeleteOrganizationRunnerGroup returned error: %v", err)
	}

	const methodName = "DeleteOrganizationRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.DeleteOrganizationRunnerGroup(ctx, "\n", 2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.DeleteOrganizationRunnerGroup(ctx, "o", 2)
	})
}

func TestActionsService_CreateOrganizationRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":2,"name":"octo-runner-group","visibility":"selected","default":false,"selected_repositories_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories","runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners","hosted_runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/hosted-runners","network_configuration_id":"EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA","inherited":false,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]}`)
	})

	ctx := t.Context()
	req := CreateRunnerGroupRequest{
		Name:                     Ptr("octo-runner-group"),
		Visibility:               Ptr("selected"),
		AllowsPublicRepositories: Ptr(true),
		RestrictedToWorkflows:    Ptr(false),
		SelectedWorkflows:        []string{},
		NetworkConfigurationID:   Ptr("EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA"),
	}
	group, _, err := client.Actions.CreateOrganizationRunnerGroup(ctx, "o", req)
	if err != nil {
		t.Errorf("Actions.CreateOrganizationRunnerGroup returned error: %v", err)
	}

	want := &RunnerGroup{
		ID:                       Ptr(int64(2)),
		Name:                     Ptr("octo-runner-group"),
		Visibility:               Ptr("selected"),
		Default:                  Ptr(false),
		SelectedRepositoriesURL:  Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories"),
		RunnersURL:               Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners"),
		HostedRunnersURL:         Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/hosted-runners"),
		NetworkConfigurationID:   Ptr("EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA"),
		Inherited:                Ptr(false),
		AllowsPublicRepositories: Ptr(true),
		RestrictedToWorkflows:    Ptr(false),
		SelectedWorkflows:        []string{},
	}

	if !cmp.Equal(group, want) {
		t.Errorf("Actions.CreateOrganizationRunnerGroup returned %+v, want %+v", group, want)
	}

	const methodName = "CreateOrganizationRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.CreateOrganizationRunnerGroup(ctx, "\n", req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.CreateOrganizationRunnerGroup(ctx, "o", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateOrganizationRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"id":2,"name":"octo-runner-group","visibility":"selected","default":false,"selected_repositories_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories","runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners","hosted_runners_url":"https://api.github.com/orgs/octo-org/actions/runner_groups/2/hosted-runners","network_configuration_id":"EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA","inherited":false,"allows_public_repositories":true,"restricted_to_workflows":false,"selected_workflows":[]}`)
	})

	ctx := t.Context()
	req := UpdateRunnerGroupRequest{
		Name:                     Ptr("octo-runner-group"),
		Visibility:               Ptr("selected"),
		AllowsPublicRepositories: Ptr(true),
		RestrictedToWorkflows:    Ptr(false),
		SelectedWorkflows:        []string{},
		NetworkConfigurationID:   Ptr("EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA"),
	}
	group, _, err := client.Actions.UpdateOrganizationRunnerGroup(ctx, "o", 2, req)
	if err != nil {
		t.Errorf("Actions.UpdateOrganizationRunnerGroup returned error: %v", err)
	}

	want := &RunnerGroup{
		ID:                       Ptr(int64(2)),
		Name:                     Ptr("octo-runner-group"),
		Visibility:               Ptr("selected"),
		Default:                  Ptr(false),
		SelectedRepositoriesURL:  Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/repositories"),
		RunnersURL:               Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/runners"),
		HostedRunnersURL:         Ptr("https://api.github.com/orgs/octo-org/actions/runner_groups/2/hosted-runners"),
		NetworkConfigurationID:   Ptr("EC486D5D793175D7E3B29C27318D5C1AAE49A7833FC85F2E82C3D2C54AC7D3BA"),
		Inherited:                Ptr(false),
		AllowsPublicRepositories: Ptr(true),
		RestrictedToWorkflows:    Ptr(false),
		SelectedWorkflows:        []string{},
	}

	if !cmp.Equal(group, want) {
		t.Errorf("Actions.UpdateOrganizationRunnerGroup returned %+v, want %+v", group, want)
	}

	const methodName = "UpdateOrganizationRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.UpdateOrganizationRunnerGroup(ctx, "\n", 2, req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.UpdateOrganizationRunnerGroup(ctx, "o", 2, req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListRepositoryAccessRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "1", "page": "1"})
		fmt.Fprint(w, `{"total_count": 1, "repositories": [{"id": 43, "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5", "name": "Hello-World", "full_name": "octocat/Hello-World"}]}`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 1}
	groups, _, err := client.Actions.ListRepositoryAccessRunnerGroup(ctx, "o", 2, opts)
	if err != nil {
		t.Errorf("Actions.ListRepositoryAccessRunnerGroup returned error: %v", err)
	}

	want := &ListRepositories{
		TotalCount: Ptr(1),
		Repositories: []*Repository{
			{ID: Ptr(int64(43)), NodeID: Ptr("MDEwOlJlcG9zaXRvcnkxMjk2MjY5"), Name: Ptr("Hello-World"), FullName: Ptr("octocat/Hello-World")},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Actions.ListRepositoryAccessRunnerGroup returned %+v, want %+v", groups, want)
	}

	const methodName = "ListRepositoryAccessRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListRepositoryAccessRunnerGroup(ctx, "\n", 2, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListRepositoryAccessRunnerGroup(ctx, "o", 2, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetRepositoryAccessRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/repositories", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	req := SetRepoAccessRunnerGroupRequest{
		SelectedRepositoryIDs: []int64{
			1,
			2,
		},
	}

	ctx := t.Context()
	_, err := client.Actions.SetRepositoryAccessRunnerGroup(ctx, "o", 2, req)
	if err != nil {
		t.Errorf("Actions.SetRepositoryAccessRunnerGroup returned error: %v", err)
	}

	const methodName = "SetRepositoryAccessRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetRepositoryAccessRunnerGroup(ctx, "\n", 2, req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetRepositoryAccessRunnerGroup(ctx, "o", 2, req)
	})
}

func TestActionsService_AddRepositoryAccessRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/repositories/42", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	ctx := t.Context()
	_, err := client.Actions.AddRepositoryAccessRunnerGroup(ctx, "o", 2, 42)
	if err != nil {
		t.Errorf("Actions.AddRepositoryAccessRunnerGroup returned error: %v", err)
	}

	const methodName = "AddRepositoryAccessRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.AddRepositoryAccessRunnerGroup(ctx, "\n", 2, 42)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.AddRepositoryAccessRunnerGroup(ctx, "o", 2, 42)
	})
}

func TestActionsService_RemoveRepositoryAccessRunnerGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/repositories/42", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.RemoveRepositoryAccessRunnerGroup(ctx, "o", 2, 42)
	if err != nil {
		t.Errorf("Actions.RemoveRepositoryAccessRunnerGroup returned error: %v", err)
	}

	const methodName = "RemoveRepositoryAccessRunnerGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RemoveRepositoryAccessRunnerGroup(ctx, "\n", 2, 42)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RemoveRepositoryAccessRunnerGroup(ctx, "o", 2, 42)
	})
}

func TestActionsService_ListRunnerGroupHostedRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/hosted-runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
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

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	runners, _, err := client.Actions.ListRunnerGroupHostedRunners(ctx, "o", 2, opts)
	if err != nil {
		t.Errorf("Actions.ListRunnerGroupHostedRunners returned error: %v", err)
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
	if !cmp.Equal(runners, want) {
		t.Errorf("Actions.ListRunnerGroupHostedRunners returned %+v, want %+v", runners, want)
	}

	const methodName = "ListRunnerGroupHostedRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListRunnerGroupHostedRunners(ctx, "\n", 2, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListRunnerGroupHostedRunners(ctx, "o", 2, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListRunnerGroupRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":2,"runners":[{"id":23,"name":"MBP","os":"macos","status":"online"},{"id":24,"name":"iMac","os":"macos","status":"offline"}]}`)
	})

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	runners, _, err := client.Actions.ListRunnerGroupRunners(ctx, "o", 2, opts)
	if err != nil {
		t.Errorf("Actions.ListRunnerGroupRunners returned error: %v", err)
	}

	want := &Runners{
		TotalCount: 2,
		Runners: []*Runner{
			{ID: Ptr(int64(23)), Name: Ptr("MBP"), OS: Ptr("macos"), Status: Ptr("online")},
			{ID: Ptr(int64(24)), Name: Ptr("iMac"), OS: Ptr("macos"), Status: Ptr("offline")},
		},
	}
	if !cmp.Equal(runners, want) {
		t.Errorf("Actions.ListRunnerGroupRunners returned %+v, want %+v", runners, want)
	}

	const methodName = "ListRunnerGroupRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListRunnerGroupRunners(ctx, "\n", 2, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListRunnerGroupRunners(ctx, "o", 2, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetRunnerGroupRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/runners", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	req := SetRunnerGroupRunnersRequest{
		Runners: []int64{
			1,
			2,
		},
	}

	ctx := t.Context()
	_, err := client.Actions.SetRunnerGroupRunners(ctx, "o", 2, req)
	if err != nil {
		t.Errorf("Actions.SetRunnerGroupRunners returned error: %v", err)
	}

	const methodName = "SetRunnerGroupRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetRunnerGroupRunners(ctx, "\n", 2, req)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetRunnerGroupRunners(ctx, "o", 2, req)
	})
}

func TestActionsService_AddRunnerGroupRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/runners/42", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	ctx := t.Context()
	_, err := client.Actions.AddRunnerGroupRunners(ctx, "o", 2, 42)
	if err != nil {
		t.Errorf("Actions.AddRunnerGroupRunners returned error: %v", err)
	}

	const methodName = "AddRunnerGroupRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.AddRunnerGroupRunners(ctx, "\n", 2, 42)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.AddRunnerGroupRunners(ctx, "o", 2, 42)
	})
}

func TestActionsService_RemoveRunnerGroupRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runner-groups/2/runners/42", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.RemoveRunnerGroupRunners(ctx, "o", 2, 42)
	if err != nil {
		t.Errorf("Actions.RemoveRunnerGroupRunners returned error: %v", err)
	}

	const methodName = "RemoveRunnerGroupRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RemoveRunnerGroupRunners(ctx, "\n", 2, 42)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RemoveRunnerGroupRunners(ctx, "o", 2, 42)
	})
}
