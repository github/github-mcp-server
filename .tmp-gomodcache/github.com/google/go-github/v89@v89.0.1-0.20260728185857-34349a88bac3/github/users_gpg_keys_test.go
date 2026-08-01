// Copyright 2016 The go-github AUTHORS. All rights reserved.
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

func TestUsersService_ListGPGKeys_authenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/gpg_keys", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1,"primary_key_id":2}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	keys, _, err := client.Users.ListGPGKeys(ctx, "", opt)
	if err != nil {
		t.Errorf("Users.ListGPGKeys returned error: %v", err)
	}

	want := []*GPGKey{{ID: Ptr(int64(1)), PrimaryKeyID: Ptr(int64(2))}}
	if !cmp.Equal(keys, want) {
		t.Errorf("Users.ListGPGKeys = %+v, want %+v", keys, want)
	}

	const methodName = "ListGPGKeys"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListGPGKeys(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListGPGKeys(ctx, "", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_ListGPGKeys_specifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/gpg_keys", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1,"primary_key_id":2}]`)
	})

	ctx := t.Context()
	keys, _, err := client.Users.ListGPGKeys(ctx, "u", nil)
	if err != nil {
		t.Errorf("Users.ListGPGKeys returned error: %v", err)
	}

	want := []*GPGKey{{ID: Ptr(int64(1)), PrimaryKeyID: Ptr(int64(2))}}
	if !cmp.Equal(keys, want) {
		t.Errorf("Users.ListGPGKeys = %+v, want %+v", keys, want)
	}
}

func TestUsersService_ListGPGKeys_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Users.ListGPGKeys(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestUsersService_GetGPGKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/gpg_keys/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	key, _, err := client.Users.GetGPGKey(ctx, 1)
	if err != nil {
		t.Errorf("Users.GetGPGKey returned error: %v", err)
	}

	want := &GPGKey{ID: Ptr(int64(1))}
	if !cmp.Equal(key, want) {
		t.Errorf("Users.GetGPGKey = %+v, want %+v", key, want)
	}

	const methodName = "GetGPGKey"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.GetGPGKey(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.GetGPGKey(ctx, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_CreateGPGKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := `
-----BEGIN PGP PUBLIC KEY BLOCK-----
Comment: GPGTools - https://gpgtools.org

mQINBFcEd9kBEACo54TDbGhKlXKWMvJgecEUKPPcv7XdnpKdGb3LRw5MvFwT0V0f
...
=tqfb
-----END PGP PUBLIC KEY BLOCK-----`

	mux.HandleFunc("/user/gpg_keys", func(w http.ResponseWriter, r *http.Request) {
		gpgKey := struct {
			ArmoredPublicKey *string `json:"armored_public_key,omitempty"`
		}{
			ArmoredPublicKey: &input,
		}
		testMethod(t, r, "POST")
		testJSONBody(t, r, gpgKey)

		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	gpgKey, _, err := client.Users.CreateGPGKey(ctx, input)
	if err != nil {
		t.Errorf("Users.CreateGPGKey returned error: %v", err)
	}

	want := &GPGKey{ID: Ptr(int64(1))}
	if !cmp.Equal(gpgKey, want) {
		t.Errorf("Users.GetGPGKey = %+v, want %+v", gpgKey, want)
	}

	const methodName = "CreateGPGKey"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.CreateGPGKey(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_DeleteGPGKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/gpg_keys/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Users.DeleteGPGKey(ctx, 1)
	if err != nil {
		t.Errorf("Users.DeleteGPGKey returned error: %v", err)
	}

	const methodName = "DeleteGPGKey"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.DeleteGPGKey(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.DeleteGPGKey(ctx, 1)
	})
}
