// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"net/http"
	"testing"
)

func TestUsersService_PromoteSiteAdmin(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/site_admin", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.PromoteSiteAdmin(ctx, "u")
	if err != nil {
		t.Errorf("Users.PromoteSiteAdmin returned error: %v", err)
	}

	const methodName = "PromoteSiteAdmin"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.PromoteSiteAdmin(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.PromoteSiteAdmin(ctx, "u")
	})
}

func TestUsersService_DemoteSiteAdmin(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/site_admin", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.DemoteSiteAdmin(ctx, "u")
	if err != nil {
		t.Errorf("Users.DemoteSiteAdmin returned error: %v", err)
	}

	const methodName = "DemoteSiteAdmin"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.DemoteSiteAdmin(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.DemoteSiteAdmin(ctx, "u")
	})
}

func TestUsersService_Suspend(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/suspended", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.Suspend(ctx, "u", nil)
	if err != nil {
		t.Errorf("Users.Suspend returned error: %v", err)
	}

	const methodName = "Suspend"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.Suspend(ctx, "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.Suspend(ctx, "u", nil)
	})
}

func TestUsersServiceReason_Suspend(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &UserSuspendOptions{Reason: Ptr("test")}

	mux.HandleFunc("/users/u/suspended", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.Suspend(ctx, "u", input)
	if err != nil {
		t.Errorf("Users.Suspend returned error: %v", err)
	}
}

func TestUsersService_Unsuspend(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/suspended", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Users.Unsuspend(ctx, "u")
	if err != nil {
		t.Errorf("Users.Unsuspend returned error: %v", err)
	}

	const methodName = "Unsuspend"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.Unsuspend(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.Unsuspend(ctx, "u")
	})
}
