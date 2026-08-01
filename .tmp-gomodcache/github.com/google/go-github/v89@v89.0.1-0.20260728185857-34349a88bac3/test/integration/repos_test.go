// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v89/github"
)

func TestRepositories_CRUD(t *testing.T) {
	skipIfMissingAuth(t)

	repo := createRandomTestRepository(t, "", true)

	// update the repository description
	repo.Description = github.Ptr("description")
	repo.DefaultBranch = nil // FIXME: this shouldn't be necessary
	_, _, err := client.Repositories.Edit(t.Context(), *repo.Owner.Login, *repo.Name, repo)
	if err != nil {
		t.Fatalf("Repositories.Edit() returned error: %v", err)
	}

	// delete the repository
	_, err = client.Repositories.Delete(t.Context(), *repo.Owner.Login, *repo.Name)
	if err != nil {
		t.Fatalf("Repositories.Delete() returned error: %v", err)
	}

	// verify that the repository was deleted
	_, resp, err := client.Repositories.Get(t.Context(), *repo.Owner.Login, *repo.Name)
	if err == nil {
		t.Fatal("Test repository still exists after deleting it.")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Repositories.Get() returned error: %v", err)
	}
}

func TestRepositories_BranchesTags(t *testing.T) {
	// branches
	branches, _, err := client.Repositories.ListBranches(t.Context(), "git", "git", nil)
	if err != nil {
		t.Fatalf("Repositories.ListBranches() returned error: %v", err)
	}

	if len(branches) == 0 {
		t.Fatal("Repositories.ListBranches('git', 'git') returned no branches")
	}

	_, _, err = client.Repositories.GetBranch(t.Context(), "git", "git", *branches[0].Name, 0)
	if err != nil {
		t.Fatalf("Repositories.GetBranch() returned error: %v", err)
	}

	// tags
	tags, _, err := client.Repositories.ListTags(t.Context(), "git", "git", nil)
	if err != nil {
		t.Fatalf("Repositories.ListTags() returned error: %v", err)
	}

	if len(tags) == 0 {
		t.Fatal("Repositories.ListTags('git', 'git') returned no tags")
	}
}

func TestRepositories_EditBranches(t *testing.T) {
	skipIfMissingAuth(t)

	repo := createRandomTestRepository(t, "", true)

	branch, _, err := client.Repositories.GetBranch(t.Context(), *repo.Owner.Login, *repo.Name, "master", 0)
	if err != nil {
		t.Fatalf("Repositories.GetBranch() returned error: %v", err)
	}

	if *branch.Protected {
		t.Fatalf("Branch %v of repo %v is already protected", "master", *repo.Name)
	}

	protectionRequest := &github.ProtectionRequest{
		RequiredStatusChecks: &github.RequiredStatusChecks{
			Strict:   true,
			Contexts: &[]string{"continuous-integration"},
		},
		RequiredPullRequestReviews: &github.PullRequestReviewsEnforcementRequest{
			DismissStaleReviews: true,
		},
		EnforceAdmins: true,
		// TODO: Only organization repositories can have users and team restrictions.
		//       In order to be able to test these Restrictions, need to add support
		//       for creating temporary organization repositories.
		Restrictions:     nil,
		BlockCreations:   github.Ptr(false),
		LockBranch:       github.Ptr(false),
		AllowForkSyncing: github.Ptr(false),
	}

	protection, _, err := client.Repositories.UpdateBranchProtection(t.Context(), *repo.Owner.Login, *repo.Name, "master", protectionRequest)
	if err != nil {
		t.Fatalf("Repositories.UpdateBranchProtection() returned error: %v", err)
	}

	want := &github.Protection{
		RequiredStatusChecks: &github.RequiredStatusChecks{
			Strict:   true,
			Contexts: &[]string{"continuous-integration"},
		},
		RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
			DismissStaleReviews:          true,
			RequiredApprovingReviewCount: 0,
		},
		EnforceAdmins: &github.AdminEnforcement{
			URL:     github.Ptr("https://api.github.com/repos/" + *repo.Owner.Login + "/" + *repo.Name + "/branches/master/protection/enforce_admins"),
			Enabled: true,
		},
		Restrictions: nil,
		BlockCreations: &github.BlockCreations{
			Enabled: github.Ptr(false),
		},
		LockBranch: &github.LockBranch{
			Enabled: github.Ptr(false),
		},
		AllowForkSyncing: &github.AllowForkSyncing{
			Enabled: github.Ptr(false),
		},
	}
	if !cmp.Equal(protection, want) {
		t.Errorf("Repositories.UpdateBranchProtection() returned %+v, want %+v", protection, want)
	}

	_, err = client.Repositories.Delete(t.Context(), *repo.Owner.Login, *repo.Name)
	if err != nil {
		t.Fatalf("Repositories.Delete() returned error: %v", err)
	}
}

func TestRepositories_ListByAuthenticatedUser(t *testing.T) {
	skipIfMissingAuth(t)

	_, _, err := client.Repositories.ListByAuthenticatedUser(t.Context(), nil)
	if err != nil {
		t.Fatalf("Repositories.ListByAuthenticatedUser() returned error: %v", err)
	}
}

func TestRepositories_ListByUser(t *testing.T) {
	_, _, err := client.Repositories.ListByUser(t.Context(), "google", nil)
	if err != nil {
		t.Fatalf("Repositories.ListByUser('google') returned error: %v", err)
	}

	opt := github.RepositoryListByUserOptions{Sort: "created"}
	repos, _, err := client.Repositories.ListByUser(t.Context(), "google", &opt)
	if err != nil {
		t.Fatalf("Repositories.List('google') with Sort opt returned error: %v", err)
	}
	for i, repo := range repos {
		if i > 0 && (*repos[i-1].CreatedAt).Time.Before((*repo.CreatedAt).Time) {
			t.Fatal("Repositories.ListByUser('google') with default descending Sort returned incorrect order")
		}
	}
}

func TestRepositories_DownloadReleaseAsset(t *testing.T) {
	skipIfMissingAuth(t)

	rc, _, err := client.Repositories.DownloadReleaseAsset(t.Context(), "andersjanmyr", "goose", 484892, http.DefaultClient)
	if err != nil {
		t.Fatalf("Repositories.DownloadReleaseAsset(andersjanmyr, goose, 484892, true) returned error: %v", err)
	}
	defer func() { _ = rc.Close() }()
	_, err = io.Copy(io.Discard, rc)
	if err != nil {
		t.Fatalf("Repositories.DownloadReleaseAsset(andersjanmyr, goose, 484892, true) returned error: %v", err)
	}
}

func TestRepositories_Autolinks(t *testing.T) {
	skipIfMissingAuth(t)

	repo := createRandomTestRepository(t, "", true)

	body := github.CreateAutolinkRequest{
		KeyPrefix:      "TICKET-",
		URLTemplate:    "https://example.com/TICKET?query=<num>",
		IsAlphanumeric: github.Ptr(false),
	}

	actionlink, _, err := client.Repositories.CreateAutolink(t.Context(), *repo.Owner.Login, *repo.Name, body)
	if err != nil {
		t.Fatalf("Repositories.CreateAutolink() returned error: %v", err)
	}

	if !cmp.Equal(actionlink.GetKeyPrefix(), body.KeyPrefix) ||
		!cmp.Equal(actionlink.GetURLTemplate(), body.URLTemplate) ||
		!cmp.Equal(actionlink.IsAlphanumeric, body.IsAlphanumeric) {
		t.Errorf("Repositories.CreateAutolink() returned %+v, want %+v", actionlink, body)
	}

	_, err = client.Repositories.Delete(t.Context(), *repo.Owner.Login, *repo.Name)
	if err != nil {
		t.Fatalf("Repositories.Delete() returned error: %v", err)
	}
}
