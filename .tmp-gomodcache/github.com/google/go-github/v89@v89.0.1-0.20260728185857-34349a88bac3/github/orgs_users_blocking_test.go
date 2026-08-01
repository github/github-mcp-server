// Copyright 2017 The go-github AUTHORS. All rights reserved.
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

func TestOrganizationsService_ListBlockedUsers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/blocks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{
			"login": "octocat"
		}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	blockedUsers, _, err := client.Organizations.ListBlockedUsers(ctx, "o", opt)
	if err != nil {
		t.Errorf("Organizations.ListBlockedUsers returned error: %v", err)
	}

	want := []*User{{Login: Ptr("octocat")}}
	if !cmp.Equal(blockedUsers, want) {
		t.Errorf("Organizations.ListBlockedUsers returned %+v, want %+v", blockedUsers, want)
	}

	const methodName = "ListBlockedUsers"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListBlockedUsers(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListBlockedUsers(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_IsBlocked(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/blocks/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	isBlocked, _, err := client.Organizations.IsBlocked(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.IsBlocked returned error: %v", err)
	}
	if want := true; isBlocked != want {
		t.Errorf("Organizations.IsBlocked returned %+v, want %+v", isBlocked, want)
	}

	const methodName = "IsBlocked"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.IsBlocked(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.IsBlocked(ctx, "o", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_BlockUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/blocks/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Organizations.BlockUser(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.BlockUser returned error: %v", err)
	}

	const methodName = "BlockUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.BlockUser(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.BlockUser(ctx, "o", "u")
	})
}

func TestOrganizationsService_UnblockUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/blocks/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Organizations.UnblockUser(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.UnblockUser returned error: %v", err)
	}

	const methodName = "UnblockUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.UnblockUser(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.UnblockUser(ctx, "o", "u")
	})
}
