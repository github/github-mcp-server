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

func TestCodespacesService_ListInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o1/codespaces", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"total_count":1,"codespaces":[{"id":1}]}`)
	})

	opts := &ListOptions{Page: 1, PerPage: 10}
	ctx := t.Context()
	got, _, err := client.Codespaces.ListInOrg(ctx, "o1", opts)
	if err != nil {
		t.Fatalf("Codespaces.ListInOrg returned error: %v", err)
	}

	want := &ListCodespaces{
		TotalCount: Ptr(1),
		Codespaces: []*Codespace{
			{ID: Ptr(int64(1))},
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Codespaces.ListInOrg = %+v, want %+v", got, want)
	}
	const methodName = "ListInOrg"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Codespaces.ListInOrg(ctx, "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.ListInOrg(ctx, "o1", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_SetOrgAccessControl(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := CodespacesOrgAccessControlRequest{
		Visibility:        "selected_members",
		SelectedUsernames: []string{"u1", "u2"},
	}

	mux.HandleFunc("/orgs/o1/codespaces/access", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, req)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Codespaces.SetOrgAccessControl(ctx, "o1", req)
	if err != nil {
		t.Fatalf("Codespaces.SetOrgAccessControl returned error: %v", err)
	}

	const methodName = "SetOrgAccessControl"
	testBadOptions(t, methodName, func() error {
		_, err := client.Codespaces.SetOrgAccessControl(ctx, "\n", req)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Codespaces.SetOrgAccessControl(ctx, "o1", req)
	})
}

func TestEnterpriseService_AddUsersToOrgAccess(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := []string{"u1"}

	mux.HandleFunc("/orgs/o1/codespaces/access/selected_users", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, struct {
			SelectedUsernames []string `json:"selected_usernames"`
		}{
			SelectedUsernames: req,
		})
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Codespaces.AddUsersToOrgAccess(ctx, "o1", req)
	if err != nil {
		t.Fatalf("AddUsersToOrgAccess returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("AddUsersToOrgAccess returned nil Response")
	}

	const methodName = "AddUsersToOrgAccess"
	testBadOptions(t, methodName, func() error {
		_, err := client.Codespaces.AddUsersToOrgAccess(ctx, "\n", req)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Codespaces.AddUsersToOrgAccess(ctx, "o1", req)
	})
}

func TestEnterpriseService_RemoveUsersFromOrgAccess(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	req := []string{"u1"}

	mux.HandleFunc("/orgs/o1/codespaces/access/selected_users", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, struct {
			SelectedUsernames []string `json:"selected_usernames"`
		}{
			SelectedUsernames: req,
		})
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Codespaces.RemoveUsersFromOrgAccess(ctx, "o1", req)
	if err != nil {
		t.Fatalf("RemoveUsersFromOrgAccess returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("RemoveUsersFromOrgAccess returned nil Response")
	}

	const methodName = "RemoveUsersFromOrgAccess"
	testBadOptions(t, methodName, func() error {
		_, err := client.Codespaces.RemoveUsersFromOrgAccess(ctx, "\n", req)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Codespaces.RemoveUsersFromOrgAccess(ctx, "o1", req)
	})
}

func TestCodespacesService_ListUserCodespacesInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o1/members/u1/codespaces", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"total_count":1,"codespaces":[{"id":1}]}`)
	})

	opts := &ListOptions{Page: 1, PerPage: 10}
	ctx := t.Context()
	got, _, err := client.Codespaces.ListUserCodespacesInOrg(ctx, "o1", "u1", opts)
	if err != nil {
		t.Fatalf("Codespaces.ListUserCodespacesInOrg returned error: %v", err)
	}

	want := &ListCodespaces{
		TotalCount: Ptr(1),
		Codespaces: []*Codespace{
			{ID: Ptr(int64(1))},
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Codespaces.ListUserCodespacesInOrg = %+v, want %+v", got, want)
	}
	const methodName = "ListUserCodespacesInOrg"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Codespaces.ListUserCodespacesInOrg(ctx, "\n", "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.ListUserCodespacesInOrg(ctx, "o1", "u1", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteUserCodespaceInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o1/members/u1/codespaces/c1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Codespaces.DeleteUserCodespaceInOrg(ctx, "o1", "u1", "c1")
	if err != nil {
		t.Fatalf("DeleteUserCodespaceInOrg returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("DeleteUserCodespaceInOrg returned nil Response")
	}

	const methodName = "DeleteUserCodespaceInOrg"
	testBadOptions(t, methodName, func() error {
		_, err := client.Codespaces.DeleteUserCodespaceInOrg(ctx, "\n", "u1", "c1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Codespaces.DeleteUserCodespaceInOrg(ctx, "o1", "u1", "c1")
	})
}

func TestEnterpriseService_StopUserCodespaceInOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o1/members/u1/codespaces/c1/stop", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Codespaces.StopUserCodespaceInOrg(ctx, "o1", "u1", "c1")
	if err != nil {
		t.Fatalf("StopUserCodespaceInOrg returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("StopUserCodespaceInOrg returned nil Response")
	}

	const methodName = "StopUserCodespaceInOrg"
	testBadOptions(t, methodName, func() error {
		_, err := client.Codespaces.StopUserCodespaceInOrg(ctx, "\n", "u1", "c1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Codespaces.StopUserCodespaceInOrg(ctx, "o1", "u1", "c1")
	})
}
