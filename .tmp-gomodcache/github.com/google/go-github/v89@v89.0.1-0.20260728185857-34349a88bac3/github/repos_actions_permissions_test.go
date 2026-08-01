// Copyright 2022 The go-github AUTHORS. All rights reserved.
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

func TestRepositoriesService_GetActionsPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"enabled": true, "allowed_actions": "all", "sha_pinning_required": true}`)
	})

	ctx := t.Context()
	org, _, err := client.Repositories.GetActionsPermissions(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetActionsPermissions returned error: %v", err)
	}
	want := &ActionsPermissionsRepository{Enabled: Ptr(true), AllowedActions: Ptr("all"), SHAPinningRequired: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Repositories.GetActionsPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "GetActionsPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetActionsPermissions(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetActionsPermissions(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateActionsPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsPermissionsRepository{Enabled: Ptr(true), AllowedActions: Ptr("selected"), SHAPinningRequired: Ptr(true)}

	mux.HandleFunc("/repos/o/r/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"enabled": true, "allowed_actions": "selected", "sha_pinning_required": true}`)
	})

	ctx := t.Context()
	org, _, err := client.Repositories.UpdateActionsPermissions(ctx, "o", "r", *input)
	if err != nil {
		t.Errorf("Repositories.UpdateActionsPermissions returned error: %v", err)
	}

	want := &ActionsPermissionsRepository{Enabled: Ptr(true), AllowedActions: Ptr("selected"), SHAPinningRequired: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Repositories.UpdateActionsPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "UpdateActionsPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UpdateActionsPermissions(ctx, "\n", "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateActionsPermissions(ctx, "o", "r", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetDefaultWorkflowPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/permissions/workflow", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{ "default_workflow_permissions": "read", "can_approve_pull_request_reviews": true }`)
	})

	ctx := t.Context()
	org, _, err := client.Repositories.GetDefaultWorkflowPermissions(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetDefaultWorkflowPermissions returned error: %v", err)
	}
	want := &DefaultWorkflowPermissionRepository{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Repositories.GetDefaultWorkflowPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "GetDefaultWorkflowPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetDefaultWorkflowPermissions(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetDefaultWorkflowPermissions(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateDefaultWorkflowPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &DefaultWorkflowPermissionRepository{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}

	mux.HandleFunc("/repos/o/r/actions/permissions/workflow", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{ "default_workflow_permissions": "read", "can_approve_pull_request_reviews": true }`)
	})

	ctx := t.Context()
	org, _, err := client.Repositories.UpdateDefaultWorkflowPermissions(ctx, "o", "r", *input)
	if err != nil {
		t.Errorf("Repositories.UpdateDefaultWorkflowPermissions returned error: %v", err)
	}

	want := &DefaultWorkflowPermissionRepository{DefaultWorkflowPermissions: Ptr("read"), CanApprovePullRequestReviews: Ptr(true)}
	if !cmp.Equal(org, want) {
		t.Errorf("Repositories.UpdateDefaultWorkflowPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "UpdateDefaultWorkflowPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UpdateDefaultWorkflowPermissions(ctx, "\n", "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateDefaultWorkflowPermissions(ctx, "o", "r", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetArtifactAndLogRetentionPeriod(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/permissions/artifact-and-log-retention", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"days": 90, "maximum_allowed_days": 365}`)
	})

	ctx := t.Context()
	period, _, err := client.Repositories.GetArtifactAndLogRetentionPeriod(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetArtifactAndLogRetentionPeriod returned error: %v", err)
	}

	want := &ArtifactPeriod{
		Days:               Ptr(90),
		MaximumAllowedDays: Ptr(365),
	}
	if !cmp.Equal(period, want) {
		t.Errorf("Repositories.GetArtifactAndLogRetentionPeriod = %+v, want %+v", period, want)
	}

	const methodName = "GetArtifactAndLogRetentionPeriod"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetArtifactAndLogRetentionPeriod(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetArtifactAndLogRetentionPeriod(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateArtifactAndLogRetentionPeriod(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ArtifactPeriodOpt{Days: Ptr(90)}

	mux.HandleFunc("/repos/o/r/actions/permissions/artifact-and-log-retention", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Repositories.UpdateArtifactAndLogRetentionPeriod(ctx, "o", "r", *input)
	if err != nil {
		t.Errorf("Repositories.UpdateArtifactAndLogRetentionPeriod returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Repositories.UpdateArtifactAndLogRetentionPeriod = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateArtifactAndLogRetentionPeriod"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.UpdateArtifactAndLogRetentionPeriod(ctx, "\n", "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.UpdateArtifactAndLogRetentionPeriod(ctx, "o", "r", *input)
	})
}

func TestRepositoriesService_GetPrivateRepoForkPRWorkflowSettings(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/permissions/fork-pr-workflows-private-repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"run_workflows_from_fork_pull_requests": true, "send_write_tokens_to_workflows": false, "send_secrets_and_variables": true, "require_approval_for_fork_pr_workflows": false}`)
	})

	ctx := t.Context()
	permissions, _, err := client.Repositories.GetPrivateRepoForkPRWorkflowSettings(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetPrivateRepoForkPRWorkflowSettings returned error: %v", err)
	}
	want := &WorkflowsPermissions{
		RunWorkflowsFromForkPullRequests:  Ptr(true),
		SendWriteTokensToWorkflows:        Ptr(false),
		SendSecretsAndVariables:           Ptr(true),
		RequireApprovalForForkPRWorkflows: Ptr(false),
	}
	if !cmp.Equal(permissions, want) {
		t.Errorf("Repositories.GetPrivateRepoForkPRWorkflowSettings returned %+v, want %+v", permissions, want)
	}

	const methodName = "GetPrivateRepoForkPRWorkflowSettings"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetPrivateRepoForkPRWorkflowSettings(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetPrivateRepoForkPRWorkflowSettings(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdatePrivateRepoForkPRWorkflowSettings(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &WorkflowsPermissionsOpt{
		RunWorkflowsFromForkPullRequests: true,
		SendWriteTokensToWorkflows:       Ptr(false),
		SendSecretsAndVariables:          Ptr(true),
	}

	mux.HandleFunc("/repos/o/r/actions/permissions/fork-pr-workflows-private-repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Repositories.UpdatePrivateRepoForkPRWorkflowSettings(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Repositories.UpdatePrivateRepoForkPRWorkflowSettings returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Repositories.UpdatePrivateRepoForkPRWorkflowSettings = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdatePrivateRepoForkPRWorkflowSettings"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.UpdatePrivateRepoForkPRWorkflowSettings(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.UpdatePrivateRepoForkPRWorkflowSettings(ctx, "o", "r", input)
	})
}

func TestActionsService_GetForkPRContributorApprovalPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/permissions/fork-pr-contributor-approval", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"approval_policy": "require_approval"}`)
	})

	ctx := t.Context()
	policy, _, err := client.Actions.GetForkPRContributorApprovalPermissions(ctx, "o", "r")
	if err != nil {
		t.Errorf("Actions.GetForkPRContributorApprovalPermissions returned error: %v", err)
	}
	want := &ContributorApprovalPermissions{ApprovalPolicy: "require_approval"}
	if !cmp.Equal(policy, want) {
		t.Errorf("Actions.GetForkPRContributorApprovalPermissions returned %+v, want %+v", policy, want)
	}

	const methodName = "GetForkPRContributorApprovalPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetForkPRContributorApprovalPermissions(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetForkPRContributorApprovalPermissions(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_UpdateForkPRContributorApprovalPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ContributorApprovalPermissions{ApprovalPolicy: "require_approval"}

	mux.HandleFunc("/repos/o/r/actions/permissions/fork-pr-contributor-approval", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Actions.UpdateForkPRContributorApprovalPermissions(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Actions.UpdateForkPRContributorApprovalPermissions returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Actions.UpdateForkPRContributorApprovalPermissions = %v, want %v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "UpdateForkPRContributorApprovalPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.UpdateForkPRContributorApprovalPermissions(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.UpdateForkPRContributorApprovalPermissions(ctx, "o", "r", input)
	})
}
