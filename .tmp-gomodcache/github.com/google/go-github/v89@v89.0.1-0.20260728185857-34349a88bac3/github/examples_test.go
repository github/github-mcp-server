// Copyright 2016 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// These examples are inlined in godoc.

package github_test

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-github/v89/github"
)

func ExampleMarkdownService_Render() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	input := "# heading #\n\nLink to issue #1"
	opt := &github.MarkdownOptions{Mode: "gfm", Context: "google/go-github"}

	ctx := context.Background()
	output, _, err := client.Markdown.Render(ctx, input, opt)
	if err != nil {
		log.Fatalf("Error rendering markdown: %v", err)
	}

	fmt.Println(output)
}

func ExampleRepositoriesService_GetReadme() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	ctx := context.Background()
	readme, _, err := client.Repositories.GetReadme(ctx, "google", "go-github", nil)
	if err != nil {
		log.Fatalf("Error getting README: %v", err)
	}

	content, err := readme.GetContent()
	if err != nil {
		log.Fatalf("Error getting README content: %v", err)
	}

	fmt.Printf("google/go-github README:\n%v\n", content)
}

func ExampleRepositoriesService_ListByUser() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	user := "willnorris"
	opt := &github.RepositoryListByUserOptions{Type: "owner", Sort: "updated", Direction: "desc"}

	ctx := context.Background()
	repos, _, err := client.Repositories.ListByUser(ctx, user, opt)
	if err != nil {
		log.Fatalf("error listing repositories by user: %v", err)
	}

	fmt.Printf("Recently updated repositories by %q: %v", user, github.Stringify(repos))
}

func ExampleRepositoriesService_CreateFile() {
	// In this example we're creating a new file in a repository using the
	// Contents API. Only 1 file per commit can be managed through that API.

	// Note that authentication is needed here as you are performing a modification
	// so you will need to modify the example to provide authentication to
	// github.NewClient(). See the following documentation for more information
	// on how to authenticate with the client:
	// https://pkg.go.dev/github.com/google/go-github/v89/github#hdr-Authentication
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	ctx := context.Background()
	fileContent := []byte("This is the content of my file\nand the 2nd line of it")

	// Note: the file needs to be absent from the repository as you are not
	// specifying a SHA reference here.
	opts := &github.RepositoryContentFileOptions{
		Message:   github.Ptr("This is my commit message"),
		Content:   fileContent,
		Branch:    github.Ptr("master"),
		Committer: &github.CommitAuthor{Name: github.Ptr("FirstName LastName"), Email: github.Ptr("user@example.com")},
	}
	if _, _, err := client.Repositories.CreateFile(ctx, "myOrganization", "myRepository", "myNewFile.md", opts); err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
}

func ExampleUsersService_ListAll() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	ctx := context.Background()
	opts := &github.UserListOptions{}
	for {
		users, _, err := client.Users.ListAll(ctx, opts)
		if err != nil {
			log.Fatalf("Error listing users: %v", err)
		}
		if len(users) == 0 {
			break
		}
		opts.Since = users[len(users)-1].GetID()
		// Process users...
	}
}

func ExamplePullRequestsService_Create() {
	// In this example we're creating a PR and displaying the HTML url at the end.

	// Note that authentication is needed here as you are performing a modification
	// so you will need to modify the example to provide authentication to
	// github.NewClient(). See the following documentation for more information
	// on how to authenticate with the client:
	// https://pkg.go.dev/github.com/google/go-github/v89/github#hdr-Authentication
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	newPR := github.CreatePullRequest{
		Title:               github.Ptr("My awesome pull request"),
		Head:                "branch_to_merge",
		Base:                "master",
		Body:                github.Ptr("This is the description of the PR created with the package `github.com/google/go-github/github`"),
		MaintainerCanModify: github.Ptr(true),
	}

	ctx := context.Background()
	pr, _, err := client.PullRequests.Create(ctx, "myOrganization", "myRepository", newPR)
	if err != nil {
		log.Fatalf("Error creating pull request: %v", err)
	}

	fmt.Printf("PR created: %v\n", pr.GetHTMLURL())
}

func ExampleTeamsService_ListTeams() {
	// This example shows how to get a team ID corresponding to a given team name.

	// Note that authentication is needed here as you are performing a lookup on
	// an organization's administrative configuration, so you will need to modify
	// the example to to provide authentication to github.NewClient(). See the
	//  following documentation for more information on how to authenticate with
	// the client:
	// https://pkg.go.dev/github.com/google/go-github/v89/github#hdr-Authentication
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	teamName := "Developers team"
	ctx := context.Background()
	opts := &github.ListOptions{}

	for {
		teams, resp, err := client.Teams.ListTeams(ctx, "myOrganization", opts)
		if err != nil {
			log.Fatalf("error listing teams: %v", err)
		}
		for _, t := range teams {
			if t.GetName() == teamName {
				fmt.Printf("Team %q has ID %v\n", teamName, t.GetID())
				return
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	fmt.Printf("Team %q was not found\n", teamName)
}

func ExampleUsersService_ListUserSocialAccounts() {
	client, err := github.NewClient()
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}
	ctx := context.Background()
	opts := &github.ListOptions{}
	for {
		accounts, resp, err := client.Users.ListUserSocialAccounts(ctx, "shreyjain13", opts)
		if err != nil {
			log.Fatalf("Error listing user social accounts: %v", err)
		}
		if resp.NextPage == 0 || len(accounts) == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
}
