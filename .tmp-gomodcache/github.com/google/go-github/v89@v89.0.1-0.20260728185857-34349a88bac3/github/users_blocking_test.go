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

func TestUsersService_ListBlockedUsers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/blocks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{
			"login": "octocat"
		}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	blockedUsers, _, err := client.Users.ListBlockedUsers(ctx, opt)
	if err != nil {
		t.Errorf("Users.ListBlockedUsers returned error: %v", err)
	}

	want := []*User{{Login: Ptr("octocat")}}
	if !cmp.Equal(blockedUsers, want) {
		t.Errorf("Users.ListBlockedUsers returned %+v, want %+v", blockedUsers, want)
	}

	const methodName = "ListBlockedUsers"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListBlockedUsers(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_IsBlocked(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/blocks/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	isBlocked, _, err := client.Users.IsBlocked(ctx, "u")
	if err != nil {
		t.Errorf("Users.IsBlocked returned error: %v", err)
	}
	if want := true; isBlocked != want {
		t.Errorf("Users.IsBlocked returned %+v, want %+v", isBlocked, want)
	}

	const methodName = "IsBlocked"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.IsBlocked(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.IsBlocked(ctx, "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_BlockUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/blocks/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.BlockUser(ctx, "u")
	if err != nil {
		t.Errorf("Users.BlockUser returned error: %v", err)
	}

	const methodName = "BlockUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.BlockUser(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.BlockUser(ctx, "u")
	})
}

func TestUsersService_UnblockUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/blocks/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeBlockUsersPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.UnblockUser(ctx, "u")
	if err != nil {
		t.Errorf("Users.UnblockUser returned error: %v", err)
	}

	const methodName = "UnblockUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.UnblockUser(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.UnblockUser(ctx, "u")
	})
}
