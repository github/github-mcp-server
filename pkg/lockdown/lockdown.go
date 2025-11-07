package lockdown

import (
	"context"
	"fmt"
	"strings"

	"github.com/shurcooL/githubv4"
)

// ShouldRemoveContent determines if content should be removed based on
// lockdown mode rules. It checks if the repository is private and if the user
// has push access to the repository.
func ShouldRemoveContent(ctx context.Context, client *githubv4.Client, username, owner, repo string) (bool, error) {
	isPrivate, hasPushAccess, err := repoAccessInfo(ctx, client, username, owner, repo)
	if err != nil {
		return false, err
	}

	// Do not filter content for private repositories
	if isPrivate {
		return false, nil
	}

	return !hasPushAccess, nil
}

func repoAccessInfo(ctx context.Context, client *githubv4.Client, username, owner, repo string) (bool, bool, error) {
	if client == nil {
		return false, false, fmt.Errorf("nil GraphQL client")
	}

	var query struct {
		Repository struct {
			IsPrivate     githubv4.Boolean
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
		return false, false, fmt.Errorf("failed to query repository access info: %w", err)
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

	return bool(query.Repository.IsPrivate), hasPush, nil
}
