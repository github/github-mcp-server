// Copyright 2026 The go-github AUTHORS. All rights reserved.
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

func TestRepositoriesService_EnableImmutableReleases(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/owner/repo/immutable-releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Repositories.EnableImmutableReleases(ctx, "owner", "repo")
	if err != nil {
		t.Errorf("Repositories.EnableImmutableReleases returned error: %v", err)
	}

	const methodName = "EnableImmutableReleases"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.EnableImmutableReleases(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.EnableImmutableReleases(ctx, "owner", "repo")
	})
}

func TestRepositoriesService_DisableImmutableReleases(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/owner/repo/immutable-releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Repositories.DisableImmutableReleases(ctx, "owner", "repo")
	if err != nil {
		t.Errorf("Repositories.DisableImmutableReleases returned error: %v", err)
	}

	const methodName = "DisableImmutableReleases"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DisableImmutableReleases(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DisableImmutableReleases(ctx, "owner", "repo")
	})
}

func TestRepositoriesService_AreImmutableReleasesEnabled(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/owner/repo/immutable-releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"enabled": true, "enforced_by_owner": false}`)
	})

	ctx := t.Context()
	status, _, err := client.Repositories.AreImmutableReleasesEnabled(ctx, "owner", "repo")
	if err != nil {
		t.Errorf("Repositories.AreImmutableReleasesEnabled returned error: %v", err)
	}
	want := &RepoImmutableReleasesStatus{Enabled: Ptr(true), EnforcedByOwner: Ptr(false)}
	if !cmp.Equal(status, want) {
		t.Errorf("Repositories.AreImmutableReleasesEnabled returned %+v, want %+v", status, want)
	}

	const methodName = "AreImmutableReleasesEnabled"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.AreImmutableReleasesEnabled(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.AreImmutableReleasesEnabled(ctx, "owner", "repo")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
