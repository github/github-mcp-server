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

func TestActionsService_GetActionsPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"enabled_repositories": "all", "allowed_actions": "all", "sha_pinning_required": true}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.GetActionsPermissions(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetActionsPermissions returned error: %v", err)
	}
	want := &ActionsPermissions{EnabledRepositories: Ptr("all"), AllowedActions: Ptr("all"), SHAPinningRequired: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.GetActionsPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "GetActionsPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetActionsPermissions(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetActionsPermissions(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateActionsPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsPermissions{EnabledRepositories: Ptr("all"), AllowedActions: Ptr("selected"), SHAPinningRequired: Ptr(true)}

	mux.HandleFunc("/orgs/o/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"enabled_repositories": "all", "allowed_actions": "selected", "sha_pinning_required": true}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.UpdateActionsPermissions(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateActionsPermissions returned error: %v", err)
	}

	want := &ActionsPermissions{EnabledRepositories: Ptr("all"), AllowedActions: Ptr("selected"), SHAPinningRequired: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.UpdateActionsPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "UpdateActionsPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.UpdateActionsPermissions(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.UpdateActionsPermissions(ctx, "o", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListEnabledReposInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "1",
		})
		fmt.Fprint(w, `{"total_count":2,"repositories":[{"id":2}, {"id": 3}]}`)
	})

	ctx := t.Context()
	opt := &ListOptions{
		Page: 1,
	}
	got, _, err := client.Actions.ListEnabledReposInOrg(ctx, "o", opt)
	if err != nil {
		t.Errorf("Actions.ListEnabledReposInOrg returned error: %v", err)
	}

	want := &ActionsEnabledOnOrgRepos{TotalCount: int(2), Repositories: []*Repository{
		{ID: Ptr(int64(2))},
		{ID: Ptr(int64(3))},
	}}
	if !cmp.Equal(got, want) {
		t.Errorf("Actions.ListEnabledReposInOrg returned %+v, want %+v", got, want)
	}

	const methodName = "ListEnabledRepos"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListEnabledReposInOrg(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListEnabledReposInOrg(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetEnabledReposInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []int64{123, 1234}

	mux.HandleFunc("/orgs/o/actions/permissions/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, struct {
			IDs []int64 `json:"selected_repository_ids"`
		}{
			IDs: input,
		})
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.SetEnabledReposInOrg(ctx, "o", input)
	if err != nil {
		t.Errorf("Actions.SetEnabledReposInOrg returned error: %v", err)
	}

	const methodName = "SetEnabledRepos"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetEnabledReposInOrg(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetEnabledReposInOrg(ctx, "o", input)
	})
}

func TestActionsService_AddEnabledReposInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/repositories/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.AddEnabledReposInOrg(ctx, "o", 123)
	if err != nil {
		t.Errorf("Actions.AddEnabledReposInOrg returned error: %v", err)
	}

	const methodName = "AddEnabledReposInOrg"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.AddEnabledReposInOrg(ctx, "\n", 123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.AddEnabledReposInOrg(ctx, "o", 123)
	})
}

func TestActionsService_RemoveEnabledReposInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/repositories/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.RemoveEnabledReposInOrg(ctx, "o", 123)
	if err != nil {
		t.Errorf("Actions.RemoveEnabledReposInOrg returned error: %v", err)
	}

	const methodName = "RemoveEnabledReposInOrg"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RemoveEnabledReposInOrg(ctx, "\n", 123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RemoveEnabledReposInOrg(ctx, "o", 123)
	})
}

func TestActionsService_GetActionsAllowed(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.GetActionsAllowed(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetActionsAllowed returned error: %v", err)
	}
	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.GetActionsAllowed returned %+v, want %+v", org, want)
	}

	const methodName = "GetActionsAllowed"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetActionsAllowed(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetActionsAllowed(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetActionsAllowed_emptyPatterns(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":[]}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.GetActionsAllowed(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetActionsAllowed returned error: %v", err)
	}
	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{}}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.GetActionsAllowed returned %+v, want %+v", org, want)
	}

	const methodName = "GetActionsAllowed"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetActionsAllowed(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetActionsAllowed(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateActionsAllowed(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.UpdateActionsAllowed(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateActionsAllowed returned error: %v", err)
	}

	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.UpdateActionsAllowed returned %+v, want %+v", org, want)
	}

	const methodName = "UpdateActionsAllowed"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.UpdateActionsAllowed(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.UpdateActionsAllowed(ctx, "o", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateActionsAllowed_emptyPatterns(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{}}

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":[]}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.UpdateActionsAllowed(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateActionsAllowed returned error: %v", err)
	}

	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{}}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.UpdateActionsAllowed returned %+v, want %+v", org, want)
	}
}

func TestActionsService_UpdateActionsAllowed_nilPatterns(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: nil}

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.UpdateActionsAllowed(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateActionsAllowed returned error: %v", err)
	}

	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.UpdateActionsAllowed returned %+v, want %+v", org, want)
	}
}

func TestActionsService_UpdateActionsAllowed_omittedPatterns(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false)}

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.UpdateActionsAllowed(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateActionsAllowed returned error: %v", err)
	}

	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.UpdateActionsAllowed returned %+v, want %+v", org, want)
	}
}

func TestActionsService_GetDefaultWorkflowPermissionsInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/workflow", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{ "default_workflow_permissions": "read", "can_approve_pull_request_reviews": true }`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.GetDefaultWorkflowPermissionsInOrganization(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetDefaultWorkflowPermissionsInOrganization returned error: %v", err)
	}
	want := &DefaultWorkflowPermissionOrganization{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.GetDefaultWorkflowPermissionsInOrganization returned %+v, want %+v", org, want)
	}

	const methodName = "GetDefaultWorkflowPermissionsInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetDefaultWorkflowPermissionsInOrganization(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetDefaultWorkflowPermissionsInOrganization(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateDefaultWorkflowPermissionsInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &DefaultWorkflowPermissionOrganization{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}

	mux.HandleFunc("/orgs/o/actions/permissions/workflow", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{ "default_workflow_permissions": "read", "can_approve_pull_request_reviews": true }`)
	})

	ctx := t.Context()
	org, _, err := client.Actions.UpdateDefaultWorkflowPermissionsInOrganization(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateDefaultWorkflowPermissionsInOrganization returned error: %v", err)
	}

	want := &DefaultWorkflowPermissionOrganization{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Actions.UpdateDefaultWorkflowPermissionsInOrganization returned %+v, want %+v", org, want)
	}

	const methodName = "UpdateDefaultWorkflowPermissionsInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.UpdateDefaultWorkflowPermissionsInOrganization(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.UpdateDefaultWorkflowPermissionsInOrganization(ctx, "o", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetArtifactAndLogRetentionPeriodInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/artifact-and-log-retention", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"days": 90, "maximum_allowed_days": 365}`)
	})

	ctx := t.Context()
	period, _, err := client.Actions.GetArtifactAndLogRetentionPeriodInOrganization(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetArtifactAndLogRetentionPeriodInOrganization returned error: %v", err)
	}

	want := &ArtifactPeriod{
		Days:               Ptr(90),
		MaximumAllowedDays: Ptr(365),
	}
	if !cmp.Equal(period, want) {
		t.Errorf("Actions.GetArtifactAndLogRetentionPeriodInOrganization = %+v, want %+v", period, want)
	}

	const methodName = "GetArtifactAndLogRetentionPeriodInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetArtifactAndLogRetentionPeriodInOrganization(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetArtifactAndLogRetentionPeriodInOrganization(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateArtifactAndLogRetentionPeriodInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ArtifactPeriodOpt{Days: Ptr(90)}

	mux.HandleFunc("/orgs/o/actions/permissions/artifact-and-log-retention", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdateArtifactAndLogRetentionPeriodInOrganization(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateArtifactAndLogRetentionPeriodInOrganization returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdateArtifactAndLogRetentionPeriodInOrganization = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateArtifactAndLogRetentionPeriodInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdateArtifactAndLogRetentionPeriodInOrganization(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdateArtifactAndLogRetentionPeriodInOrganization(ctx, "o", *input)
	})
}

func TestActionsService_GetSelfHostedRunnersSettingsInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/self-hosted-runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"enabled_repositories": "all", "selected_repositories_url": "https://api.github.com/orgs/octo-org/actions/permissions/self-hosted-runners/repositories"}`)
	})

	ctx := t.Context()
	settings, _, err := client.Actions.GetSelfHostedRunnersSettingsInOrganization(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetSelfHostedRunnersSettingsInOrganization returned error: %v", err)
	}
	want := &SelfHostedRunnersSettingsOrganization{
		EnabledRepositories:     Ptr("all"),
		SelectedRepositoriesURL: Ptr("https://api.github.com/orgs/octo-org/actions/permissions/self-hosted-runners/repositories"),
	}
	if !cmp.Equal(settings, want) {
		t.Errorf("Actions.GetSelfHostedRunnersSettingsInOrganization returned %+v, want %+v", settings, want)
	}

	const methodName = "GetSelfHostedRunnersSettingsInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetSelfHostedRunnersSettingsInOrganization(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetSelfHostedRunnersSettingsInOrganization(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateSelfHostedRunnersSettingsInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &SelfHostedRunnersSettingsOrganizationOpt{EnabledRepositories: Ptr("selected")}

	mux.HandleFunc("/orgs/o/actions/permissions/self-hosted-runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdateSelfHostedRunnersSettingsInOrganization(ctx, "o", *input)
	if err != nil {
		t.Errorf("Actions.UpdateSelfHostedRunnersSettingsInOrganization returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdateSelfHostedRunnersSettingsInOrganization = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateSelfHostedRunnersSettingsInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdateSelfHostedRunnersSettingsInOrganization(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdateSelfHostedRunnersSettingsInOrganization(ctx, "o", *input)
	})
}

func TestActionsService_ListRepositoriesSelfHostedRunnersAllowedInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/self-hosted-runners/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "1",
		})
		fmt.Fprint(w, `{"total_count":2,"repositories":[{"id":2}, {"id": 3}]}`)
	})

	ctx := t.Context()
	opt := &ListOptions{
		Page: 1,
	}
	got, _, err := client.Actions.ListRepositoriesSelfHostedRunnersAllowedInOrganization(ctx, "o", opt)
	if err != nil {
		t.Errorf("Actions.ListRepositoriesSelfHostedRunnersAllowedInOrganization returned error: %v", err)
	}

	want := &SelfHostedRunnersAllowedRepos{TotalCount: int(2), Repositories: []*Repository{
		{ID: Ptr(int64(2))},
		{ID: Ptr(int64(3))},
	}}
	if !cmp.Equal(got, want) {
		t.Errorf("Actions.ListRepositoriesSelfHostedRunnersAllowedInOrganization returned %+v, want %+v", got, want)
	}

	const methodName = "ListRepositoriesSelfHostedRunnersAllowedInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListRepositoriesSelfHostedRunnersAllowedInOrganization(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListRepositoriesSelfHostedRunnersAllowedInOrganization(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetRepositoriesSelfHostedRunnersAllowedInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []int64{123, 1234}

	mux.HandleFunc("/orgs/o/actions/permissions/self-hosted-runners/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, struct {
			IDs []int64 `json:"selected_repository_ids"`
		}{
			IDs: input,
		})
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.SetRepositoriesSelfHostedRunnersAllowedInOrganization(ctx, "o", input)
	if err != nil {
		t.Errorf("Actions.SetRepositoriesSelfHostedRunnersAllowedInOrganization returned error: %v", err)
	}

	const methodName = "SetRepositoriesSelfHostedRunnersAllowedInOrganization"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetRepositoriesSelfHostedRunnersAllowedInOrganization(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetRepositoriesSelfHostedRunnersAllowedInOrganization(ctx, "o", input)
	})
}

func TestActionsService_AddRepositorySelfHostedRunnersAllowedInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/self-hosted-runners/repositories/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.AddRepositorySelfHostedRunnersAllowedInOrganization(ctx, "o", 123)
	if err != nil {
		t.Errorf("Actions.AddRepositorySelfHostedRunnersAllowedInOrganization returned error: %v", err)
	}

	const methodName = "AddRepositorySelfHostedRunnersAllowedInOrganization"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.AddRepositorySelfHostedRunnersAllowedInOrganization(ctx, "\n", 123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.AddRepositorySelfHostedRunnersAllowedInOrganization(ctx, "o", 123)
	})
}

func TestActionsService_RemoveRepositorySelfHostedRunnersAllowedInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/self-hosted-runners/repositories/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.RemoveRepositorySelfHostedRunnersAllowedInOrganization(ctx, "o", 123)
	if err != nil {
		t.Errorf("Actions.RemoveRepositorySelfHostedRunnersAllowedInOrganization returned error: %v", err)
	}

	const methodName = "RemoveRepositorySelfHostedRunnersAllowedInOrganization"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RemoveRepositorySelfHostedRunnersAllowedInOrganization(ctx, "\n", 123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RemoveRepositorySelfHostedRunnersAllowedInOrganization(ctx, "o", 123)
	})
}

func TestActionsService_GetPrivateRepoForkPRWorkflowSettingsInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/fork-pr-workflows-private-repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"run_workflows_from_fork_pull_requests": true, "send_write_tokens_to_workflows": false, "send_secrets_and_variables": true, "require_approval_for_fork_pr_workflows": false}`)
	})

	ctx := t.Context()
	permissions, _, err := client.Actions.GetPrivateRepoForkPRWorkflowSettingsInOrganization(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetPrivateRepoForkPRWorkflowSettingsInOrganization returned error: %v", err)
	}
	want := &WorkflowsPermissions{
		RunWorkflowsFromForkPullRequests:  Ptr(true),
		SendWriteTokensToWorkflows:        Ptr(false),
		SendSecretsAndVariables:           Ptr(true),
		RequireApprovalForForkPRWorkflows: Ptr(false),
	}
	if !cmp.Equal(permissions, want) {
		t.Errorf("Actions.GetPrivateRepoForkPRWorkflowSettingsInOrganization returned %+v, want %+v", permissions, want)
	}

	const methodName = "GetPrivateRepoForkPRWorkflowSettingsInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetPrivateRepoForkPRWorkflowSettingsInOrganization(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetPrivateRepoForkPRWorkflowSettingsInOrganization(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdatePrivateRepoForkPRWorkflowSettingsInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &WorkflowsPermissionsOpt{
		RunWorkflowsFromForkPullRequests: true,
		SendWriteTokensToWorkflows:       Ptr(false),
		SendSecretsAndVariables:          Ptr(true),
	}

	mux.HandleFunc("/orgs/o/actions/permissions/fork-pr-workflows-private-repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdatePrivateRepoForkPRWorkflowSettingsInOrganization(ctx, "o", input)
	if err != nil {
		t.Errorf("Actions.UpdatePrivateRepoForkPRWorkflowSettingsInOrganization returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdatePrivateRepoForkPRWorkflowSettingsInOrganization = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdatePrivateRepoForkPRWorkflowSettingsInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdatePrivateRepoForkPRWorkflowSettingsInOrganization(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdatePrivateRepoForkPRWorkflowSettingsInOrganization(ctx, "o", input)
	})
}

func TestActionsService_GetOrganizationForkPRContributorApprovalPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/fork-pr-contributor-approval", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"approval_policy": "require_approval"}`)
	})

	ctx := t.Context()
	policy, _, err := client.Actions.GetOrganizationForkPRContributorApprovalPermissions(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetOrganizationForkPRContributorApprovalPermissions returned error: %v", err)
	}
	want := &ContributorApprovalPermissions{ApprovalPolicy: "require_approval"}
	if !cmp.Equal(policy, want) {
		t.Errorf("Actions.GetOrganizationForkPRContributorApprovalPermissions returned %+v, want %+v", policy, want)
	}

	const methodName = "GetOrganizationForkPRContributorApprovalPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetOrganizationForkPRContributorApprovalPermissions(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetOrganizationForkPRContributorApprovalPermissions(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateOrganizationForkPRContributorApprovalPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ContributorApprovalPermissions{ApprovalPolicy: "require_approval"}

	mux.HandleFunc("/orgs/o/actions/permissions/fork-pr-contributor-approval", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdateOrganizationForkPRContributorApprovalPermissions(ctx, "o", input)
	if err != nil {
		t.Errorf("Actions.UpdateOrganizationForkPRContributorApprovalPermissions returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdateOrganizationForkPRContributorApprovalPermissions = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateOrganizationForkPRContributorApprovalPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdateOrganizationForkPRContributorApprovalPermissions(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdateOrganizationForkPRContributorApprovalPermissions(ctx, "o", input)
	})
}
