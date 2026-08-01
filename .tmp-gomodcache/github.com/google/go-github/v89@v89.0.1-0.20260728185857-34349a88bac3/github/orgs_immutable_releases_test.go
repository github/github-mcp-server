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

func TestOrganizationsService_GetImmutableReleasesSettings(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/settings/immutable-releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"enforced_repositories": "selected",
			"selected_repositories_url": "https://api.github.com/orgs/o/r"
		}`)
	})

	ctx := t.Context()
	settings, _, err := client.Organizations.GetImmutableReleasesSettings(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.GetImmutableReleasesSettings returned error: %v", err)
	}

	want := &ImmutableReleaseSettings{
		EnforcedRepositories:    Ptr("selected"),
		SelectedRepositoriesURL: Ptr("https://api.github.com/orgs/o/r"),
	}

	if !cmp.Equal(settings, want) {
		t.Errorf("Organizations.GetImmutableReleasesSettings returned %+v, want %+v", settings, want)
	}

	const methodName = "GetImmutableReleasesSettings"

	testBadOptions(t, methodName, func() error {
		_, _, err := client.Organizations.GetImmutableReleasesSettings(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetImmutableReleasesSettings(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_UpdateImmutableReleasesSettings(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ImmutableReleasePolicy{
		EnforcedRepositories: Ptr("selected"),
	}

	mux.HandleFunc("/orgs/o/settings/immutable-releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
		fmt.Fprint(w, `{"enforced_repositories":"selected"}`)
	})

	ctx := t.Context()
	resp, err := client.Organizations.UpdateImmutableReleasesSettings(ctx, "o", input)
	if err != nil {
		t.Errorf("Organizations.UpdateImmutableReleasesSettings returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204 No Content, got %v", resp.StatusCode)
	}

	const methodName = "UpdateImmutableReleasesSettings"

	testBadOptions(t, methodName, func() error {
		_, err := client.Organizations.UpdateImmutableReleasesSettings(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		resp, err := client.Organizations.UpdateImmutableReleasesSettings(ctx, "o", input)
		return resp, err
	})
}

func TestOrganizationsService_ListImmutableReleaseRepositories(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	responseBody := `{
		"total_count": 2,
		"repositories": [
			{"id": 1, "name": "repo1"},
			{"id": 2, "name": "repo2"}
		]
	}`

	mux.HandleFunc("/orgs/o/settings/immutable-releases/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, responseBody)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 10}
	repos, _, err := client.Organizations.ListImmutableReleaseRepositories(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListImmutableReleaseRepositories returned error: %v", err)
	}

	want := &ListRepositories{
		TotalCount: Ptr(2),
		Repositories: []*Repository{
			{ID: Ptr(int64(1)), Name: Ptr("repo1")},
			{ID: Ptr(int64(2)), Name: Ptr("repo2")},
		},
	}

	if !cmp.Equal(repos, want) {
		t.Errorf("Organizations.ListImmutableReleaseRepositories returned %+v, want %+v", repos, want)
	}

	const methodName = "ListImmutableReleaseRepositories"

	testBadOptions(t, methodName, func() error {
		_, _, err := client.Organizations.ListImmutableReleaseRepositories(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListImmutableReleaseRepositories(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_SetImmutableReleaseRepositories(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []int64{1, 2, 3}
	mux.HandleFunc("/orgs/o/settings/immutable-releases/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		want := setImmutableReleasesRepositoriesOptions{SelectedRepositoryIDs: input}
		testJSONBody(t, r, want)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Organizations.SetImmutableReleaseRepositories(ctx, "o", input)
	if err != nil {
		t.Fatalf("SetImmutableReleaseRepositories returned error: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204 No Content, got %v", resp.StatusCode)
	}

	const methodName = "SetImmutableReleaseRepositories"

	testBadOptions(t, methodName, func() error {
		_, err := client.Organizations.SetImmutableReleaseRepositories(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.SetImmutableReleaseRepositories(ctx, "o", input)
	})
}

func TestOrganizationsService_EnableRepositoryForImmutableRelease(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	repoID := int64(42)

	mux.HandleFunc(fmt.Sprintf("/orgs/o/settings/immutable-releases/repositories/%v", repoID), func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Organizations.EnableRepositoryForImmutableRelease(ctx, "o", repoID)
	if err != nil {
		t.Errorf("EnableRepositoryForImmutableRelease returned error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204 No Content, got %v", resp.StatusCode)
	}

	const methodName = "EnableRepositoryForImmutableRelease"

	testBadOptions(t, methodName, func() error {
		_, err := client.Organizations.EnableRepositoryForImmutableRelease(ctx, "o", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.EnableRepositoryForImmutableRelease(ctx, "o", repoID)
	})
}

func TestOrganizationsService_DisableRepositoryForImmutableRelease(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	repoID := int64(42)

	mux.HandleFunc(fmt.Sprintf("/orgs/o/settings/immutable-releases/repositories/%v", repoID), func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Organizations.DisableRepositoryForImmutableRelease(ctx, "o", repoID)
	if err != nil {
		t.Errorf("DisableRepositoryForImmutableRelease returned error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204 No Content, got %v", resp.StatusCode)
	}

	const methodName = "DisableRepositoryForImmutableRelease"

	testBadOptions(t, methodName, func() error {
		_, err := client.Organizations.DisableRepositoryForImmutableRelease(ctx, "o", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.DisableRepositoryForImmutableRelease(ctx, "o", repoID)
	})
}
