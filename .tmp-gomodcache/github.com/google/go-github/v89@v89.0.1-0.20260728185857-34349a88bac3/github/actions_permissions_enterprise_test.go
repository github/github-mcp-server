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

func TestActionsService_GetActionsPermissionsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"enabled_organizations": "all", "allowed_actions": "all"}`)
	})

	ctx := t.Context()
	ent, _, err := client.Actions.GetActionsPermissionsInEnterprise(ctx, "e")
	if err != nil {
		t.Errorf("Actions.GetActionsPermissionsInEnterprise returned error: %v", err)
	}
	want := &ActionsPermissionsEnterprise{EnabledOrganizations: Ptr("all"), AllowedActions: Ptr("all")}
	if !cmp.Equal(ent, want) {
		t.Errorf("Actions.GetActionsPermissionsInEnterprise returned %+v, want %+v", ent, want)
	}

	const methodName = "GetActionsPermissionsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetActionsPermissionsInEnterprise(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetActionsPermissionsInEnterprise(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateActionsPermissionsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsPermissionsEnterprise{EnabledOrganizations: Ptr("all"), AllowedActions: Ptr("selected")}

	mux.HandleFunc("/enterprises/e/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"enabled_organizations": "all", "allowed_actions": "selected"}`)
	})

	ctx := t.Context()
	ent, _, err := client.Actions.UpdateActionsPermissionsInEnterprise(ctx, "e", *input)
	if err != nil {
		t.Errorf("Actions.UpdateActionsPermissionsInEnterprise returned error: %v", err)
	}

	want := &ActionsPermissionsEnterprise{EnabledOrganizations: Ptr("all"), AllowedActions: Ptr("selected")}
	if !cmp.Equal(ent, want) {
		t.Errorf("Actions.UpdateActionsPermissionsInEnterprise returned %+v, want %+v", ent, want)
	}

	const methodName = "UpdateActionsPermissionsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.UpdateActionsPermissionsInEnterprise(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.UpdateActionsPermissionsInEnterprise(ctx, "e", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListEnabledOrgsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/organizations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "1",
		})
		fmt.Fprint(w, `{"total_count":2,"organizations":[{"id":2}, {"id":3}]}`)
	})

	ctx := t.Context()
	opt := &ListOptions{
		Page: 1,
	}
	got, _, err := client.Actions.ListEnabledOrgsInEnterprise(ctx, "e", opt)
	if err != nil {
		t.Errorf("Actions.ListEnabledOrgsInEnterprise returned error: %v", err)
	}

	want := &ActionsEnabledOnEnterpriseRepos{TotalCount: int(2), Organizations: []*Organization{
		{ID: Ptr(int64(2))},
		{ID: Ptr(int64(3))},
	}}
	if !cmp.Equal(got, want) {
		t.Errorf("Actions.ListEnabledOrgsInEnterprise returned %+v, want %+v", got, want)
	}

	const methodName = "ListEnabledOrgsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListEnabledOrgsInEnterprise(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListEnabledOrgsInEnterprise(ctx, "e", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetEnabledOrgsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []int64{123, 1234}

	mux.HandleFunc("/enterprises/e/actions/permissions/organizations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, struct {
			IDs []int64 `json:"selected_organization_ids"`
		}{
			IDs: input,
		})
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.SetEnabledOrgsInEnterprise(ctx, "e", input)
	if err != nil {
		t.Errorf("Actions.SetEnabledOrgsInEnterprise returned error: %v", err)
	}

	const methodName = "SetEnabledOrgsInEnterprise"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetEnabledOrgsInEnterprise(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetEnabledOrgsInEnterprise(ctx, "e", input)
	})
}

func TestActionsService_AddEnabledOrgInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/organizations/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.AddEnabledOrgInEnterprise(ctx, "e", 123)
	if err != nil {
		t.Errorf("Actions.AddEnabledOrgInEnterprise returned error: %v", err)
	}

	const methodName = "AddEnabledOrgInEnterprise"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.AddEnabledOrgInEnterprise(ctx, "\n", 123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.AddEnabledOrgInEnterprise(ctx, "e", 123)
	})
}

func TestActionsService_RemoveEnabledOrgInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/organizations/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Actions.RemoveEnabledOrgInEnterprise(ctx, "e", 123)
	if err != nil {
		t.Errorf("Actions.RemoveEnabledOrgInEnterprise returned error: %v", err)
	}

	const methodName = "RemoveEnabledOrgInEnterprise"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RemoveEnabledOrgInEnterprise(ctx, "\n", 123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RemoveEnabledOrgInEnterprise(ctx, "e", 123)
	})
}

func TestActionsService_GetActionsAllowedInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	ent, _, err := client.Actions.GetActionsAllowedInEnterprise(ctx, "e")
	if err != nil {
		t.Errorf("Actions.GetActionsAllowedInEnterprise returned error: %v", err)
	}
	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(ent, want) {
		t.Errorf("Actions.GetActionsAllowedInEnterprise returned %+v, want %+v", ent, want)
	}

	const methodName = "GetActionsAllowedInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetActionsAllowedInEnterprise(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetActionsAllowedInEnterprise(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateActionsAllowedInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}

	mux.HandleFunc("/enterprises/e/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	ent, _, err := client.Actions.UpdateActionsAllowedInEnterprise(ctx, "e", *input)
	if err != nil {
		t.Errorf("Actions.UpdateActionsAllowedInEnterprise returned error: %v", err)
	}

	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(ent, want) {
		t.Errorf("Actions.UpdateActionsAllowedInEnterprise returned %+v, want %+v", ent, want)
	}

	const methodName = "UpdateActionsAllowedInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.UpdateActionsAllowedInEnterprise(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.UpdateActionsAllowedInEnterprise(ctx, "e", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetDefaultWorkflowPermissionsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/workflow", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{ "default_workflow_permissions": "read", "can_approve_pull_request_reviews": true }`)
	})

	ctx := t.Context()
	ent, _, err := client.Actions.GetDefaultWorkflowPermissionsInEnterprise(ctx, "e")
	if err != nil {
		t.Errorf("Actions.GetDefaultWorkflowPermissionsInEnterprise returned error: %v", err)
	}
	want := &DefaultWorkflowPermissionEnterprise{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}
	if !cmp.Equal(ent, want) {
		t.Errorf("Actions.GetDefaultWorkflowPermissionsInEnterprise returned %+v, want %+v", ent, want)
	}

	const methodName = "GetDefaultWorkflowPermissionsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetDefaultWorkflowPermissionsInEnterprise(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetDefaultWorkflowPermissionsInEnterprise(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateDefaultWorkflowPermissionsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &DefaultWorkflowPermissionEnterprise{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}

	mux.HandleFunc("/enterprises/e/actions/permissions/workflow", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{ "default_workflow_permissions": "read", "can_approve_pull_request_reviews": true }`)
	})

	ctx := t.Context()
	ent, _, err := client.Actions.UpdateDefaultWorkflowPermissionsInEnterprise(ctx, "e", *input)
	if err != nil {
		t.Errorf("Actions.UpdateDefaultWorkflowPermissionsInEnterprise returned error: %v", err)
	}

	want := &DefaultWorkflowPermissionEnterprise{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}
	if !cmp.Equal(ent, want) {
		t.Errorf("Actions.UpdateDefaultWorkflowPermissionsInEnterprise returned %+v, want %+v", ent, want)
	}

	const methodName = "UpdateDefaultWorkflowPermissionsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.UpdateDefaultWorkflowPermissionsInEnterprise(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.UpdateDefaultWorkflowPermissionsInEnterprise(ctx, "e", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetArtifactAndLogRetentionPeriodInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/artifact-and-log-retention", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"days": 90, "maximum_allowed_days": 365}`)
	})

	ctx := t.Context()
	period, _, err := client.Actions.GetArtifactAndLogRetentionPeriodInEnterprise(ctx, "e")
	if err != nil {
		t.Errorf("Actions.GetArtifactAndLogRetentionPeriodInEnterprise returned error: %v", err)
	}

	want := &ArtifactPeriod{
		Days:               Ptr(90),
		MaximumAllowedDays: Ptr(365),
	}
	if !cmp.Equal(period, want) {
		t.Errorf("Actions.GetArtifactAndLogRetentionPeriodInEnterprise = %+v, want %+v", period, want)
	}

	const methodName = "GetArtifactAndLogRetentionPeriodInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetArtifactAndLogRetentionPeriodInEnterprise(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetArtifactAndLogRetentionPeriodInEnterprise(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateArtifactAndLogRetentionPeriodInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ArtifactPeriodOpt{Days: Ptr(90)}

	mux.HandleFunc("/enterprises/e/actions/permissions/artifact-and-log-retention", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdateArtifactAndLogRetentionPeriodInEnterprise(ctx, "e", *input)
	if err != nil {
		t.Errorf("Actions.UpdateArtifactAndLogRetentionPeriodInEnterprise returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdateArtifactAndLogRetentionPeriodInEnterprise = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateArtifactAndLogRetentionPeriodInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdateArtifactAndLogRetentionPeriodInEnterprise(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdateArtifactAndLogRetentionPeriodInEnterprise(ctx, "e", *input)
	})
}

func TestActionsService_GetSelfHostedRunnerPermissionsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/self-hosted-runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"disable_self_hosted_runners_for_all_orgs": true}`)
	})

	ctx := t.Context()
	permissions, _, err := client.Actions.GetSelfHostedRunnerPermissionsInEnterprise(ctx, "e")
	if err != nil {
		t.Errorf("Actions.GetSelfHostedRunnerPermissionsInEnterprise returned error: %v", err)
	}
	want := &SelfHostRunnerPermissionsEnterprise{DisableSelfHostedRunnersForAllOrgs: Ptr(true)}
	if !cmp.Equal(permissions, want) {
		t.Errorf("Actions.GetSelfHostedRunnerPermissionsInEnterprise returned %+v, want %+v", permissions, want)
	}

	const methodName = "GetSelfHostedRunnerPermissionsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetSelfHostedRunnerPermissionsInEnterprise(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetSelfHostedRunnerPermissionsInEnterprise(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateSelfHostedRunnerPermissionsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &SelfHostRunnerPermissionsEnterprise{DisableSelfHostedRunnersForAllOrgs: Ptr(false)}

	mux.HandleFunc("/enterprises/e/actions/permissions/self-hosted-runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdateSelfHostedRunnerPermissionsInEnterprise(ctx, "e", *input)
	if err != nil {
		t.Errorf("Actions.UpdateSelfHostedRunnerPermissionsInEnterprise returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdateSelfHostedRunnerPermissionsInEnterprise = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateSelfHostedRunnerPermissionsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdateSelfHostedRunnerPermissionsInEnterprise(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdateSelfHostedRunnerPermissionsInEnterprise(ctx, "e", *input)
	})
}

func TestActionsService_GetPrivateRepoForkPRWorkflowSettingsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/fork-pr-workflows-private-repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"run_workflows_from_fork_pull_requests": true, "send_write_tokens_to_workflows": false, "send_secrets_and_variables": true, "require_approval_for_fork_pr_workflows": false}`)
	})

	ctx := t.Context()
	permissions, _, err := client.Actions.GetPrivateRepoForkPRWorkflowSettingsInEnterprise(ctx, "e")
	if err != nil {
		t.Errorf("Actions.GetPrivateRepoForkPRWorkflowSettingsInEnterprise returned error: %v", err)
	}
	want := &WorkflowsPermissions{
		RunWorkflowsFromForkPullRequests:  Ptr(true),
		SendWriteTokensToWorkflows:        Ptr(false),
		SendSecretsAndVariables:           Ptr(true),
		RequireApprovalForForkPRWorkflows: Ptr(false),
	}
	if !cmp.Equal(permissions, want) {
		t.Errorf("Actions.GetPrivateRepoForkPRWorkflowSettingsInEnterprise returned %+v, want %+v", permissions, want)
	}

	const methodName = "GetPrivateRepoForkPRWorkflowSettingsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetPrivateRepoForkPRWorkflowSettingsInEnterprise(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetPrivateRepoForkPRWorkflowSettingsInEnterprise(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &WorkflowsPermissionsOpt{
		RunWorkflowsFromForkPullRequests: true,
		SendWriteTokensToWorkflows:       Ptr(false),
		SendSecretsAndVariables:          Ptr(true),
	}

	mux.HandleFunc("/enterprises/e/actions/permissions/fork-pr-workflows-private-repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise(ctx, "e", input)
	if err != nil {
		t.Errorf("Actions.UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise(ctx, "e", input)
	})
}

func TestActionsService_GetEnterpriseForkPRContributorApprovalPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/permissions/fork-pr-contributor-approval", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"approval_policy": "require_approval"}`)
	})

	ctx := t.Context()
	policy, _, err := client.Actions.GetEnterpriseForkPRContributorApprovalPermissions(ctx, "e")
	if err != nil {
		t.Errorf("Actions.GetEnterpriseForkPRContributorApprovalPermissions returned error: %v", err)
	}
	want := &ContributorApprovalPermissions{ApprovalPolicy: "require_approval"}
	if !cmp.Equal(policy, want) {
		t.Errorf("Actions.GetEnterpriseForkPRContributorApprovalPermissions returned %+v, want %+v", policy, want)
	}

	const methodName = "GetEnterpriseForkPRContributorApprovalPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetEnterpriseForkPRContributorApprovalPermissions(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetEnterpriseForkPRContributorApprovalPermissions(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateEnterpriseForkPRContributorApprovalPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ContributorApprovalPermissions{ApprovalPolicy: "require_approval"}

	mux.HandleFunc("/enterprises/e/actions/permissions/fork-pr-contributor-approval", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdateEnterpriseForkPRContributorApprovalPermissions(ctx, "e", input)
	if err != nil {
		t.Errorf("Actions.UpdateEnterpriseForkPRContributorApprovalPermissions returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdateEnterpriseForkPRContributorApprovalPermissions = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateEnterpriseForkPRContributorApprovalPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdateEnterpriseForkPRContributorApprovalPermissions(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdateEnterpriseForkPRContributorApprovalPermissions(ctx, "e", input)
	})
}
