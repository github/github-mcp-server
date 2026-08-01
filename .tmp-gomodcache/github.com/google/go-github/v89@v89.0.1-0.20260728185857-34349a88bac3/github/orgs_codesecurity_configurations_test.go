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

func TestOrganizationsService_ListCodeSecurityConfigurations(t *testing.T) {
	t.Parallel()
	opts := &ListOrgCodeSecurityConfigurationOptions{Before: "1", After: "2", PerPage: 30, TargetType: "all"}
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/code-security/configurations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"before": "1", "after": "2", "per_page": "30", "target_type": "all"})
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

	configurations, _, err := client.Organizations.ListCodeSecurityConfigurations(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListCodeSecurityConfigurations returned error: %v", err)
	}

	want := []*CodeSecurityConfiguration{
		{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")},
		{ID: Ptr(int64(2)), Name: "config2", Description: "desc2", PrivateVulnerabilityReporting: Ptr("enabled")},
	}
	if !cmp.Equal(configurations, want) {
		t.Errorf("Organizations.ListCodeSecurityConfigurations returned %+v, want %+v", configurations, want)
	}
	const methodName = "ListCodeSecurityConfigurations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListCodeSecurityConfigurations(ctx, "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListCodeSecurityConfigurations(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	mux.HandleFunc("/orgs/o/code-security/configurations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id":1,
			"name":"config1",
			"description":"desc1",
			"code_scanning_default_setup": "enabled"
		}`)
	})

	configuration, _, err := client.Organizations.GetCodeSecurityConfiguration(ctx, "o", 1)
	if err != nil {
		t.Errorf("Organizations.GetCodeSecurityConfiguration returned error: %v", err)
	}

	want := &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Organizations.GetCodeSecurityConfiguration returned %+v, want %+v", configuration, want)
	}

	const methodName = "GetCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetCodeSecurityConfiguration(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetCodeSecurityConfiguration(ctx, "o", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	input := CodeSecurityConfiguration{
		Name:                     "config1",
		Description:              "desc1",
		CodeScanningDefaultSetup: Ptr("enabled"),
	}

	mux.HandleFunc("/orgs/o/code-security/configurations", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id":1,
			"name":"config1",
			"description":"desc1",
			"code_scanning_default_setup": "enabled"
		}`)
	})

	configuration, _, err := client.Organizations.CreateCodeSecurityConfiguration(ctx, "o", input)
	if err != nil {
		t.Errorf("Organizations.CreateCodeSecurityConfiguration returned error: %v", err)
	}

	want := &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Organizations.CreateCodeSecurityConfiguration returned %+v, want %+v", configuration, want)
	}

	const methodName = "CreateCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateCodeSecurityConfiguration(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateCodeSecurityConfiguration(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateCodeSecurityConfigurationWithDelegatedBypass(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	input := CodeSecurityConfiguration{
		Name:                          "config1",
		Description:                   "desc1",
		SecretProtection:              Ptr("enabled"), // required to configure bypass
		SecretScanning:                Ptr("enabled"), // required to configure bypass
		SecretScanningPushProtection:  Ptr("enabled"), // required to configure bypass
		SecretScanningDelegatedBypass: Ptr("enabled"),
		SecretScanningDelegatedBypassOptions: &SecretScanningDelegatedBypassOptions{
			Reviewers: []*BypassReviewer{
				{
					ReviewerType: "TEAM",
					ReviewerID:   456,
				},
				{
					ReviewerType: "ROLE",
					ReviewerID:   789,
				},
			},
		},
	}

	mux.HandleFunc("/orgs/o/code-security/configurations", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id":123,
			"name":"config1",
			"description":"desc1",
			"secret_protection": "enabled",
			"secret_scanning": "enabled",
			"secret_scanning_push_protection": "enabled",
			"secret_scanning_delegated_bypass": "enabled",
			"secret_scanning_delegated_bypass_options": {
				"reviewers": [
					{
						"security_configuration_id": 123,
						"reviewer_type": "TEAM",	
						"reviewer_id": 456
					},
					{
						"security_configuration_id": 123,
						"reviewer_type": "ROLE",	
						"reviewer_id": 789
					}
				]			
			}
		}`)
	})

	configuration, _, err := client.Organizations.CreateCodeSecurityConfiguration(ctx, "o", input)
	if err != nil {
		t.Errorf("Organizations.CreateCodeSecurityConfiguration with Bypass returned error: %v", err)
	}

	want := &CodeSecurityConfiguration{
		ID:                            Ptr(int64(123)),
		Name:                          "config1",
		Description:                   "desc1",
		SecretProtection:              Ptr("enabled"),
		SecretScanning:                Ptr("enabled"),
		SecretScanningPushProtection:  Ptr("enabled"),
		SecretScanningDelegatedBypass: Ptr("enabled"),
		SecretScanningDelegatedBypassOptions: &SecretScanningDelegatedBypassOptions{
			Reviewers: []*BypassReviewer{
				{
					SecurityConfigurationID: Ptr(int64(123)),
					ReviewerType:            "TEAM",
					ReviewerID:              456,
				},
				{
					SecurityConfigurationID: Ptr(int64(123)),
					ReviewerType:            "ROLE",
					ReviewerID:              789,
				},
			},
		},
	}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Organizations.CreateCodeSecurityConfiguration with Bypass returned %+v, want %+v", configuration, want)
	}

	const methodName = "CreateCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateCodeSecurityConfiguration(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateCodeSecurityConfiguration(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListDefaultCodeSecurityConfigurations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	mux.HandleFunc("/orgs/o/code-security/configurations/defaults", func(w http.ResponseWriter, r *http.Request) {
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

	configurations, _, err := client.Organizations.ListDefaultCodeSecurityConfigurations(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.ListDefaultCodeSecurityConfigurations returned error: %v", err)
	}

	want := []*CodeSecurityConfigurationWithDefaultForNewRepos{
		{DefaultForNewRepos: Ptr("public"), Configuration: &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}},
		{DefaultForNewRepos: Ptr("private_and_internal"), Configuration: &CodeSecurityConfiguration{ID: Ptr(int64(2)), Name: "config2", Description: "desc2", PrivateVulnerabilityReporting: Ptr("enabled")}},
	}
	if !cmp.Equal(configurations, want) {
		t.Errorf("Organizations.ListDefaultCodeSecurityConfigurations returned %+v, want %+v", configurations, want)
	}

	const methodName = "ListDefaultCodeSecurityConfigurations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListDefaultCodeSecurityConfigurations(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListDefaultCodeSecurityConfigurations(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_DetachCodeSecurityConfigurationsFromRepositories(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	ctx := t.Context()

	mux.HandleFunc("/orgs/o/code-security/configurations/detach", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	resp, err := client.Organizations.DetachCodeSecurityConfigurationsFromRepositories(ctx, "o", []int64{1})
	if err != nil {
		t.Errorf("Organizations.DetachCodeSecurityConfigurationsFromRepositories returned error: %v", err)
	}

	want := http.StatusNoContent
	if resp.StatusCode != want {
		t.Errorf("Organizations.DetachCodeSecurityConfigurationsFromRepositories returned status %v, want %v", resp.StatusCode, want)
	}

	const methodName = "DetachCodeSecurityConfigurationsFromRepositories"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.DetachCodeSecurityConfigurationsFromRepositories(ctx, "\n", []int64{1})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		resp, err := client.Organizations.DetachCodeSecurityConfigurationsFromRepositories(ctx, "o", []int64{1})
		return resp, err
	})
}

func TestOrganizationsService_UpdateCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	input := CodeSecurityConfiguration{
		Name:                     "config1",
		Description:              "desc1",
		CodeScanningDefaultSetup: Ptr("enabled"),
	}

	mux.HandleFunc("/orgs/o/code-security/configurations/1", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id":1,
			"name":"config1",
			"description":"desc1",
			"code_scanning_default_setup": "enabled"
		}`)
	})

	configuration, _, err := client.Organizations.UpdateCodeSecurityConfiguration(ctx, "o", 1, input)
	if err != nil {
		t.Errorf("Organizations.UpdateCodeSecurityConfiguration returned error: %v", err)
	}

	want := &CodeSecurityConfiguration{ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}
	if !cmp.Equal(configuration, want) {
		t.Errorf("Organizations.UpdateCodeSecurityConfiguration returned %+v, want %+v", configuration, want)
	}

	const methodName = "UpdateCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.UpdateCodeSecurityConfiguration(ctx, "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.UpdateCodeSecurityConfiguration(ctx, "o", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_DeleteCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/code-security/configurations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	resp, err := client.Organizations.DeleteCodeSecurityConfiguration(ctx, "o", 1)
	if err != nil {
		t.Errorf("Organizations.DeleteCodeSecurityConfiguration returned error: %v", err)
	}

	want := http.StatusNoContent
	if resp.StatusCode != want {
		t.Errorf("Organizations.DeleteCodeSecurityConfiguration returned status %v, want %v", resp.StatusCode, want)
	}

	const methodName = "DeleteCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.DeleteCodeSecurityConfiguration(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		resp, err := client.Organizations.DeleteCodeSecurityConfiguration(ctx, "o", 1)
		return resp, err
	})
}

func TestOrganizationsService_AttachCodeSecurityConfigurationToRepositories(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	request := struct {
		Scope                 string  `json:"scope"`
		SelectedRepositoryIDs []int64 `json:"selected_repository_ids,omitempty"`
	}{
		Scope:                 "selected",
		SelectedRepositoryIDs: []int64{5, 20},
	}

	mux.HandleFunc("/orgs/o/code-security/configurations/1/attach", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, request)
		w.WriteHeader(http.StatusAccepted)
	})

	resp, err := client.Organizations.AttachCodeSecurityConfigurationToRepositories(ctx, "o", int64(1), "selected", []int64{5, 20})
	if err != nil {
		t.Errorf("Organizations.AttachCodeSecurityConfigurationToRepositories returned error: %v", err)
	}

	want := http.StatusAccepted
	if resp.StatusCode != want {
		t.Errorf("Organizations.AttachCodeSecurityConfigurationToRepositories returned status %v, want %v", resp.StatusCode, want)
	}

	const methodName = "AttachCodeSecurityConfigurationToRepositories"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.AttachCodeSecurityConfigurationToRepositories(ctx, "\n", -1, "", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		resp, err := client.Organizations.AttachCodeSecurityConfigurationToRepositories(ctx, "o", 1, "selected", []int64{5, 20})
		return resp, err
	})
}

func TestOrganizationsService_SetDefaultCodeSecurityConfiguration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/code-security/configurations/1/defaults", func(w http.ResponseWriter, r *http.Request) {
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
	got, resp, err := client.Organizations.SetDefaultCodeSecurityConfiguration(ctx, "o", 1, "all")
	if err != nil {
		t.Errorf("Organizations.SetDefaultCodeSecurityConfiguration returned error: %v", err)
	}
	wantStatus := http.StatusOK
	if resp.StatusCode != wantStatus {
		t.Errorf("Organizations.SetDefaultCodeSecurityConfiguration returned status %v, want %v", resp.StatusCode, wantStatus)
	}
	want := &CodeSecurityConfigurationWithDefaultForNewRepos{
		DefaultForNewRepos: Ptr("all"),
		Configuration: &CodeSecurityConfiguration{
			ID: Ptr(int64(1)), Name: "config1", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled"),
		},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("Organizations.SetDefaultCodeSecurityConfiguration returned %+v, want %+v", got, want)
	}

	const methodName = "SetDefaultCodeSecurityConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.SetDefaultCodeSecurityConfiguration(ctx, "\n", -1, "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.SetDefaultCodeSecurityConfiguration(ctx, "o", 1, "all")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListCodeSecurityConfigurationRepositories(t *testing.T) {
	t.Parallel()
	opts := &ListCodeSecurityConfigurationRepositoriesOptions{Before: "1", After: "2", PerPage: 30, Status: "attached"}
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/code-security/configurations/1/repositories", func(w http.ResponseWriter, r *http.Request) {
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

	attachments, _, err := client.Organizations.ListCodeSecurityConfigurationRepositories(ctx, "o", 1, opts)
	if err != nil {
		t.Errorf("Organizations.ListCodeSecurityConfigurationRepositories returned error: %v", err)
	}
	want := []*RepositoryAttachment{
		{Status: Ptr("attached"), Repository: &Repository{ID: Ptr(int64(8)), Name: Ptr("repo8")}},
		{Status: Ptr("attached"), Repository: &Repository{ID: Ptr(int64(42)), Name: Ptr("repo42")}},
	}
	if !cmp.Equal(attachments, want) {
		t.Errorf("Organizations.ListCodeSecurityConfigurationRepositories returned %+v, want %+v", attachments, want)
	}

	const methodName = "ListCodeSecurityConfigurationRepositories"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListCodeSecurityConfigurationRepositories(ctx, "\n", -1, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListCodeSecurityConfigurationRepositories(ctx, "o", 1, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetCodeSecurityConfigurationForRepository(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo8/code-security-configuration", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
		    "state": "attached",
			"configuration": {
				"id":42,
				"name":"config42",
				"description":"desc1",
				"code_scanning_default_setup": "enabled"
			}
		}`)
	})

	rc, _, err := client.Organizations.GetCodeSecurityConfigurationForRepository(ctx, "o", "repo8")
	if err != nil {
		t.Errorf("Organizations.GetCodeSecurityConfigurationForRepository returned error: %v", err)
	}
	c := &CodeSecurityConfiguration{ID: Ptr(int64(42)), Name: "config42", Description: "desc1", CodeScanningDefaultSetup: Ptr("enabled")}
	want := &RepositoryCodeSecurityConfiguration{
		State:         Ptr("attached"),
		Configuration: c,
	}
	if !cmp.Equal(rc, want) {
		t.Errorf("Organizations.GetCodeSecurityConfigurationForRepository returned %+v, want %+v", rc, want)
	}

	const methodName = "GetCodeSecurityConfigurationForRepository"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetCodeSecurityConfigurationForRepository(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetCodeSecurityConfigurationForRepository(ctx, "o", "repo8")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
