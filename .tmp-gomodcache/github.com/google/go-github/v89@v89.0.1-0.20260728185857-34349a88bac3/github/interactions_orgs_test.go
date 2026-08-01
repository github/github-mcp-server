// Copyright 2019 The go-github AUTHORS. All rights reserved.
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

func TestInteractionsService_GetRestrictionsForOrgs(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/interaction-limits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeInteractionRestrictionsPreview)
		fmt.Fprint(w, `{"origin":"organization"}`)
	})

	ctx := t.Context()
	organizationInteractions, _, err := client.Interactions.GetRestrictionsForOrg(ctx, "o")
	if err != nil {
		t.Errorf("Interactions.GetRestrictionsForOrg returned error: %v", err)
	}

	want := &InteractionRestriction{Origin: Ptr("organization")}
	if !cmp.Equal(organizationInteractions, want) {
		t.Errorf("Interactions.GetRestrictionsForOrg returned %+v, want %+v", organizationInteractions, want)
	}

	const methodName = "GetRestrictionsForOrg"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Interactions.GetRestrictionsForOrg(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Interactions.GetRestrictionsForOrg(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestInteractionsService_UpdateRestrictionsForOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &InteractionRestriction{Limit: Ptr("existing_users")}

	mux.HandleFunc("/orgs/o/interaction-limits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeInteractionRestrictionsPreview)
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"origin":"organization"}`)
	})

	ctx := t.Context()
	organizationInteractions, _, err := client.Interactions.UpdateRestrictionsForOrg(ctx, "o", input.GetLimit())
	if err != nil {
		t.Errorf("Interactions.UpdateRestrictionsForOrg returned error: %v", err)
	}

	want := &InteractionRestriction{Origin: Ptr("organization")}
	if !cmp.Equal(organizationInteractions, want) {
		t.Errorf("Interactions.UpdateRestrictionsForOrg returned %+v, want %+v", organizationInteractions, want)
	}

	const methodName = "UpdateRestrictionsForOrg"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Interactions.UpdateRestrictionsForOrg(ctx, "\n", input.GetLimit())
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Interactions.UpdateRestrictionsForOrg(ctx, "o", input.GetLimit())
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestInteractionsService_RemoveRestrictionsFromOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/interaction-limits", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeInteractionRestrictionsPreview)
	})

	ctx := t.Context()
	_, err := client.Interactions.RemoveRestrictionsFromOrg(ctx, "o")
	if err != nil {
		t.Errorf("Interactions.RemoveRestrictionsFromOrg returned error: %v", err)
	}

	const methodName = "RemoveRestrictionsFromOrg"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Interactions.RemoveRestrictionsFromOrg(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Interactions.RemoveRestrictionsFromOrg(ctx, "o")
	})
}
