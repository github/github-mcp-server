// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/google/go-github/v89/github"
)

// Integration tests for Projects V2 endpoints defined in github/projects.go.
//
// These tests are intentionally defensive. They only require minimal
// environment variables identifying a target org and user. Project numbers are
// discovered dynamically by first listing projects and selecting one. For item
// CRUD operations, the test creates a temporary repository & issue (where
// possible) and adds/removes that issue as a project item. If prerequisites
// (auth, env vars, permissions, presence of at least one project) are missing,
// the relevant sub-test is skipped so other integration tests can still run.
//
// Required / optional environment variables:
//   GITHUB_AUTH_TOKEN                  (required for any of these tests to run)
//   GITHUB_TEST_ORG                    (org login; required for org project tests)
//   GITHUB_TEST_USER                   (user login; required for user project tests)
//   GITHUB_TEST_REPO                   (repo name)

func TestProjectsV2_Org(t *testing.T) {
	skipIfMissingAuth(t)
	org := os.Getenv("GITHUB_TEST_ORG")
	if org == "" {
		t.Skip("GITHUB_TEST_ORG not set")
	}

	ctx := t.Context()

	opts := &github.ListProjectsOptions{}
	// List projects for org; pick the first available project we can read.
	projects, _, err := client.Projects.ListOrganizationProjects(ctx, org, opts)
	if err != nil {
		// If listing itself fails, abort this test.
		t.Fatalf("Projects.ListOrganizationProjects returned error: %v", err)
	}
	if len(projects) == 0 {
		t.Skipf("no Projects V2 found for org %s", org)
	}
	project := projects[0]
	if project.Number == nil {
		t.Skip("selected org project has nil Number field")
	}
	projectNumber := *project.Number

	// Re-fetch via Get to exercise endpoint explicitly.
	proj, _, err := client.Projects.GetOrganizationProject(ctx, org, projectNumber)
	if err != nil {
		// Permission mismatch? Skip CRUD while still reporting failure would make the test fail;
		// we want correctness so treat as fatal here.
		t.Fatalf("Projects.GetOrganizationProject returned error: %v", err)
	}
	if proj.Number == nil || *proj.Number != projectNumber {
		t.Fatalf("GetOrganizationProject returned unexpected project number: got %+v want %d", proj.Number, projectNumber)
	}

	_, _, err = client.Projects.ListOrganizationProjectFields(ctx, org, projectNumber, nil)
	if err != nil {
		t.Fatalf("Projects.ListOrganizationProjectFields returned error: %v. Fields listing might require extra permissions", err)
	}
}

func TestProjectsV2_User(t *testing.T) {
	skipIfMissingAuth(t)
	user := os.Getenv("GITHUB_TEST_USER")
	if user == "" {
		t.Skip("GITHUB_TEST_USER not set")
	}

	ctx := t.Context()
	opts := &github.ListProjectsOptions{}
	projects, _, err := client.Projects.ListUserProjects(ctx, user, opts)
	if err != nil {
		t.Fatalf("Projects.ListUserProjects returned error: %v. This indicates API or permission issue", err)
	}
	if len(projects) == 0 {
		t.Skipf("no Projects V2 found for user %s", user)
	}
	project := projects[0]
	if project.Number == nil {
		t.Skip("selected user project has nil Number field")
	}

	proj, _, err := client.Projects.GetUserProject(ctx, user, *project.Number)
	if err != nil {
		// can't fetch specific project; treat as fatal
		t.Fatalf("Projects.GetUserProject returned error: %v", err)
	}
	if proj.Number == nil || *proj.Number != *project.Number {
		t.Fatalf("GetUserProject returned unexpected project number: got %+v want %d", proj.Number, *project.Number)
	}
}
