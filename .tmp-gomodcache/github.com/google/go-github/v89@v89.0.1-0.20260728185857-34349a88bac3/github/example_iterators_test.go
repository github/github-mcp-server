// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github_test

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-github/v89/github"
)

func ExampleRepositoriesService_ListByUserIter() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}
	ctx := context.Background()

	// List all repositories for a user using the iterator.
	// This automatically handles pagination.
	// Note that if `opts` is `nil`, a new empty `opts` will be created and used within the iterator.
	opts := &github.RepositoryListByUserOptions{Type: "public"}
	for repo, err := range client.Repositories.ListByUserIter(ctx, "octocat", opts) {
		if err != nil {
			log.Fatalf("Error listing repositories by user: %v", err)
		}
		fmt.Println(repo.GetName())
	}
}
