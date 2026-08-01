// Copyright 2013 The go-github AUTHORS. All rights reserved.
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

func TestUsersService_ListEmails(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{
			"email": "user@example.com",
			"verified": false,
			"primary": true
		}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	emails, _, err := client.Users.ListEmails(ctx, opt)
	if err != nil {
		t.Errorf("Users.ListEmails returned error: %v", err)
	}

	want := []*UserEmail{{Email: Ptr("user@example.com"), Verified: Ptr(false), Primary: Ptr(true)}}
	if !cmp.Equal(emails, want) {
		t.Errorf("Users.ListEmails returned %+v, want %+v", emails, want)
	}

	const methodName = "ListEmails"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListEmails(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_AddEmails(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []string{"new@example.com"}

	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{"email":"old@example.com"}, {"email":"new@example.com"}]`)
	})

	ctx := t.Context()
	emails, _, err := client.Users.AddEmails(ctx, input)
	if err != nil {
		t.Errorf("Users.AddEmails returned error: %v", err)
	}

	want := []*UserEmail{
		{Email: Ptr("old@example.com")},
		{Email: Ptr("new@example.com")},
	}
	if !cmp.Equal(emails, want) {
		t.Errorf("Users.AddEmails returned %+v, want %+v", emails, want)
	}

	const methodName = "AddEmails"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.AddEmails(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_DeleteEmails(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []string{"user@example.com"}

	mux.HandleFunc("/user/emails", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, input)
	})

	ctx := t.Context()
	_, err := client.Users.DeleteEmails(ctx, input)
	if err != nil {
		t.Errorf("Users.DeleteEmails returned error: %v", err)
	}

	const methodName = "DeleteEmails"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.DeleteEmails(ctx, input)
	})
}

func TestUsersService_SetEmailVisibility(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &UserEmail{Visibility: Ptr("private")}

	mux.HandleFunc("/user/email/visibility", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{
			"email": "user@example.com",
			"verified": false,
			"primary": true,
			"visibility": "private"
		}]`)
	})

	ctx := t.Context()
	emails, _, err := client.Users.SetEmailVisibility(ctx, "private")
	if err != nil {
		t.Errorf("Users.SetEmailVisibility returned error: %v", err)
	}

	want := []*UserEmail{{Email: Ptr("user@example.com"), Verified: Ptr(false), Primary: Ptr(true), Visibility: Ptr("private")}}
	if !cmp.Equal(emails, want) {
		t.Errorf("Users.SetEmailVisibility returned %+v, want %+v", emails, want)
	}

	const methodName = "SetEmailVisibility"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.SetEmailVisibility(ctx, "private")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
