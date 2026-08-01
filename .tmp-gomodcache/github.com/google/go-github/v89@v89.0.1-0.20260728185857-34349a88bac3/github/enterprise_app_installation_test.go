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

func TestEnterpriseService_ListAppInstallableOrganizations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/apps/installable_organizations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1, "login":"org1"}]`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 10}
	got, _, err := client.Enterprise.ListAppInstallableOrganizations(ctx, "e", opts)
	if err != nil {
		t.Fatalf("Enterprise.ListAppInstallableOrganizations returned error: %v", err)
	}

	want := []*InstallableOrganization{
		{ID: int64(1), Login: "org1"},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.ListAppInstallableOrganizations = %+v, want %+v", got, want)
	}

	const methodName = "ListAppInstallableOrganizations"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.ListAppInstallableOrganizations(ctx, "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListAppInstallableOrganizations(ctx, "e", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListAppAccessibleOrganizationRepositories(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/apps/installable_organizations/org1/accessible_repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":10, "name":"repo1", "full_name":"org1/repo1"}]`)
	})

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	repos, _, err := client.Enterprise.ListAppAccessibleOrganizationRepositories(ctx, "e", "org1", opts)
	if err != nil {
		t.Errorf("Enterprise.ListAppAccessibleOrganizationRepositories returned error: %v", err)
	}

	want := []*AccessibleRepository{
		{ID: int64(10), Name: "repo1", FullName: "org1/repo1"},
	}

	if !cmp.Equal(repos, want) {
		t.Errorf("Enterprise.ListAppAccessibleOrganizationRepositories returned %+v, want %+v", repos, want)
	}

	const methodName = "ListAppAccessibleOrganizationRepositories"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListAppAccessibleOrganizationRepositories(ctx, "\n", "org1", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListAppAccessibleOrganizationRepositories(ctx, "e", "org1", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListAppInstallations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/apps/organizations/org1/installations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `[{"id":99}]`)
	})

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	installations, _, err := client.Enterprise.ListAppInstallations(ctx, "e", "org1", opts)
	if err != nil {
		t.Errorf("ListAppInstallations returned error: %v", err)
	}
	want := []*Installation{
		{ID: Ptr(int64(99))},
	}

	if !cmp.Equal(installations, want) {
		t.Errorf("ListAppInstallations returned %+v, want %+v", installations, want)
	}

	const methodName = "ListAppInstallations"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListAppInstallations(ctx, "\n", "org1", &ListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListAppInstallations(ctx, "e", "org1", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_InstallApp(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := InstallAppRequest{
		ClientID:            "cid",
		RepositorySelection: "selected",
		Repositories:        []string{"r1", "r2"},
	}

	mux.HandleFunc("/enterprises/e/apps/organizations/org1/installations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, req)
		fmt.Fprint(w, `{"id":555}`)
	})

	ctx := t.Context()
	installation, _, err := client.Enterprise.InstallApp(ctx, "e", "org1", req)
	if err != nil {
		t.Errorf("InstallApp returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(555))}

	if !cmp.Equal(installation, want) {
		t.Errorf("InstallApp returned %+v, want %+v", installation, want)
	}

	const methodName = "InstallApp"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.InstallApp(ctx, "e", "org1", req)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UninstallApp(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/apps/organizations/org1/installations/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Enterprise.UninstallApp(ctx, "e", "org1", 123)
	if err != nil {
		t.Errorf("UninstallApp returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("UninstallApp returned status %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UninstallApp"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.UninstallApp(ctx, "e", "org1", 123)
	})
}

func TestEnterpriseService_ListRepositoriesForOrgAppInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/apps/organizations/o/installations/1/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "1"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	repos, _, err := client.Enterprise.ListRepositoriesForOrgAppInstallation(ctx, "e", "o", 1, &ListOptions{Page: 1})
	if err != nil {
		t.Errorf("Enterprise.ListRepositoriesForOrgAppInstallation returned error: %v", err)
	}

	want := []*AccessibleRepository{{ID: 1}}
	if diff := cmp.Diff(repos, want); diff != "" {
		t.Errorf("Enterprise.ListRepositoriesForOrgAppInstallation returned diff (-want +got):\n%v", diff)
	}

	const methodName = "ListRepositoriesForOrgAppInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListRepositoriesForOrgAppInstallation(ctx, "\n", "\n", -1, &ListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.Enterprise.ListRepositoriesForOrgAppInstallation(ctx, "e", "o", 1, &ListOptions{})
		return resp, err
	})
}

func TestEnterpriseService_UpdateAppInstallationRepositories(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := UpdateAppInstallationRepositoriesRequest{
		RepositorySelection: Ptr("selected"),
		Repositories:        []string{"hello-world", "hello-world-2"},
	}

	mux.HandleFunc("/enterprises/e/apps/organizations/o/installations/1/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1, "repository_selection":"selected"}`)
	})

	ctx := t.Context()
	inst, _, err := client.Enterprise.UpdateAppInstallationRepositories(ctx, "e", "o", 1, input)
	if err != nil {
		t.Errorf("Enterprise.UpdateAppInstallationRepositories returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(1)), RepositorySelection: Ptr("selected")}
	if diff := cmp.Diff(inst, want); diff != "" {
		t.Errorf("Enterprise.UpdateAppInstallationRepositories returned diff (-want +got):\n%v", diff)
	}

	const methodName = "UpdateAppInstallationRepositories"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.UpdateAppInstallationRepositories(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.Enterprise.UpdateAppInstallationRepositories(ctx, "e", "o", 1, input)
		return resp, err
	})
}

func TestEnterpriseService_AddRepositoriesToAppInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := AppInstallationRepositoriesRequest{Repositories: []string{"hello-world", "hello-world-2"}}

	mux.HandleFunc("/enterprises/e/apps/organizations/o/installations/1/repositories/add", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{"id":1},{"id":2}]`)
	})

	ctx := t.Context()
	repos, _, err := client.Enterprise.AddRepositoriesToAppInstallation(ctx, "e", "o", 1, input)
	if err != nil {
		t.Errorf("Enterprise.AddRepositoriesToAppInstallation returned error: %v", err)
	}

	want := []*AccessibleRepository{{ID: 1}, {ID: 2}}
	if diff := cmp.Diff(repos, want); diff != "" {
		t.Errorf("Enterprise.AddRepositoriesToAppInstallation returned diff (-want +got):\n%v", diff)
	}

	const methodName = "AddRepositoriesToAppInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.AddRepositoriesToAppInstallation(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.Enterprise.AddRepositoriesToAppInstallation(ctx, "e", "o", 1, input)
		return resp, err
	})
}

func TestEnterpriseService_RemoveRepositoriesFromAppInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := AppInstallationRepositoriesRequest{Repositories: []string{"hello-world", "hello-world-2"}}

	mux.HandleFunc("/enterprises/e/apps/organizations/o/installations/1/repositories/remove", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{"id":1},{"id":2}]`)
	})

	ctx := t.Context()
	repos, _, err := client.Enterprise.RemoveRepositoriesFromAppInstallation(ctx, "e", "o", 1, input)
	if err != nil {
		t.Errorf("Enterprise.RemoveRepositoriesFromAppInstallation returned error: %v", err)
	}

	want := []*AccessibleRepository{{ID: 1}, {ID: 2}}
	if diff := cmp.Diff(repos, want); diff != "" {
		t.Errorf("Enterprise.RemoveRepositoriesFromAppInstallation returned diff (-want +got):\n%v", diff)
	}

	const methodName = "RemoveRepositoriesFromAppInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.RemoveRepositoriesFromAppInstallation(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.Enterprise.RemoveRepositoriesFromAppInstallation(ctx, "e", "o", 1, input)
		return resp, err
	})
}
