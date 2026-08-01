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

func TestRepositoriesService_GetActionsAccessLevel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/permissions/access", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"access_level": "none"}`)
	})

	ctx := t.Context()
	org, _, err := client.Repositories.GetActionsAccessLevel(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetActionsAccessLevel returned error: %v", err)
	}
	want := &RepositoryActionsAccessLevel{AccessLevel: Ptr("none")}
	if !cmp.Equal(org, want) {
		t.Errorf("Repositories.GetActionsAccessLevel returned %+v, want %+v", org, want)
	}

	const methodName = "GetActionsAccessLevel"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetActionsAccessLevel(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetActionsAccessLevel(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_EditActionsAccessLevel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &RepositoryActionsAccessLevel{AccessLevel: Ptr("organization")}

	mux.HandleFunc("/repos/o/r/actions/permissions/access", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
	})

	ctx := t.Context()
	_, err := client.Repositories.EditActionsAccessLevel(ctx, "o", "r", *input)
	if err != nil {
		t.Errorf("Repositories.EditActionsAccessLevel returned error: %v", err)
	}

	const methodName = "EditActionsAccessLevel"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.EditActionsAccessLevel(ctx, "\n", "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		resp, err := client.Repositories.EditActionsAccessLevel(ctx, "o", "r", *input)
		return resp, err
	})
}
