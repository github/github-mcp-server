// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRequiredReviewer_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		data      []byte
		wantRule  []*RequiredReviewer
		wantError bool
	}{
		"User Reviewer": {
			data:      []byte(`[{"type": "User", "reviewer": {"id": 1,"login": "octocat"}}]`),
			wantRule:  []*RequiredReviewer{{Type: Ptr("User"), Reviewer: &User{ID: Ptr(int64(1)), Login: Ptr("octocat")}}},
			wantError: false,
		},
		"Team Reviewer": {
			data:      []byte(`[{"type": "Team", "reviewer": {"id": 1, "name": "Justice League"}}]`),
			wantRule:  []*RequiredReviewer{{Type: Ptr("Team"), Reviewer: &Team{ID: Ptr(int64(1)), Name: Ptr("Justice League")}}},
			wantError: false,
		},
		"Both Types Reviewer": {
			data:      []byte(`[{"type": "User", "reviewer": {"id": 1,"login": "octocat"}},{"type": "Team", "reviewer": {"id": 1, "name": "Justice League"}}]`),
			wantRule:  []*RequiredReviewer{{Type: Ptr("User"), Reviewer: &User{ID: Ptr(int64(1)), Login: Ptr("octocat")}}, {Type: Ptr("Team"), Reviewer: &Team{ID: Ptr(int64(1)), Name: Ptr("Justice League")}}},
			wantError: false,
		},
		"Empty JSON Object": {
			data:      []byte(`[]`),
			wantRule:  []*RequiredReviewer{},
			wantError: false,
		},
		"Bad JSON Object": {
			data:      []byte(`[badjson: 1]`),
			wantRule:  []*RequiredReviewer{},
			wantError: true,
		},
		"Wrong Type in Reviewer Object": {
			data:      []byte(`[{"type": 1, "reviewer": {"id": 1}}]`),
			wantRule:  []*RequiredReviewer{{Type: nil, Reviewer: nil}},
			wantError: true,
		},
		"Missing Type in Reviewer Object": {
			data:      []byte(`[{"reviewer": {"id": 1}}]`),
			wantRule:  []*RequiredReviewer{{Type: nil, Reviewer: nil}},
			wantError: true,
		},
		"Null Type in Reviewer Object": {
			data:      []byte(`[{"type": null, "reviewer": {"id": 1}}]`),
			wantRule:  []*RequiredReviewer{{Type: nil, Reviewer: nil}},
			wantError: true,
		},
		"Wrong ID Type in User Object": {
			data:      []byte(`[{"type": "User", "reviewer": {"id": "string"}}]`),
			wantRule:  []*RequiredReviewer{{Type: Ptr("User"), Reviewer: nil}},
			wantError: true,
		},
		"Wrong ID Type in Team Object": {
			data:      []byte(`[{"type": "Team", "reviewer": {"id": "string"}}]`),
			wantRule:  []*RequiredReviewer{{Type: Ptr("Team"), Reviewer: nil}},
			wantError: true,
		},
		"Wrong Type of Reviewer": {
			data:      []byte(`[{"type": "Cat", "reviewer": {"id": 1,"login": "octocat"}}]`),
			wantRule:  []*RequiredReviewer{{Type: nil, Reviewer: nil}},
			wantError: true,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rule := []*RequiredReviewer{}
			err := json.Unmarshal(test.data, &rule)
			if err != nil && !test.wantError {
				t.Error("RequiredReviewer.UnmarshalJSON returned an error when we expected nil")
			}
			if err == nil && test.wantError {
				t.Error("RequiredReviewer.UnmarshalJSON returned no error when we expected one")
			}
			if !cmp.Equal(test.wantRule, rule) {
				t.Errorf("RequiredReviewer.UnmarshalJSON expected rule %+v, got %+v", test.wantRule, rule)
			}
		})
	}
}

func TestCreateUpdateEnvironment_MarshalJSON(t *testing.T) {
	t.Parallel()
	cu := CreateUpdateEnvironment{}

	want := `{"wait_timer":0,"reviewers":null,"can_admins_bypass":true,"deployment_branch_policy":null}`
	testJSONMarshalOnly(t, cu, want)
	testJSONMarshalOnly(t, &cu, want)
}

func TestRepositoriesService_ListEnvironments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"total_count":1, "environments":[{"id":1}, {"id": 2}]}`)
	})

	opt := &EnvironmentListOptions{
		ListOptions: ListOptions{
			Page:    2,
			PerPage: 2,
		},
	}
	ctx := t.Context()
	environments, _, err := client.Repositories.ListEnvironments(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListEnvironments returned error: %v", err)
	}
	want := &EnvResponse{TotalCount: Ptr(1), Environments: []*Environment{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}}
	if !cmp.Equal(environments, want) {
		t.Errorf("Repositories.ListEnvironments returned %+v, want %+v", environments, want)
	}

	const methodName = "ListEnvironments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListEnvironments(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListEnvironments(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetEnvironment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id": 1,"name": "staging", "deployment_branch_policy": {"protected_branches": true,	"custom_branch_policies": false}, "can_admins_bypass": false}`)
	})

	ctx := t.Context()
	release, resp, err := client.Repositories.GetEnvironment(ctx, "o", "r", "e")
	if err != nil {
		t.Errorf("Repositories.GetEnvironment returned error: %v\n%v", err, resp.Body)
	}

	want := &Environment{ID: Ptr(int64(1)), Name: Ptr("staging"), DeploymentBranchPolicy: &BranchPolicy{ProtectedBranches: Ptr(true), CustomBranchPolicies: Ptr(false)}, CanAdminsBypass: Ptr(false)}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.GetEnvironment returned %+v, want %+v", release, want)
	}

	const methodName = "GetEnvironment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetEnvironment(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetEnvironment(ctx, "o", "r", "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateEnvironment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateUpdateEnvironment{
		WaitTimer: Ptr(30),
	}

	mux.HandleFunc("/repos/o/r/environments/e", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		want := &CreateUpdateEnvironment{WaitTimer: Ptr(30), CanAdminsBypass: Ptr(true)}
		testJSONBody(t, r, want)
		fmt.Fprint(w, `{"id": 1, "name": "staging",	"protection_rules": [{"id": 1, "type": "wait_timer", "wait_timer": 30}]}`)
	})

	ctx := t.Context()
	release, _, err := client.Repositories.CreateUpdateEnvironment(ctx, "o", "r", "e", input)
	if err != nil {
		t.Errorf("Repositories.CreateUpdateEnvironment returned error: %v", err)
	}

	want := &Environment{ID: Ptr(int64(1)), Name: Ptr("staging"), ProtectionRules: []*ProtectionRule{{ID: Ptr(int64(1)), Type: Ptr("wait_timer"), WaitTimer: Ptr(30)}}}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.CreateUpdateEnvironment returned %+v, want %+v", release, want)
	}

	const methodName = "CreateUpdateEnvironment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.CreateUpdateEnvironment(ctx, "\n", "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateUpdateEnvironment(ctx, "o", "r", "e", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateEnvironment_noEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateUpdateEnvironment{}
	callCount := 0

	mux.HandleFunc("/repos/o/r/environments/e", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		if callCount == 0 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			callCount++
		} else {
			want := &CreateUpdateEnvironment{}
			testJSONBody(t, r, want)
			fmt.Fprint(w, `{"id": 1, "name": "staging",	"protection_rules": []}`)
		}
	})

	ctx := t.Context()
	release, _, err := client.Repositories.CreateUpdateEnvironment(ctx, "o", "r", "e", input)
	if err != nil {
		t.Errorf("Repositories.CreateUpdateEnvironment returned error: %v", err)
	}

	want := &Environment{ID: Ptr(int64(1)), Name: Ptr("staging"), ProtectionRules: []*ProtectionRule{}}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.CreateUpdateEnvironment returned %+v, want %+v", release, want)
	}
}

func TestRepositoriesService_createNewEnvNoEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateUpdateEnvironment{
		DeploymentBranchPolicy: &BranchPolicy{
			ProtectedBranches:    Ptr(true),
			CustomBranchPolicies: Ptr(false),
		},
	}

	mux.HandleFunc("/repos/o/r/environments/e", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id": 1, "name": "staging",	"protection_rules": [{"id": 1, "node_id": "id", "type": "branch_policy"}], "deployment_branch_policy": {"protected_branches": true, "custom_branch_policies": false}}`)
	})

	ctx := t.Context()
	release, _, err := client.Repositories.createNewEnvNoEnterprise(ctx, "repos/o/r/environments/e", input)
	if err != nil {
		t.Errorf("Repositories.createNewEnvNoEnterprise returned error: %v", err)
	}

	want := &Environment{
		ID:   Ptr(int64(1)),
		Name: Ptr("staging"),
		ProtectionRules: []*ProtectionRule{
			{
				ID:     Ptr(int64(1)),
				NodeID: Ptr("id"),
				Type:   Ptr("branch_policy"),
			},
		},
		DeploymentBranchPolicy: &BranchPolicy{
			ProtectedBranches:    Ptr(true),
			CustomBranchPolicies: Ptr(false),
		},
	}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.createNewEnvNoEnterprise returned %+v, want %+v", release, want)
	}

	const methodName = "createNewEnvNoEnterprise"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.createNewEnvNoEnterprise(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.createNewEnvNoEnterprise(ctx, "repos/o/r/environments/e", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DeleteEnvironment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Repositories.DeleteEnvironment(ctx, "o", "r", "e")
	if err != nil {
		t.Errorf("Repositories.DeleteEnvironment returned error: %v", err)
	}

	const methodName = "DeleteEnvironment"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DeleteEnvironment(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteEnvironment(ctx, "o", "r", "e")
	})
}
