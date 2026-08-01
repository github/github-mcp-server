// Copyright 2018 The go-github AUTHORS. All rights reserved.
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

func TestInteractionsService_GetRestrictionsForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/interaction-limits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeInteractionRestrictionsPreview)
		fmt.Fprint(w, `{"origin":"repository"}`)
	})

	ctx := t.Context()
	repoInteractions, _, err := client.Interactions.GetRestrictionsForRepo(ctx, "o", "r")
	if err != nil {
		t.Errorf("Interactions.GetRestrictionsForRepo returned error: %v", err)
	}

	want := &InteractionRestriction{Origin: Ptr("repository")}
	if !cmp.Equal(repoInteractions, want) {
		t.Errorf("Interactions.GetRestrictionsForRepo returned %+v, want %+v", repoInteractions, want)
	}

	const methodName = "GetRestrictionsForRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Interactions.GetRestrictionsForRepo(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Interactions.GetRestrictionsForRepo(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestInteractionsService_UpdateRestrictionsForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &InteractionRestriction{Limit: Ptr("existing_users")}

	mux.HandleFunc("/repos/o/r/interaction-limits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeInteractionRestrictionsPreview)
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"origin":"repository"}`)
	})

	ctx := t.Context()
	repoInteractions, _, err := client.Interactions.UpdateRestrictionsForRepo(ctx, "o", "r", input.GetLimit())
	if err != nil {
		t.Errorf("Interactions.UpdateRestrictionsForRepo returned error: %v", err)
	}

	want := &InteractionRestriction{Origin: Ptr("repository")}
	if !cmp.Equal(repoInteractions, want) {
		t.Errorf("Interactions.UpdateRestrictionsForRepo returned %+v, want %+v", repoInteractions, want)
	}

	const methodName = "UpdateRestrictionsForRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Interactions.UpdateRestrictionsForRepo(ctx, "\n", "\n", input.GetLimit())
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Interactions.UpdateRestrictionsForRepo(ctx, "o", "r", input.GetLimit())
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestInteractionsService_RemoveRestrictionsFromRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/interaction-limits", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeInteractionRestrictionsPreview)
	})

	ctx := t.Context()
	_, err := client.Interactions.RemoveRestrictionsFromRepo(ctx, "o", "r")
	if err != nil {
		t.Errorf("Interactions.RemoveRestrictionsFromRepo returned error: %v", err)
	}

	const methodName = "RemoveRestrictionsFromRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Interactions.RemoveRestrictionsFromRepo(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Interactions.RemoveRestrictionsFromRepo(ctx, "o", "r")
	})
}
