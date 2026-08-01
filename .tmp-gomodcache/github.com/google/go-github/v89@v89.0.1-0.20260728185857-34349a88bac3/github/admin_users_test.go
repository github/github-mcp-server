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

func TestAdminUsers_Create(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateUserRequest{Login: "github", Email: Ptr("email@example.com"), Suspended: Ptr(false)}

	mux.HandleFunc("/admin/users", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"login":"github","id":1}`)
	})

	ctx := t.Context()
	org, _, err := client.Admin.CreateUser(ctx, input)
	if err != nil {
		t.Errorf("Admin.CreateUser returned error: %v", err)
	}

	want := &User{ID: Ptr(int64(1)), Login: Ptr("github")}
	if !cmp.Equal(org, want) {
		t.Errorf("Admin.CreateUser returned %+v, want %+v", org, want)
	}

	const methodName = "CreateUser"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Admin.CreateUser(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAdminUsers_Delete(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/admin/users/github", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Admin.DeleteUser(ctx, "github")
	if err != nil {
		t.Errorf("Admin.DeleteUser returned error: %v", err)
	}

	const methodName = "DeleteUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Admin.DeleteUser(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Admin.DeleteUser(ctx, "github")
	})
}

func TestUserImpersonation_Create(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &ImpersonateUserOptions{Scopes: []string{"repo"}}

	mux.HandleFunc("/admin/users/github/authorizations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, opt)
		fmt.Fprint(w, `{"id": 1234,
		"url": "https://example.com/authorizations",
		"app": {
		  "name": "GitHub Site Administrator",
		  "url": "https://docs.github.com/en/rest/enterprise/users/",
		  "client_id": "1234"
		},
		"token": "1234",
		"hashed_token": "1234",
		"token_last_eight": "1234",
		"note": null,
		"note_url": null,
		"created_at": `+refTimeStr(1136178000)+`,
		"updated_at": `+refTimeStr(1136178001)+`,
		"scopes": [
		  "repo"
		],
		"fingerprint": null}`)
	})

	ctx := t.Context()
	auth, _, err := client.Admin.CreateUserImpersonation(ctx, "github", opt)
	if err != nil {
		t.Errorf("Admin.CreateUserImpersonation returned error: %v", err)
	}

	want := &UserAuthorization{
		ID:  Ptr(int64(1234)),
		URL: Ptr("https://example.com/authorizations"),
		App: &OAuthAPP{
			Name:     Ptr("GitHub Site Administrator"),
			URL:      Ptr("https://docs.github.com/en/rest/enterprise/users/"),
			ClientID: Ptr("1234"),
		},
		Token:          Ptr("1234"),
		HashedToken:    Ptr("1234"),
		TokenLastEight: Ptr("1234"),
		Note:           nil,
		NoteURL:        nil,
		CreatedAt:      refTimestamp(1136178000),
		UpdatedAt:      refTimestamp(1136178001),
		Scopes:         []string{"repo"},
		Fingerprint:    nil,
	}
	if !cmp.Equal(auth, want) {
		t.Errorf("Admin.CreateUserImpersonation returned %+v, want %+v", auth, want)
	}

	const methodName = "CreateUserImpersonation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Admin.CreateUserImpersonation(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Admin.CreateUserImpersonation(ctx, "github", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUserImpersonation_Delete(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/admin/users/github/authorizations", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Admin.DeleteUserImpersonation(ctx, "github")
	if err != nil {
		t.Errorf("Admin.DeleteUserImpersonation returned error: %v", err)
	}

	const methodName = "DeleteUserImpersonation"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Admin.DeleteUserImpersonation(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Admin.DeleteUserImpersonation(ctx, "github")
	})
}
