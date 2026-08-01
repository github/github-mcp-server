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

func TestUsersService_ListKeys_authenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/keys", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	keys, _, err := client.Users.ListKeys(ctx, "", opt)
	if err != nil {
		t.Errorf("Users.ListKeys returned error: %v", err)
	}

	want := []*Key{{ID: Ptr(int64(1))}}
	if !cmp.Equal(keys, want) {
		t.Errorf("Users.ListKeys returned %+v, want %+v", keys, want)
	}

	const methodName = "ListKeys"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListKeys(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListKeys(ctx, "", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_ListKeys_specifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/keys", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	keys, _, err := client.Users.ListKeys(ctx, "u", nil)
	if err != nil {
		t.Errorf("Users.ListKeys returned error: %v", err)
	}

	want := []*Key{{ID: Ptr(int64(1))}}
	if !cmp.Equal(keys, want) {
		t.Errorf("Users.ListKeys returned %+v, want %+v", keys, want)
	}
}

func TestUsersService_ListKeys_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Users.ListKeys(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestUsersService_GetKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/keys/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	key, _, err := client.Users.GetKey(ctx, 1)
	if err != nil {
		t.Errorf("Users.GetKey returned error: %v", err)
	}

	want := &Key{ID: Ptr(int64(1))}
	if !cmp.Equal(key, want) {
		t.Errorf("Users.GetKey returned %+v, want %+v", key, want)
	}

	const methodName = "GetKey"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.GetKey(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.GetKey(ctx, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_CreateKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Key{Key: Ptr("k"), Title: Ptr("t")}

	mux.HandleFunc("/user/keys", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	key, _, err := client.Users.CreateKey(ctx, input)
	if err != nil {
		t.Errorf("Users.CreateKey returned error: %v", err)
	}

	want := &Key{ID: Ptr(int64(1))}
	if !cmp.Equal(key, want) {
		t.Errorf("Users.CreateKey returned %+v, want %+v", key, want)
	}

	const methodName = "CreateKey"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.CreateKey(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_DeleteKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/keys/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Users.DeleteKey(ctx, 1)
	if err != nil {
		t.Errorf("Users.DeleteKey returned error: %v", err)
	}

	const methodName = "DeleteKey"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.DeleteKey(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.DeleteKey(ctx, 1)
	})
}
