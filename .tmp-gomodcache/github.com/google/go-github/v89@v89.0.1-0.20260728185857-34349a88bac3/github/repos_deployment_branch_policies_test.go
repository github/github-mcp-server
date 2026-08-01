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

func TestRepositoriesService_ListDeploymentBranchPolicies(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment-branch-policies", func(w http.ResponseWriter, r *http.Request) {
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "30",
		})
		fmt.Fprint(w, `{"total_count":2, "branch_policies":[{"id":1}, {"id": 2}]}`)
	})

	opts := &ListOptions{Page: 1, PerPage: 30}
	ctx := t.Context()
	got, _, err := client.Repositories.ListDeploymentBranchPolicies(ctx, "o", "r", "e", opts)
	if err != nil {
		t.Errorf("Repositories.ListDeploymentBranchPolicies returned error: %v", err)
	}

	want := &DeploymentBranchPolicyResponse{
		BranchPolicies: []*DeploymentBranchPolicy{
			{ID: Ptr(int64(1))},
			{ID: Ptr(int64(2))},
		},
		TotalCount: Ptr(2),
	}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.ListDeploymentBranchPolicies = %+v, want %+v", got, want)
	}

	const methodName = "ListDeploymentBranchPolicies"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListDeploymentBranchPolicies(ctx, "\n", "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListDeploymentBranchPolicies(ctx, "o", "r", "e", opts)
		if got != nil {
			t.Errorf("got non-nil Repositories.ListDeploymentBranchPolicies response: %+v", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetDeploymentBranchPolicy(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment-branch-policies/1", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetDeploymentBranchPolicy(ctx, "o", "r", "e", 1)
	if err != nil {
		t.Errorf("Repositories.GetDeploymentBranchPolicy returned error: %v", err)
	}

	want := &DeploymentBranchPolicy{ID: Ptr(int64(1))}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.GetDeploymentBranchPolicy = %+v, want %+v", got, want)
	}

	const methodName = "GetDeploymentBranchPolicy"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetDeploymentBranchPolicy(ctx, "o", "r", "e", 1)
		if got != nil {
			t.Errorf("got non-nil Repositories.GetDeploymentBranchPolicy response: %+v", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateDeploymentBranchPolicy(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment-branch-policies", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":1, "type":"branch"}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.CreateDeploymentBranchPolicy(ctx, "o", "r", "e", CreateDeploymentBranchPolicyRequest{Name: "n", Type: Ptr("branch")})
	if err != nil {
		t.Errorf("Repositories.CreateDeploymentBranchPolicy returned error: %v", err)
	}

	want := &DeploymentBranchPolicy{ID: Ptr(int64(1)), Type: Ptr("branch")}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.CreateDeploymentBranchPolicy = %+v, want %+v", got, want)
	}

	const methodName = "CreateDeploymentBranchPolicy"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateDeploymentBranchPolicy(ctx, "o", "r", "e", CreateDeploymentBranchPolicyRequest{Name: "n"})
		if got != nil {
			t.Errorf("got non-nil Repositories.CreateDeploymentBranchPolicy response: %+v", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateDeploymentBranchPolicy(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment-branch-policies/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.UpdateDeploymentBranchPolicy(ctx, "o", "r", "e", 1, UpdateDeploymentBranchPolicyRequest{Name: "n"})
	if err != nil {
		t.Errorf("Repositories.UpdateDeploymentBranchPolicy returned error: %v", err)
	}

	want := &DeploymentBranchPolicy{ID: Ptr(int64(1))}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.UpdateDeploymentBranchPolicy = %+v, want %+v", got, want)
	}

	const methodName = "UpdateDeploymentBranchPolicy"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateDeploymentBranchPolicy(ctx, "o", "r", "e", 1, UpdateDeploymentBranchPolicyRequest{Name: "n"})
		if got != nil {
			t.Errorf("got non-nil Repositories.UpdateDeploymentBranchPolicy response: %+v", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DeleteDeploymentBranchPolicy(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment-branch-policies/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Repositories.DeleteDeploymentBranchPolicy(ctx, "o", "r", "e", 1)
	if err != nil {
		t.Errorf("Repositories.DeleteDeploymentBranchPolicy returned error: %v", err)
	}

	const methodName = "DeleteDeploymentBranchPolicy"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteDeploymentBranchPolicy(ctx, "o", "r", "e", 1)
	})
}
