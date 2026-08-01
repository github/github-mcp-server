// Copyright 2018 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMigrationService_StartUserMigration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/migrations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeMigrationsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, userMigrationJSON)
	})

	opt := &UserMigrationOptions{
		LockRepositories:   true,
		ExcludeAttachments: false,
	}

	ctx := t.Context()
	got, _, err := client.Migrations.StartUserMigration(ctx, []string{"r"}, opt)
	if err != nil {
		t.Errorf("StartUserMigration returned error: %v", err)
	}

	want := wantUserMigration
	if !cmp.Equal(want, got) {
		t.Errorf("StartUserMigration = %v, want = %v", got, want)
	}

	const methodName = "StartUserMigration"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Migrations.StartUserMigration(ctx, []string{"r"}, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestMigrationService_ListUserMigrations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/migrations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeMigrationsPreview)

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, fmt.Appendf(nil, "[%s]", userMigrationJSON))
	})

	ctx := t.Context()
	got, _, err := client.Migrations.ListUserMigrations(ctx, &ListOptions{Page: 1, PerPage: 2})
	if err != nil {
		t.Errorf("ListUserMigrations returned error %v", err)
	}

	want := []*UserMigration{wantUserMigration}
	if !cmp.Equal(want, got) {
		t.Errorf("ListUserMigrations = %v, want = %v", got, want)
	}

	const methodName = "ListUserMigrations"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Migrations.ListUserMigrations(ctx, &ListOptions{Page: 1, PerPage: 2})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestMigrationService_UserMigrationStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/migrations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeMigrationsPreview)

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, userMigrationJSON)
	})

	ctx := t.Context()
	got, _, err := client.Migrations.UserMigrationStatus(ctx, 1)
	if err != nil {
		t.Errorf("UserMigrationStatus returned error %v", err)
	}

	want := wantUserMigration
	if !cmp.Equal(want, got) {
		t.Errorf("UserMigrationStatus = %v, want = %v", got, want)
	}

	const methodName = "UserMigrationStatus"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Migrations.UserMigrationStatus(ctx, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestMigrationService_UserMigrationArchiveURL(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/migrations/1/archive", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeMigrationsPreview)

		http.Redirect(w, r, "/go-github", http.StatusFound)
	})

	mux.HandleFunc("/go-github", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")

		w.WriteHeader(http.StatusOK)
	})

	ctx := t.Context()
	got, err := client.Migrations.UserMigrationArchiveURL(ctx, 1)
	if err != nil {
		t.Errorf("UserMigrationArchiveURL returned error %v", err)
	}

	want := "/go-github"
	if !strings.HasSuffix(got, want) {
		t.Errorf("UserMigrationArchiveURL = %v, want = %v", got, want)
	}
}

func TestMigrationService_DeleteUserMigration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/migrations/1/archive", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeMigrationsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	got, err := client.Migrations.DeleteUserMigration(ctx, 1)
	if err != nil {
		t.Errorf("DeleteUserMigration returned error %v", err)
	}

	if got.StatusCode != http.StatusNoContent {
		t.Errorf("DeleteUserMigration returned status = %v, want = %v", got.StatusCode, http.StatusNoContent)
	}

	const methodName = "DeleteUserMigration"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Migrations.DeleteUserMigration(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Migrations.DeleteUserMigration(ctx, 1)
	})
}

func TestMigrationService_UnlockUserRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/migrations/1/repos/r/lock", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeMigrationsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	got, err := client.Migrations.UnlockUserRepo(ctx, 1, "r")
	if err != nil {
		t.Errorf("UnlockUserRepo returned error %v", err)
	}

	if got.StatusCode != http.StatusNoContent {
		t.Errorf("UnlockUserRepo returned status = %v, want = %v", got.StatusCode, http.StatusNoContent)
	}

	const methodName = "UnlockUserRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Migrations.UnlockUserRepo(ctx, -1, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Migrations.UnlockUserRepo(ctx, 1, "r")
	})
}

var userMigrationJSON = []byte(`{
  "id": 79,
  "guid": "0b989ba4-242f-11e5-81e1-c7b6966d2516",
  "state": "pending",
  "lock_repositories": true,
  "exclude_attachments": false,
  "url": "https://api.github.com/orgs/octo-org/migrations/79",
  "created_at": "2015-07-06T15:33:38-07:00",
  "updated_at": "2015-07-06T15:33:38-07:00",
  "repositories": [
    {
      "id": 1296269,
      "name": "Hello-World",
      "full_name": "octocat/Hello-World",
      "description": "This your first repo!"
    }
  ]
}`)

var wantUserMigration = &UserMigration{
	ID:                 Ptr(int64(79)),
	GUID:               Ptr("0b989ba4-242f-11e5-81e1-c7b6966d2516"),
	State:              Ptr("pending"),
	LockRepositories:   Ptr(true),
	ExcludeAttachments: Ptr(false),
	URL:                Ptr("https://api.github.com/orgs/octo-org/migrations/79"),
	CreatedAt:          Ptr("2015-07-06T15:33:38-07:00"),
	UpdatedAt:          Ptr("2015-07-06T15:33:38-07:00"),
	Repositories: []*Repository{
		{
			ID:          Ptr(int64(1296269)),
			Name:        Ptr("Hello-World"),
			FullName:    Ptr("octocat/Hello-World"),
			Description: Ptr("This your first repo!"),
		},
	},
}
