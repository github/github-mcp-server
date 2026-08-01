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

func TestOrganizationsService_GetActionsAllowed(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	org, _, err := client.Organizations.GetActionsAllowed(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.GetActionsAllowed returned error: %v", err)
	}
	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(org, want) {
		t.Errorf("Organizations.GetActionsAllowed returned %+v, want %+v", org, want)
	}

	const methodName = "GetActionsAllowed"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetActionsAllowed(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetActionsAllowed(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_UpdateActionsAllowed(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}

	mux.HandleFunc("/orgs/o/actions/permissions/selected-actions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"github_owned_allowed":true, "verified_allowed":false, "patterns_allowed":["a/b"]}`)
	})

	ctx := t.Context()
	org, _, err := client.Organizations.UpdateActionsAllowed(ctx, "o", *input)
	if err != nil {
		t.Errorf("Organizations.UpdateActionsAllowed returned error: %v", err)
	}

	want := &ActionsAllowed{GithubOwnedAllowed: Ptr(true), VerifiedAllowed: Ptr(false), PatternsAllowed: []string{"a/b"}}
	if !cmp.Equal(org, want) {
		t.Errorf("Organizations.UpdateActionsAllowed returned %+v, want %+v", org, want)
	}

	const methodName = "UpdateActionsAllowed"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.UpdateActionsAllowed(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.UpdateActionsAllowed(ctx, "o", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
