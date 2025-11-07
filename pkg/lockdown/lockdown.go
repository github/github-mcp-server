package lockdown

import (
	"context"
	"fmt"
	"strings"

	"github.com/shurcooL/githubv4"
)

func ShouldRemoveContent(ctx context.Context, client *githubv4.Client, username, owner, repo string) (bool, error) {
	isPrivate, err := IsPrivateRepo(ctx, client, owner, repo)
	if err != nil {
		return false, err
	}

	// Do not filter content for private repositories
	if isPrivate {
		return false, nil
	}
	hasPushAccess, err := HasPushAccess(ctx, client, username, owner, repo)
	if err != nil {
		return false, err
	}

	return !hasPushAccess, nil
}

func HasPushAccess(ctx context.Context, client *githubv4.Client, username, owner, repo string) (bool, error) {
	if client == nil {
		return false, fmt.Errorf("nil GraphQL client")
	}

	var query struct {
		Repository struct {
			Collaborators struct {
				Edges []struct {
					Permission githubv4.String
					Node       struct {
						Login githubv4.String
					}
				}
			} `graphql:"collaborators(query: $username, first: 1)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner":    githubv4.String(owner),
		"name":     githubv4.String(repo),
		"username": githubv4.String(username),
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return false, fmt.Errorf("failed to query user permissions: %w", err)
	}

	// Check if the user has push access
	hasPush := false
	for _, edge := range query.Repository.Collaborators.Edges {
		login := string(edge.Node.Login)
		if strings.EqualFold(login, username) {
			permission := string(edge.Permission)
			// WRITE, ADMIN, and MAINTAIN permissions have push access
			hasPush = permission == "WRITE" || permission == "ADMIN" || permission == "MAINTAIN"
			break
		}
	}

	return hasPush, nil
}

// IsPrivateRepo checks if a repository is private using GraphQL
func IsPrivateRepo(ctx context.Context, client *githubv4.Client, owner, repo string) (bool, error) {
	if client == nil {
		return false, fmt.Errorf("nil GraphQL client")
	}

	var query struct {
		Repository struct {
			IsPrivate githubv4.Boolean
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"name":  githubv4.String(repo),
	}

	err := client.Query(ctx, &query, variables)
	if err != nil {
		return false, fmt.Errorf("failed to query repository visibility: %w", err)
	}

	return bool(query.Repository.IsPrivate), nil
}
