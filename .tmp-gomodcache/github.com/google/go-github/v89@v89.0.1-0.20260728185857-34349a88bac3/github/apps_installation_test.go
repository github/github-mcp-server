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

func TestAppsService_ListRepos(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `{"total_count": 1,"repositories": [{"id": 1}]}`)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	repositories, _, err := client.Apps.ListRepos(ctx, opt)
	if err != nil {
		t.Errorf("Apps.ListRepos returned error: %v", err)
	}

	want := &ListRepositories{TotalCount: Ptr(1), Repositories: []*Repository{{ID: Ptr(int64(1))}}}
	if !cmp.Equal(repositories, want) {
		t.Errorf("Apps.ListRepos returned %+v, want %+v", repositories, want)
	}

	const methodName = "ListRepos"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.ListRepos(ctx, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_ListUserRepos(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/installations/1/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `{"total_count":1,"repositories": [{"id":1}]}`)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	repositories, _, err := client.Apps.ListUserRepos(ctx, 1, opt)
	if err != nil {
		t.Errorf("Apps.ListUserRepos returned error: %v", err)
	}

	want := &ListRepositories{TotalCount: Ptr(1), Repositories: []*Repository{{ID: Ptr(int64(1))}}}
	if !cmp.Equal(repositories, want) {
		t.Errorf("Apps.ListUserRepos returned %+v, want %+v", repositories, want)
	}

	const methodName = "ListUserRepos"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.ListUserRepos(ctx, -1, &ListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.ListUserRepos(ctx, 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_AddRepository(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/installations/1/repositories/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{"id":1,"name":"n","description":"d","owner":{"login":"l"},"license":{"key":"mit"}}`)
	})

	ctx := t.Context()
	repo, _, err := client.Apps.AddRepository(ctx, 1, 1)
	if err != nil {
		t.Errorf("Apps.AddRepository returned error: %v", err)
	}

	want := &Repository{ID: Ptr(int64(1)), Name: Ptr("n"), Description: Ptr("d"), Owner: &User{Login: Ptr("l")}, License: &License{Key: Ptr("mit")}}
	if !cmp.Equal(repo, want) {
		t.Errorf("AddRepository returned %+v, want %+v", repo, want)
	}

	const methodName = "AddRepository"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.AddRepository(ctx, 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_RemoveRepository(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/installations/1/repositories/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Apps.RemoveRepository(ctx, 1, 1)
	if err != nil {
		t.Errorf("Apps.RemoveRepository returned error: %v", err)
	}

	const methodName = "RemoveRepository"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Apps.RemoveRepository(ctx, 1, 1)
	})
}

func TestAppsService_RevokeInstallationToken(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/installation/token", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Apps.RevokeInstallationToken(ctx)
	if err != nil {
		t.Errorf("Apps.RevokeInstallationToken returned error: %v", err)
	}

	const methodName = "RevokeInstallationToken"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Apps.RevokeInstallationToken(ctx)
	})
}
