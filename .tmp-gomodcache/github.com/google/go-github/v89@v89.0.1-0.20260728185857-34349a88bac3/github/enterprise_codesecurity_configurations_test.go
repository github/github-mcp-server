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

func TestEnterpriseService_ListCodeSecurityConfigurations(t *testing.T) {
	t.Parallel()
	opts := &ListEnterpriseCodeSecurityConfigurationOptions{Before: "1", After: "2", PerPage: 30}
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/code-security/configurations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"before": "1", "after": "2", "per_page": "30"})
		fmt.Fprint(w, `[
		{
			"id":1,
			"name":"config1",
			"description":"desc1",
			"code_scanning_default_setup": "enabled"
		},
		{
			"id":2,
			"name":"config2",
			"description":"desc2",
			"private_vulnerability_reporting": "enabled"
		}]`)
	})

	configurations, _, err := client.Enterprise.ListCodeSecurityConfigurations(ctx, "e", opts)
	if err != nil {
		t.Errorf("Enterprise.ListCodeSecurityConfigurations returned error: %v", err)
	}

	want := []*CodeSecurityConfiguration{
		{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")},
		{ID: Ptr(int64(2)), Name: "config2", Description: "desc2", PrivateVulnerabilityReporting: Ptr("enabled")},
	}
	if !cmp.Equal(configurations, want) {
		t.Errorf("Enterprise.ListCodeSecurityConfigurations returned %+v, want %+v", configurations, want)
	}
	const methodName = "ListCodeSecurityConfigurations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListCodeSecurityConfigurations(ctx, "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListCodeSecurityConfigurations(ctx, "e", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	mux.HandleFunc("/enterprises/e/code-security/configurations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id":1,
			"name":"config1",
			"description":"desc1",
			"code_scanning_default_setup": "enabled"
		}`)
	})

	configuration, _, err := client.Enterprise.GetCodeSecurityConfiguration(ctx, "e", 1)
	if err != nil {
		t.Errorf("Enterprise.GetCodeSecurityConfiguration returned error: %v", err)
	}

	want := &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Enterprise.GetCodeSecurityConfiguration returned %+v, want %+v", configuration, want)
	}

	const methodName = "GetCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetCodeSecurityConfiguration(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetCodeSecurityConfiguration(ctx, "e", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	input := CodeSecurityConfiguration{
		Name:                     "config1",
		Description:              "desc1",
		CodeScanningDefaultSetup: Ptr("enabled"),
	}

	mux.HandleFunc("/enterprises/e/code-security/configurations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id":1,
			"name":"config1",
			"description":"desc1",
			"code_scanning_default_setup": "enabled"
		}`)
	})

	configuration, _, err := client.Enterprise.CreateCodeSecurityConfiguration(ctx, "e", input)
	if err != nil {
		t.Errorf("Enterprise.CreateCodeSecurityConfiguration returned error: %v", err)
	}

	want := &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Enterprise.CreateCodeSecurityConfiguration returned %+v, want %+v", configuration, want)
	}

	const methodName = "CreateCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.CreateCodeSecurityConfiguration(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateCodeSecurityConfiguration(ctx, "e", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListDefaultCodeSecurityConfigurations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	mux.HandleFunc("/enterprises/e/code-security/configurations/defaults", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
		{
			"default_for_new_repos": "public",
			"configuration": {
				"id":1,
				"name":"config1",
				"description":"desc1",
				"code_scanning_default_setup": "enabled"
			}
		},
		{
			"default_for_new_repos": "private_and_internal",
			"configuration": {
				"id":2,
				"name":"config2",
				"description":"desc2",
				"private_vulnerability_reporting": "enabled"
			}
		}
	]`)
	})

	configurations, _, err := client.Enterprise.ListDefaultCodeSecurityConfigurations(ctx, "e")
	if err != nil {
		t.Errorf("Enterprise.ListDefaultCodeSecurityConfigurations returned error: %v", err)
	}

	want := []*CodeSecurityConfigurationWithDefaultForNewRepos{
		{DefaultForNewRepos: Ptr("public"), Configuration: &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}},
		{DefaultForNewRepos: Ptr("private_and_internal"), Configuration: &CodeSecurityConfiguration{ID: Ptr(int64(2)), Name: "config2", Description: "desc2", PrivateVulnerabilityReporting: Ptr("enabled")}},
	}
	if !cmp.Equal(configurations, want) {
		t.Errorf("Enterprise.ListDefaultCodeSecurityConfigurations returned %+v, want %+v", configurations, want)
	}

	const methodName = "ListDefaultCodeSecurityConfigurations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListDefaultCodeSecurityConfigurations(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListDefaultCodeSecurityConfigurations(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	input := CodeSecurityConfiguration{
		Name:                     "config1",
		Description:              "desc1",
		CodeScanningDefaultSetup: Ptr("enabled"),
	}

	mux.HandleFunc("/enterprises/e/code-security/configurations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id":1,
			"name":"config1",
			"description":"desc1",
			"code_scanning_default_setup": "enabled"
		}`)
	})

	configuration, _, err := client.Enterprise.UpdateCodeSecurityConfiguration(ctx, "e", 1, input)
	if err != nil {
		t.Errorf("Enterprise.UpdateCodeSecurityConfiguration returned error: %v", err)
	}

	want := &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Enterprise.UpdateCodeSecurityConfiguration returned %+v, want %+v", configuration, want)
	}

	const methodName = "UpdateCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.UpdateCodeSecurityConfiguration(ctx, "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateCodeSecurityConfiguration(ctx, "e", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/code-security/configurations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	resp, err := client.Enterprise.DeleteCodeSecurityConfiguration(ctx, "e", 1)
	if err != nil {
		t.Errorf("Enterprise.DeleteCodeSecurityConfiguration returned error: %v", err)
	}

	want := http.StatusNoContent
	if resp.StatusCode != want {
		t.Errorf("Enterprise.DeleteCodeSecurityConfiguration returned status %v, want %v", resp.StatusCode, want)
	}

	const methodName = "DeleteCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.DeleteCodeSecurityConfiguration(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		resp, err := client.Enterprise.DeleteCodeSecurityConfiguration(ctx, "e", 1)
		return resp, err
	})
}

func TestEnterpriseService_AttachCodeSecurityConfigurationToRepositories(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)
	input := struct {
		Scope string `json:"scope"`
	}{
		Scope: "all_without_configurations",
	}
	mux.HandleFunc("/enterprises/e/code-security/configurations/1/attach", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusAccepted)
	})

	resp, err := client.Enterprise.AttachCodeSecurityConfigurationToRepositories(ctx, "e", int64(1), input.Scope)
	if err != nil {
		t.Errorf("Enterprise.AttachCodeSecurityConfigurationToRepositories returned error: %v", err)
	}

	want := http.StatusAccepted
	if resp.StatusCode != want {
		t.Errorf("Enterprise.AttachCodeSecurityConfigurationToRepositories returned status %v, want %v", resp.StatusCode, want)
	}

	const methodName = "AttachCodeSecurityConfigurationToRepositories"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.AttachCodeSecurityConfigurationToRepositories(ctx, "\n", -1, "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		resp, err := client.Enterprise.AttachCodeSecurityConfigurationToRepositories(ctx, "e", 1, input.Scope)
		return resp, err
	})
}

func TestEnterpriseService_SetDefaultCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/code-security/configurations/1/defaults", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `
		{
			"default_for_new_repos": "all",
			"configuration":
				{
					"id": 1,
					"name": "config1",
					"description": "desc1",
					"code_scanning_default_setup": "enabled"
				}
		}`)
	})
	got, resp, err := client.Enterprise.SetDefaultCodeSecurityConfiguration(ctx, "e", 1, "all")
	if err != nil {
		t.Errorf("Enterprise.SetDefaultCodeSecurityConfiguration returned error: %v", err)
	}
	wantStatus := http.StatusOK
	if resp.StatusCode != wantStatus {
		t.Errorf("Enterprise.SetDefaultCodeSecurityConfiguration returned status %v, want %v", resp.StatusCode, wantStatus)
	}
	want := &CodeSecurityConfigurationWithDefaultForNewRepos{
		DefaultForNewRepos: Ptr("all"),
		Configuration: &CodeSecurityConfiguration{
			ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled"),
		},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.SetDefaultCodeSecurityConfiguration returned %+v, want %+v", got, want)
	}

	const methodName = "SetDefaultCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.SetDefaultCodeSecurityConfiguration(ctx, "\n", -1, "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.SetDefaultCodeSecurityConfiguration(ctx, "e", 1, "all")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListCodeSecurityConfigurationRepositories(t *testing.T) {
	t.Parallel()
	opts := &ListCodeSecurityConfigurationRepositoriesOptions{Before: "1", After: "2", PerPage: 30, Status: "attached"}
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/code-security/configurations/1/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"before": "1", "after": "2", "per_page": "30", "status": "attached"})
		fmt.Fprint(w, `[
		{
			"status": "attached",
			"repository": {
				"id":8,
				"name":"repo8"
			}
		},
		{
			"status": "attached",
			"repository": {
				"id":42,
				"name":"repo42"
			}
		}
	]`)
	})

	attachments, _, err := client.Enterprise.ListCodeSecurityConfigurationRepositories(ctx, "e", 1, opts)
	if err != nil {
		t.Errorf("Enterprise.ListCodeSecurityConfigurationRepositories returned error: %v", err)
	}
	want := []*RepositoryAttachment{
		{Status: Ptr("attached"), Repository: &Repository{ID: Ptr(int64(8)), Name: Ptr("repo8")}},
		{Status: Ptr("attached"), Repository: &Repository{ID: Ptr(int64(42)), Name: Ptr("repo42")}},
	}
	if !cmp.Equal(attachments, want) {
		t.Errorf("Enterprise.ListCodeSecurityConfigurationRepositories returned %+v, want %+v", attachments, want)
	}

	const methodName = "ListCodeSecurityConfigurationRepositories"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListCodeSecurityConfigurationRepositories(ctx, "\n", -1, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListCodeSecurityConfigurationRepositories(ctx, "e", 1, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
