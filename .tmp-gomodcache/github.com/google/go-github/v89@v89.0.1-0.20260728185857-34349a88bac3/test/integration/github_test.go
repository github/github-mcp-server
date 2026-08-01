// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/google/go-github/v89/github"
)

// client is a github.Client with the default http.Client. It is authorized if auth is true.
// auth indicates whether tests are being run with an OAuth token
// that is defined in the GITHUB_AUTH_TOKEN environment variable.
var client, auth = sync.OnceValues(func() (*github.Client, bool) {
	token := os.Getenv("GITHUB_AUTH_TOKEN")
	if token == "" {
		c, err := github.NewClient()
		if err != nil {
			log.Fatalf("Error creating GitHub client: %v", err)
		}
		return c, false
	}

	c, err := github.NewClient(github.WithAuthToken(token))
	if err != nil {
		log.Fatalf("Error creating GitHub client with token: %v", err)
	}
	return c, true
})()

func skipIfMissingAuth(t *testing.T) {
	if !auth {
		t.Skipf("No OAuth token - skipping portions of %v\n", t.Name())
	}
}

func createRandomTestRepository(t *testing.T, owner string, autoinit bool) *github.Repository {
	t.Helper()

	// determine the owner to use if one wasn't specified
	if owner == "" {
		owner = os.Getenv("GITHUB_OWNER")
		if owner == "" {
			me, _, err := client.Users.Get(t.Context(), "")
			if err != nil {
				t.Fatalf("Users.Get returned error: %v", err)
			}
			owner = *me.Login
		}
	}

	// create random repo name that does not currently exist
	var repoName string
	for {
		repoName = fmt.Sprintf("test-%v", rand.Int())
		_, resp, err := client.Repositories.Get(t.Context(), owner, repoName)
		if err != nil {
			if resp.StatusCode == http.StatusNotFound {
				// found a nonexistent repo, perfect
				break
			}

			t.Fatalf("Repositories.Get returned error: %v", err)
		}
	}

	// create the repository
	repo, _, err := client.Repositories.Create(
		t.Context(),
		owner,
		&github.Repository{
			Name:     &repoName,
			AutoInit: &autoinit,
		},
	)
	if err != nil {
		t.Fatalf("Repositories.Create returned error: %v", err)
	}

	return repo
}
