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

func TestUsersService_ListSocialAccounts(t *testing.T) {
	t.Parallel()

	client, mux, _ := setup(t)

	mux.HandleFunc("/user/social_accounts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{
			"provider": "example",
			"url": "https://example.com"
		}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	accounts, _, err := client.Users.ListSocialAccounts(ctx, opt)
	if err != nil {
		t.Errorf("Users.ListSocialAccounts returned error: %v", err)
	}

	want := []*SocialAccount{{Provider: Ptr("example"), URL: Ptr("https://example.com")}}
	if !cmp.Equal(accounts, want) {
		t.Errorf("Users.ListSocialAccounts returned %#v, want %#v", accounts, want)
	}

	const methodName = "ListSocialAccounts"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListSocialAccounts(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_AddSocialAccounts(t *testing.T) {
	t.Parallel()

	client, mux, _ := setup(t)

	input := socialAccountsRequest{
		AccountURLs: []string{"https://example.com"},
	}

	mux.HandleFunc("/user/social_accounts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{"provider":"example","url":"https://example.com"}]`)
	})

	ctx := t.Context()
	accounts, _, err := client.Users.AddSocialAccounts(ctx, input.AccountURLs)
	if err != nil {
		t.Errorf("Users.AddSocialAccounts returned error: %v", err)
	}

	want := []*SocialAccount{
		{Provider: Ptr("example"), URL: Ptr("https://example.com")},
	}
	if !cmp.Equal(accounts, want) {
		t.Errorf("Users.AddSocialAccounts returned %#v, want %#v", accounts, want)
	}

	const methodName = "AddSocialAccounts"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.AddSocialAccounts(ctx, input.AccountURLs)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_DeleteSocialAccounts(t *testing.T) {
	t.Parallel()

	client, mux, _ := setup(t)

	input := socialAccountsRequest{
		AccountURLs: []string{"https://example.com"},
	}

	mux.HandleFunc("/user/social_accounts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.DeleteSocialAccounts(ctx, input.AccountURLs)
	if err != nil {
		t.Errorf("Users.DeleteSocialAccounts returned error: %v", err)
	}

	const methodName = "DeleteSocialAccounts"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.DeleteSocialAccounts(ctx, input.AccountURLs)
	})
}

func TestUsersService_ListUserSocialAccounts(t *testing.T) {
	t.Parallel()

	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/social_accounts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{
			"provider": "example",
			"url": "https://example.com"
		}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	accounts, _, err := client.Users.ListUserSocialAccounts(ctx, "u", opt)
	if err != nil {
		t.Errorf("Users.ListUserSocialAccounts returned error: %v", err)
	}

	want := []*SocialAccount{{Provider: Ptr("example"), URL: Ptr("https://example.com")}}
	if !cmp.Equal(accounts, want) {
		t.Errorf("Users.ListUserSocialAccounts returned %#v, want %#v", accounts, want)
	}

	const methodName = "ListUserSocialAccounts"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListUserSocialAccounts(ctx, "u", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
