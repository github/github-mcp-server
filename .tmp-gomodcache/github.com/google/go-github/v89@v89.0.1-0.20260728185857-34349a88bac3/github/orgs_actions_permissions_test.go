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

func TestOrganizationsService_GetActionsPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"enabled_repositories": "all", "allowed_actions": "all"}`)
	})

	ctx := t.Context()
	org, _, err := client.Organizations.GetActionsPermissions(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.GetActionsPermissions returned error: %v", err)
	}
	want := &ActionsPermissions{EnabledRepositories: Ptr("all"), AllowedActions: Ptr("all")}
	if !cmp.Equal(org, want) {
		t.Errorf("Organizations.GetActionsPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "GetActionsPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetActionsPermissions(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetActionsPermissions(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_UpdateActionsPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsPermissions{EnabledRepositories: Ptr("all"), AllowedActions: Ptr("selected")}

	mux.HandleFunc("/orgs/o/actions/permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"enabled_repositories": "all", "allowed_actions": "selected"}`)
	})

	ctx := t.Context()
	org, _, err := client.Organizations.UpdateActionsPermissions(ctx, "o", *input)
	if err != nil {
		t.Errorf("Organizations.UpdateActionsPermissions returned error: %v", err)
	}

	want := &ActionsPermissions{EnabledRepositories: Ptr("all"), AllowedActions: Ptr("selected")}
	if !cmp.Equal(org, want) {
		t.Errorf("Organizations.UpdateActionsPermissions returned %+v, want %+v", org, want)
	}

	const methodName = "UpdateActionsPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.UpdateActionsPermissions(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.UpdateActionsPermissions(ctx, "o", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
